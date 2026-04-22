package httpapp

import (
	"errors"
	"sync"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

const sidebarKindItem = "item"

type inMemoryRepo struct {
	mu          sync.Mutex
	collections map[string]*domain.Collection
}

type inMemoryLayoutRepo struct {
	mu     sync.Mutex
	layout []domain.SidebarEntry
}

func (r *inMemoryLayoutRepo) Load() ([]domain.SidebarEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.SidebarEntry{}, r.layout...), nil
}

func (r *inMemoryLayoutRepo) Save(layout []domain.SidebarEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.layout = append([]domain.SidebarEntry{}, layout...)
	return nil
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
	svc, err := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}}, &inMemoryLayoutRepo{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
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
	svc, err := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}}, &inMemoryLayoutRepo{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	if err := svc.DeleteCollection("nonexistent"); err == nil {
		t.Error("expected error, got nil")
	}
}

func newSvc(t *testing.T) *CollectionService {
	t.Helper()
	svc, err := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}}, &inMemoryLayoutRepo{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	return svc
}

func mustCreate(t *testing.T, svc *CollectionService, name string) domain.Collection {
	t.Helper()
	col, err := svc.CreateCollection(name)
	if err != nil {
		t.Fatalf("CreateCollection(%q): %v", name, err)
	}
	return col
}

func TestCollectionService_GetCollections_Empty(t *testing.T) {
	svc := newSvc(t)
	cols := svc.GetCollections()
	if len(cols) != 0 {
		t.Errorf("expected empty, got %d", len(cols))
	}
}

func TestCollectionService_GetCollections_SortedByName(t *testing.T) {
	svc := newSvc(t)
	mustCreate(t, svc, "Zebra")
	mustCreate(t, svc, "Apple")
	mustCreate(t, svc, "Mango")

	cols := svc.GetCollections()
	if len(cols) != 3 {
		t.Fatalf("expected 3, got %d", len(cols))
	}
	if cols[0].Name != "Apple" || cols[1].Name != "Mango" || cols[2].Name != "Zebra" {
		t.Errorf("not sorted: %v %v %v", cols[0].Name, cols[1].Name, cols[2].Name)
	}
}

func TestCollectionService_Delete_Success(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "ToDelete")
	if err := svc.DeleteCollection(col.ID); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
	if len(svc.GetCollections()) != 0 {
		t.Error("expected 0 collections after delete")
	}
}

func TestCollectionService_RenameCollection_Success(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "OldName")
	if err := svc.RenameCollection(col.ID, "NewName"); err != nil {
		t.Fatalf("RenameCollection: %v", err)
	}
	cols := svc.GetCollections()
	if len(cols) != 1 || cols[0].Name != "NewName" {
		t.Errorf("expected name NewName, got %v", cols)
	}
}

func TestCollectionService_RenameCollection_NotFound(t *testing.T) {
	svc := newSvc(t)
	err := svc.RenameCollection("nonexistent", "Name")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_AddFolder_ToRoot(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	item, err := svc.AddFolder(col.ID, "", "MyFolder")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}
	if item.Type != domain.ItemTypeFolder {
		t.Errorf("expected folder type, got %q", item.Type)
	}
	if item.Name != "MyFolder" {
		t.Errorf("expected name MyFolder, got %q", item.Name)
	}
	if item.ID == "" {
		t.Error("expected non-empty ID")
	}
	cols := svc.GetCollections()
	if len(cols[0].Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(cols[0].Items))
	}
}

func TestCollectionService_AddFolder_ToParentFolder(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	parent, _ := svc.AddFolder(col.ID, "", "Parent")

	child, err := svc.AddFolder(col.ID, parent.ID, "Child")
	if err != nil {
		t.Fatalf("AddFolder to parent: %v", err)
	}
	if child.Name != "Child" {
		t.Errorf("expected Child, got %q", child.Name)
	}
	// verify in state
	cols := svc.GetCollections()
	parentNode := cols[0].Items[0]
	if len(parentNode.Children) != 1 || parentNode.Children[0].ID != child.ID {
		t.Error("child not found under parent")
	}
}

func TestCollectionService_AddFolder_CollectionNotFound(t *testing.T) {
	svc := newSvc(t)
	_, err := svc.AddFolder("nonexistent", "", "Folder")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_AddFolder_ParentNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	_, err := svc.AddFolder(col.ID, "nonexistent-parent", "Folder")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_AddFolder_ParentIsRequest(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HttpRequest{ID: "r1", Name: "Req"}
	item, _ := svc.AddRequest(col.ID, "", req)

	_, err := svc.AddFolder(col.ID, item.ID, "Folder")
	if err == nil {
		t.Error("expected error when parent is a request, got nil")
	}
}

func TestCollectionService_AddRequest_ToRoot(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HttpRequest{ID: "req1", Name: "GET /api", Method: "GET", URL: "http://example.com"}
	item, err := svc.AddRequest(col.ID, "", req)
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	if item.Type != domain.ItemTypeRequest {
		t.Errorf("expected request type, got %q", item.Type)
	}
	if item.ID != "req1" {
		t.Errorf("expected ID req1, got %q", item.ID)
	}
}

func TestCollectionService_AddRequest_AutoGeneratesID(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HttpRequest{Name: "No ID"}
	item, err := svc.AddRequest(col.ID, "", req)
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	if item.ID == "" {
		t.Error("expected auto-generated ID, got empty")
	}
}

func TestCollectionService_AddRequest_CollectionNotFound(t *testing.T) {
	svc := newSvc(t)
	req := domain.HttpRequest{Name: "Req"}
	_, err := svc.AddRequest("nonexistent", "", req)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_AddRequest_ToFolder(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	folder, _ := svc.AddFolder(col.ID, "", "Folder")
	req := domain.HttpRequest{ID: "r1", Name: "Req"}

	item, err := svc.AddRequest(col.ID, folder.ID, req)
	if err != nil {
		t.Fatalf("AddRequest to folder: %v", err)
	}
	cols := svc.GetCollections()
	folderNode := cols[0].Items[0]
	if len(folderNode.Children) != 1 || folderNode.Children[0].ID != item.ID {
		t.Error("request not found under folder")
	}
}

func TestCollectionService_AddRequest_ParentNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HttpRequest{Name: "Req"}
	_, err := svc.AddRequest(col.ID, "nonexistent", req)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_UpdateRequest_Success(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HttpRequest{ID: "r1", Name: "Old", Method: "GET", URL: "http://old.com"}
	svc.AddRequest(col.ID, "", req)

	updated := domain.HttpRequest{ID: "r1", Name: "New", Method: "POST", URL: "http://new.com"}
	if err := svc.UpdateRequest(col.ID, updated); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}

	cols := svc.GetCollections()
	node := cols[0].Items[0]
	if node.Name != "New" {
		t.Errorf("node.Name = %q, want %q", node.Name, "New")
	}
	if node.Request.URL != "http://new.com" {
		t.Errorf("Request.URL = %q, want %q", node.Request.URL, "http://new.com")
	}
}

func TestCollectionService_UpdateRequest_CollectionNotFound(t *testing.T) {
	svc := newSvc(t)
	err := svc.UpdateRequest("nonexistent", domain.HttpRequest{ID: "r1"})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_UpdateRequest_RequestNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	err := svc.UpdateRequest(col.ID, domain.HttpRequest{ID: "nonexistent"})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_RenameItem_Folder(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	folder, _ := svc.AddFolder(col.ID, "", "OldFolder")

	if err := svc.RenameItem(col.ID, folder.ID, "NewFolder"); err != nil {
		t.Fatalf("RenameItem: %v", err)
	}
	cols := svc.GetCollections()
	if cols[0].Items[0].Name != "NewFolder" {
		t.Errorf("expected NewFolder, got %q", cols[0].Items[0].Name)
	}
}

func TestCollectionService_RenameItem_Request(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HttpRequest{ID: "r1", Name: "OldReq"}
	svc.AddRequest(col.ID, "", req)

	if err := svc.RenameItem(col.ID, "r1", "NewReq"); err != nil {
		t.Fatalf("RenameItem: %v", err)
	}
	cols := svc.GetCollections()
	node := cols[0].Items[0]
	if node.Name != "NewReq" {
		t.Errorf("node.Name = %q, want %q", node.Name, "NewReq")
	}
	if node.Request.Name != "NewReq" {
		t.Errorf("Request.Name = %q, want %q", node.Request.Name, "NewReq")
	}
}

func TestCollectionService_RenameItem_CollectionNotFound(t *testing.T) {
	svc := newSvc(t)
	if err := svc.RenameItem("nonexistent", "item", "name"); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_RenameItem_ItemNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	if err := svc.RenameItem(col.ID, "nonexistent", "name"); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_DeleteItem_Success(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HttpRequest{ID: "r1", Name: "Req"}
	svc.AddRequest(col.ID, "", req)

	if err := svc.DeleteItem(col.ID, "r1"); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	cols := svc.GetCollections()
	if len(cols[0].Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(cols[0].Items))
	}
}

func TestCollectionService_DeleteItem_CollectionNotFound(t *testing.T) {
	svc := newSvc(t)
	if err := svc.DeleteItem("nonexistent", "item"); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_DeleteItem_ItemNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	if err := svc.DeleteItem(col.ID, "nonexistent"); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_NewCollectionService_LoadsExisting(t *testing.T) {
	existing := &domain.Collection{ID: "c1", Name: "Existing", Items: []*domain.TreeItem{}}
	repo := &inMemoryRepo{collections: map[string]*domain.Collection{"c1": existing}}
	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	cols := svc.GetCollections()
	if len(cols) != 1 || cols[0].ID != "c1" {
		t.Errorf("expected pre-loaded collection, got %v", cols)
	}
}

// --- errorRepo helpers ---

type errorLoadRepo struct {
	inMemoryRepo
}

func (r *errorLoadRepo) Load() ([]domain.Collection, error) {
	return nil, errors.New("load error")
}

type countingSaveRepo struct {
	inMemoryRepo
	saveCount int
	failAfter int
}

func (r *countingSaveRepo) Save(c *domain.Collection) error {
	r.saveCount++
	if r.saveCount > r.failAfter {
		return errors.New("save error")
	}
	return r.inMemoryRepo.Save(c)
}

// --- NewCollectionService ---

func TestNewCollectionService_RepoLoadError(t *testing.T) {
	_, err := NewCollectionService(&errorLoadRepo{}, &inMemoryLayoutRepo{})
	if err == nil {
		t.Error("expected error from repo.Load, got nil")
	}
}

func TestNewCollectionService_OrderInitialization(t *testing.T) {
	// 全コレクションの Order がゼロの場合、名前順で振り直される。
	cols := map[string]*domain.Collection{
		"c1": {ID: "c1", Name: "Zebra", Items: []*domain.TreeItem{}, Order: 0},
		"c2": {ID: "c2", Name: "Apple", Items: []*domain.TreeItem{}, Order: 0},
		"c3": {ID: "c3", Name: "Mango", Items: []*domain.TreeItem{}, Order: 0},
	}
	svc, err := NewCollectionService(&inMemoryRepo{collections: cols}, &inMemoryLayoutRepo{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	result := svc.GetCollections()
	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}
	if result[0].Name != "Apple" || result[1].Name != "Mango" || result[2].Name != "Zebra" {
		t.Errorf("unexpected order: %v %v %v", result[0].Name, result[1].Name, result[2].Name)
	}
	if result[0].Order != 0 || result[1].Order != 1 || result[2].Order != 2 {
		t.Errorf("unexpected Order values: %d %d %d", result[0].Order, result[1].Order, result[2].Order)
	}
}

// --- GetRootItems ---

func TestCollectionService_GetRootItems_Empty(t *testing.T) {
	svc := newSvc(t)
	items := svc.GetRootItems()
	if len(items) != 0 {
		t.Errorf("expected empty, got %d", len(items))
	}
}

func TestCollectionService_GetRootItems_WithItems(t *testing.T) {
	svc := newSvc(t)
	req := domain.HttpRequest{ID: "r1", Name: "Req"}
	if _, err := svc.AddRequest(domain.RootCollectionID, "", req); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	items := svc.GetRootItems()
	if len(items) != 1 || items[0].ID != "r1" {
		t.Errorf("expected [r1], got %v", items)
	}
}

// --- CreateCollection error ---

func TestCollectionService_CreateCollection_RepoError(t *testing.T) {
	// root コレクション作成 (1回) の後に失敗させる。
	repo := &countingSaveRepo{
		inMemoryRepo: inMemoryRepo{collections: map[string]*domain.Collection{}},
		failAfter:    1,
	}
	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	_, err = svc.CreateCollection("ShouldFail")
	if err == nil {
		t.Error("expected error from repo.Save, got nil")
	}
}

// --- MoveCollection ---

func TestCollectionService_MoveCollection_NotFound(t *testing.T) {
	svc := newSvc(t)
	if err := svc.MoveCollection("nonexistent", 0); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_MoveCollection_ToPosition(t *testing.T) {
	svc := newSvc(t)
	mustCreate(t, svc, "Alpha")
	b := mustCreate(t, svc, "Beta")
	mustCreate(t, svc, "Gamma")

	// 初期順序は Alpha(0), Beta(1), Gamma(2)。Beta を 0 に移動。
	if err := svc.MoveCollection(b.ID, 0); err != nil {
		t.Fatalf("MoveCollection: %v", err)
	}
	cols := svc.GetCollections()
	if cols[0].Name != "Beta" {
		t.Errorf("expected Beta first, got %v", cols[0].Name)
	}
}

func TestCollectionService_MoveCollection_ToEnd(t *testing.T) {
	svc := newSvc(t)
	a := mustCreate(t, svc, "Alpha")
	mustCreate(t, svc, "Beta")
	mustCreate(t, svc, "Gamma")

	// position が範囲外 → 末尾へ。
	if err := svc.MoveCollection(a.ID, 99); err != nil {
		t.Fatalf("MoveCollection: %v", err)
	}
	cols := svc.GetCollections()
	if cols[len(cols)-1].Name != "Alpha" {
		t.Errorf("expected Alpha last, got %v", cols[len(cols)-1].Name)
	}
}

// --- MoveItem ---

func TestCollectionService_MoveItem_SourceNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	err := svc.MoveItem("nonexistent", "item", col.ID, "", 0)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_MoveItem_TargetNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	err := svc.MoveItem(col.ID, "item", "nonexistent", "", 0)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_MoveItem_ItemNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	err := svc.MoveItem(col.ID, "nonexistent", col.ID, "", 0)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_MoveItem_TargetParentNotFolder(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	r1, _ := svc.AddRequest(col.ID, "", domain.HttpRequest{ID: "r1", Name: "R1"})
	r2, _ := svc.AddRequest(col.ID, "", domain.HttpRequest{ID: "r2", Name: "R2"})

	// r1 をターゲット親として移動しようとする（r1 はリクエストなのでエラー）。
	err := svc.MoveItem(col.ID, r2.ID, col.ID, r1.ID, 0)
	if err == nil {
		t.Error("expected error when target parent is not folder, got nil")
	}
}

func TestCollectionService_MoveItem_SameCollection(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	svc.AddRequest(col.ID, "", domain.HttpRequest{ID: "r1", Name: "R1"})
	svc.AddRequest(col.ID, "", domain.HttpRequest{ID: "r2", Name: "R2"})

	// r2 (index 1) を position 0 に移動 → [r2, r1]
	if err := svc.MoveItem(col.ID, "r2", col.ID, "", 0); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	cols := svc.GetCollections()
	items := cols[0].Items
	if len(items) != 2 || items[0].ID != "r2" || items[1].ID != "r1" {
		t.Errorf("unexpected order: %v %v", items[0].ID, items[1].ID)
	}
}

func TestCollectionService_MoveItem_AcrossCollections(t *testing.T) {
	svc := newSvc(t)
	col1 := mustCreate(t, svc, "Col1")
	col2 := mustCreate(t, svc, "Col2")
	svc.AddRequest(col1.ID, "", domain.HttpRequest{ID: "r1", Name: "R1"})

	if err := svc.MoveItem(col1.ID, "r1", col2.ID, "", 0); err != nil {
		t.Fatalf("MoveItem across collections: %v", err)
	}
	cols := svc.GetCollections()
	col1State := cols[0]
	col2State := cols[1]
	if col1State.Name == "Col2" {
		col1State, col2State = col2State, col1State
	}
	if len(col1State.Items) != 0 {
		t.Errorf("col1 should be empty, got %d items", len(col1State.Items))
	}
	if len(col2State.Items) != 1 || col2State.Items[0].ID != "r1" {
		t.Errorf("col2 should have r1, got %v", col2State.Items)
	}
}

func TestCollectionService_MoveItem_ToFolder(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	folder, _ := svc.AddFolder(col.ID, "", "Folder")
	svc.AddRequest(col.ID, "", domain.HttpRequest{ID: "r1", Name: "R1"})

	if err := svc.MoveItem(col.ID, "r1", col.ID, folder.ID, 0); err != nil {
		t.Fatalf("MoveItem to folder: %v", err)
	}
	cols := svc.GetCollections()
	folderNode := cols[0].Items[0]
	if len(folderNode.Children) != 1 || folderNode.Children[0].ID != "r1" {
		t.Errorf("expected r1 under folder, got %v", folderNode.Children)
	}
}

// --- GetSidebarLayout ---

func TestCollectionService_GetSidebarLayout_ExistingLayout(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{
		layout: []domain.SidebarEntry{
			{Kind: "collection", ID: "c1"},
			{Kind: "collection", ID: "c2"},
		},
	}
	svc, err := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}}, layoutRepo)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	if len(layout) != 2 || layout[0].ID != "c1" || layout[1].ID != "c2" {
		t.Errorf("unexpected layout: %v", layout)
	}
}

func TestCollectionService_GetSidebarLayout_FirstCall(t *testing.T) {
	// Order が付いた複数コレクションを持つ状態で初回 GetSidebarLayout を呼ぶと
	// Order 順にコレクションがレイアウトに並ぶことを確認する。
	cols := map[string]*domain.Collection{
		"c1": {ID: "c1", Name: "B", Items: []*domain.TreeItem{}, Order: 1},
		"c2": {ID: "c2", Name: "A", Items: []*domain.TreeItem{}, Order: 0},
	}
	svc, err := NewCollectionService(&inMemoryRepo{collections: cols}, &inMemoryLayoutRepo{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}

	collectionEntries := make([]domain.SidebarEntry, 0)
	for _, e := range layout {
		if e.Kind == "collection" {
			collectionEntries = append(collectionEntries, e)
		}
	}
	if len(collectionEntries) < 2 {
		t.Fatalf("expected at least 2 collection entries, got %d", len(collectionEntries))
	}
	if collectionEntries[0].ID != "c2" || collectionEntries[1].ID != "c1" {
		t.Errorf("expected c2(Order=0) before c1(Order=1), got %v", collectionEntries)
	}
}

// --- MoveSidebarEntry ---

func TestCollectionService_MoveSidebarEntry_NotFound(t *testing.T) {
	svc := newSvc(t)
	// 空のレイアウトに対して存在しない ID を指定するとエラー。
	err := svc.MoveSidebarEntry("collection", "nonexistent", 0)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_MoveSidebarEntry_Success(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{
		layout: []domain.SidebarEntry{
			{Kind: "collection", ID: "c1"},
			{Kind: "collection", ID: "c2"},
			{Kind: "collection", ID: "c3"},
		},
	}
	svc, err := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}}, layoutRepo)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	// c3 を position 0 に移動 → [c3, c1, c2]
	if err := svc.MoveSidebarEntry("collection", "c3", 0); err != nil {
		t.Fatalf("MoveSidebarEntry: %v", err)
	}
	layout, _ := svc.GetSidebarLayout()
	if layout[0].ID != "c3" || layout[1].ID != "c1" || layout[2].ID != "c2" {
		t.Errorf("unexpected layout after move: %v", layout)
	}
}

// --- MoveItemToSidebar ---

func TestCollectionService_MoveItemToSidebar_Success(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	svc.AddRequest(col.ID, "", domain.HttpRequest{ID: "r1", Name: "R1"})

	if err := svc.MoveItemToSidebar(col.ID, "r1", 0); err != nil {
		t.Fatalf("MoveItemToSidebar: %v", err)
	}

	// r1 は col から取り除かれ __root__ に追加される。
	cols := svc.GetCollections()
	if len(cols[0].Items) != 0 {
		t.Errorf("col should be empty after MoveItemToSidebar, got %d", len(cols[0].Items))
	}
	rootItems := svc.GetRootItems()
	if len(rootItems) != 1 || rootItems[0].ID != "r1" {
		t.Errorf("expected r1 in root, got %v", rootItems)
	}

	// サイドバーレイアウトにアイテムが含まれていること。
	layout, _ := svc.GetSidebarLayout()
	found := false
	for _, e := range layout {
		if e.Kind == sidebarKindItem && e.ID == "r1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("r1 not found in sidebar layout after MoveItemToSidebar")
	}
}

// --- AddFolder/AddRequest to RootCollection updates layout ---

func TestCollectionService_AddFolder_ToRootCollection_UpdatesLayout(t *testing.T) {
	svc := newSvc(t)
	item, err := svc.AddFolder(domain.RootCollectionID, "", "Folder")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}
	layout, _ := svc.GetSidebarLayout()
	found := false
	for _, e := range layout {
		if e.Kind == sidebarKindItem && e.ID == item.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sidebar layout does not contain item %q", item.ID)
	}
}

func TestCollectionService_AddRequest_ToRootCollection_UpdatesLayout(t *testing.T) {
	svc := newSvc(t)
	item, err := svc.AddRequest(domain.RootCollectionID, "", domain.HttpRequest{ID: "r1", Name: "R1"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	layout, _ := svc.GetSidebarLayout()
	found := false
	for _, e := range layout {
		if e.Kind == sidebarKindItem && e.ID == item.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sidebar layout does not contain item %q", item.ID)
	}
}

// --- DeleteItem from RootCollection updates layout ---

func TestCollectionService_DeleteItem_FromRootCollection_UpdatesLayout(t *testing.T) {
	// Pre-populate root collection with an item and layout with the corresponding entry.
	root := &domain.Collection{
		ID:   domain.RootCollectionID,
		Name: domain.RootCollectionID,
		Items: []*domain.TreeItem{
			{Type: domain.ItemTypeRequest, ID: "r1", Name: "R1", Children: []*domain.TreeItem{}},
		},
	}
	layoutRepo := &inMemoryLayoutRepo{
		layout: []domain.SidebarEntry{{Kind: sidebarKindItem, ID: "r1"}},
	}
	repo := &inMemoryRepo{collections: map[string]*domain.Collection{domain.RootCollectionID: root}}
	svc, err := NewCollectionService(repo, layoutRepo)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	if err := svc.DeleteItem(domain.RootCollectionID, "r1"); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	layout, _ := svc.GetSidebarLayout()
	for _, e := range layout {
		if e.Kind == sidebarKindItem && e.ID == "r1" {
			t.Error("r1 should be removed from layout after DeleteItem")
		}
	}
}

// --- Concurrent read/write ---

func TestCollectionService_ConcurrentReadWrite(t *testing.T) {
	// go test -race でデータ競合が検出されないことを確認する。
	svc := newSvc(t)
	mustCreate(t, svc, "Init")

	const goroutines = 10
	done := make(chan struct{}, goroutines*2)

	for range goroutines {
		go func() {
			svc.GetCollections()
			done <- struct{}{}
		}()
		go func() {
			svc.CreateCollection("concurrent")
			done <- struct{}{}
		}()
	}
	for range goroutines * 2 {
		<-done
	}
}
