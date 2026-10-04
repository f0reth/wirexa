// Command e2e-broker はフルスタック e2e (frontend/playwright.integration.config.ts) 用の
// MQTT ブローカーを起動する。internal/integration の Go 統合テストと同じ mochi-mqtt を使う。
//
// テストがアプリ以外のクライアントとして publish したり、ブローカー側の障害を起こしたりできるよう、
// 操作用の HTTP も開く。
//
//	GET    /healthz                                    起動の確認 (Playwright の webServer.url)
//	GET    /clients                                    接続中のクライアントと、その購読を JSON で返す
//	POST   /publish?topic=<t>&qos=<0-2>&retain=<bool>  本文をペイロードとして publish する
//	POST   /disconnect-clients                         接続中のクライアントをすべて切る
//	POST   /deny?filter=<f>                            以後、そのフィルターへの購読を拒否する
//	DELETE /deny                                       拒否するフィルターをすべて消す
//
// MQTT は平文の TCP のほかに、WebSocket (-ws) と TLS (-tls) でも待ち受ける。TLS の証明書は起動の
// たびに作る自己署名で、アプリからは信頼されない (証明書の検証で接続が失敗することを確かめるのに使う)。
//
// どれもループバックアドレスで待ち受ける前提で、認証は無い。
package main

import (
	"cmp"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
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
// 接続時のパスワードは、/clients で返せるようクライアント ID ごとに覚える (mochi は持たない)。
// 判定はブローカーの接続ごとの goroutine から、登録と解除は操作用 HTTP から呼ばれる。
type denyHook struct {
	mqtt.HookBase
	mu        sync.RWMutex
	filters   map[string]struct{}
	passwords map[string]string
}

func newDenyHook() *denyHook {
	return &denyHook{
		filters:   make(map[string]struct{}),
		passwords: make(map[string]string),
	}
}

func (h *denyHook) ID() string { return "e2e-deny" }

func (h *denyHook) Provides(b byte) bool {
	return b == mqtt.OnConnectAuthenticate || b == mqtt.OnACLCheck
}

func (h *denyHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool { //nolint:gocritic // 引数の型は mochi の Hook インターフェースが決める
	h.mu.Lock()
	defer h.mu.Unlock()
	h.passwords[cl.ID] = string(pk.Connect.Password)
	return true
}

// password は、id のクライアントが最後に接続したときのパスワードを返す。
func (h *denyHook) password(id string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.passwords[id]
}

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
	wsAddr := flag.String("ws", "127.0.0.1:18833", "MQTT (WebSocket) の待ち受けアドレス")
	tlsAddr := flag.String("tls", "127.0.0.1:18834", "MQTT (TLS、自己署名の証明書) の待ち受けアドレス")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, addrs{mqtt: *mqttAddr, ws: *wsAddr, tls: *tlsAddr, http: *httpAddr})
	stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e-broker: %v\n", err)
		os.Exit(1)
	}
}

// addrs は待ち受けアドレス。
type addrs struct {
	mqtt, ws, tls, http string
}

// selfSignedTLSConfig は 127.0.0.1 と localhost 向けの自己署名の証明書を持つ設定を作る。
func selfSignedTLSConfig() (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "wirexa e2e broker"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
	}, nil
}

// run はブローカーと操作用 HTTP を起動し、ctx が終わるまで待つ。
func run(ctx context.Context, a addrs) error {
	// 接続ごとの Info ログは e2e の出力を埋めるので、警告以上だけを出す。
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	server := mqtt.New(&mqtt.Options{InlineClient: true, Logger: logger})
	hook := newDenyHook()
	if err := server.AddHook(hook, nil); err != nil {
		return fmt.Errorf("add hook: %w", err)
	}
	if err := server.AddListener(listeners.NewTCP(listeners.Config{ID: "e2e", Address: a.mqtt})); err != nil {
		return fmt.Errorf("listen mqtt %s: %w", a.mqtt, err)
	}
	if err := server.AddListener(listeners.NewWebsocket(listeners.Config{ID: "e2e-ws", Address: a.ws})); err != nil {
		return fmt.Errorf("listen websocket %s: %w", a.ws, err)
	}
	tlsConfig, err := selfSignedTLSConfig()
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}
	if err := server.AddListener(listeners.NewTCP(listeners.Config{ID: "e2e-tls", Address: a.tls, TLSConfig: tlsConfig})); err != nil {
		return fmt.Errorf("listen tls %s: %w", a.tls, err)
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
		Addr:              a.http,
		Handler:           newControlHandler(server, hook),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()

	select {
	case err := <-errCh:
		return fmt.Errorf("serve http %s: %w", a.http, err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown http: %w", err)
	}
	return nil
}

// clientInfo は /clients が返すクライアント 1 件分。
type clientInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	// Listener はクライアントが接続したリスナーの ID (e2e / e2e-ws / e2e-tls)。
	Listener      string             `json:"listener"`
	Subscriptions []subscriptionInfo `json:"subscriptions"`
}

type subscriptionInfo struct {
	Filter string `json:"filter"`
	QoS    byte   `json:"qos"`
}

// connectedClients は接続中のクライアントを ID 順に返す。publish に使うインラインクライアントと、
// 切断済みでブローカーに残っているだけのクライアントは含めない。
func connectedClients(server *mqtt.Server, hook *denyHook) []clientInfo {
	clients := []clientInfo{}
	for _, client := range server.Clients.GetAll() {
		if client.Net.Inline || client.Closed() {
			continue
		}
		subs := []subscriptionInfo{}
		subscribed := client.State.Subscriptions.GetAll()
		for filter := range subscribed {
			subs = append(subs, subscriptionInfo{Filter: filter, QoS: subscribed[filter].Qos})
		}
		slices.SortFunc(subs, func(a, b subscriptionInfo) int { return cmp.Compare(a.Filter, b.Filter) })
		clients = append(clients, clientInfo{
			ID:            client.ID,
			Username:      string(client.Properties.Username),
			Password:      hook.password(client.ID),
			Listener:      client.Net.Listener,
			Subscriptions: subs,
		})
	}
	slices.SortFunc(clients, func(a, b clientInfo) int { return cmp.Compare(a.ID, b.ID) })
	return clients
}

// newControlHandler は操作用 HTTP のハンドラを返す。
func newControlHandler(server *mqtt.Server, hook *denyHook) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /clients", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:gosec // テスト用のブローカーで、届いたパスワードを確かめるために返す
		if err := json.NewEncoder(w).Encode(connectedClients(server, hook)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
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
