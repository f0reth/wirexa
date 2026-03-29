package httpinfra

import (
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

func TestJSONFileRepository_SaveAndLoad(t *testing.T) {
	repo, _ := NewJSONFileRepository(t.TempDir())
	col := domain.Collection{ID: "c1", Name: "Test"}
	_ = repo.Save(&col)
	cols, _ := repo.Load()
	if len(cols) != 1 || cols[0].Name != "Test" {
		t.Fail()
	}
}

func TestJSONFileRepository_Delete(t *testing.T) {
	repo, _ := NewJSONFileRepository(t.TempDir())
	col := domain.Collection{ID: "c1", Name: "ToDelete", Items: []*domain.TreeItem{}}
	if err := repo.Save(&col); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete("c1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	cols, err := repo.Load()
	if err != nil {
		t.Fatalf("Load after delete: %v", err)
	}
	if len(cols) != 0 {
		t.Errorf("expected 0 collections after delete, got %d", len(cols))
	}
}

func TestJSONFileRepository_LoadMultiple(t *testing.T) {
	repo, _ := NewJSONFileRepository(t.TempDir())
	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		col := domain.Collection{ID: name, Name: name, Items: []*domain.TreeItem{}}
		if err := repo.Save(&col); err != nil {
			t.Fatalf("Save(%q): %v", name, err)
		}
	}

	cols, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cols) != 3 {
		t.Errorf("expected 3 collections, got %d", len(cols))
	}
}

func TestJSONFileRepository_SaveOverwrite(t *testing.T) {
	repo, _ := NewJSONFileRepository(t.TempDir())
	col := domain.Collection{ID: "c1", Name: "Original", Items: []*domain.TreeItem{}}
	repo.Save(&col)

	col.Name = "Updated"
	if err := repo.Save(&col); err != nil {
		t.Fatalf("Save updated: %v", err)
	}

	cols, _ := repo.Load()
	if len(cols) != 1 || cols[0].Name != "Updated" {
		t.Errorf("expected Updated, got %+v", cols)
	}
}

func TestJSONFileRepository_Delete_NonExistent(t *testing.T) {
	repo, _ := NewJSONFileRepository(t.TempDir())
	err := repo.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent collection, got nil")
	}
}

func TestJSONFileRepository_SavePreservesItems(t *testing.T) {
	repo, _ := NewJSONFileRepository(t.TempDir())
	req := &domain.HttpRequest{ID: "r1", Name: "GET /api", Method: "GET", URL: "http://example.com"}
	col := domain.Collection{
		ID:   "c1",
		Name: "WithItems",
		Items: []*domain.TreeItem{
			{Type: domain.ItemTypeRequest, ID: "r1", Name: "GET /api", Request: req, Children: []*domain.TreeItem{}},
		},
	}
	repo.Save(&col)

	cols, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("expected 1 collection, got %d", len(cols))
	}
	if len(cols[0].Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(cols[0].Items))
	}
	if cols[0].Items[0].Request.URL != "http://example.com" {
		t.Errorf("URL = %q, want http://example.com", cols[0].Items[0].Request.URL)
	}
}
