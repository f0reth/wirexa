package openapiinfra

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/openapi"
)

func newTestRepo(t *testing.T) (*RecentRepository, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "openapi-recents.json")
	return NewRecentRepository(path), path
}

func TestRecentRepository_Load_MissingFileReturnsEmpty(t *testing.T) {
	repo, _ := newTestRepo(t)

	items, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("Load = %#v, want empty non-nil slice", items)
	}
}

func TestRecentRepository_Load_CorruptReturnsErrCorruptData(t *testing.T) {
	repo, path := newTestRepo(t)
	if err := os.WriteFile(path, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Load(); !errors.Is(err, cmn.ErrCorruptData) {
		t.Fatalf("Load on corrupt file: want ErrCorruptData, got %v", err)
	}
}

// 読み込み自体の失敗 (ここではパスがディレクトリ) は破損と区別する。
func TestRecentRepository_Load_UnreadableIsNotCorrupt(t *testing.T) {
	repo, path := newTestRepo(t)
	if err := os.Mkdir(path, 0o750); err != nil {
		t.Fatal(err)
	}

	_, err := repo.Load()
	if err == nil {
		t.Fatal("Load on unreadable path should fail")
	}
	if errors.Is(err, cmn.ErrCorruptData) {
		t.Fatalf("unreadable file must not be reported as corrupt: %v", err)
	}
}

func TestRecentRepository_SaveLoadRoundTrip(t *testing.T) {
	repo, _ := newTestRepo(t)
	want := []domain.OpenAPIRecent{
		{Path: "/a.yaml", Name: "a.yaml", LastOpenedAt: "2026-09-23T00:00:00Z", Order: 0},
		{Path: "/b.yaml", Name: "b.yaml", LastOpenedAt: "2026-09-22T00:00:00Z", Order: 1},
	}
	if err := repo.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Load len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRecentRepository_Quarantine(t *testing.T) {
	repo, path := newTestRepo(t)
	original := []byte("{invalid json}")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	dest, err := repo.Quarantine()
	if err != nil {
		t.Fatalf("Quarantine: %v", err)
	}
	if dest != path+".corrupt" {
		t.Errorf("dest = %q, want %q", dest, path+".corrupt")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("original file should have been moved away")
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, original) {
		t.Errorf("quarantined content = %q, want %q", got, original)
	}

	items, err := repo.Load()
	if err != nil {
		t.Fatalf("Load after quarantine: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Load after quarantine = %+v, want empty", items)
	}
}
