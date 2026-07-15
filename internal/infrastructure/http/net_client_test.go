package httpinfra

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

func doGet(t *testing.T, body []byte, contentType string) domain.HTTPResponse {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	res, err := NewNetClient().Do(context.Background(), domain.HTTPRequest{
		ID:     "test",
		Method: http.MethodGet,
		URL:    srv.URL,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return res
}

func TestNetClient_UTF8Body_NotBase64(t *testing.T) {
	res := doGet(t, []byte(`{"ok":true}`), "application/json")
	if res.BodyBase64 {
		t.Fatalf("expected BodyBase64=false for valid UTF-8, got true")
	}
	if res.Body != `{"ok":true}` {
		t.Fatalf("body mismatch: %q", res.Body)
	}
}

func TestNetClient_BinaryBody_Base64(t *testing.T) {
	// 不正な UTF-8 シーケンスを含むバイナリ
	binary := []byte{0x00, 0x01, 0xff, 0xfe, 0x80, 0x48, 0x69}
	res := doGet(t, binary, "application/octet-stream")
	if !res.BodyBase64 {
		t.Fatalf("expected BodyBase64=true for non-UTF-8 body")
	}
	got, err := base64.StdEncoding.DecodeString(res.Body)
	if err != nil {
		t.Fatalf("body is not valid base64: %v", err)
	}
	if !bytes.Equal(got, binary) {
		t.Fatalf("decoded body mismatch: got %v want %v", got, binary)
	}
}

// TestNetClient_ReusesConnection は同一設定の連続リクエストで TCP コネクションが再利用される (keep-alive が効く) ことを確認する。
func TestNetClient_ReusesConnection(t *testing.T) {
	var mu sync.Mutex
	newConns := 0

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			mu.Lock()
			newConns++
			mu.Unlock()
		}
	}
	srv.Start()
	defer srv.Close()

	c := NewNetClient()
	defer c.Cleanup()
	for i := range 3 {
		if _, err := c.Do(context.Background(), domain.HTTPRequest{
			ID:     fmt.Sprintf("req-%d", i),
			Method: http.MethodGet,
			URL:    srv.URL,
		}); err != nil {
			t.Fatalf("Do #%d: %v", i, err)
		}
	}

	mu.Lock()
	got := newConns
	mu.Unlock()
	if got != 1 {
		t.Fatalf("expected connection reuse (1 new conn), got %d", got)
	}
}

// TestNetClient_TransportCache は Transport が設定ごとにキャッシュ・再利用されることを確認する。
func TestNetClient_TransportCache(t *testing.T) {
	c := NewNetClient()
	defer c.Cleanup()

	base := domain.RequestSettings{ProxyMode: "system"}
	first := c.transportFor(base)

	// Transport に影響しない設定 (Timeout / リダイレクト) が違っても同じ Transport を使う。
	same := c.transportFor(domain.RequestSettings{ProxyMode: "system", TimeoutSec: 5, DisableRedirects: true})
	if first != same {
		t.Fatal("expected the same transport for settings that do not affect the transport")
	}

	// Transport に影響する設定が違えば別 Transport になる。
	insecure := c.transportFor(domain.RequestSettings{ProxyMode: "system", InsecureSkipVerify: true})
	if first == insecure {
		t.Fatal("expected a distinct transport for InsecureSkipVerify=true")
	}
	if insecure.TLSClientConfig == nil || !insecure.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify to be applied to the transport")
	}
}

// TestNetClient_TransportCache_Evicts はキャッシュが上限を超えたら破棄されることを確認する
// (ProxyURL がユーザー入力のため、キーが無制限に増えないこと)。
func TestNetClient_TransportCache_Evicts(t *testing.T) {
	c := NewNetClient()
	defer c.Cleanup()

	for i := range maxCachedTransports + 1 {
		c.transportFor(domain.RequestSettings{
			ProxyMode: "custom",
			ProxyURL:  fmt.Sprintf("http://proxy-%d.example", i),
		})
	}

	c.mu.Lock()
	size := len(c.transports)
	c.mu.Unlock()
	if size > maxCachedTransports {
		t.Fatalf("transport cache exceeded its cap: %d", size)
	}
}

// TestNetClient_Timeout はタイムアウト時にユーザー向けの文言でエラーが返ることを確認する。
func TestNetClient_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer srv.Close()

	c := NewNetClient()
	defer c.Cleanup()
	_, err := c.Do(context.Background(), domain.HTTPRequest{
		ID:       "timeout",
		Method:   http.MethodGet,
		URL:      srv.URL,
		Settings: domain.RequestSettings{TimeoutSec: 1},
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a timeout message, got %q", err)
	}
}

func TestNetClient_ImageBody_Base64(t *testing.T) {
	// 画像は非 UTF-8 バイナリなので汎用 base64 経路を通る（image/* 特判の廃止を確認）。
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	res := doGet(t, png, "image/png")
	if !res.BodyBase64 {
		t.Fatalf("expected BodyBase64=true for image body")
	}
	if res.Body != base64.StdEncoding.EncodeToString(png) {
		t.Fatalf("image body not base64-encoded as expected")
	}
}
