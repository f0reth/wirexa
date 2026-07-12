package httpinfra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// isolateTempDir は TMP/TEMP を専用ディレクトリに向け、os.TempDir() ベースの
// 一時ファイル生成・掃除をテスト間で隔離する。
func isolateTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	return dir
}

func TestSweepStaleTempFiles(t *testing.T) {
	dir := isolateTempDir(t)
	stale := filepath.Join(dir, "wirexa-response-abc123")
	keep := filepath.Join(dir, "unrelated.txt")
	if err := os.WriteFile(stale, []byte("x"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatalf("write keep: %v", err)
	}

	SweepStaleTempFiles()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale temp file removed, err=%v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("unrelated file must be kept: %v", err)
	}
}

func TestNetClient_Cleanup(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "wirexa-response-tracked")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := NewNetClient()
	c.tempFiles.Store("req-1", f)

	c.Cleanup()

	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Fatalf("expected tracked temp file removed, err=%v", err)
	}
	if _, ok := c.tempFiles.Load("req-1"); ok {
		t.Fatalf("expected tempFiles entry deleted")
	}
}

// 同一 req.ID で打ち切りレスポンスを連続受信しても、一時ファイルがオーファン化せず
// 常に 1 個以下に保たれることを確認する。
func TestNetClient_TruncatedResend_NoOrphan(t *testing.T) {
	dir := isolateTempDir(t)
	// maxBody の最小は 1MB。それを超える 2MB を返して打ち切りを発生させる。
	body := make([]byte, 2*1024*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := NewNetClient()
	req := domain.HttpRequest{
		ID:       "same-id",
		Method:   http.MethodGet,
		URL:      srv.URL,
		Settings: domain.RequestSettings{MaxResponseBodyMB: 1},
	}

	for i := range 2 {
		res, err := c.Do(context.Background(), req)
		if err != nil {
			t.Fatalf("Do #%d: %v", i, err)
		}
		if !res.BodyTruncated {
			t.Fatalf("expected truncated response on iteration %d", i)
		}
	}

	matches, err := filepath.Glob(filepath.Join(dir, "wirexa-response-*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) > 1 {
		t.Fatalf("expected at most 1 temp file, found %d: %v", len(matches), matches)
	}
}
