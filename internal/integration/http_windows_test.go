//go:build integration && windows

// このファイルは Windows 固有の選択ファイルの開き方 (共有モードと MAX_PATH を超えるパス) を
// Handler から確かめる。CI (ubuntu) では実行されないので、Windows のローカルで
// task go:test:integration を実行して確かめる。
package integration

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
)

// TestHTTP_SendRequest_WindowsSelectedFile は、他のハンドルが書き込み用に開いているファイルが
// ErrSelectedFileInUse になることと、MAX_PATH を超えるパスのファイルを送れることを、
// token からパスを引いて開くまでの経路を通して確認する。
func TestHTTP_SendRequest_WindowsSelectedFile(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(data))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	lastBody := func() string {
		mu.Lock()
		defer mu.Unlock()
		if len(bodies) == 0 {
			return ""
		}
		return bodies[len(bodies)-1]
	}
	fileRequest := func(token string) httpdomain.HTTPRequest {
		return httpdomain.HTTPRequest{
			Method: "POST",
			URL:    srv.URL,
			Body:   httpdomain.RequestBody{Type: httpdomain.BodyTypeFile, File: httpdomain.FileReference{Token: token}},
		}
	}

	t.Run("書き込み用に開かれているファイル", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "writing.log")
		if err := os.WriteFile(p, []byte("line 1\n"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		h := newHTTPHandlerWithDialog(t, &fileDialog{path: p})
		picked, err := h.OpenFilePicker("")
		if err != nil {
			t.Fatalf("OpenFilePicker: %v", err)
		}
		w, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatalf("OpenFile: %v", err)
		}
		defer func() { _ = w.Close() }()

		_, err = h.SendRequest("exec-in-use", fileRequest(picked.Token))
		if !errors.Is(err, httpdomain.ErrSelectedFileInUse) {
			t.Fatalf("SendRequest while open for writing: want ErrSelectedFileInUse, got %v", err)
		}
		if strings.Contains(err.Error(), p) {
			t.Errorf("error leaks the path: %q", err)
		}

		// 書き手が閉じれば同じ token と execution ID で送れる。
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if _, err := h.SendRequest("exec-in-use", fileRequest(picked.Token)); err != nil {
			t.Fatalf("SendRequest after the writer closed: %v", err)
		}
		if got := lastBody(); got != "line 1\n" {
			t.Errorf("body = %q, want the file content", got)
		}
	})

	t.Run("MAX_PATH を超えるパス", func(t *testing.T) {
		// 作成と登録には接頭辞 (\\?\) の無い絶対パスを使う。os は内部で接頭辞を付けるが、
		// 送信時の CreateFile は longPath の変換を通らないと開けない。
		dir := t.TempDir()
		for len(dir) < 300 {
			dir = filepath.Join(dir, strings.Repeat("d", 50))
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		p := filepath.Join(dir, "long.txt")
		if err := os.WriteFile(p, []byte("long path bytes"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		h := newHTTPHandlerWithDialog(t, &fileDialog{path: p})
		picked, err := h.OpenFilePicker("")
		if err != nil {
			t.Fatalf("OpenFilePicker: %v", err)
		}
		if _, err := h.SendRequest("exec-long", fileRequest(picked.Token)); err != nil {
			t.Fatalf("SendRequest: %v", err)
		}
		if got := lastBody(); got != "long path bytes" {
			t.Errorf("body = %q, want the file content", got)
		}
	})
}
