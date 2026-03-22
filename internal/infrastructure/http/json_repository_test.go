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
