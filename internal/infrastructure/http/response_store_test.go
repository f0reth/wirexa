package httpinfra

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// newTestStore は隔離した base directory を使う ResponseStore を返す。
func newTestStore(t *testing.T) (*ResponseStore, string) {
	t.Helper()
	dir := t.TempDir()
	s := NewResponseStore(dir)
	t.Cleanup(s.Cleanup)
	return s, dir
}

// spillString は id を予約して data を一時ファイルへ書き、ready にする。
func spillString(s *ResponseStore, id, data, contentType string) error {
	if err := s.Begin(id); err != nil {
		return err
	}
	defer s.Finish(id)
	return s.Spill(id, int64(len(data)), contentType, func(f *os.File) (int64, error) {
		n, err := f.WriteString(data)
		return int64(n), err
	})
}

func mustSpill(t *testing.T, s *ResponseStore, id, data string) {
	t.Helper()
	if err := spillString(s, id, data, "application/json"); err != nil {
		t.Fatalf("spill %s: %v", id, err)
	}
}

// trackedPath は ready のエントリが保持する一時ファイルのパスを返す。
func trackedPath(t *testing.T, s *ResponseStore, id string) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		t.Fatalf("entry %s is not tracked", id)
	}
	return e.path
}

func TestResponseStore_SaveCopiesAndDropsEntry(t *testing.T) {
	s, _ := newTestStore(t)
	mustSpill(t, s, "exec-1", "full body")
	src := trackedPath(t, s, "exec-1")

	lease, err := s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}
	if lease.ContentType() != "application/json" {
		t.Fatalf("ContentType = %q, want application/json", lease.ContentType())
	}
	dst := filepath.Join(t.TempDir(), "saved.json")
	if err = lease.SaveTo(dst); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "full body" {
		t.Fatalf("saved content = %q, err=%v", got, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("temp file must be removed after save, err=%v", err)
	}
	// 保存済み ID は未追跡と同様に拒否する。
	if _, err := s.AcquireSave("exec-1"); !errors.Is(err, domain.ErrResponseUnavailable) {
		t.Fatalf("re-save: want ErrResponseUnavailable, got %v", err)
	}
}

func TestResponseStore_RejectsUntrackedID(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.AcquireSave("unknown"); !errors.Is(err, domain.ErrResponseUnavailable) {
		t.Fatalf("AcquireSave: want ErrResponseUnavailable, got %v", err)
	}
	if err := s.Discard("unknown"); !errors.Is(err, domain.ErrResponseUnavailable) {
		t.Fatalf("Discard: want ErrResponseUnavailable, got %v", err)
	}
}

func TestResponseStore_ReleaseKeepsFileForRetry(t *testing.T) {
	s, _ := newTestStore(t)
	mustSpill(t, s, "exec-1", "body")
	src := trackedPath(t, s, "exec-1")

	lease, err := s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}
	lease.Release()

	if _, err = os.Stat(src); err != nil {
		t.Fatalf("temp file must be kept after cancel: %v", err)
	}
	lease, err = s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave after cancel: %v", err)
	}
	lease.Release()
}

// 保存中は同じ ID の破棄・送信・二重保存・TTL 回収を拒否する。
func TestResponseStore_SavingBlocksOtherOperations(t *testing.T) {
	s, _ := newTestStore(t)
	now := time.Now()
	s.now = func() time.Time { return now }
	mustSpill(t, s, "exec-1", "body")

	lease, err := s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}
	if err := s.Discard("exec-1"); !errors.Is(err, domain.ErrResponseBusy) {
		t.Fatalf("Discard while saving: want ErrResponseBusy, got %v", err)
	}
	if err := s.Begin("exec-1"); !errors.Is(err, domain.ErrResponseBusy) {
		t.Fatalf("Begin while saving: want ErrResponseBusy, got %v", err)
	}
	if _, err := s.AcquireSave("exec-1"); !errors.Is(err, domain.ErrResponseBusy) {
		t.Fatalf("second AcquireSave: want ErrResponseBusy, got %v", err)
	}

	now = now.Add(2 * responseReadyTTL)
	if err := s.Begin("other"); err != nil { // TTL 回収を走らせる
		t.Fatalf("Begin other: %v", err)
	}
	s.Finish("other")
	if err := lease.SaveTo(filepath.Join(t.TempDir(), "out")); err != nil {
		t.Fatalf("SaveTo after TTL sweep: %v", err)
	}
}

func TestResponseStore_BeginRejectsTrackedID(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Begin("exec-1"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := s.Begin("exec-1"); !errors.Is(err, domain.ErrResponseBusy) {
		t.Fatalf("Begin while running: want ErrResponseBusy, got %v", err)
	}
	s.Finish("exec-1")

	mustSpill(t, s, "exec-1", "body")
	if err := s.Begin("exec-1"); !errors.Is(err, domain.ErrResponseBusy) {
		t.Fatalf("Begin while ready: want ErrResponseBusy, got %v", err)
	}
	if err := s.Discard("exec-1"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if err := s.Begin("exec-1"); err != nil {
		t.Fatalf("Begin after Discard: %v", err)
	}
}

func TestResponseStore_CopyFailureRestoresReady(t *testing.T) {
	s, _ := newTestStore(t)
	mustSpill(t, s, "exec-1", "body")
	src := trackedPath(t, s, "exec-1")
	s.ops.copyFile = func(_, _ string) error { return errors.New("disk full at " + src) }

	lease, err := s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}
	err = lease.SaveTo(filepath.Join(t.TempDir(), "out"))
	if !errors.Is(err, domain.ErrSaveResponseFailed) {
		t.Fatalf("SaveTo: want ErrSaveResponseFailed, got %v", err)
	}
	if err.Error() != domain.ErrSaveResponseFailed.Error() {
		t.Fatalf("error must not leak OS details, got %q", err)
	}
	if _, err = os.Stat(src); err != nil {
		t.Fatalf("temp file must be kept for retry: %v", err)
	}
	s.ops = defaultResponseFileOps
	lease, err = s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave after failed copy: %v", err)
	}
	lease.Release()
}

func TestResponseStore_RemoveFailureRestoresReady(t *testing.T) {
	s, _ := newTestStore(t)
	mustSpill(t, s, "exec-1", "body")
	s.ops.remove = func(string) error { return errors.New("permission denied") }

	lease, err := s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}
	if err = lease.SaveTo(filepath.Join(t.TempDir(), "out")); !errors.Is(err, domain.ErrSaveResponseFailed) {
		t.Fatalf("SaveTo: want ErrSaveResponseFailed, got %v", err)
	}
	s.ops = defaultResponseFileOps
	lease, err = s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave after failed remove: %v", err)
	}
	lease.Release()
}

// コピー失敗時に元ファイルも失われていれば、追跡も外して上限の計上を解放する。
func TestResponseStore_LostSourceDropsEntry(t *testing.T) {
	s, _ := newTestStore(t)
	mustSpill(t, s, "exec-1", "body")
	s.ops.copyFile = func(src, _ string) error {
		_ = os.Remove(src)
		return errors.New("source vanished")
	}

	lease, err := s.AcquireSave("exec-1")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}
	if err := lease.SaveTo(filepath.Join(t.TempDir(), "out")); !errors.Is(err, domain.ErrSaveResponseFailed) {
		t.Fatalf("SaveTo: want ErrSaveResponseFailed, got %v", err)
	}
	if _, err := s.AcquireSave("exec-1"); !errors.Is(err, domain.ErrResponseUnavailable) {
		t.Fatalf("AcquireSave after lost source: want ErrResponseUnavailable, got %v", err)
	}
	if s.files != 0 || s.total != 0 {
		t.Fatalf("accounting leaked: files=%d total=%d", s.files, s.total)
	}
}

func TestResponseStore_DiscardFailureKeepsEntry(t *testing.T) {
	s, _ := newTestStore(t)
	mustSpill(t, s, "exec-1", "body")
	s.ops.remove = func(string) error { return errors.New("busy") }

	if err := s.Discard("exec-1"); !errors.Is(err, domain.ErrDiscardResponseFailed) {
		t.Fatalf("Discard: want ErrDiscardResponseFailed, got %v", err)
	}
	s.ops = defaultResponseFileOps
	if err := s.Discard("exec-1"); err != nil {
		t.Fatalf("retry Discard: %v", err)
	}
}

func TestResponseStore_FileCountLimit(t *testing.T) {
	s, _ := newTestStore(t)
	for i := range maxResponseFiles {
		mustSpill(t, s, fmt.Sprintf("exec-%d", i), "x")
	}
	if err := spillString(s, "overflow", "x", ""); !errors.Is(err, domain.ErrResponseStorageLimit) {
		t.Fatalf("spill over the file limit: want ErrResponseStorageLimit, got %v", err)
	}
	// 既存ファイルは無断で消さない。破棄すれば空きができる。
	if err := s.Discard("exec-0"); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	mustSpill(t, s, "overflow", "x")
}

// 書き込み中の spill は絶対上限分を予約するため、並行する spill が総容量をすり抜けられない。
func TestResponseStore_TotalBytesReservedDuringSpill(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Begin("big"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	var inner error
	err := s.Spill("big", maxResponseTotalBytes, "", func(f *os.File) (int64, error) {
		inner = spillString(s, "second", "x", "")
		n, werr := f.WriteString("small")
		return int64(n), werr
	})
	if err != nil {
		t.Fatalf("outer spill: %v", err)
	}
	if !errors.Is(inner, domain.ErrResponseStorageLimit) {
		t.Fatalf("spill during a full reservation: want ErrResponseStorageLimit, got %v", inner)
	}
	// 完了後は実サイズへ精算されるので、次の spill は通る。
	mustSpill(t, s, "second", "x")
}

func TestResponseStore_ConcurrentSpillLimit(t *testing.T) {
	s, _ := newTestStore(t)
	nested := func(id string, next func() error) error {
		if err := s.Begin(id); err != nil {
			return err
		}
		defer s.Finish(id)
		return s.Spill(id, 1, "", func(f *os.File) (int64, error) {
			if err := next(); err != nil {
				return 0, err
			}
			n, err := f.WriteString("x")
			return int64(n), err
		})
	}
	third := func() error { return spillString(s, "c", "x", "") }
	err := nested("a", func() error {
		return nested("b", func() error {
			if err := third(); !errors.Is(err, domain.ErrResponseStorageLimit) {
				return fmt.Errorf("third concurrent spill: want ErrResponseStorageLimit, got %w", err)
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResponseStore_SpillWriteFailureCleansUp(t *testing.T) {
	s, dir := newTestStore(t)
	if err := s.Begin("exec-1"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	err := s.Spill("exec-1", 16, "", func(*os.File) (int64, error) { return 0, errSpillWrite })
	if !errors.Is(err, errSpillWrite) {
		t.Fatalf("Spill: want errSpillWrite, got %v", err)
	}
	s.Finish("exec-1")
	if files := tempFilesIn(t, dir); len(files) != 0 {
		t.Fatalf("failed spill left files: %v", files)
	}
	if s.files != 0 || s.total != 0 || s.spilling != 0 {
		t.Fatalf("accounting leaked: files=%d total=%d spilling=%d", s.files, s.total, s.spilling)
	}
	if err := s.Begin("exec-1"); err != nil {
		t.Fatalf("Begin after failed spill: %v", err)
	}
}

func TestResponseStore_TTLEvictsReadyOnly(t *testing.T) {
	s, _ := newTestStore(t)
	now := time.Now()
	s.now = func() time.Time { return now }
	mustSpill(t, s, "old", "body")
	src := trackedPath(t, s, "old")

	now = now.Add(responseReadyTTL)
	if _, err := s.AcquireSave("old"); !errors.Is(err, domain.ErrResponseUnavailable) {
		t.Fatalf("AcquireSave after TTL: want ErrResponseUnavailable, got %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expired temp file must be removed, err=%v", err)
	}
}

func TestResponseStore_CleanupKeepsSavingAndStopsNewOperations(t *testing.T) {
	s, dir := newTestStore(t)
	mustSpill(t, s, "ready", "a")
	mustSpill(t, s, "saving", "b")
	readyPath := trackedPath(t, s, "ready")
	savingPath := trackedPath(t, s, "saving")
	lease, err := s.AcquireSave("saving")
	if err != nil {
		t.Fatalf("AcquireSave: %v", err)
	}

	s.Cleanup()

	if _, err := os.Stat(readyPath); !os.IsNotExist(err) {
		t.Fatalf("ready temp file must be removed, err=%v", err)
	}
	if _, err := os.Stat(savingPath); err != nil {
		t.Fatalf("saving temp file must not be removed concurrently: %v", err)
	}
	if err := s.Begin("new"); !errors.Is(err, domain.ErrResponseUnavailable) {
		t.Fatalf("Begin after Cleanup: want ErrResponseUnavailable, got %v", err)
	}

	// 終了後に保存がキャンセルされたら、その場で回収する。
	lease.Release()
	if files := tempFilesIn(t, dir); len(files) != 0 {
		t.Fatalf("expected no temp files left, found %v", files)
	}
}

// 保存・破棄・TTL・送信を並行実行しても、二重削除や計上漏れが起きないことを -race で確認する。
func TestResponseStore_ConcurrentOperations(t *testing.T) {
	s, dir := newTestStore(t)
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			id := fmt.Sprintf("exec-%d", i%4)
			_ = spillString(s, id, "body", "text/plain")
			if lease, err := s.AcquireSave(id); err == nil {
				if i%2 == 0 {
					_ = lease.SaveTo(filepath.Join(t.TempDir(), "out"))
				} else {
					lease.Release()
				}
			}
			_ = s.Discard(id)
		})
	}
	wg.Wait()

	for i := range 4 {
		_ = s.Discard(fmt.Sprintf("exec-%d", i))
	}
	if s.files != 0 || s.total != 0 || s.spilling != 0 || len(s.entries) != 0 {
		t.Fatalf("accounting leaked: files=%d total=%d spilling=%d entries=%d", s.files, s.total, s.spilling, len(s.entries))
	}
	if files := tempFilesIn(t, dir); len(files) != 0 {
		t.Fatalf("orphan temp files: %v", files)
	}
}
