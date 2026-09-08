package store

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// item はテスト用の最小ドメイン型。
type item struct {
	ID   string
	Name string
}

// fakeRepo は Repository[item] のインメモリモック。
type fakeRepo struct {
	items   map[string]item
	loadErr error
	saveErr error
	delErr  error
	saved   []item // repo.Save に渡ったアイテムの記録
	delIDs  []string
}

func newFakeRepo(items ...item) *fakeRepo {
	r := &fakeRepo{items: make(map[string]item)}
	for _, it := range items {
		r.items[it.ID] = it
	}
	return r
}

func (r *fakeRepo) Load() ([]item, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	result := make([]item, 0, len(r.items))
	for _, it := range r.items {
		result = append(result, it)
	}
	return result, nil
}

func (r *fakeRepo) Save(it *item) error {
	r.saved = append(r.saved, *it)
	if r.saveErr != nil {
		return r.saveErr
	}
	r.items[it.ID] = *it
	return nil
}

func (r *fakeRepo) Delete(id string) error {
	r.delIDs = append(r.delIDs, id)
	if r.delErr != nil {
		return r.delErr
	}
	delete(r.items, id)
	return nil
}

func newStore(t *testing.T, repo *fakeRepo) *CachedStore[item] {
	t.Helper()
	cs, err := NewCachedStore[item](
		"item", repo,
		func(it item) string { return it.ID },
		func(it *item, id string) { it.ID = id },
	)
	if err != nil {
		t.Fatalf("NewCachedStore: %v", err)
	}
	return cs
}

func TestCachedStore_New_LoadsItems(t *testing.T) {
	cs := newStore(t, newFakeRepo(item{ID: "a", Name: "A"}, item{ID: "b", Name: "B"}))
	if got := cs.GetAll(); len(got) != 2 {
		t.Errorf("expected 2 items, got %d", len(got))
	}
}

func TestCachedStore_New_LoadError(t *testing.T) {
	repo := newFakeRepo()
	repo.loadErr = errors.New("disk error")
	_, err := NewCachedStore[item](
		"item", repo,
		func(it item) string { return it.ID },
		func(it *item, id string) { it.ID = id },
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCachedStore_GetAll_ReturnsCopy(t *testing.T) {
	cs := newStore(t, newFakeRepo(item{ID: "a", Name: "Original"}))
	got := cs.GetAll()
	got[0].Name = "Modified"
	if again := cs.GetAll(); again[0].Name != "Original" {
		t.Error("GetAll should return a copy, not a reference")
	}
}

func TestCachedStore_Save_GeneratesIDWhenEmpty(t *testing.T) {
	cs := newStore(t, newFakeRepo())
	saved, err := cs.Save(item{Name: "NoID"})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ID == "" {
		t.Error("expected auto-generated ID, got empty")
	}
	if len(cs.GetAll()) != 1 {
		t.Error("expected 1 item after save")
	}
}

func TestCachedStore_Save_KeepsProvidedID(t *testing.T) {
	cs := newStore(t, newFakeRepo())
	saved, err := cs.Save(item{ID: "x", Name: "WithID"})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ID != "x" {
		t.Errorf("ID = %q, want x", saved.ID)
	}
}

func TestCachedStore_Save_RepoError(t *testing.T) {
	repo := newFakeRepo()
	repo.saveErr = errors.New("write error")
	cs := newStore(t, repo)
	if _, err := cs.Save(item{ID: "x"}); err == nil {
		t.Fatal("expected error, got nil")
	}
	// repo.Save が失敗したらキャッシュは更新されない
	if len(cs.GetAll()) != 0 {
		t.Error("cache should not be updated on repo save error")
	}
}

func TestCachedStore_Delete_Success(t *testing.T) {
	cs := newStore(t, newFakeRepo(item{ID: "a", Name: "A"}))
	if err := cs.Delete("a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(cs.GetAll()) != 0 {
		t.Error("expected 0 items after delete")
	}
}

func TestCachedStore_Delete_NotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.delErr = errors.New("should not be called")
	cs := newStore(t, repo)

	err := cs.Delete("missing")
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
	// 存在確認で弾かれるので repo.Delete は呼ばれない
	if len(repo.delIDs) != 0 {
		t.Errorf("repo.Delete should not be called, got %v", repo.delIDs)
	}
}

func TestCachedStore_Delete_RepoError(t *testing.T) {
	repo := newFakeRepo(item{ID: "a", Name: "A"})
	repo.delErr = errors.New("delete error")
	cs := newStore(t, repo)

	err := cs.Delete("a")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); ok {
		t.Error("expected repo error, got NotFoundError")
	}
	// repo エラー時はキャッシュに残る
	if len(cs.GetAll()) != 1 {
		t.Error("cache should not be updated on repo delete error")
	}
}
