package httpinfra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

func TestSweepStaleTempFiles(t *testing.T) {
	baseDir := t.TempDir()

	secret, err := sessionSecret(baseDir)
	if err != nil {
		t.Fatalf("sessionSecret: %v", err)
	}

	sessionDir := filepath.Join(baseDir, "wirexa-http-abc123")
	if err := os.Mkdir(sessionDir, 0o700); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	inSession := filepath.Join(sessionDir, "response-1")
	if err := os.WriteFile(inSession, []byte("x"), 0o600); err != nil {
		t.Fatalf("write %s: %v", inSession, err)
	}
	marker := filepath.Join(sessionDir, ".wirexa-session")
	if err := os.WriteFile(marker, secret, 0o600); err != nil {
		t.Fatalf("write %s: %v", marker, err)
	}

	// 名前の接頭辞だけが一致し、marker file を持たない無関係なディレクトリ。
	// Wirexa が作成したものではないため sweep で削除されてはならない。
	foreignDir := filepath.Join(baseDir, "wirexa-http-notmine")
	if err := os.Mkdir(foreignDir, 0o700); err != nil {
		t.Fatalf("mkdir foreign dir: %v", err)
	}
	foreignFile := filepath.Join(foreignDir, "data.txt")
	if err := os.WriteFile(foreignFile, []byte("not wirexa's"), 0o600); err != nil {
		t.Fatalf("write %s: %v", foreignFile, err)
	}

	// 固定の文字列を marker に書き込んだだけのディレクトリ。
	// 乱数シークレットと一致しないので、sweep で削除されてはならない。
	// 同じ OS ユーザーの別プロセスが marker を真似ても偽装できないことを確かめる。
	forgedDir := filepath.Join(baseDir, "wirexa-http-forged")
	if err := os.Mkdir(forgedDir, 0o700); err != nil {
		t.Fatalf("mkdir forged dir: %v", err)
	}
	forgedMarker := filepath.Join(forgedDir, ".wirexa-session")
	if err := os.WriteFile(forgedMarker, []byte("wirexa-http-response-store"), 0o600); err != nil {
		t.Fatalf("write %s: %v", forgedMarker, err)
	}

	// 接頭辞に一致しないファイルは sweep の対象外。
	keep := filepath.Join(baseDir, "unrelated.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatalf("write %s: %v", keep, err)
	}

	// logger が nil でも panic しない。
	SweepStaleTempFiles(baseDir, nil)

	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Fatalf("expected stale session dir removed, err=%v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("unrelated file must be kept: %v", err)
	}
	if _, err := os.Stat(foreignFile); err != nil {
		t.Fatalf("foreign dir without a Wirexa marker must be kept: %v", err)
	}
	if _, err := os.Stat(forgedMarker); err != nil {
		t.Fatalf("dir with a forged (publicly-known) marker value must be kept: %v", err)
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
// ID は保存済みリクエストの永続 ID で、一時ファイルの追跡には使われない。
func truncatedRequest(t *testing.T, body []byte) domain.HTTPRequest {
	t.Helper()
	return domain.HTTPRequest{
		ID:       "saved-1",
		Method:   http.MethodGet,
		URL:      serveBytes(t, body),
		Settings: domain.RequestSettings{MaxResponseBodyMB: 1},
	}
}

func TestNetClient_Cleanup(t *testing.T) {
	dir := t.TempDir()
	c := NewNetClient(nil, dir)
	if _, err := c.Do(context.Background(), "exec-big", truncatedRequest(t, make([]byte, 2*1024*1024))); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if files := tempFilesIn(t, dir); len(files) != 1 {
		t.Fatalf("expected 1 tracked temp file before Cleanup, found %v", files)
	}

	c.Cleanup()

	if files := tempFilesIn(t, dir); len(files) != 0 {
		t.Fatalf("expected tracked temp files removed, found %v", files)
	}
	if _, err := c.Responses().AcquireSave("exec-big"); !errors.Is(err, domain.ErrResponseUnavailable) {
		t.Fatalf("AcquireSave after Cleanup: want ErrResponseUnavailable, got %v", err)
	}
}

// 上限に収まるレスポンスでは一時ファイルを一切作らず、execution ID の予約も残さないことを確認する。
func TestNetClient_WithinLimit_NoTempFile(t *testing.T) {
	dir := t.TempDir()
	c := NewNetClient(nil, dir)

	req := domain.HTTPRequest{
		ID:     "saved-1",
		Method: http.MethodGet,
		URL:    serveBytes(t, []byte("hello")),
	}
	res, err := c.Do(context.Background(), "exec-small", req)
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
	// 予約が解放されているので、同じ execution ID で再送できる。
	if _, err := c.Do(context.Background(), "exec-small", req); err != nil {
		t.Fatalf("re-send with the same execution ID after completion: %v", err)
	}
}

// 上限超過時に Body が maxBody で切り詰められ、一時ファイルには全文が残ることを確認する。
func TestNetClient_Truncated_TempFileHoldsFullBody(t *testing.T) {
	dir := t.TempDir()
	const maxBody = 1 * 1024 * 1024
	full := bytes.Repeat([]byte("a"), maxBody+512)

	c := NewNetClient(nil, dir)
	res, err := c.Do(context.Background(), "exec-big", truncatedRequest(t, full))
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
	lease, err := c.Responses().AcquireSave("exec-big")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}
	lease.Release()
}

// 絶対上限に達したら受信を打ち切り、BodyCapped を立てることを確認する。
// 1 GiB を実際に流すのは非現実的なため、maxTempBytes を差し替えて検証する。
func TestNetClient_AbsoluteLimit_CapsBody(t *testing.T) {
	dir := t.TempDir()
	const hardLimit = 8 * 1024
	full := bytes.Repeat([]byte("b"), 64*1024)

	c := NewNetClient(nil, dir)
	c.maxTempBytes = hardLimit

	res, err := c.Do(context.Background(), "exec-huge", domain.HTTPRequest{
		ID:     "saved-1",
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
	dir := t.TempDir()
	c := NewNetClient(nil, dir)
	req := truncatedRequest(t, make([]byte, 2*1024*1024))

	// 追跡は execution ID 単位なので、再送を拒否させるには同じ execution ID を使う。
	if _, err := c.Do(context.Background(), "exec-same", req); err != nil {
		t.Fatalf("first Do: %v", err)
	}
	if _, err := c.Do(context.Background(), "exec-same", req); !errors.Is(err, domain.ErrResponseBusy) {
		t.Fatalf("re-send while tracked: want ErrResponseBusy, got %v", err)
	}
	if files := tempFilesIn(t, dir); len(files) != 1 {
		t.Fatalf("the first temp file must be kept, found %v", files)
	}

	if err := c.Responses().Discard("exec-same"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := c.Do(context.Background(), "exec-same", req); err != nil {
		t.Fatalf("re-send after Discard: %v", err)
	}
	if files := tempFilesIn(t, dir); len(files) != 1 {
		t.Fatalf("expected exactly 1 temp file, found %v", files)
	}
}

// 同じ保存済みリクエストでも execution ID が異なれば一時ファイルは別々に追跡され、
// 一方の Discard が他方を消さないことを確認する。
func TestNetClient_TruncatedSameRequest_TrackedPerExecution(t *testing.T) {
	dir := t.TempDir()
	c := NewNetClient(nil, dir)
	req := truncatedRequest(t, make([]byte, 2*1024*1024))

	for _, execID := range []string{"exec-1", "exec-2"} {
		if _, err := c.Do(context.Background(), execID, req); err != nil {
			t.Fatalf("Do(%s): %v", execID, err)
		}
	}
	if files := tempFilesIn(t, dir); len(files) != 2 {
		t.Fatalf("expected 1 temp file per execution, found %v", files)
	}

	if err := c.Responses().Discard("exec-1"); err != nil {
		t.Fatalf("Discard(exec-1): %v", err)
	}
	if files := tempFilesIn(t, dir); len(files) != 1 {
		t.Fatalf("discarding one execution must keep the other, found %v", files)
	}
	lease, err := c.Responses().AcquireSave("exec-2")
	if err != nil {
		t.Fatalf("AcquireSave(exec-2) after discarding exec-1: %v", err)
	}
	lease.Release()
}

// errReader は常に err を返す io.Reader。
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// 一時ファイルへの書き込み失敗は分類済みの errSpillWrite にし、パスを含めない。
func TestWriteSpill_WriteErrorIsClassified(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "response-*")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	_, _, err = writeSpill(f, 1024, []byte("head"), []byte("p"), strings.NewReader("rest"))
	if !errors.Is(err, errSpillWrite) {
		t.Fatalf("writeSpill error = %v, want errSpillWrite", err)
	}
	if strings.Contains(err.Error(), f.Name()) {
		t.Fatalf("error must not contain the temp path: %q", err)
	}
}

// 残りの本文の読み込みエラーは "failed to read response" で包み、書き込みの失敗と区別する。
func TestWriteSpill_ReadErrorIsNotWriteError(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "response-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	errBroken := errors.New("connection reset")

	_, _, err = writeSpill(f, 1024, []byte("head"), []byte("p"), errReader{errBroken})
	if errors.Is(err, errSpillWrite) {
		t.Fatalf("a read error must not be classified as errSpillWrite: %v", err)
	}
	if !errors.Is(err, errBroken) || !strings.HasPrefix(err.Error(), "failed to read response: ") {
		t.Fatalf("writeSpill error = %v, want a wrapped read error", err)
	}
}

// シークレットファイルの長さが不正 (空など) なら作り直す。
// 空のシークレットを返すと marker が一致せず、sweep が session directory を消せなくなる。
func TestSessionSecret_RecreatesWhenLengthIsInvalid(t *testing.T) {
	baseDir := t.TempDir()
	secretPath := filepath.Join(baseDir, sessionSecretFile)
	if err := os.WriteFile(secretPath, nil, 0o600); err != nil {
		t.Fatalf("write empty secret: %v", err)
	}

	secret, err := sessionSecret(baseDir)
	if err != nil {
		t.Fatalf("sessionSecret: %v", err)
	}
	if len(secret) != sessionSecretSize {
		t.Fatalf("len(secret) = %d, want %d", len(secret), sessionSecretSize)
	}
	stored, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read secret: %v", err)
	}
	if !bytes.Equal(stored, secret) {
		t.Fatal("the secret file must be overwritten with the returned secret")
	}

	// 作り直したあとに作った session directory は sweep で消える。
	store := NewResponseStore(baseDir)
	if err := store.Begin("exec-1"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := store.Spill("exec-1", 1, "text/plain", func(f *os.File) (int64, error) {
		n, werr := f.WriteString("x")
		return int64(n), werr
	}); err != nil {
		t.Fatalf("Spill: %v", err)
	}
	if files := tempFilesIn(t, baseDir); len(files) != 1 {
		t.Fatalf("expected 1 temp file before the sweep, found %v", files)
	}

	SweepStaleTempFiles(baseDir, nil)

	if dirs, _ := filepath.Glob(filepath.Join(baseDir, responseSessionDirPrefix+"*")); len(dirs) != 0 {
		t.Fatalf("expected the session dir swept, found %v", dirs)
	}
}

// sweepLogger は Error の呼び出しを 1 行ずつ記録するテスト用ロガー。
type sweepLogger struct{ errors []string }

var _ cmn.Logger = (*sweepLogger)(nil)

func (l *sweepLogger) Info(string, ...any)  {}
func (l *sweepLogger) Debug(string, ...any) {}
func (l *sweepLogger) Error(msg string, args ...any) {
	l.errors = append(l.errors, msg+" "+fmt.Sprint(args...))
}

// newStaleSessionDir は baseDir に、sweep の対象になる session directory を作って返す。
func newStaleSessionDir(t *testing.T, baseDir, name string) string {
	t.Helper()
	secret, err := sessionSecret(baseDir)
	if err != nil {
		t.Fatalf("sessionSecret: %v", err)
	}
	dir := filepath.Join(baseDir, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, sessionMarkerFile), secret, 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	return dir
}

// session directory を削除できないときは、パスを含めずに 1 件記録して残りの sweep を続ける。
func TestSweepStaleTempFiles_LogsRemoveFailureWithoutPath(t *testing.T) {
	baseDir := t.TempDir()
	locked := newStaleSessionDir(t, baseDir, "wirexa-http-locked")
	removable := newStaleSessionDir(t, baseDir, "wirexa-http-removable")

	orig := removeSessionDir
	removeSessionDir = func(path string) error {
		if path == locked {
			return &fs.PathError{Op: "unlinkat", Path: path, Err: fs.ErrPermission}
		}
		return orig(path)
	}
	t.Cleanup(func() { removeSessionDir = orig })

	logger := &sweepLogger{}
	SweepStaleTempFiles(baseDir, logger)

	if len(logger.errors) != 1 {
		t.Fatalf("expected 1 logged error, got %v", logger.errors)
	}
	got := logger.errors[0]
	if strings.Contains(got, baseDir) {
		t.Fatalf("log must not contain the path: %q", got)
	}
	if !strings.Contains(got, "wirexa-http-locked") || !strings.Contains(got, fs.ErrPermission.Error()) {
		t.Fatalf("log must name the dir by basename and carry the cause: %q", got)
	}
	if _, err := os.Stat(removable); !os.IsNotExist(err) {
		t.Fatalf("the sweep must continue after a failure, err=%v", err)
	}
}

// シークレットを読み書きできないときは、パスを含めずに 1 件記録して sweep を諦める。
func TestSweepStaleTempFiles_LogsSecretFailureWithoutPath(t *testing.T) {
	baseDir := t.TempDir()
	// シークレットのパスをディレクトリにして、読み込みも作成も失敗させる。
	if err := os.Mkdir(filepath.Join(baseDir, sessionSecretFile), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	logger := &sweepLogger{}
	SweepStaleTempFiles(baseDir, logger)

	if len(logger.errors) != 1 {
		t.Fatalf("expected 1 logged error, got %v", logger.errors)
	}
	if strings.Contains(logger.errors[0], baseDir) {
		t.Fatalf("log must not contain the path: %q", logger.errors[0])
	}
}
