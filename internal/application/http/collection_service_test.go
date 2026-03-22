package httpapp

import (
	"sync"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

type inMemoryRepo struct {
	mu          sync.Mutex
	collections map[string]*domain.Collection
}

func (r *inMemoryRepo) Load() ([]domain.Collection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]domain.Collection, 0, len(r.collections))
	for _, c := range r.collections {
		result = append(result, *c)
	}
	return result, nil
}

func (r *inMemoryRepo) Save(c *domain.Collection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.collections[c.ID] = &cp
	return nil
}

func (r *inMemoryRepo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.collections, id)
	return nil
}

func TestCollectionService_Create(t *testing.T) {
	svc := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}})
	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	col, err := svc.CreateCollection("My API")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if col.Name != "My API" {
		t.Errorf("got name %q, want %q", col.Name, "My API")
	}
	if col.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCollectionService_DeleteNotFound(t *testing.T) {
	svc := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}})
	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := svc.DeleteCollection("nonexistent"); err == nil {
		t.Error("expected error, got nil")
	}
}
