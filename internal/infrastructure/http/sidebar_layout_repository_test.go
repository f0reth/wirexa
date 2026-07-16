package httpinfra

import (
	"os"
	"path/filepath"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

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

func TestSidebarLayoutRepository_LoadCorruptReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sidebar_layout.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	repo := NewSidebarLayoutRepository(path)

	if _, err := repo.Load(); err == nil {
		t.Fatalf("expected error for corrupt file")
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
