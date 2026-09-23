package httpapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"sync"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

const (
	testColNameMyAPI = "My API"
	testColNameApple = "Apple"
	testColNameMango = "Mango"
	testColNameZebra = "Zebra"
	testReqName      = "Req"
)

// errFakeSave はフェイクリポジトリが注入する保存エラー。
var errFakeSave = errors.New("save error")

// inMemoryRepo はコレクションリポジトリのフェイク。
// failSaveFrom / failDelete に ID を登録すると、その ID の書き込みだけを失敗させられる。
// saves は Save が成功した順にコレクション ID を記録し、書き込み順序の検証に使う。
// deletes は Delete が成功した順にコレクション ID を記録する。
// 保存・読み出しでディープコピーを取るのは、実ファイルと同じく呼び出し側の
// オブジェクトと保存済みの内容が共有されないようにするため。
type inMemoryRepo struct {
	collections map[string]*domain.Collection
	// failSaveFrom は ID ごとに「何回目の Save から失敗させるか」(1 始まり) を持つ。
	// 2 以上を入れると最初の書き込みだけ成功させられるため、
	// 「本体の書き込みは通ったが巻き戻しの書き戻しが失敗する」状況を作れる。
	failSaveFrom map[string]int
	saveCounts   map[string]int
	failDelete   map[string]error
	saves        []string
	deletes      []string
	// failAllSavesFrom は ID を問わず n 回目以降の Save を失敗させる。0 なら無効。
	// 採番される ID を事前に知れない新規作成の失敗注入に使う。
	failAllSavesFrom int
	saveCount        int
	// existing は Load では返さないがファイルとしては残っている ID。
	// 読み込みで読み飛ばされたファイルを模す。
	existing map[string]bool
	// existsErr は Exists が返すエラー。有無を確認できない状況を模す。
	existsErr error
	mu        sync.Mutex
}

// newFakeRepo は cols を保存済みとして持つ inMemoryRepo を生成する。
func newFakeRepo(cols ...*domain.Collection) *inMemoryRepo {
	r := &inMemoryRepo{
		collections:  make(map[string]*domain.Collection, len(cols)),
		failSaveFrom: map[string]int{},
		saveCounts:   map[string]int{},
		failDelete:   map[string]error{},
	}
	for _, c := range cols {
		r.collections[c.ID] = c.Clone()
	}
	return r
}

// failSaveFromNth は id への n 回目以降の Save を失敗させる。
func (r *inMemoryRepo) failSaveFromNth(id string, n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failSaveFrom[id] = n
}

// failSaveAlways は id への Save を常に失敗させる。
func (r *inMemoryRepo) failSaveAlways(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failSaveFrom[id] = 1
}

// failAllSavesFromNth は ID を問わず n 回目以降の Save を失敗させる。
func (r *inMemoryRepo) failAllSavesFromNth(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failAllSavesFrom = n
}

// snapshot は保存済みコレクションの写しを返す。存在しない場合は nil。
func (r *inMemoryRepo) snapshot(id string) *domain.Collection {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.collections[id].Clone()
}

// saveOrder は Save が成功した順に並んだコレクション ID を返す。
func (r *inMemoryRepo) saveOrder() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.saves...)
}

// deleteOrder は Delete が成功した順に並んだコレクション ID を返す。
func (r *inMemoryRepo) deleteOrder() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.deletes...)
}

// inMemoryLayoutRepo はサイドバーレイアウトリポジトリのフェイク。
// Quarantine は成功すると保存済みレイアウトと loadErr を消し、ファイルが退避された後の状態を模す。
// Save が成功した場合も loadErr を消す。
type inMemoryLayoutRepo struct {
	loadErr       error
	saveErr       error
	quarantineErr error
	layout        []domain.SidebarEntry
	quarantines   int
	mu            sync.Mutex
}

func (r *inMemoryLayoutRepo) Quarantine() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.quarantines++
	if r.quarantineErr != nil {
		return "", r.quarantineErr
	}
	r.loadErr = nil
	r.layout = nil
	return "sidebar_layout.json.corrupt", nil
}

// quarantineCount は Quarantine が呼ばれた回数を返す。
func (r *inMemoryLayoutRepo) quarantineCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.quarantines
}

func (r *inMemoryLayoutRepo) Load() ([]domain.SidebarEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	return append([]domain.SidebarEntry{}, r.layout...), nil
}

func (r *inMemoryLayoutRepo) Save(layout []domain.SidebarEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return r.saveErr
	}
	r.layout = append([]domain.SidebarEntry{}, layout...)
	// 書き込めたファイルは次から正常に読める (壊れたファイルを上書きした後の状態)。
	r.loadErr = nil
	return nil
}

// snapshot は保存済みレイアウトの写しを返す（ディスク上の内容の検証用）。
func (r *inMemoryLayoutRepo) snapshot() []domain.SidebarEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.SidebarEntry{}, r.layout...)
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
	r.saveCount++
	r.saveCounts[c.ID]++
	if r.failAllSavesFrom > 0 && r.saveCount >= r.failAllSavesFrom {
		return errFakeSave
	}
	if n, ok := r.failSaveFrom[c.ID]; ok && r.saveCounts[c.ID] >= n {
		return errFakeSave
	}
	r.saves = append(r.saves, c.ID)
	r.collections[c.ID] = c.Clone()
	return nil
}

func (r *inMemoryRepo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failDelete[id]; err != nil {
		return err
	}
	r.deletes = append(r.deletes, id)
	delete(r.collections, id)
	return nil
}

func (r *inMemoryRepo) Exists(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.existsErr != nil {
		return false, r.existsErr
	}
	_, ok := r.collections[id]
	return ok || r.existing[id], nil
}

func TestCollectionService_Create(t *testing.T) {
	svc, err := NewCollectionService(newFakeRepo(), &inMemoryLayoutRepo{}, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	col, err := svc.CreateCollection(testColNameMyAPI)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if col.Name != testColNameMyAPI {
		t.Errorf("got name %q, want %q", col.Name, testColNameMyAPI)
	}
	if col.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCollectionService_DeleteNotFound(t *testing.T) {
	svc, err := NewCollectionService(newFakeRepo(), &inMemoryLayoutRepo{}, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	if err := svc.DeleteCollection("nonexistent"); err == nil {
		t.Error("expected error, got nil")
	}
}

func newSvc(t *testing.T) *CollectionService {
	t.Helper()
	svc, err := NewCollectionService(newFakeRepo(), &inMemoryLayoutRepo{}, nil)
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

// findColItem はキャッシュ上のコレクション内からアイテムを ID で探す。
func findColItem(t *testing.T, svc *CollectionService, collectionID, itemID string) (*domain.TreeItem, *domain.TreeItem, bool) {
	t.Helper()
	svc.mu.RLock()
	c, ok := svc.cache[collectionID]
	svc.mu.RUnlock()
	if !ok {
		t.Fatalf("collection %q not found", collectionID)
	}
	return c.FindNode(itemID)
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
	mustCreate(t, svc, testColNameZebra)
	mustCreate(t, svc, testColNameApple)
	mustCreate(t, svc, testColNameMango)

	cols := svc.GetCollections()
	if len(cols) != 3 {
		t.Fatalf("expected 3, got %d", len(cols))
	}
	if cols[0].Name != testColNameApple || cols[1].Name != testColNameMango || cols[2].Name != testColNameZebra {
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
	req := domain.HTTPRequest{ID: "r1", Name: testReqName}
	item, _ := svc.AddRequest(col.ID, "", req)

	_, err := svc.AddFolder(col.ID, item.ID, "Folder")
	if err == nil {
		t.Error("expected error when parent is a request, got nil")
	}
}

func TestCollectionService_AddRequest_ToRoot(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HTTPRequest{ID: "req1", Name: "GET /api", Method: "GET", URL: "http://example.com"}
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
	req := domain.HTTPRequest{Name: "No ID"}
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
	req := domain.HTTPRequest{Name: testReqName}
	_, err := svc.AddRequest("nonexistent", "", req)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_AddRequest_ToFolder(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	folder, _ := svc.AddFolder(col.ID, "", "Folder")
	req := domain.HTTPRequest{ID: "r1", Name: testReqName}

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
	req := domain.HTTPRequest{Name: testReqName}
	_, err := svc.AddRequest(col.ID, "nonexistent", req)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_UpdateRequest_Success(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HTTPRequest{ID: "r1", Name: "Old", Method: "GET", URL: "http://old.com"}
	svc.AddRequest(col.ID, "", req)

	updated := domain.HTTPRequest{ID: "r1", Name: "New", Method: "POST", URL: "http://new.com"}
	if err := svc.UpdateRequest(col.ID, updated); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}

	cols := svc.GetCollections()
	node := cols[0].Items[0]
	if node.Name != "Old" {
		t.Errorf("node.Name = %q, want %q (UpdateRequest must not change name)", node.Name, "Old")
	}
	if node.Request.URL != "http://new.com" {
		t.Errorf("Request.URL = %q, want %q", node.Request.URL, "http://new.com")
	}
	if node.Request.Method != http.MethodPost {
		t.Errorf("Request.Method = %q, want %q", node.Request.Method, "POST")
	}
}

func TestCollectionService_UpdateRequest_PreservesNameWhenEmpty(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	req := domain.HTTPRequest{ID: "r1", Name: testColNameMyAPI, Method: "GET", URL: "http://old.com"}
	svc.AddRequest(col.ID, "", req)

	updated := domain.HTTPRequest{ID: "r1", Name: "", Method: "POST", URL: "http://new.com"}
	if err := svc.UpdateRequest(col.ID, updated); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}

	cols := svc.GetCollections()
	node := cols[0].Items[0]
	if node.Name != testColNameMyAPI {
		t.Errorf("node.Name = %q, want %q (empty name must be ignored)", node.Name, testColNameMyAPI)
	}
}

func TestCollectionService_UpdateRequest_CollectionNotFound(t *testing.T) {
	svc := newSvc(t)
	err := svc.UpdateRequest("nonexistent", domain.HTTPRequest{ID: "r1"})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_UpdateRequest_RequestNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	err := svc.UpdateRequest(col.ID, domain.HTTPRequest{ID: "nonexistent"})
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
	req := domain.HTTPRequest{ID: "r1", Name: "OldReq"}
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
	req := domain.HTTPRequest{ID: "r1", Name: testReqName}
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
	repo := newFakeRepo(existing)
	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, nil)
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

// --- NewCollectionService ---

func TestNewCollectionService_RepoLoadError(t *testing.T) {
	_, err := NewCollectionService(&errorLoadRepo{}, &inMemoryLayoutRepo{}, nil)
	if err == nil {
		t.Error("expected error from repo.Load, got nil")
	}
}

// --- NewCollectionService: __root__ の自動作成 ---

func TestNewCollectionService_CreatesRootWhenFileMissing(t *testing.T) {
	repo := newFakeRepo()
	if _, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, nil); err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	if repo.snapshot(domain.RootCollectionID) == nil {
		t.Fatal("root collection should be created and saved when its file does not exist")
	}
}

// newSvcWithUnloadableRoot は __root__ のファイルが残っているのに読み込めなかった状態で起動する。
// setup でフェイクに「ファイルは存在する」または「有無を確認できない」状態を仕込む。
func newSvcWithUnloadableRoot(t *testing.T, setup func(*inMemoryRepo)) (*CollectionService, *inMemoryRepo, *recordingLogger) {
	t.Helper()
	repo := newFakeRepo(&domain.Collection{ID: "c1", Name: "C1", Items: []*domain.TreeItem{}})
	setup(repo)
	logger := &recordingLogger{}
	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, logger)
	if err != nil {
		t.Fatalf("NewCollectionService should start without the root: %v", err)
	}
	return svc, repo, logger
}

func TestNewCollectionService_UnloadableRoot_IsNotOverwritten(t *testing.T) {
	cases := map[string]func(*inMemoryRepo){
		"ファイルが残っている": func(r *inMemoryRepo) { r.existing = map[string]bool{domain.RootCollectionID: true} },
		"有無を確認できない":  func(r *inMemoryRepo) { r.existsErr = errors.New("stat error") },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo, logger := newSvcWithUnloadableRoot(t, setup)

			if slices.Contains(repo.saveOrder(), domain.RootCollectionID) {
				t.Fatal("root collection must not be saved when its file could not be loaded")
			}
			if logger.errors == 0 {
				t.Error("expected an error log explaining why the root is unavailable")
			}
			if items := svc.GetRootItems(); len(items) != 0 {
				t.Errorf("GetRootItems = %v, want empty", items)
			}
			layout, err := svc.GetSidebarLayout()
			if err != nil {
				t.Fatalf("GetSidebarLayout: %v", err)
			}
			assertLayout(t, layout, "c:c1")
		})
	}
}

func TestNewCollectionService_UnloadableRoot_RejectsRootWrites(t *testing.T) {
	svc, repo, _ := newSvcWithUnloadableRoot(t, func(r *inMemoryRepo) {
		r.existing = map[string]bool{domain.RootCollectionID: true}
	})
	if _, err := svc.AddRequest("c1", "", domain.HTTPRequest{ID: "r1", Name: testReqName}); err != nil {
		t.Fatalf("AddRequest to another collection should succeed: %v", err)
	}

	assertRootNotFound := func(op string, err error) {
		t.Helper()
		nf, ok := errors.AsType[*cmn.NotFoundError](err)
		if !ok || nf.ID != domain.RootCollectionID {
			t.Errorf("%s: expected NotFound for the root, got %v", op, err)
		}
	}
	_, err := svc.AddRequest(domain.RootCollectionID, "", domain.HTTPRequest{ID: "r2", Name: testReqName})
	assertRootNotFound("AddRequest", err)
	assertRootNotFound("MoveItemToSidebar", svc.MoveItemToSidebar("c1", "r1", 0))
	assertRootNotFound("MoveItem", svc.MoveItem("c1", "r1", domain.RootCollectionID, "", 0))

	if slices.Contains(repo.saveOrder(), domain.RootCollectionID) {
		t.Error("root collection must not be saved by rejected operations")
	}
	if _, _, ok := cachedCollection(t, svc, "c1").FindNode("r1"); !ok {
		t.Error("the item should stay in the source collection after rejected moves")
	}
}

func TestNewCollectionService_UnloadableRoot_RejectsRootDeleteAndRename(t *testing.T) {
	cases := map[string]func(*inMemoryRepo){
		"ファイルが残っている": func(r *inMemoryRepo) { r.existing = map[string]bool{domain.RootCollectionID: true} },
		"有無を確認できない":  func(r *inMemoryRepo) { r.existsErr = errors.New("stat error") },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo, _ := newSvcWithUnloadableRoot(t, setup)

			// キャッシュに root が無くても NotFound ではなく ValidationError を返す。
			if _, ok := errors.AsType[*cmn.ValidationError](svc.DeleteCollection(domain.RootCollectionID)); !ok {
				t.Error("DeleteCollection: expected ValidationError for the root")
			}
			if _, ok := errors.AsType[*cmn.ValidationError](svc.RenameCollection(domain.RootCollectionID, "x")); !ok {
				t.Error("RenameCollection: expected ValidationError for the root")
			}
			// 読めなかった __root__ のファイルには触れない。
			if slices.Contains(repo.saveOrder(), domain.RootCollectionID) {
				t.Error("root collection must not be saved by rejected operations")
			}
			if slices.Contains(repo.deleteOrder(), domain.RootCollectionID) {
				t.Error("root collection must not be deleted by rejected operations")
			}
		})
	}
}

// --- 予約済み root collection の保護 ---

func TestCollectionService_DeleteCollection_RootRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	if _, err := svc.AddRequest(domain.RootCollectionID, "", domain.HTTPRequest{ID: "r1", Name: "R1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	err := svc.DeleteCollection(domain.RootCollectionID)
	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if slices.Contains(repo.deleteOrder(), domain.RootCollectionID) {
		t.Error("root collection must not be deleted from the repository")
	}
	if _, _, ok := cachedCollection(t, svc, domain.RootCollectionID).FindNode("r1"); !ok {
		t.Error("root item disappeared from the cache")
	}
	persisted := repo.snapshot(domain.RootCollectionID)
	if persisted == nil || len(persisted.Items) != 1 || persisted.Items[0].ID != "r1" {
		t.Errorf("persisted root = %v, want it to keep r1", persisted)
	}
	if items := svc.GetRootItems(); len(items) != 1 || items[0].ID != "r1" {
		t.Errorf("GetRootItems = %v, want [r1]", items)
	}
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout, "i:r1")

	// 拒否が root を壊していないこと。
	if _, err := svc.AddRequest(domain.RootCollectionID, "", domain.HTTPRequest{ID: "r2", Name: "R2"}); err != nil {
		t.Errorf("AddRequest to the root after rejected delete: %v", err)
	}
}

func TestCollectionService_RenameCollection_RootRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	savesBefore := len(repo.saveOrder())

	err := svc.RenameCollection(domain.RootCollectionID, "x")
	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if got := repo.saveOrder()[savesBefore:]; slices.Contains(got, domain.RootCollectionID) {
		t.Errorf("root collection must not be saved by a rejected rename, saves = %v", got)
	}
	if got := cachedCollection(t, svc, domain.RootCollectionID).Name; got != domain.RootCollectionID {
		t.Errorf("cached root name = %q, want %q", got, domain.RootCollectionID)
	}
	if got := repo.snapshot(domain.RootCollectionID).Name; got != domain.RootCollectionID {
		t.Errorf("persisted root name = %q, want %q", got, domain.RootCollectionID)
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
	req := domain.HTTPRequest{ID: "r1", Name: testReqName}
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
	repo := newFakeRepo()
	repo.failAllSavesFromNth(2)
	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	_, err = svc.CreateCollection("ShouldFail")
	if err == nil {
		t.Error("expected error from repo.Save, got nil")
	}
}

func TestCollectionService_CreateCollection_EmptyNameReturnsValidationError(t *testing.T) {
	for _, name := range []string{"", "   ", "\t\n"} {
		repo := newFakeRepo()
		svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, nil)
		if err != nil {
			t.Fatalf("NewCollectionService: %v", err)
		}
		_, err = svc.CreateCollection(name)
		if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
			t.Fatalf("CreateCollection(%q): expected ValidationError, got %T (%v)", name, err, err)
		}
		if len(svc.GetCollections()) != 0 {
			t.Errorf("CreateCollection(%q): expected no collection created", name)
		}
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
	r1, _ := svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})
	r2, _ := svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r2", Name: "R2"})

	// r1 をターゲット親として移動しようとする（r1 はリクエストなのでエラー）。
	err := svc.MoveItem(col.ID, r2.ID, col.ID, r1.ID, 0)
	if err == nil {
		t.Error("expected error when target parent is not folder, got nil")
	}
	// #6: 検証失敗時に r2 が失われていないこと。
	if _, _, ok := findColItem(t, svc, col.ID, "r2"); !ok {
		t.Error("r2 was lost from source collection after failed move")
	}
}

func TestCollectionService_MoveItem_TargetParentNotFound_PreservesItem(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})

	// 存在しない親を指定した移動はエラーになり、アイテムは残る (#6)。
	err := svc.MoveItem(col.ID, "r1", col.ID, "nonexistent-parent", 0)
	if err == nil {
		t.Fatal("expected error for nonexistent target parent, got nil")
	}
	if _, _, ok := findColItem(t, svc, col.ID, "r1"); !ok {
		t.Error("r1 was lost from source collection after failed move")
	}
}

func TestCollectionService_MoveItem_IntoOwnSubtree_Rejected(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	f, _ := svc.AddFolder(col.ID, "", "F")
	f2, _ := svc.AddFolder(col.ID, f.ID, "F2")
	svc.AddRequest(col.ID, f.ID, domain.HTTPRequest{ID: "r1", Name: "R1"})

	// フォルダ F を自身へ移動 → 拒否。
	if err := svc.MoveItem(col.ID, f.ID, col.ID, f.ID, 0); err == nil {
		t.Error("expected error when moving folder into itself, got nil")
	}
	// フォルダ F を子孫フォルダ F2 へ移動 → 拒否。
	err := svc.MoveItem(col.ID, f.ID, col.ID, f2.ID, 0)
	if err == nil {
		t.Fatal("expected error when moving folder into its descendant, got nil")
	}
	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
	// F とその中身が温存されていること (#6)。
	if _, _, ok := findColItem(t, svc, col.ID, f.ID); !ok {
		t.Error("F was lost after rejected self-subtree move")
	}
	if _, _, ok := findColItem(t, svc, col.ID, f2.ID); !ok {
		t.Error("f2 was lost after rejected self-subtree move")
	}
	if _, _, ok := findColItem(t, svc, col.ID, "r1"); !ok {
		t.Error("r1 was lost after rejected self-subtree move")
	}
}

func TestCollectionService_MoveItem_SameCollection(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})
	svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r2", Name: "R2"})

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
	svc.AddRequest(col1.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})

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
	svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})

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

// newSvcWithOrderedLayout はコレクション ID を並べたレイアウトを保存済みにして
// CollectionService を組み立てる。実データと突合されるため、コレクション本体も
// リポジトリに用意する。
func newSvcWithOrderedLayout(t *testing.T, ids ...string) (*CollectionService, *inMemoryLayoutRepo) {
	t.Helper()
	cols := make([]*domain.Collection, 0, len(ids))
	layout := make([]domain.SidebarEntry, 0, len(ids))
	for _, id := range ids {
		cols = append(cols, &domain.Collection{ID: id, Name: id, Items: []*domain.TreeItem{}})
		layout = append(layout, domain.SidebarEntry{Kind: sidebarKindCollection, ID: id})
	}
	layoutRepo := &inMemoryLayoutRepo{layout: layout}
	svc, err := NewCollectionService(newFakeRepo(cols...), layoutRepo, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	return svc, layoutRepo
}

func TestCollectionService_GetSidebarLayout_ExistingLayout(t *testing.T) {
	svc, _ := newSvcWithOrderedLayout(t, "c1", "c2")
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout, "c:c1", "c:c2")
}

func TestCollectionService_GetSidebarLayout_FirstCall(t *testing.T) {
	// 複数コレクションを持つ状態で初回 GetSidebarLayout を呼ぶと
	// 名前順にコレクションがレイアウトに並ぶことを確認する。
	cols := []*domain.Collection{
		{ID: "c1", Name: "B", Items: []*domain.TreeItem{}},
		{ID: "c2", Name: "A", Items: []*domain.TreeItem{}},
	}
	svc, err := NewCollectionService(newFakeRepo(cols...), &inMemoryLayoutRepo{}, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}

	collectionEntries := make([]domain.SidebarEntry, 0)
	for _, e := range layout {
		if e.Kind == sidebarKindCollection {
			collectionEntries = append(collectionEntries, e)
		}
	}
	if len(collectionEntries) < 2 {
		t.Fatalf("expected at least 2 collection entries, got %d", len(collectionEntries))
	}
	if collectionEntries[0].ID != "c2" || collectionEntries[1].ID != "c1" {
		t.Errorf("expected c2(Name=A) before c1(Name=B), got %v", collectionEntries)
	}
}

// --- MoveSidebarEntry ---

func TestCollectionService_MoveSidebarEntry_NotFound(t *testing.T) {
	svc := newSvc(t)
	// 空のレイアウトに対して存在しない ID を指定するとエラー。
	err := svc.MoveSidebarEntry(sidebarKindCollection, "nonexistent", 0)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCollectionService_MoveSidebarEntry_Success(t *testing.T) {
	svc, _ := newSvcWithOrderedLayout(t, "c1", "c2", "c3")
	// c3 を position 0 に移動 → [c3, c1, c2]
	if err := svc.MoveSidebarEntry(sidebarKindCollection, "c3", 0); err != nil {
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
	svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})

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
	item, err := svc.AddRequest(domain.RootCollectionID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})
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
	repo := newFakeRepo(root)
	svc, err := NewCollectionService(repo, layoutRepo, nil)
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

// --- UpdateRequest: node is folder ---

func TestCollectionService_UpdateRequest_NodeIsFolder_ReturnsNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	folder, _ := svc.AddFolder(col.ID, "", "Folder")

	err := svc.UpdateRequest(col.ID, domain.HTTPRequest{ID: folder.ID, Name: "Renamed"})
	if err == nil {
		t.Fatal("expected error when updating a folder node, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

// --- AddFolder / AddRequest: repo.Save error ---

func TestCollectionService_AddFolder_RepoSaveError(t *testing.T) {
	// root コレクション作成(1回)後に失敗させる。
	repo := newFakeRepo()
	repo.failAllSavesFromNth(2)
	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	_, err = svc.AddFolder(domain.RootCollectionID, "", "Folder")
	if err == nil {
		t.Error("expected error from repo.Save, got nil")
	}
}

func TestCollectionService_AddRequest_RepoSaveError(t *testing.T) {
	repo := newFakeRepo()
	repo.failAllSavesFromNth(2)
	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	_, err = svc.AddRequest(domain.RootCollectionID, "", domain.HTTPRequest{Name: "R"})
	if err == nil {
		t.Error("expected error from repo.Save, got nil")
	}
}

// --- layoutRepo エラー伝搬 ---

// レイアウトは導出値なので、その書き込みに失敗してもコレクション本体の変更は成功する。
// 欠落・stale なエントリは読み出し時の突合で修復される。

func TestCollectionService_CreateCollection_LayoutRepoError_StillSucceeds(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{}
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, layoutRepo)
	layoutRepo.loadErr = errors.New("layout load error")

	col, err := svc.CreateCollection("Col")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if repo.snapshot(col.ID) == nil {
		t.Error("collection was not persisted")
	}
	layoutRepo.loadErr = nil
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout, "c:"+col.ID)
}

func TestCollectionService_AddFolder_LayoutRepoError_StillSucceeds(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{}
	svc := newSvcWithRepo(t, newFakeRepo(), layoutRepo)
	layoutRepo.loadErr = errors.New("layout load error")

	item, err := svc.AddFolder(domain.RootCollectionID, "", "Folder")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}
	layoutRepo.loadErr = nil
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout, "i:"+item.ID)
}

func TestCollectionService_DeleteCollection_LayoutRepoError_StillSucceeds(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{}
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, layoutRepo)
	col := mustCreate(t, svc, "Col")
	layoutRepo.loadErr = errors.New("layout load error")

	if err := svc.DeleteCollection(col.ID); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
	if repo.snapshot(col.ID) != nil {
		t.Error("collection was not deleted from the repository")
	}
	// レイアウトには stale エントリが残るが、読み出し時の突合で除去される。
	layoutRepo.loadErr = nil
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout)
}

func TestCollectionService_DeleteItem_LayoutRepoError_StillSucceeds(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{}
	svc := newSvcWithRepo(t, newFakeRepo(), layoutRepo)
	item, addErr := svc.AddRequest(domain.RootCollectionID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})
	if addErr != nil {
		t.Fatalf("AddRequest: %v", addErr)
	}
	layoutRepo.loadErr = errors.New("layout load error")

	if err := svc.DeleteItem(domain.RootCollectionID, item.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	layoutRepo.loadErr = nil
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout)
}

func TestCollectionService_MoveItemToSidebar_LayoutRepoError_StillSucceeds(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{}
	svc := newSvcWithRepo(t, newFakeRepo(), layoutRepo)
	col := mustCreate(t, svc, "Col")
	if _, err := svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	layoutRepo.loadErr = errors.New("layout load error")

	if err := svc.MoveItemToSidebar(col.ID, "r1", 0); err != nil {
		t.Fatalf("MoveItemToSidebar: %v", err)
	}
	if items := svc.GetRootItems(); len(items) != 1 || items[0].ID != "r1" {
		t.Errorf("root items = %v, want [r1]", items)
	}
	// 指定位置には入らないが、突合によりサイドバー末尾に現れる。
	layoutRepo.loadErr = nil
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout, "c:"+col.ID, "i:r1")
}

// --- NewCollectionService: layoutRepo エラー ---

// newFakeRepoForLayout はレイアウト再生成の検証用に、名前順と ID 順が異なる 2 コレクションと
// __root__ 直下の 2 アイテムを持つリポジトリを返す。再生成後の並びは c:a, c:z, i:r1, i:r2。
func newFakeRepoForLayout() *inMemoryRepo {
	return newFakeRepo(
		&domain.Collection{ID: "z", Name: testColNameZebra, Items: []*domain.TreeItem{}},
		&domain.Collection{ID: "a", Name: testColNameApple, Items: []*domain.TreeItem{}},
		colWith(domain.RootCollectionID, req("r1"), req("r2")),
	)
}

var errCorruptLayout = fmt.Errorf("%w: bad json", cmn.ErrCorruptData)

func TestNewCollectionService_CorruptLayout_QuarantinesAndRegenerates(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{loadErr: errCorruptLayout}
	svc, err := NewCollectionService(newFakeRepoForLayout(), layoutRepo, &recordingLogger{})
	if err != nil {
		t.Fatalf("NewCollectionService should start with a corrupt layout: %v", err)
	}
	if n := layoutRepo.quarantineCount(); n != 1 {
		t.Errorf("Quarantine calls = %d, want 1", n)
	}
	assertLayout(t, layoutRepo.snapshot(), "c:a", "c:z", "i:r1", "i:r2")

	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout, "c:a", "c:z", "i:r1", "i:r2")
}

func TestNewCollectionService_CorruptLayout_QuarantineFails_StillRegenerates(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{loadErr: errCorruptLayout, quarantineErr: errors.New("rename failed")}
	logger := &recordingLogger{}
	svc, err := NewCollectionService(newFakeRepoForLayout(), layoutRepo, logger)
	if err != nil {
		t.Fatalf("NewCollectionService should start when quarantine fails: %v", err)
	}
	if logger.errors == 0 {
		t.Error("quarantine failure should be logged")
	}
	// 再生成可能データなので、退避できなくても再生成した内容で上書きする。
	assertLayout(t, layoutRepo.snapshot(), "c:a", "c:z", "i:r1", "i:r2")

	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout, "c:a", "c:z", "i:r1", "i:r2")
}

func TestNewCollectionService_LayoutReadError_StartsWithoutTouchingFile(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{loadErr: errors.New("permission denied"), layout: entries("c:stale")}
	logger := &recordingLogger{}
	svc, err := NewCollectionService(newFakeRepoForLayout(), layoutRepo, logger)
	if err != nil {
		t.Fatalf("NewCollectionService should start when the layout cannot be read: %v", err)
	}
	if logger.errors == 0 {
		t.Error("read failure should be logged")
	}
	if n := layoutRepo.quarantineCount(); n != 0 {
		t.Errorf("Quarantine must not be called on read errors (calls = %d)", n)
	}
	// Save されていればフェイクの layout が書き換わる。
	assertLayout(t, layoutRepo.snapshot(), "c:stale")

	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout should not fail on read errors: %v", err)
	}
	assertLayout(t, layout, "c:a", "c:z", "i:r1", "i:r2")

	if err := svc.MoveSidebarEntry(sidebarKindCollection, "z", 0); err == nil {
		t.Error("MoveSidebarEntry should fail while the layout cannot be read")
	}
	assertLayout(t, layoutRepo.snapshot(), "c:stale")
}

func TestNewCollectionService_LayoutRepoSaveError_StillStarts(t *testing.T) {
	// レイアウトの保存失敗は起動を止めない。読み取り専用ディレクトリなどでも
	// コレクションが読めている限りアプリは立ち上がり、突合済みの並びで動作する。
	layoutRepo := &inMemoryLayoutRepo{saveErr: errors.New("layout save error")}
	existing := &domain.Collection{ID: "c1", Name: "C1", Items: []*domain.TreeItem{}}
	svc, err := NewCollectionService(newFakeRepo(existing), layoutRepo, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	layout, err := svc.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayout(t, layout, "c:c1")
}

// --- MoveItemToSidebar: error cases ---

func TestCollectionService_MoveItemToSidebar_SourceNotFound(t *testing.T) {
	svc := newSvc(t)
	err := svc.MoveItemToSidebar("nonexistent", "item", 0)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestCollectionService_MoveItemToSidebar_ItemNotFound(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	err := svc.MoveItemToSidebar(col.ID, "nonexistent", 0)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestCollectionService_MoveItemToSidebar_NegativePosition_AppendsToEnd(t *testing.T) {
	svc := newSvc(t)
	col := mustCreate(t, svc, "Col")
	svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"})

	if err := svc.MoveItemToSidebar(col.ID, "r1", -1); err != nil {
		t.Fatalf("MoveItemToSidebar: %v", err)
	}
	layout, _ := svc.GetSidebarLayout()
	last := layout[len(layout)-1]
	if last.Kind != sidebarKindItem || last.ID != "r1" {
		t.Errorf("expected r1 at end of layout, got %v", last)
	}
}

// --- MoveSidebarEntry: out-of-bounds position ---

func TestCollectionService_MoveSidebarEntry_OutOfBoundsPosition_AppendsToEnd(t *testing.T) {
	svc, _ := newSvcWithOrderedLayout(t, "c1", "c2", "c3")
	// position < 0 → 末尾追加
	if err := svc.MoveSidebarEntry(sidebarKindCollection, "c1", -1); err != nil {
		t.Fatalf("MoveSidebarEntry: %v", err)
	}
	layout, _ := svc.GetSidebarLayout()
	if layout[len(layout)-1].ID != "c1" {
		t.Errorf("expected c1 at end, got %v", layout[len(layout)-1].ID)
	}
}

// --- CreateCollection: updates layout ---

func TestCollectionService_CreateCollection_UpdatesLayout(t *testing.T) {
	svc := newSvc(t)
	col, err := svc.CreateCollection("NewCol")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	layout, _ := svc.GetSidebarLayout()
	found := false
	for _, e := range layout {
		if e.Kind == sidebarKindCollection && e.ID == col.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sidebar layout does not contain collection entry %q", col.ID)
	}
}

// --- Concurrent read/write ---

func TestCollectionService_ConcurrentReadWrite(t *testing.T) {
	// go test -race でデータ競合が検出されないことを確認する (CI は -race 付きで実行する)。
	// 読み手は戻り値を JSON 化し、Wails の応答と同じく戻り値の中身までロック外で走査する。
	svc := newSvc(t)
	mustCreate(t, svc, "Init")

	const goroutines = 10
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			if _, err := json.Marshal(svc.GetCollections()); err != nil {
				t.Errorf("marshal collections: %v", err)
			}
		})
		wg.Go(func() { svc.CreateCollection("concurrent") })
	}
	wg.Wait()
}

// --- copy-on-write: 保存失敗時にキャッシュもディスクも変わらない ---

// newSvcWithLogger は logger 付きで CollectionService を組み立てる。
func newSvcWithLogger(t *testing.T, repo *inMemoryRepo, logger cmn.Logger) *CollectionService {
	t.Helper()
	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, logger)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	return svc
}

// newSvcWithRepo は失敗注入できるフェイクリポジトリから CollectionService を組み立てる。
func newSvcWithRepo(t *testing.T, repo *inMemoryRepo, layoutRepo *inMemoryLayoutRepo) *CollectionService {
	t.Helper()
	svc, err := NewCollectionService(repo, layoutRepo, nil)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	return svc
}

// cachedCollection はキャッシュ上のコレクションを返す。
func cachedCollection(t *testing.T, svc *CollectionService, id string) *domain.Collection {
	t.Helper()
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	c, ok := svc.cache[id]
	if !ok {
		t.Fatalf("collection %q not found in cache", id)
	}
	return c
}

func TestCollectionService_RenameCollection_SaveError_LeavesCacheUnchanged(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	col := mustCreate(t, svc, "OldName")
	repo.failSaveAlways(col.ID)

	if err := svc.RenameCollection(col.ID, "NewName"); err == nil {
		t.Fatal("expected error from repo.Save, got nil")
	}
	if got := cachedCollection(t, svc, col.ID).Name; got != "OldName" {
		t.Errorf("cached name = %q, want OldName", got)
	}
	if got := repo.snapshot(col.ID).Name; got != "OldName" {
		t.Errorf("persisted name = %q, want OldName", got)
	}
}

func TestCollectionService_UpdateRequest_SaveError_LeavesCacheUnchanged(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	col := mustCreate(t, svc, "Col")
	if _, err := svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1", URL: "http://old.example.com"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	repo.failSaveAlways(col.ID)

	err := svc.UpdateRequest(col.ID, domain.HTTPRequest{ID: "r1", URL: "http://new.example.com"})
	if err == nil {
		t.Fatal("expected error from repo.Save, got nil")
	}
	node, _, ok := findColItem(t, svc, col.ID, "r1")
	if !ok {
		t.Fatal("r1 not found in cache")
	}
	if node.Request.URL != "http://old.example.com" {
		t.Errorf("cached URL = %q, want the pre-update value", node.Request.URL)
	}
	if got := repo.snapshot(col.ID).Items[0].Request.URL; got != "http://old.example.com" {
		t.Errorf("persisted URL = %q, want the pre-update value", got)
	}
}

func TestCollectionService_RenameItem_SaveError_LeavesCacheUnchanged(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	col := mustCreate(t, svc, "Col")
	if _, err := svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "OldReq"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	repo.failSaveAlways(col.ID)

	if err := svc.RenameItem(col.ID, "r1", "NewReq"); err == nil {
		t.Fatal("expected error from repo.Save, got nil")
	}
	node, _, _ := findColItem(t, svc, col.ID, "r1")
	if node.Name != "OldReq" {
		t.Errorf("cached name = %q, want OldReq", node.Name)
	}
	if got := repo.snapshot(col.ID).Items[0].Name; got != "OldReq" {
		t.Errorf("persisted name = %q, want OldReq", got)
	}
}

func TestCollectionService_DeleteItem_SaveError_KeepsItem(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	col := mustCreate(t, svc, "Col")
	if _, err := svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	repo.failSaveAlways(col.ID)

	if err := svc.DeleteItem(col.ID, "r1"); err == nil {
		t.Fatal("expected error from repo.Save, got nil")
	}
	if _, _, ok := findColItem(t, svc, col.ID, "r1"); !ok {
		t.Error("r1 disappeared from the cache although the save failed")
	}
	if len(repo.snapshot(col.ID).Items) != 1 {
		t.Error("r1 disappeared from the repository although the save failed")
	}
}

func TestCollectionService_AddItem_SaveError_LeavesCacheUnchanged(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	col := mustCreate(t, svc, "Col")
	repo.failSaveAlways(col.ID)

	if _, err := svc.AddFolder(col.ID, "", "Folder"); err == nil {
		t.Fatal("expected error from repo.Save, got nil")
	}
	if items := cachedCollection(t, svc, col.ID).Items; len(items) != 0 {
		t.Errorf("cached items = %d, want 0 (保存に失敗した追加は残らない)", len(items))
	}
	if items := repo.snapshot(col.ID).Items; len(items) != 0 {
		t.Errorf("persisted items = %d, want 0", len(items))
	}
}

func TestCollectionService_MoveItem_SourceSaveError_RollsBackTarget(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	src := mustCreate(t, svc, "Src")
	dst := mustCreate(t, svc, "Dst")
	if _, err := svc.AddRequest(src.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	repo.failSaveAlways(src.ID)

	if err := svc.MoveItem(src.ID, "r1", dst.ID, "", -1); err == nil {
		t.Fatal("expected error from repo.Save, got nil")
	}
	// キャッシュは操作前のまま。
	if _, _, ok := findColItem(t, svc, src.ID, "r1"); !ok {
		t.Error("r1 disappeared from the source collection in the cache")
	}
	if items := cachedCollection(t, svc, dst.ID).Items; len(items) != 0 {
		t.Errorf("cached target items = %d, want 0", len(items))
	}
	// ディスク上も操作前のまま（移動先の書き込みは巻き戻される）。
	if items := repo.snapshot(dst.ID).Items; len(items) != 0 {
		t.Errorf("persisted target items = %d, want 0 (rollback されるべき)", len(items))
	}
	if items := repo.snapshot(src.ID).Items; len(items) != 1 {
		t.Errorf("persisted source items = %d, want 1", len(items))
	}
}

func TestCollectionService_MoveItemToSidebar_SourceSaveError_RollsBackRoot(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	col := mustCreate(t, svc, "Col")
	if _, err := svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	repo.failSaveAlways(col.ID)

	if err := svc.MoveItemToSidebar(col.ID, "r1", 0); err == nil {
		t.Fatal("expected error from repo.Save, got nil")
	}
	if _, _, ok := findColItem(t, svc, col.ID, "r1"); !ok {
		t.Error("r1 disappeared from the source collection in the cache")
	}
	if items := svc.GetRootItems(); len(items) != 0 {
		t.Errorf("root items = %d, want 0", len(items))
	}
	if items := repo.snapshot(domain.RootCollectionID).Items; len(items) != 0 {
		t.Errorf("persisted root items = %d, want 0 (rollback されるべき)", len(items))
	}
}

func TestCollectionService_MoveSidebarEntry_SaveError_LeavesLayoutUnchanged(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{}
	svc := newSvcWithRepo(t, newFakeRepo(), layoutRepo)
	c1 := mustCreate(t, svc, "C1")
	c2 := mustCreate(t, svc, "C2")
	before := layoutRepo.snapshot()
	layoutRepo.saveErr = errors.New("save error")

	if err := svc.MoveSidebarEntry(sidebarKindCollection, c2.ID, 0); err == nil {
		t.Fatal("expected error from layout Save, got nil")
	}
	layoutRepo.saveErr = nil
	after := layoutRepo.snapshot()
	if len(after) != len(before) || after[0].ID != c1.ID {
		t.Errorf("layout = %v, want unchanged %v", after, before)
	}
}

// --- 書き込み順序: 追加してから削除 ---

// lastSaves は保存順の末尾 n 件を返す。準備段階の書き込みを除いて検証するために使う。
func lastSaves(repo *inMemoryRepo, n int) []string {
	order := repo.saveOrder()
	if len(order) < n {
		return order
	}
	return order[len(order)-n:]
}

func TestCollectionService_MoveItem_AcrossCollections_WritesTargetFirst(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	src := mustCreate(t, svc, "Src")
	dst := mustCreate(t, svc, "Dst")
	if _, err := svc.AddRequest(src.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	if err := svc.MoveItem(src.ID, "r1", dst.ID, "", -1); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}
	// クラッシュ時の残骸が喪失ではなく重複になることの根拠なので順序を直接固定する。
	got := lastSaves(repo, 2)
	if len(got) != 2 || got[0] != dst.ID || got[1] != src.ID {
		t.Errorf("save order = %v, want [target source] = [%s %s]", got, dst.ID, src.ID)
	}
}

func TestCollectionService_MoveItemToSidebar_WritesRootFirst(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	col := mustCreate(t, svc, "Col")
	if _, err := svc.AddRequest(col.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	if err := svc.MoveItemToSidebar(col.ID, "r1", 0); err != nil {
		t.Fatalf("MoveItemToSidebar: %v", err)
	}
	got := lastSaves(repo, 2)
	if len(got) != 2 || got[0] != domain.RootCollectionID || got[1] != col.ID {
		t.Errorf("save order = %v, want [%s %s]", got, domain.RootCollectionID, col.ID)
	}
}

func TestCollectionService_MoveItem_RollbackFailure_ReturnsOriginalErrorAndLogs(t *testing.T) {
	repo := newFakeRepo()
	logger := &recordingLogger{}
	svc := newSvcWithLogger(t, repo, logger)
	src := mustCreate(t, svc, "Src")
	dst := mustCreate(t, svc, "Dst")
	if _, err := svc.AddRequest(src.ID, "", domain.HTTPRequest{ID: "r1", Name: "R1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	// 移動先の書き込みは通り、移動元の書き込みと移動先の巻き戻しが失敗する状況を作る。
	// 移動先は作成時とこの移動で2回書かれるので、3回目 (巻き戻し) から失敗させる。
	repo.failSaveFromNth(dst.ID, 3)
	repo.failSaveAlways(src.ID)
	logger.errors = 0

	err := svc.MoveItem(src.ID, "r1", dst.ID, "", -1)
	if !errors.Is(err, errFakeSave) {
		t.Fatalf("MoveItem error = %v, want the original save error", err)
	}
	if logger.errors == 0 {
		t.Error("rollback failure was not logged")
	}
	// 巻き戻しに失敗してもキャッシュは操作前のまま (差し替えは永続化成功後のみ)。
	if _, _, ok := findColItem(t, svc, src.ID, "r1"); !ok {
		t.Error("r1 disappeared from the source collection in the cache")
	}
}

func TestCollectionService_ConcurrentUpdateWithSaveFailures(t *testing.T) {
	// 保存失敗を注入しながら並行更新しても、キャッシュとリポジトリが食い違わないことを
	// 確認する (CI は -race 付きで実行する)。
	repo := newFakeRepo()
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})
	col := mustCreate(t, svc, "Col")
	// 2回目以降の保存を失敗させ、成功と失敗が混ざる状態にする。
	repo.failSaveFromNth(col.ID, 2)

	const goroutines = 10
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() { svc.RenameCollection(col.ID, fmt.Sprintf("Renamed%d", i)) })
		wg.Go(func() { svc.GetCollections() })
	}
	wg.Wait()

	// 保存に失敗した名前がキャッシュへ漏れていないこと。
	if got, want := cachedCollection(t, svc, col.ID).Name, repo.snapshot(col.ID).Name; got != want {
		t.Errorf("cached name = %q, persisted name = %q; they must not diverge", got, want)
	}
}

// --- 返却値・入力値とキャッシュの切り離し ---

// richRequest は参照型のフィールド (Headers / Params / Contents / FormData) を
// すべて埋めたリクエストを返す。共有の有無を各階層で確かめるために使う。
func richRequest(id string) domain.HTTPRequest {
	return domain.HTTPRequest{
		ID:      id,
		Name:    testReqName,
		Method:  http.MethodPost,
		Headers: []domain.KeyValuePair{{Key: "X-H", Value: "h", Enabled: true}},
		Params:  []domain.KeyValuePair{{Key: "q", Value: "p", Enabled: true}},
		Body: domain.RequestBody{
			Type:     domain.BodyTypeFormData,
			Contents: map[string]string{"json": "{}"},
			FormData: []domain.FormRow{{Key: "f", Value: "v", Kind: domain.FormRowKindText, Enabled: true}},
		},
	}
}

// scribbleItems は items 以下のあらゆる階層を書き換える。
// 行儀の悪い呼び出し側が戻り値を書き換えることを模す。
func scribbleItems(items []*domain.TreeItem) {
	for i, item := range items {
		item.Name = "scribbled"
		if r := item.Request; r != nil {
			r.Name = "scribbled"
			for j := range r.Headers {
				r.Headers[j].Value = "scribbled"
			}
			for j := range r.Params {
				r.Params[j].Value = "scribbled"
			}
			if r.Body.Contents != nil {
				r.Body.Contents["json"] = "scribbled"
			}
			for j := range r.Body.FormData {
				r.Body.FormData[j].Value = "scribbled"
			}
		}
		scribbleItems(item.Children)
		items[i] = &domain.TreeItem{ID: "replaced"}
	}
}

// assertCacheEqual はキャッシュ上のコレクションが want と同じ内容であることを確かめる。
func assertCacheEqual(t *testing.T, svc *CollectionService, want *domain.Collection) {
	t.Helper()
	if got := cachedCollection(t, svc, want.ID); !reflect.DeepEqual(got, want) {
		t.Errorf("cache changed through an aliased value:\n got  %+v\n want %+v", got, want)
	}
}

// newSvcWithNestedRequest はフォルダ F の下に richRequest("r1") を置いた
// コレクションを collectionID に用意する。collectionID が空なら新規コレクションを作る。
func newSvcWithNestedRequest(t *testing.T, collectionID string) (*CollectionService, string) {
	t.Helper()
	svc := newSvc(t)
	if collectionID == "" {
		collectionID = mustCreate(t, svc, "Col").ID
	}
	folder, err := svc.AddFolder(collectionID, "", "F")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}
	if _, err := svc.AddRequest(collectionID, folder.ID, richRequest("r1")); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	return svc, collectionID
}

func TestCollectionService_GetCollections_ReturnsDeepCopy(t *testing.T) {
	svc, colID := newSvcWithNestedRequest(t, "")
	want := cachedCollection(t, svc, colID).Clone()

	got := svc.GetCollections()
	if len(got) != 1 {
		t.Fatalf("collections = %d, want 1", len(got))
	}
	got[0].Name = "scribbled"
	got[0].Items = append(got[0].Items, &domain.TreeItem{ID: "appended"})
	scribbleItems(got[0].Items)

	assertCacheEqual(t, svc, want)
	if again := svc.GetCollections(); !reflect.DeepEqual(again[0], *want) {
		t.Errorf("GetCollections changed through an aliased value:\n got  %+v\n want %+v", again[0], *want)
	}
}

func TestCollectionService_GetRootItems_ReturnsDeepCopy(t *testing.T) {
	svc, _ := newSvcWithNestedRequest(t, domain.RootCollectionID)
	want := cachedCollection(t, svc, domain.RootCollectionID).Clone()

	scribbleItems(svc.GetRootItems())

	assertCacheEqual(t, svc, want)
	if again := svc.GetRootItems(); !reflect.DeepEqual(again, want.Items) {
		t.Errorf("GetRootItems changed through an aliased value:\n got  %+v\n want %+v", again, want.Items)
	}
}

func TestCollectionService_AddedItems_AreNotAliased(t *testing.T) {
	svc := newSvc(t)

	col := mustCreate(t, svc, "Col")
	want := cachedCollection(t, svc, col.ID).Clone()
	col.Items = append(col.Items, &domain.TreeItem{ID: "appended"})
	assertCacheEqual(t, svc, want)

	folder, err := svc.AddFolder(col.ID, "", "F")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}
	want = cachedCollection(t, svc, col.ID).Clone()
	folder.Name = "scribbled"
	folder.Children = append(folder.Children, &domain.TreeItem{ID: "appended"})
	assertCacheEqual(t, svc, want)

	item, err := svc.AddRequest(col.ID, "", richRequest("r1"))
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	want = cachedCollection(t, svc, col.ID).Clone()
	scribbleItems([]*domain.TreeItem{item})
	assertCacheEqual(t, svc, want)
}

// scribbleRequest は呼び出し後の引数 req の参照型フィールドを書き換える。
func scribbleRequest(req *domain.HTTPRequest) {
	req.Headers[0].Value = "scribbled"
	req.Params[0].Value = "scribbled"
	req.Body.Contents["json"] = "scribbled"
	req.Body.FormData[0].Value = "scribbled"
}

func TestCollectionService_AddRequest_DoesNotRetainArgument(t *testing.T) {
	svc := newSvc(t)
	req := richRequest("r1")
	if _, err := svc.AddRequest(domain.RootCollectionID, "", req); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	want := cachedCollection(t, svc, domain.RootCollectionID).Clone()

	scribbleRequest(&req)

	assertCacheEqual(t, svc, want)
}

func TestCollectionService_UpdateRequest_DoesNotRetainArgument(t *testing.T) {
	svc := newSvc(t)
	if _, err := svc.AddRequest(domain.RootCollectionID, "", domain.HTTPRequest{ID: "r1"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	req := richRequest("r1")
	if err := svc.UpdateRequest(domain.RootCollectionID, req); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}
	want := cachedCollection(t, svc, domain.RootCollectionID).Clone()

	scribbleRequest(&req)

	assertCacheEqual(t, svc, want)
}

// file キーの削除はキャッシュへ取り込むコピーにだけ行い、呼び出し側の map は書き換えない。
func TestCollectionService_AddRequest_DoesNotMutateArgument(t *testing.T) {
	svc := newSvc(t)
	req := domain.HTTPRequest{ID: "r1", Method: http.MethodPost, Body: fileBody()}
	if _, err := svc.AddRequest(domain.RootCollectionID, "", req); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	if _, ok := req.Body.Contents[domain.BodyTypeFile]; !ok {
		t.Errorf("AddRequest removed the file key from the caller's contents: %v", req.Body.Contents)
	}

	req = domain.HTTPRequest{ID: "r1", Method: http.MethodPost, Body: fileBody()}
	if err := svc.UpdateRequest(domain.RootCollectionID, req); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}
	if _, ok := req.Body.Contents[domain.BodyTypeFile]; !ok {
		t.Errorf("UpdateRequest removed the file key from the caller's contents: %v", req.Body.Contents)
	}
}

// JSON 化 (Wails の応答を模す) と更新と、戻り値を書き換える呼び出し側を並行に走らせ、
// go test -race でデータ競合が検出されないことを確認する。
// 戻り値がキャッシュを共有していれば、書き換えと別ゴルーチンの JSON 化が同じオブジェクトに
// 触れて検出される。copy-on-write が崩れてキャッシュを直接書き換えた場合も、
// 書き手と JSON 化の組み合わせで検出される。
func TestCollectionService_ConcurrentMarshalAndUpdate(t *testing.T) {
	svc, colID := newSvcWithNestedRequest(t, "")
	if _, err := svc.AddRequest(domain.RootCollectionID, "", richRequest("r2")); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	rootFolder, err := svc.AddFolder(domain.RootCollectionID, "", "RF")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}

	const iterations = 50
	var wg sync.WaitGroup
	// 読み手: 戻り値を JSON 化する。
	wg.Go(func() {
		for range iterations {
			if _, err := json.Marshal(svc.GetCollections()); err != nil {
				t.Errorf("marshal collections: %v", err)
			}
		}
	})
	wg.Go(func() {
		for range iterations {
			if _, err := json.Marshal(svc.GetRootItems()); err != nil {
				t.Errorf("marshal root items: %v", err)
			}
		}
	})
	// 書き手: 各種の変更系メソッドを呼ぶ。
	wg.Go(func() {
		for i := range iterations {
			if err := svc.UpdateRequest(colID, richRequest("r1")); err != nil {
				t.Errorf("UpdateRequest: %v", err)
			}
			if err := svc.RenameItem(colID, "r1", fmt.Sprintf("r1-%d", i)); err != nil {
				t.Errorf("RenameItem: %v", err)
			}
		}
	})
	wg.Go(func() {
		for i := range iterations {
			if _, err := svc.AddRequest(domain.RootCollectionID, "", richRequest("")); err != nil {
				t.Errorf("AddRequest: %v", err)
			}
			// r2 を __root__ 直下とフォルダ RF の間で往復させる。
			parentID := ""
			if i%2 == 0 {
				parentID = rootFolder.ID
			}
			if err := svc.MoveItem(domain.RootCollectionID, "r2", domain.RootCollectionID, parentID, -1); err != nil {
				t.Errorf("MoveItem: %v", err)
			}
		}
	})
	// 行儀の悪い呼び出し側: 戻り値を書き換える。
	wg.Go(func() {
		for range iterations {
			for _, c := range svc.GetCollections() {
				scribbleItems(c.Items)
			}
			scribbleItems(svc.GetRootItems())
		}
	})
	wg.Wait()
}
