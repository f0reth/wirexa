package httpinfra

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// 全フィールドを埋めたレイアウトを保存・復元しても値を失わない。検査対象は型から決まるので、
// domain 型にフィールドを足して永続化 DTO や変換関数への追加を忘れるとここで検出される。
func TestSidebarLayoutRepository_RoundTripKeepsEveryField(t *testing.T) {
	var want []domain.SidebarEntry
	testutil.Populate(t, &want)

	repo := NewSidebarLayoutRepository(filepath.Join(t.TempDir(), "sidebar_layout.json"))
	if err := repo.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// 既存の保存形式 (golden) を読めて、書き戻すと同じ JSON になる。
func TestSidebarLayoutRepository_GoldenFormat(t *testing.T) {
	golden := testutil.ReadGolden(t, filepath.Join("testdata", "sidebar_layout.golden.json"))
	path := filepath.Join(t.TempDir(), "sidebar_layout.json")
	if err := os.WriteFile(path, golden, 0o600); err != nil {
		t.Fatalf("write golden: %v", err)
	}
	repo := NewSidebarLayoutRepository(path)

	loaded, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []domain.SidebarEntry{{Kind: "collection", ID: "col-1"}, {Kind: "item", ID: "req-1"}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("golden load mismatch:\n got %+v\nwant %+v", loaded, want)
	}

	if err = repo.Save(loaded); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	testutil.AssertJSONEqual(t, saved, golden)
}

// 永続化 DTO はどの階層でも domain 型を埋め込まない。
func TestSidebarLayoutRepository_StoredDTOHasNoDomainTypes(t *testing.T) {
	testutil.AssertNoTypesFrom(t, []storedSidebarEntry{}, testutil.DomainPkg)
}

func TestSidebarLayoutRepository_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sidebar_layout.json")
	repo := NewSidebarLayoutRepository(path)
	want := []domain.SidebarEntry{
		{Kind: "collection", ID: "col-1"},
		{Kind: "item", ID: "req-1"},
	}

	if err := repo.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Load len = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// ファイル未作成でもエラーにせず空を返す。初回起動時にこの経路を通る。
func TestSidebarLayoutRepository_LoadMissingReturnsEmpty(t *testing.T) {
	repo := NewSidebarLayoutRepository(filepath.Join(t.TempDir(), "absent.json"))

	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load on missing file = %+v, want empty", got)
	}
}

func TestSidebarLayoutRepository_LoadCorruptReturnsErrCorruptData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sidebar_layout.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	repo := NewSidebarLayoutRepository(path)

	if _, err := repo.Load(); !errors.Is(err, cmn.ErrCorruptData) {
		t.Fatalf("Load on corrupt file: want ErrCorruptData, got %v", err)
	}
}

// 読み込み自体の失敗 (ここではパスがディレクトリ) は破損と区別する。
func TestSidebarLayoutRepository_LoadUnreadableIsNotCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sidebar_layout.json")
	if err := os.Mkdir(path, 0o750); err != nil {
		t.Fatal(err)
	}
	repo := NewSidebarLayoutRepository(path)

	_, err := repo.Load()
	if err == nil {
		t.Fatal("Load on unreadable path should fail")
	}
	if errors.Is(err, cmn.ErrCorruptData) {
		t.Fatalf("unreadable file must not be reported as corrupt: %v", err)
	}
}

func TestSidebarLayoutRepository_QuarantineMovesCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sidebar_layout.json")
	corrupt := []byte("{ not json")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	repo := NewSidebarLayoutRepository(path)

	dest, err := repo.Quarantine()
	if err != nil {
		t.Fatalf("Quarantine: %v", err)
	}
	if dest != path+".corrupt" {
		t.Errorf("Quarantine dest = %q, want %q", dest, path+".corrupt")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read quarantined file: %v", err)
	}
	if !bytes.Equal(got, corrupt) {
		t.Errorf("quarantined content = %q, want %q", got, corrupt)
	}

	layout, err := repo.Load()
	if err != nil {
		t.Fatalf("Load after Quarantine: %v", err)
	}
	if layout == nil || len(layout) != 0 {
		t.Fatalf("Load after Quarantine = %#v, want empty non-nil slice", layout)
	}
}

func TestSidebarLayoutRepository_LoadNullReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sidebar_layout.json")
	if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := NewSidebarLayoutRepository(path)

	layout, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if layout == nil || len(layout) != 0 {
		t.Fatalf("Load on null = %#v, want empty non-nil slice", layout)
	}
}

func TestSidebarLayoutRepository_SaveOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sidebar_layout.json")
	repo := NewSidebarLayoutRepository(path)

	if err := repo.Save([]domain.SidebarEntry{{Kind: "collection", ID: "old-1"}, {Kind: "item", ID: "old-2"}}); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := repo.Save([]domain.SidebarEntry{{Kind: "item", ID: "new"}}); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("Load after overwrite = %+v, want single entry new", got)
	}
}
