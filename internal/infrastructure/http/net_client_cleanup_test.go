package httpinfra

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// isolateTempDir は各 OS の一時ディレクトリ環境変数を専用ディレクトリに向け、
// os.TempDir() ベースの一時ファイル生成・掃除をテスト間で隔離する。
func isolateTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	return dir
}

func TestSweepStaleTempFiles(t *testing.T) {
	dir := isolateTempDir(t)
	sessionDir := filepath.Join(dir, "wirexa-http-abc123")
	if err := os.Mkdir(sessionDir, 0o700); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	inSession := filepath.Join(sessionDir, "response-1")
	legacy := filepath.Join(dir, "wirexa-response-abc123")
	keep := filepath.Join(dir, "unrelated.txt")
	for _, p := range []string{inSession, legacy, keep} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	SweepStaleTempFiles()

	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Fatalf("expected stale session dir removed, err=%v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("expected legacy temp file removed, err=%v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("unrelated file must be kept: %v", err)
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

// tempFilesIn は隔離した TEMP ディレクトリの session directory に残っている一時ファイルの一覧を返す。
func tempFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "wirexa-http-*", "response-*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

// truncatedRequest は maxBody 1MB を超えるレスポンスを返すリクエストを組み立てる。
func truncatedRequest(t *testing.T, id string, body []byte) domain.HTTPRequest {
	t.Helper()
	return domain.HTTPRequest{
		ID:       id,
		Method:   http.MethodGet,
		URL:      serveBytes(t, body),
		Settings: domain.RequestSettings{MaxResponseBodyMB: 1},
	}
}

func TestNetClient_Cleanup(t *testing.T) {
	dir := isolateTempDir(t)
	c := NewNetClient()
	if _, err := c.Do(context.Background(), truncatedRequest(t, "big", make([]byte, 2*1024*1024))); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if files := tempFilesIn(t, dir); len(files) != 1 {
		t.Fatalf("expected 1 tracked temp file before Cleanup, found %v", files)
	}

	c.Cleanup()

	if files := tempFilesIn(t, dir); len(files) != 0 {
		t.Fatalf("expected tracked temp files removed, found %v", files)
	}
	if _, err := c.Responses().AcquireSave("big"); !errors.Is(err, domain.ErrResponseUnavailable) {
		t.Fatalf("AcquireSave after Cleanup: want ErrResponseUnavailable, got %v", err)
	}
}

// 上限に収まるレスポンスでは一時ファイルを一切作らず、execution ID の予約も残さないことを確認する。
func TestNetClient_WithinLimit_NoTempFile(t *testing.T) {
	dir := isolateTempDir(t)
	c := NewNetClient()

	req := domain.HTTPRequest{
		ID:     "small",
		Method: http.MethodGet,
		URL:    serveBytes(t, []byte("hello")),
	}
	res, err := c.Do(context.Background(), req)
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
	// 予約が解放されているので、同じ ID で再送できる。
	if _, err := c.Do(context.Background(), req); err != nil {
		t.Fatalf("re-send with the same ID after completion: %v", err)
	}
}

// 上限超過時に Body が maxBody で切り詰められ、一時ファイルには全文が残ることを確認する。
func TestNetClient_Truncated_TempFileHoldsFullBody(t *testing.T) {
	dir := isolateTempDir(t)
	const maxBody = 1 * 1024 * 1024
	full := bytes.Repeat([]byte("a"), maxBody+512)

	c := NewNetClient()
	res, err := c.Do(context.Background(), truncatedRequest(t, "big", full))
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
	saved, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	if !bytes.Equal(saved, full) {
		t.Fatalf("temp file holds %d bytes, want the full %d", len(saved), len(full))
	}

	// 一時ファイルは execution ID で保存できる状態で追跡されている。
	lease, err := c.Responses().AcquireSave("big")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}
	lease.Release()
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

// 同じ execution ID の打ち切りレスポンスが追跡中の間は再送を拒否し、旧エントリを暗黙に置換しない。
// 破棄した後は同じ ID で再送でき、一時ファイルは常に 1 個以下に保たれる。
func TestNetClient_TruncatedResend_RejectedUntilDiscard(t *testing.T) {
	dir := isolateTempDir(t)
	c := NewNetClient()
	req := truncatedRequest(t, "same-id", make([]byte, 2*1024*1024))

	if _, err := c.Do(context.Background(), req); err != nil {
		t.Fatalf("first Do: %v", err)
	}
	if _, err := c.Do(context.Background(), req); !errors.Is(err, domain.ErrResponseBusy) {
		t.Fatalf("re-send while tracked: want ErrResponseBusy, got %v", err)
	}
	if files := tempFilesIn(t, dir); len(files) != 1 {
		t.Fatalf("the first temp file must be kept, found %v", files)
	}

	if err := c.Responses().Discard("same-id"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := c.Do(context.Background(), req); err != nil {
		t.Fatalf("re-send after Discard: %v", err)
	}
	if files := tempFilesIn(t, dir); len(files) != 1 {
		t.Fatalf("expected exactly 1 temp file, found %v", files)
	}
}
