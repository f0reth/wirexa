package openapiapp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	domain "github.com/f0reth/Wirexa/internal/domain/openapi"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// memFiles はインメモリの FileAccess。呼び出し回数を記録する。
type memFiles struct {
	data     map[string][]byte
	writeErr error
	reads    int
	writes   int
	mu       sync.Mutex
}

func newMemFiles() *memFiles {
	return &memFiles{data: make(map[string][]byte)}
}

func (f *memFiles) ReadFile(path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	d, ok := f.data[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return d, nil
}

func (f *memFiles) WriteFile(path string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	if f.writeErr != nil {
		return f.writeErr
	}
	f.data[path] = slices.Clone(data)
	return nil
}

// memRepo はインメモリの RecentRepository。Save に渡された一覧と呼び出し回数を記録する。
type memRepo struct {
	items         []domain.OpenAPIRecent
	loadErr       error
	saveErr       error
	quarantineErr error
	saves         [][]domain.OpenAPIRecent
	quarantines   int
	mu            sync.Mutex
}

func (r *memRepo) Load() ([]domain.OpenAPIRecent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	return slices.Clone(r.items), nil
}

func (r *memRepo) Save(items []domain.OpenAPIRecent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saves = append(r.saves, slices.Clone(items))
	r.items = slices.Clone(items)
	return nil
}

func (r *memRepo) Quarantine() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.quarantines++
	if r.quarantineErr != nil {
		return "", r.quarantineErr
	}
	return "openapi-recents.json.corrupt", nil
}

func (r *memRepo) saveCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.saves)
}

// recordLogger は Error の呼び出し回数を記録する Logger。
type recordLogger struct {
	testutil.NoopLogger
	errors int
	mu     sync.Mutex
}

func (l *recordLogger) Error(_ string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors++
}

func (l *recordLogger) errorCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.errors
}

// fixedClock は呼ぶたびに 1 分ずつ進む時計を返す。
func fixedClock() func() time.Time {
	t := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	return func() time.Time {
		t = t.Add(time.Minute)
		return t
	}
}

// spec はテスト用の OpenAPI ファイルパスを返す (Clean 済み)。
func spec(name string) string {
	return filepath.Join("specs", name)
}

func newTestService(t *testing.T, repo *memRepo) (*FileService, *memFiles) {
	t.Helper()
	files := newMemFiles()
	s := NewFileService(repo, files, testutil.NoopLogger{})
	s.now = fixedClock()
	return s, files
}

func TestReadFileDeniesUngrantedPath(t *testing.T) {
	s, files := newTestService(t, &memRepo{})
	files.data[spec("secret.yaml")] = []byte("openapi: 3.0.0")

	if _, err := s.ReadFile(spec("secret.yaml")); !errors.Is(err, domain.ErrFileAccessDenied) {
		t.Fatalf("ReadFile on ungranted path: want ErrFileAccessDenied, got %v", err)
	}
	if files.reads != 0 {
		t.Fatalf("FileAccess.ReadFile must not be called on denial (reads = %d)", files.reads)
	}
}

func TestWriteFileDeniesUngrantedPath(t *testing.T) {
	s, files := newTestService(t, &memRepo{})

	if err := s.WriteFile(spec("out.yaml"), "data"); !errors.Is(err, domain.ErrFileAccessDenied) {
		t.Fatalf("WriteFile on ungranted path: want ErrFileAccessDenied, got %v", err)
	}
	if files.writes != 0 {
		t.Fatalf("FileAccess.WriteFile must not be called on denial (writes = %d)", files.writes)
	}
}

func TestGrantedPathReadWrite(t *testing.T) {
	s, _ := newTestService(t, &memRepo{})
	target := s.OpenSelected(spec("spec.yaml"))

	if err := s.WriteFile(target, "openapi: 3.1.0"); err != nil {
		t.Fatalf("WriteFile on granted path: %v", err)
	}
	got, err := s.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile on granted path: %v", err)
	}
	if got != "openapi: 3.1.0" {
		t.Fatalf("ReadFile content = %q", got)
	}
}

func TestGrantIsPathCleaned(t *testing.T) {
	s, files := newTestService(t, &memRepo{})
	target := spec("spec.yaml")
	files.data[target] = []byte("openapi: 3.0.0")

	if got := s.OpenSelected(target); got != target {
		t.Fatalf("OpenSelected = %q, want %q", got, target)
	}
	// 非正規化パス (途中に ./ を含む) でも同一とみなされること。
	messy := "specs" + string(filepath.Separator) + "." + string(filepath.Separator) + "spec.yaml"
	if _, err := s.ReadFile(messy); err != nil {
		t.Fatalf("ReadFile should treat cleaned paths as equal: %v", err)
	}
}

func TestRecentsSeedGrantsOnStartup(t *testing.T) {
	target := spec("seeded.yaml")
	repo := &memRepo{items: []domain.OpenAPIRecent{
		{Path: target, Name: "seeded.yaml", Order: 0, LastOpenedAt: "2026-07-12T00:00:00Z"},
	}}
	s, files := newTestService(t, repo)
	files.data[target] = []byte("openapi: 3.0.0")

	if _, err := s.ReadFile(target); err != nil {
		t.Fatalf("seeded recents path should be granted: %v", err)
	}
	if got := s.GetRecents(); len(got) != 1 || got[0].Path != target {
		t.Fatalf("GetRecents = %+v", got)
	}
}

func TestGetRecentsReturnsNonNilWhenEmpty(t *testing.T) {
	s, _ := newTestService(t, &memRepo{})
	if got := s.GetRecents(); got == nil {
		t.Fatal("GetRecents must return a non-nil slice so that RPC returns [] instead of null")
	}
}

func TestRemoveRecentRevokesGrant(t *testing.T) {
	s, files := newTestService(t, &memRepo{})
	target := s.OpenSelected(spec("spec.yaml"))
	files.data[target] = []byte("openapi: 3.0.0")

	if err := s.RemoveRecent(target); err != nil {
		t.Fatalf("RemoveRecent: %v", err)
	}
	if _, err := s.ReadFile(target); !errors.Is(err, domain.ErrFileAccessDenied) {
		t.Fatalf("after RemoveRecent, ReadFile should be denied, got %v", err)
	}
	if got := s.GetRecents(); len(got) != 0 {
		t.Fatalf("GetRecents after remove = %+v", got)
	}
}

// newRecentsABC は a.yaml b.yaml c.yaml をこの順で登録したサービスを返す。
func newRecentsABC(t *testing.T) *FileService {
	t.Helper()
	s, _ := newTestService(t, &memRepo{})
	for _, name := range []string{"a.yaml", "b.yaml", "c.yaml"} {
		s.OpenSelected(spec(name))
	}
	return s
}

// assertRecentOrder は GetRecents の並びと Order の振り直しを検証する。
func assertRecentOrder(t *testing.T, s *FileService, want ...string) {
	t.Helper()
	got := s.GetRecents()
	if len(got) != len(want) {
		t.Fatalf("GetRecents len = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Path != spec(w) || got[i].Order != i {
			t.Fatalf("order[%d] = %+v, want path %s order %d", i, got[i], spec(w), i)
		}
	}
}

func TestMoveRecentReorders(t *testing.T) {
	s := newRecentsABC(t)
	// c を先頭へ。
	if err := s.MoveRecent(spec("c.yaml"), 0); err != nil {
		t.Fatalf("MoveRecent: %v", err)
	}
	assertRecentOrder(t, s, "c.yaml", "a.yaml", "b.yaml")
}

// index は対象を取り除いた後のスライスに対する位置なので、3 件から 1 件抜いた
// len == 2 がちょうど末尾を指す。
func TestMoveRecentIndexAtLenAppendsToEnd(t *testing.T) {
	s := newRecentsABC(t)
	if err := s.MoveRecent(spec("a.yaml"), 2); err != nil {
		t.Fatalf("MoveRecent: %v", err)
	}
	assertRecentOrder(t, s, "b.yaml", "c.yaml", "a.yaml")
}

func TestMoveRecentIndexBeyondLenAppendsToEnd(t *testing.T) {
	s := newRecentsABC(t)
	if err := s.MoveRecent(spec("a.yaml"), 99); err != nil {
		t.Fatalf("MoveRecent: %v", err)
	}
	assertRecentOrder(t, s, "b.yaml", "c.yaml", "a.yaml")
}

// 負の index は末尾に追加する。domain.InsertAt が定める挿入契約であり、
// insertAt / insertEntryAt 由来の 2 箇所と揃えるために先頭挿入から変更した。
func TestMoveRecentNegativeIndexAppendsToEnd(t *testing.T) {
	s := newRecentsABC(t)
	if err := s.MoveRecent(spec("a.yaml"), -1); err != nil {
		t.Fatalf("MoveRecent: %v", err)
	}
	assertRecentOrder(t, s, "b.yaml", "c.yaml", "a.yaml")
}

func TestOpenSelectedSamePathUpdatesLastOpenedAt(t *testing.T) {
	s, _ := newTestService(t, &memRepo{})
	s.OpenSelected(spec("a.yaml"))
	s.OpenSelected(spec("b.yaml"))
	first := s.GetRecents()[0].LastOpenedAt

	s.OpenSelected(spec("a.yaml"))

	got := s.GetRecents()
	if len(got) != 2 {
		t.Fatalf("GetRecents len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].Path != spec("a.yaml") || got[0].Order != 0 {
		t.Fatalf("re-opened entry should keep its position: %+v", got[0])
	}
	if got[0].LastOpenedAt <= first {
		t.Fatalf("LastOpenedAt = %q, want newer than %q", got[0].LastOpenedAt, first)
	}
}

func TestOpenSelectedTrimsToMaxRecents(t *testing.T) {
	s, _ := newTestService(t, &memRepo{})
	for i := range maxRecents {
		s.OpenSelected(spec(fmt.Sprintf("%02d.yaml", i)))
	}

	s.OpenSelected(spec("new.yaml"))

	got := s.GetRecents()
	if len(got) != maxRecents {
		t.Fatalf("GetRecents len = %d, want %d", len(got), maxRecents)
	}
	for i, it := range got {
		if it.Path == spec("00.yaml") {
			t.Fatalf("oldest entry should have been trimmed: %+v", it)
		}
		if it.Order != i {
			t.Fatalf("order[%d] = %d, want %d", i, it.Order, i)
		}
	}
	if !slices.ContainsFunc(got, func(it domain.OpenAPIRecent) bool { return it.Path == spec("new.yaml") }) {
		t.Fatal("newly opened entry should be kept")
	}
}

func TestNewFileService_CorruptQuarantinedThenSaves(t *testing.T) {
	repo := &memRepo{loadErr: fmt.Errorf("%w: bad json", domain.ErrRecentsCorrupt)}
	logger := &recordLogger{}
	s := NewFileService(repo, newMemFiles(), logger)

	if repo.quarantines != 1 {
		t.Fatalf("Quarantine calls = %d, want 1", repo.quarantines)
	}
	if logger.errorCount() == 0 {
		t.Fatal("quarantine should be logged")
	}
	if got := s.GetRecents(); len(got) != 0 {
		t.Fatalf("GetRecents = %+v, want empty", got)
	}

	target := s.OpenSelected(spec("a.yaml"))

	if repo.saveCount() != 1 {
		t.Fatalf("Save calls = %d, want 1", repo.saveCount())
	}
	if saved := repo.saves[0]; len(saved) != 1 || saved[0].Path != target {
		t.Fatalf("saved = %+v, want only %s", saved, target)
	}
}

func TestNewFileService_CorruptQuarantineFailsDoesNotSave(t *testing.T) {
	repo := &memRepo{
		loadErr:       fmt.Errorf("%w: bad json", domain.ErrRecentsCorrupt),
		quarantineErr: errors.New("rename failed"),
	}
	logger := &recordLogger{}
	s := NewFileService(repo, newMemFiles(), logger)
	if logger.errorCount() == 0 {
		t.Fatal("quarantine failure should be logged")
	}

	for _, name := range []string{"a.yaml", "b.yaml", "c.yaml"} {
		s.OpenSelected(spec(name))
	}
	if err := s.RemoveRecent(spec("b.yaml")); err != nil {
		t.Fatalf("RemoveRecent: %v", err)
	}
	if err := s.MoveRecent(spec("c.yaml"), 0); err != nil {
		t.Fatalf("MoveRecent: %v", err)
	}

	if repo.saveCount() != 0 {
		t.Fatalf("Save must not be called while the original file is kept (saves = %d)", repo.saveCount())
	}
	assertRecentOrder(t, s, "c.yaml", "a.yaml")
}

func TestNewFileService_ReadErrorDoesNotTouchFile(t *testing.T) {
	repo := &memRepo{loadErr: errors.New("permission denied")}
	logger := &recordLogger{}
	s := NewFileService(repo, newMemFiles(), logger)
	if logger.errorCount() == 0 {
		t.Fatal("read failure should be logged")
	}

	s.OpenSelected(spec("a.yaml"))
	s.OpenSelected(spec("b.yaml"))
	if err := s.MoveRecent(spec("b.yaml"), 0); err != nil {
		t.Fatalf("MoveRecent: %v", err)
	}
	if err := s.RemoveRecent(spec("a.yaml")); err != nil {
		t.Fatalf("RemoveRecent: %v", err)
	}

	if repo.quarantines != 0 {
		t.Fatalf("Quarantine must not be called on read errors (calls = %d)", repo.quarantines)
	}
	if repo.saveCount() != 0 {
		t.Fatalf("Save must not be called on read errors (saves = %d)", repo.saveCount())
	}
	assertRecentOrder(t, s, "b.yaml")
}

// 保存に失敗した操作はメモリにも反映しない (コピーオンライト)。
func TestRemoveAndMoveKeepStateOnSaveFailure(t *testing.T) {
	repo := &memRepo{}
	s, files := newTestService(t, repo)
	for _, name := range []string{"a.yaml", "b.yaml", "c.yaml"} {
		s.OpenSelected(spec(name))
	}
	files.data[spec("a.yaml")] = []byte("openapi: 3.0.0")
	before := s.GetRecents()
	repo.saveErr = errors.New("disk full")

	if err := s.RemoveRecent(spec("a.yaml")); err == nil {
		t.Fatal("RemoveRecent should return the save error")
	}
	if err := s.MoveRecent(spec("c.yaml"), 0); err == nil {
		t.Fatal("MoveRecent should return the save error")
	}

	if got := s.GetRecents(); !slices.Equal(got, before) {
		t.Fatalf("GetRecents changed after failed saves:\n got  %+v\n want %+v", got, before)
	}
	// 一覧に残っている以上、許可も残す。
	if _, err := s.ReadFile(spec("a.yaml")); err != nil {
		t.Fatalf("grant should be kept when RemoveRecent fails: %v", err)
	}
}

func TestOpenSelectedSucceedsWhenRecentsSaveFails(t *testing.T) {
	repo := &memRepo{saveErr: errors.New("disk full")}
	files := newMemFiles()
	logger := &recordLogger{}
	s := NewFileService(repo, files, logger)
	target := spec("a.yaml")
	files.data[target] = []byte("openapi: 3.0.0")

	if got := s.OpenSelected(target); got != target {
		t.Fatalf("OpenSelected = %q, want %q", got, target)
	}
	if _, err := s.ReadFile(target); err != nil {
		t.Fatalf("path should be granted even if recents save fails: %v", err)
	}
	if got := s.GetRecents(); len(got) != 0 {
		t.Fatalf("GetRecents = %+v, want empty (save failed)", got)
	}
	if logger.errorCount() == 0 {
		t.Fatal("recents save failure should be logged")
	}
}

func TestSaveSelectedWritesAndAddsRecent(t *testing.T) {
	s, files := newTestService(t, &memRepo{})
	target := spec("new.yaml")

	got, err := s.SaveSelected(target, "openapi: 3.1.0")
	if err != nil {
		t.Fatalf("SaveSelected: %v", err)
	}
	if got != target {
		t.Fatalf("SaveSelected = %q, want %q", got, target)
	}
	if string(files.data[target]) != "openapi: 3.1.0" {
		t.Fatalf("written content = %q", files.data[target])
	}
	assertRecentOrder(t, s, "new.yaml")
}

func TestSaveSelectedWriteFailureDoesNotAddRecent(t *testing.T) {
	s, files := newTestService(t, &memRepo{})
	files.writeErr = errors.New("disk full")

	if _, err := s.SaveSelected(spec("new.yaml"), "openapi: 3.1.0"); err == nil {
		t.Fatal("SaveSelected should return the write error")
	}
	if got := s.GetRecents(); len(got) != 0 {
		t.Fatalf("GetRecents = %+v, want empty", got)
	}
}

// ReadFile と recents の変更を並行実行しても競合しないこと (-race で検証する)。
func TestConcurrentReadAndRecentsUpdates(t *testing.T) {
	s, files := newTestService(t, &memRepo{})
	target := s.OpenSelected(spec("a.yaml"))
	files.data[target] = []byte("openapi: 3.0.0")

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_, _ = s.ReadFile(target) // 並行実行での競合検出が目的で、結果は見ない
		}()
		go func() {
			defer wg.Done()
			_ = s.RemoveRecent(spec(fmt.Sprintf("%d.yaml", i))) // memRepo の保存は失敗しない
		}()
		go func() {
			defer wg.Done()
			s.OpenSelected(spec(fmt.Sprintf("%d.yaml", i)))
		}()
	}
	wg.Wait()

	if _, err := s.ReadFile(target); err != nil {
		t.Fatalf("ReadFile after concurrent updates: %v", err)
	}
}
