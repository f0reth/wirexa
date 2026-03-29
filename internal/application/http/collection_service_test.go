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
	svc, err := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}})
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
	svc, err := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	if err := svc.DeleteCollection("nonexistent"); err == nil {
		t.Error("expected error, got nil")
	}
}

func newSvc(t *testing.T) *CollectionService {
	t.Helper()
	svc, err := NewCollectionService(&inMemoryRepo{collections: map[string]*domain.Collection{}})
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
	svc, err := NewCollectionService(repo)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	cols := svc.GetCollections()
	if len(cols) != 1 || cols[0].ID != "c1" {
		t.Errorf("expected pre-loaded collection, got %v", cols)
	}
}
