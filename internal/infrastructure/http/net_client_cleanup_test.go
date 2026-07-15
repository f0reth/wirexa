package httpinfra

import (
	"bytes"
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

// serveBytes は body をそのまま返すテストサーバを起動する。
func serveBytes(t *testing.T, body []byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// tempFilesIn は隔離した TEMP ディレクトリに残っている一時ファイルの一覧を返す。
func tempFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "wirexa-response-*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

// 上限に収まるレスポンスでは一時ファイルを一切作らないことを確認する。
func TestNetClient_WithinLimit_NoTempFile(t *testing.T) {
	dir := isolateTempDir(t)
	c := NewNetClient()

	res, err := c.Do(context.Background(), domain.HTTPRequest{
		ID:     "small",
		Method: http.MethodGet,
		URL:    serveBytes(t, []byte("hello")),
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if res.BodyTruncated || res.BodyCapped {
		t.Fatalf("expected complete response, got truncated=%v capped=%v", res.BodyTruncated, res.BodyCapped)
	}
	if res.Body != "hello" {
		t.Fatalf("body = %q, want %q", res.Body, "hello")
	}
	if res.Size != 5 {
		t.Fatalf("size = %d, want 5", res.Size)
	}
	if files := tempFilesIn(t, dir); len(files) != 0 {
		t.Fatalf("expected no temp file for a response within the limit, found %v", files)
	}
}

// 上限超過時に Body が maxBody で切り詰められ、一時ファイルには全文が残ることを確認する。
func TestNetClient_Truncated_TempFileHoldsFullBody(t *testing.T) {
	dir := isolateTempDir(t)
	const maxBody = 1 * 1024 * 1024
	full := bytes.Repeat([]byte("a"), maxBody+512)

	c := NewNetClient()
	res, err := c.Do(context.Background(), domain.HTTPRequest{
		ID:       "big",
		Method:   http.MethodGet,
		URL:      serveBytes(t, full),
		Settings: domain.RequestSettings{MaxResponseBodyMB: 1},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if !res.BodyTruncated {
		t.Fatalf("expected BodyTruncated")
	}
	if res.BodyCapped {
		t.Fatalf("unexpected BodyCapped: the body is far below the absolute limit")
	}
	if len(res.Body) != maxBody {
		t.Fatalf("len(Body) = %d, want %d", len(res.Body), maxBody)
	}
	if res.Size != int64(len(full)) {
		t.Fatalf("Size = %d, want %d", res.Size, len(full))
	}

	files := tempFilesIn(t, dir)
	if len(files) != 1 {
		t.Fatalf("expected exactly 1 temp file, found %v", files)
	}
	if got := c.ConsumeTempFilePath("big"); got != files[0] {
		t.Fatalf("ConsumeTempFilePath = %q, want %q", got, files[0])
	}
	saved, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	if !bytes.Equal(saved, full) {
		t.Fatalf("temp file holds %d bytes, want the full %d", len(saved), len(full))
	}
}

// 絶対上限に達したら受信を打ち切り、BodyCapped を立てることを確認する。
// 1 GiB を実際に流すのは非現実的なため、maxTempBytes を差し替えて検証する。
func TestNetClient_AbsoluteLimit_CapsBody(t *testing.T) {
	dir := isolateTempDir(t)
	const hardLimit = 8 * 1024
	full := bytes.Repeat([]byte("b"), 64*1024)

	c := NewNetClient()
	c.maxTempBytes = hardLimit

	res, err := c.Do(context.Background(), domain.HTTPRequest{
		ID:     "huge",
		Method: http.MethodGet,
		URL:    serveBytes(t, full),
		// maxBody は maxTempBytes までクランプされるので、大きな値を指定しても上限は hardLimit になる。
		Settings: domain.RequestSettings{MaxResponseBodyMB: 100},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if !res.BodyTruncated || !res.BodyCapped {
		t.Fatalf("expected truncated+capped, got truncated=%v capped=%v", res.BodyTruncated, res.BodyCapped)
	}
	if res.Size != hardLimit {
		t.Fatalf("Size = %d, want %d (clamped to the absolute limit)", res.Size, hardLimit)
	}

	files := tempFilesIn(t, dir)
	if len(files) != 1 {
		t.Fatalf("expected exactly 1 temp file, found %v", files)
	}
	info, err := os.Stat(files[0])
	if err != nil {
		t.Fatalf("stat temp file: %v", err)
	}
	if info.Size() != hardLimit {
		t.Fatalf("temp file size = %d, want %d", info.Size(), hardLimit)
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
	req := domain.HTTPRequest{
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
