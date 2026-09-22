package httpinfra

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// captureServer は受信したボディと Content-Type を記録するテストサーバを起動する。
func captureServer(t *testing.T) (url string, body, contentType *string, hits *atomic.Int32) {
	t.Helper()
	body, contentType, hits = new(string), new(string), new(atomic.Int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		data, _ := io.ReadAll(r.Body)
		*body = string(data)
		*contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, body, contentType, hits
}

// file body は token を registry で解決したファイルを送り、名前と Content-Type も registry の値を使う。
func TestNetClient_FileBodyUsesRegistry(t *testing.T) {
	path := writeTempFile(t, "payload.txt", "file bytes")
	reg := NewFileRegistry()
	sel, err := reg.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	url, body, contentType, _ := captureServer(t)

	_, err = NewNetClient(reg, t.TempDir()).Do(context.Background(), "exec-1", domain.HTTPRequest{
		ID:     "file-1",
		Method: http.MethodPost,
		URL:    url,
		Body: domain.RequestBody{
			Type: domain.BodyTypeFile,
			File: domain.FileReference{Token: sel.Token, Name: "spoofed.exe", ContentType: "application/x-spoofed"},
		},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if *body != "file bytes" {
		t.Fatalf("body = %q, want the selected file", *body)
	}
	if *contentType != domain.GuessFileContentType(path) {
		t.Fatalf("Content-Type = %q, want the registry's %q", *contentType, domain.GuessFileContentType(path))
	}
}

// Contents に生のパスを入れても読まない (token が無ければ空ボディ)。
func TestNetClient_FileBodyIgnoresRawPath(t *testing.T) {
	path := writeTempFile(t, "secret.txt", "top secret")
	url, body, _, hits := captureServer(t)

	_, err := NewNetClient(NewFileRegistry(), t.TempDir()).Do(context.Background(), "exec-1", domain.HTTPRequest{
		ID:     "file-raw",
		Method: http.MethodPost,
		URL:    url,
		Body:   domain.RequestBody{Type: domain.BodyTypeFile, Contents: map[string]string{domain.BodyTypeFile: path}},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if hits.Load() != 1 || *body != "" {
		t.Fatalf("server got body %q, want an empty body", *body)
	}
}

// 解決できない参照はファイルを開く前に拒否し、リクエスト自体を送らない。
func TestNetClient_FileBodyRejectsUnresolvedReferences(t *testing.T) {
	url, _, _, hits := captureServer(t)
	c := NewNetClient(NewFileRegistry(), t.TempDir())

	for i, ref := range []domain.FileReference{
		{Token: "00112233445566778899aabbccddeeff"},
		{Name: "old.bin", NeedsReselect: true},
	} {
		_, err := c.Do(context.Background(), "exec-"+string(rune('a'+i)), domain.HTTPRequest{
			ID:     "denied-" + string(rune('a'+i)),
			Method: http.MethodPost,
			URL:    url,
			Body:   domain.RequestBody{Type: domain.BodyTypeFile, File: ref},
		})
		if !errors.Is(err, domain.ErrFileAccessDenied) {
			t.Fatalf("ref %+v: want ErrFileAccessDenied, got %v", ref, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("denied requests must not reach the server, got %d hits", hits.Load())
	}
}
