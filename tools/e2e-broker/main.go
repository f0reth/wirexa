// Command e2e-broker はフルスタック e2e (frontend/playwright.integration.config.ts) 用の
// MQTT ブローカーを起動する。internal/integration の Go 統合テストと同じ mochi-mqtt を使う。
//
// テストがアプリ以外のクライアントとして publish できるよう、操作用の HTTP も開く。
//
//	GET  /healthz                                    起動の確認 (Playwright の webServer.url)
//	POST /publish?topic=<t>&qos=<0-2>&retain=<bool>  本文をペイロードとして publish する
//
// どちらもループバックアドレスで待ち受ける前提で、認証は無い。
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
	"syscall"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// maxPayloadBytes は /publish が受け付けるペイロードの上限。
const maxPayloadBytes = 1 << 20

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
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
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
		Handler:           newControlHandler(server),
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
func newControlHandler(server *mqtt.Server) http.Handler {
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
	return mux
}
