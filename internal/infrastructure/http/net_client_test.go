package httpinfra

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
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

	res, err := NewNetClient(nil, t.TempDir()).Do(context.Background(), "exec-1", domain.HTTPRequest{
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

	c := NewNetClient(nil, t.TempDir())
	defer c.Cleanup()
	for i := range 3 {
		if _, err := c.Do(context.Background(), fmt.Sprintf("exec-%d", i), domain.HTTPRequest{
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
	c := NewNetClient(nil, t.TempDir())
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
	c := NewNetClient(nil, t.TempDir())
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

	c := NewNetClient(nil, t.TempDir())
	defer c.Cleanup()
	_, err := c.Do(context.Background(), "exec-1", domain.HTTPRequest{
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

// DisableRedirects なら 3xx を追わずにそのまま返し、既定では追う。
func TestNetClient_DisableRedirects_Returns3xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("final"))
	}))
	defer srv.Close()

	tests := []struct {
		name    string
		disable bool
		status  int
		body    string
	}{
		{"redirects disabled", true, http.StatusFound, ""},
		{"redirects followed", false, http.StatusOK, "final"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewNetClient(nil, t.TempDir())
			defer c.Cleanup()
			res, err := c.Do(context.Background(), "exec-1", domain.HTTPRequest{
				Method:   http.MethodGet,
				URL:      srv.URL + "/start",
				Settings: domain.RequestSettings{DisableRedirects: tc.disable},
			})
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			if res.StatusCode != tc.status {
				t.Fatalf("StatusCode = %d, want %d", res.StatusCode, tc.status)
			}
			if tc.body != "" && res.Body != tc.body {
				t.Fatalf("Body = %q, want %q", res.Body, tc.body)
			}
			if tc.disable && res.Headers["Location"][0] != "/final" {
				t.Fatalf("Location = %v, want /final", res.Headers["Location"])
			}
		})
	}
}

func TestResolveProxy_Modes(t *testing.T) {
	fromEnv := reflect.ValueOf(http.ProxyFromEnvironment).Pointer()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)

	for _, mode := range []string{"", "system"} {
		p := resolveProxy(domain.RequestSettings{ProxyMode: mode})
		if p == nil || reflect.ValueOf(p).Pointer() != fromEnv {
			t.Errorf("ProxyMode %q should use ProxyFromEnvironment", mode)
		}
	}
	if p := resolveProxy(domain.RequestSettings{ProxyMode: "none"}); p != nil {
		t.Error(`ProxyMode "none" should connect directly (nil)`)
	}

	p := resolveProxy(domain.RequestSettings{ProxyMode: "custom", ProxyURL: "http://proxy.local:8080"})
	if p == nil {
		t.Fatal(`ProxyMode "custom" with a valid URL should return a proxy func`)
	}
	got, err := p(req)
	if err != nil || got.String() != "http://proxy.local:8080" {
		t.Errorf("custom proxy = %v, %v; want http://proxy.local:8080", got, err)
	}

	// 空・解釈できないカスタムプロキシは、現在の仕様では黙って直結 (nil) になる。
	for _, raw := range []string{"", "http://[::1"} {
		if p := resolveProxy(domain.RequestSettings{ProxyMode: "custom", ProxyURL: raw}); p != nil {
			t.Errorf("custom proxy %q should fall back to a direct connection (nil)", raw)
		}
	}
}

// 失敗した Do のあとも ResponseStore に予約が残らず、同じ execution ID で再送できることを確かめる。
func assertExecutionReleased(t *testing.T, c *NetClient, executionID string) {
	t.Helper()
	if err := c.responses.Begin(executionID); err != nil {
		t.Fatalf("execution %s should be released after a failed Do: %v", executionID, err)
	}
	c.responses.Finish(executionID)
}

func TestNetClient_InvalidURL(t *testing.T) {
	c := NewNetClient(nil, t.TempDir())
	defer c.Cleanup()

	_, err := c.Do(context.Background(), "exec-1", domain.HTTPRequest{Method: http.MethodGet, URL: "http://[::1"})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid URL: ") {
		t.Fatalf("Do error = %v, want an invalid URL error", err)
	}
	if _, ok := errors.AsType[*url.Error](err); !ok {
		t.Fatalf("error should wrap *url.Error, got %T", errors.Unwrap(err))
	}
	assertExecutionReleased(t, c, "exec-1")
}

// hijackServer は Content-Length より短い本文を送ってから onBody を呼ぶサーバを起動する。
func hijackServer(t *testing.T, contentLength int, body []byte, onBody func(net.Conn)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer does not support hijacking")
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			t.Errorf("Hijack: %v", err)
			return
		}
		defer conn.Close()
		_, _ = fmt.Fprintf(buf, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n", contentLength)
		_, _ = buf.Write(body)
		_ = buf.Flush()
		onBody(conn)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// 本文の途中でサーバが切断したら "failed to read response" で包み、元のエラーを辿れる。
func TestNetClient_ResponseReadError(t *testing.T) {
	u := hijackServer(t, 100, []byte("partial"), func(net.Conn) {})
	c := NewNetClient(nil, t.TempDir())
	defer c.Cleanup()

	_, err := c.Do(context.Background(), "exec-1", domain.HTTPRequest{Method: http.MethodGet, URL: u})
	if err == nil || !strings.HasPrefix(err.Error(), "failed to read response: ") {
		t.Fatalf("Do error = %v, want a read error", err)
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error should wrap io.ErrUnexpectedEOF: %v", err)
	}
	assertExecutionReleased(t, c, "exec-1")
}

// 本文の読み込み中にタイムアウトしたら、タイムアウトとして分類する。
func TestNetClient_ResponseReadTimeout(t *testing.T) {
	stall := func(net.Conn) { time.Sleep(3 * time.Second) }
	tests := []struct {
		name string
		body []byte
	}{
		// maxBody に届く前に止まる: 本文の読み込みでタイムアウトする。
		{"while reading body", []byte("partial")},
		// maxBody ちょうどで止まる: 続きを確かめる 1 バイトの先読みでタイムアウトする。
		{"while peeking past maxBody", bytes.Repeat([]byte("a"), 1024*1024)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := hijackServer(t, 2*1024*1024, tc.body, stall)
			c := NewNetClient(nil, t.TempDir())
			defer c.Cleanup()

			_, err := c.Do(context.Background(), "exec-1", domain.HTTPRequest{
				Method:   http.MethodGet,
				URL:      u,
				Settings: domain.RequestSettings{TimeoutSec: 1, MaxResponseBodyMB: 1},
			})
			if err == nil || !strings.Contains(err.Error(), "timed out") {
				t.Fatalf("Do error = %v, want a timeout", err)
			}
			assertExecutionReleased(t, c, "exec-1")
		})
	}
}
