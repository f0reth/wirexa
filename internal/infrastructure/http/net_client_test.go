package httpinfra

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

func doGet(t *testing.T, body []byte, contentType string) domain.HttpResponse {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	res, err := NewNetClient().Do(context.Background(), domain.HttpRequest{
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
	if string(got) != string(binary) {
		t.Fatalf("decoded body mismatch: got %v want %v", got, binary)
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
