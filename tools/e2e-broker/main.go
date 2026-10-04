// Command e2e-broker はフルスタック e2e (frontend/playwright.integration.config.ts) 用の
// MQTT ブローカーを起動する。internal/integration の Go 統合テストと同じ mochi-mqtt を使う。
//
// テストがアプリ以外のクライアントとして publish したり、ブローカー側の障害を起こしたりできるよう、
// 操作用の HTTP も開く。
//
//	GET    /healthz                                    起動の確認 (Playwright の webServer.url)
//	POST   /publish?topic=<t>&qos=<0-2>&retain=<bool>  本文をペイロードとして publish する
//	POST   /disconnect-clients                         接続中のクライアントをすべて切る
//	POST   /deny?filter=<f>                            以後、そのフィルターへの購読を拒否する
//	DELETE /deny                                       拒否するフィルターをすべて消す
//
// どれもループバックアドレスで待ち受ける前提で、認証は無い。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// maxPayloadBytes は /publish が受け付けるペイロードの上限。
const maxPayloadBytes = 1 << 20

// errDisconnectedByControl は /disconnect-clients で切ったクライアントの切断理由。
var errDisconnectedByControl = errors.New("disconnected by e2e control")

// denyHook は接続と publish をすべて許可し、登録されたフィルターへの購読だけを ACL で拒否する。
// 判定はブローカーの接続ごとの goroutine から、登録と解除は操作用 HTTP から呼ばれる。
type denyHook struct {
	mqtt.HookBase
	mu      sync.RWMutex
	filters map[string]struct{}
}

func newDenyHook() *denyHook {
	return &denyHook{filters: make(map[string]struct{})}
}

func (h *denyHook) ID() string { return "e2e-deny" }

func (h *denyHook) Provides(b byte) bool {
	return b == mqtt.OnConnectAuthenticate || b == mqtt.OnACLCheck
}

func (h *denyHook) OnConnectAuthenticate(*mqtt.Client, packets.Packet) bool { return true }

// OnACLCheck は購読 (write が偽) のフィルターが登録済みなら拒否する。
func (h *denyHook) OnACLCheck(_ *mqtt.Client, topic string, write bool) bool {
	if write {
		return true
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, denied := h.filters[topic]
	return !denied
}

// deny は filter への購読を拒否するようにする。
func (h *denyHook) deny(filter string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.filters[filter] = struct{}{}
}

// allowAll は拒否するフィルターをすべて消す。
func (h *denyHook) allowAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	clear(h.filters)
}

func main() {
	mqttAddr := flag.String("mqtt", "127.0.0.1:18830", "MQTT (TCP) の待ち受けアドレス")
	httpAddr := flag.String("http", "127.0.0.1:18831", "操作用 HTTP の待ち受けアドレス")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, *mqttAddr, *httpAddr)
	stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e-broker: %v\n", err)
		os.Exit(1)
	}
}

// run はブローカーと操作用 HTTP を起動し、ctx が終わるまで待つ。
func run(ctx context.Context, mqttAddr, httpAddr string) error {
	// 接続ごとの Info ログは e2e の出力を埋めるので、警告以上だけを出す。
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	server := mqtt.New(&mqtt.Options{InlineClient: true, Logger: logger})
	hook := newDenyHook()
	if err := server.AddHook(hook, nil); err != nil {
		return fmt.Errorf("add hook: %w", err)
	}
	if err := server.AddListener(listeners.NewTCP(listeners.Config{ID: "e2e", Address: mqttAddr})); err != nil {
		return fmt.Errorf("listen mqtt %s: %w", mqttAddr, err)
	}
	if err := server.Serve(); err != nil {
		return fmt.Errorf("serve mqtt: %w", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			logger.Warn("close mqtt server", "error", err)
		}
	}()

	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           newControlHandler(server, hook),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()

	select {
	case err := <-errCh:
		return fmt.Errorf("serve http %s: %w", httpAddr, err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown http: %w", err)
	}
	return nil
}

// newControlHandler は操作用 HTTP のハンドラを返す。
func newControlHandler(server *mqtt.Server, hook *denyHook) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /publish", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		topic := q.Get("topic")
		if topic == "" {
			http.Error(w, "topic is required", http.StatusBadRequest)
			return
		}
		var qos byte
		switch q.Get("qos") {
		case "", "0":
		case "1":
			qos = 1
		case "2":
			qos = 2
		default:
			http.Error(w, "qos must be 0, 1 or 2", http.StatusBadRequest)
			return
		}
		retain := q.Get("retain") == "true"
		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPayloadBytes))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := server.Publish(topic, payload, retain, qos); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// ブローカーが落ちたときのように、クライアントから見て接続が切れた状態を作る。
	// publish に使うインラインクライアントは残す。
	mux.HandleFunc("POST /disconnect-clients", func(w http.ResponseWriter, _ *http.Request) {
		for _, client := range server.Clients.GetAll() {
			if client.Net.Inline {
				continue
			}
			client.Stop(errDisconnectedByControl)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /deny", func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("filter")
		if filter == "" {
			http.Error(w, "filter is required", http.StatusBadRequest)
			return
		}
		hook.deny(filter)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /deny", func(w http.ResponseWriter, _ *http.Request) {
		hook.allowAll()
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}
