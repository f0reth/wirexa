package httpinfra

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	httpapp "github.com/f0reth/Wirexa/internal/application/http"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
	"github.com/f0reth/Wirexa/internal/testutil"
)

const (
	secretToken = "session-token-7f3a"
	secretDir   = "/Users/alice/secret"
)

func newTestRepo(t *testing.T) (*CollectionRepository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := NewCollectionRepository(dir, nil)
	if err != nil {
		t.Fatalf("NewCollectionRepository: %v", err)
	}
	return repo, dir
}

// storedJSON はディレクトリ内の全 JSON ファイルを連結して返す。
func storedJSON(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var sb strings.Builder
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		sb.Write(data)
	}
	return sb.String()
}

func assertNoSecrets(t *testing.T, dir string) {
	t.Helper()
	raw := storedJSON(t, dir)
	// RequestAuth は Bearer 用に "token" キーを常に持つため、キー名ではなく値で検査する。
	for _, secret := range []string{secretToken, secretDir, `"filePath"`} {
		if strings.Contains(raw, secret) {
			t.Fatalf("persisted JSON contains %q:\n%s", secret, raw)
		}
	}
}

// fullRequest は file 参照以外の全フィールドを埋めたリクエストを返す。
func fullRequest() *domain.HTTPRequest {
	return &domain.HTTPRequest{
		Body: domain.RequestBody{
			Contents: map[string]string{"json": `{"a":1}`, "text": "hello"},
			Type:     "json",
			FormData: []domain.FormRow{
				{Key: "t", Value: "v", Kind: domain.FormRowKindText, ContentType: "text/csv", Enabled: true},
				{Key: "j", Value: `{}`, Kind: domain.FormRowKindJSON},
			},
			FormURLEncoded: []domain.FormRow{{Key: "u", Value: "1", Enabled: true}},
		},
		Auth:     domain.RequestAuth{Type: "basic", Username: "u", Password: "p", Token: "bearer-token"},
		ID:       "req-1",
		Name:     "Full",
		Method:   "POST",
		URL:      "https://example.com/api",
		Doc:      "# doc",
		Headers:  []domain.KeyValuePair{{Key: "X-A", Value: "1", Enabled: true}},
		Params:   []domain.KeyValuePair{{Key: "q", Value: "2"}},
		Settings: domain.RequestSettings{ProxyMode: "custom", ProxyURL: "http://proxy", TimeoutSec: 5, MaxResponseBodyMB: 3, InsecureSkipVerify: true, DisableRedirects: true},
	}
}

func collectionWith(req *domain.HTTPRequest) *domain.Collection {
	return &domain.Collection{
		ID:   "col-1",
		Name: "Col",
		Items: []*domain.TreeItem{{
			Type:     domain.ItemTypeFolder,
			ID:       "folder",
			Name:     "Folder",
			Children: []*domain.TreeItem{{Type: domain.ItemTypeRequest, ID: req.ID, Name: req.Name, Request: req, Children: []*domain.TreeItem{}}},
		}},
	}
}

// persistedCollection は、保存して読み戻すと意図して変わる値を c のコピー上で補正して返す。
// 往復テストの期待値に使う。保存しないフィールドを domain 型に足すときは、ここに理由と一緒に書く。
func persistedCollection(c *domain.Collection) domain.Collection {
	cp := c.Clone()
	persistedItems(cp.Items)
	return *cp
}

func persistedItems(items []*domain.TreeItem) {
	for _, it := range items {
		// JSON は nil スライスを [] として書くため、読み戻すと空スライスになる。
		if it.Children == nil {
			it.Children = []*domain.TreeItem{}
		}
		persistedItems(it.Children)
		if it.Request == nil {
			continue
		}
		body := &it.Request.Body
		body.File = persistedFileRef(body.File)
		for _, rows := range [][]domain.FormRow{body.FormData, body.FormURLEncoded} {
			for i := range rows {
				rows[i].File = persistedFileRef(rows[i].File)
			}
		}
	}
}

func persistedFileRef(ref domain.FileReference) domain.FileReference {
	// Token は現在のセッションでだけ有効なので保存しない。
	ref.Token = ""
	// ContentType は選択時に推定した表示用の値で、再選択時に決め直すので保存しない。
	ref.ContentType = ""
	// 再起動後は token が無いため、保存した参照は必ず再選択待ちになる。
	ref.NeedsReselect = true
	return ref
}

// 全階層の全フィールドを埋めたコレクションを保存・復元しても、意図した補正以外は失われない。
// 検査対象は型から決まるので、domain 型にフィールドを足して永続化 DTO や変換関数への追加を
// 忘れると、テストを書き換えなくてもここで差分として検出される。
func TestCollectionRepository_RoundTripKeepsEveryField(t *testing.T) {
	var c domain.Collection
	testutil.Populate(t, &c)

	repo, _ := newTestRepo(t)
	if err := repo.Save(&c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := persistedCollection(&c); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

// goldenCollection は testdata/collection.golden.json を読み込んだときに期待する値。
func goldenCollection() domain.Collection {
	full := &domain.HTTPRequest{
		Body: domain.RequestBody{
			Contents: map[string]string{"json": `{"a":1}`},
			File:     domain.FileReference{Name: "body.bin", NeedsReselect: true},
			Type:     domain.BodyTypeFormData,
			FormData: []domain.FormRow{
				{Key: "t", Value: "v", Kind: domain.FormRowKindText, ContentType: "text/csv", Enabled: true},
				{Key: "f", Kind: domain.FormRowKindFile, File: domain.FileReference{Name: "a.txt", NeedsReselect: true}, Enabled: true},
				{Key: "legacy", Value: "old"},
			},
			FormURLEncoded: []domain.FormRow{{Key: "u", Value: "1", Enabled: true}},
		},
		Auth:     domain.RequestAuth{Type: "basic", Username: "user", Password: "pass", Token: "bearer"},
		ID:       "req-full",
		Name:     "Full",
		Method:   "POST",
		URL:      "https://example.com/api",
		Doc:      "# doc",
		Headers:  []domain.KeyValuePair{{Key: "X-A", Value: "1", Enabled: true}, {Key: "X-B", Value: "2"}},
		Params:   []domain.KeyValuePair{{Key: "q", Value: "search", Enabled: true}},
		Settings: domain.RequestSettings{ProxyMode: "custom", ProxyURL: "http://proxy:8080", TimeoutSec: 30, MaxResponseBodyMB: 10, InsecureSkipVerify: true, DisableRedirects: true},
	}
	minimal := &domain.HTTPRequest{
		Body:    domain.RequestBody{Contents: map[string]string{}, Type: "none"},
		Auth:    domain.RequestAuth{Type: "none"},
		ID:      "req-min",
		Name:    "Minimal",
		Method:  "GET",
		Headers: []domain.KeyValuePair{},
		Params:  []domain.KeyValuePair{},
	}
	return domain.Collection{
		ID:   "col-1",
		Name: "Golden",
		Items: []*domain.TreeItem{
			{Type: domain.ItemTypeFolder, ID: "folder-1", Name: "Folder", Children: []*domain.TreeItem{
				{Type: domain.ItemTypeRequest, ID: "req-full", Name: "Full", Request: full, Children: []*domain.TreeItem{}},
			}},
			{Type: domain.ItemTypeRequest, ID: "req-min", Name: "Minimal", Request: minimal, Children: []*domain.TreeItem{}},
			{Type: domain.ItemTypeFolder, ID: "folder-empty", Name: "Empty", Children: []*domain.TreeItem{}},
		},
	}
}

// 既存の保存形式 (golden) を読めて、書き戻すと同じ JSON になる。
// 往復テストは DTO の json タグを変えても通るので、既存ファイルとの互換はここで確かめる。
func TestCollectionRepository_GoldenFormat(t *testing.T) {
	golden := testutil.ReadGolden(t, filepath.Join("testdata", "collection.golden.json"))
	repo, dir := newTestRepo(t)
	path := filepath.Join(dir, "col-1.json")
	if err := os.WriteFile(path, golden, 0o600); err != nil {
		t.Fatalf("write golden: %v", err)
	}

	loaded, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := goldenCollection(); len(loaded) != 1 || !reflect.DeepEqual(loaded[0], want) {
		t.Fatalf("golden load mismatch:\n got %+v\nwant %+v", loaded, want)
	}

	if err = repo.Save(&loaded[0]); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	testutil.AssertJSONEqual(t, saved, golden)
}

// 永続化 DTO はどの階層でも domain 型を埋め込まない。
func TestCollectionRepository_StoredDTOHasNoDomainTypes(t *testing.T) {
	testutil.AssertNoTypesFrom(t, storedCollection{}, testutil.DomainPkg)
}

// 保存時に token と実パスを捨て、basename だけを再選択待ちとして残す。
// 保存に使った runtime model (キャッシュ) の token は変更しない。
func TestCollectionRepository_SaveDropsTokensAndPaths(t *testing.T) {
	req := fullRequest()
	req.Body.Type = domain.BodyTypeFile
	req.Body.File = domain.FileReference{Token: secretToken, Name: "a.png", ContentType: "image/png"}
	req.Body.Contents[domain.BodyTypeFile] = secretDir + "/a.png"
	req.Body.FormData = []domain.FormRow{
		{Key: "selected", Kind: domain.FormRowKindFile, File: domain.FileReference{Token: secretToken, Name: "b.txt"}, Enabled: true},
	}
	col := collectionWith(req)

	repo, dir := newTestRepo(t)
	if err := repo.Save(col); err != nil {
		t.Fatalf("Save: %v", err)
	}
	assertNoSecrets(t, dir)

	if req.Body.File.Token != secretToken || req.Body.FormData[0].File.Token != secretToken {
		t.Fatal("Save must not strip tokens from the runtime model")
	}
	if req.Body.Contents[domain.BodyTypeFile] == "" {
		t.Fatal("Save must not mutate the runtime model's contents")
	}

	loaded, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := loaded[0].Items[0].Children[0].Request.Body
	if want := (domain.FileReference{Name: "a.png", NeedsReselect: true}); got.File != want {
		t.Errorf("body file = %+v, want %+v", got.File, want)
	}
	if _, ok := got.Contents[domain.BodyTypeFile]; ok {
		t.Error("loaded contents must not carry a file path")
	}
	if want := (domain.FileReference{Name: "b.txt", NeedsReselect: true}); got.FormData[0].File != want {
		t.Errorf("row file = %+v, want %+v", got.FormData[0].File, want)
	}
}

// 旧形式 (Contents["file"] と FormRow.filePath に生のパス) を basename と再選択状態へ移行し、
// 次の保存でディスク上の旧パスも消える。
func TestCollectionRepository_MigratesLegacyPaths(t *testing.T) {
	repo, dir := newTestRepo(t)
	legacy := `{"id":"col-1","name":"Col","items":[{"type":"request","id":"r1","name":"R","children":[],
		"request":{"id":"r1","name":"R","method":"POST","url":"http://x","doc":"","headers":[],"params":[],
		"auth":{"type":"none","username":"","password":"","token":""},"settings":{},
		"body":{"type":"file","contents":{"file":"C:\\Users\\alice\\secret\\a.bin","json":"{}"},
		"formData":[
			{"key":"unix","value":"","kind":"file","filePath":"/Users/alice/secret/b.txt","enabled":true},
			{"key":"trailing","value":"","kind":"file","filePath":"/Users/alice/secret/dir/","enabled":true},
			{"key":"empty","value":"","kind":"file","filePath":"","enabled":true}
		]}}}]}`
	if err := os.WriteFile(filepath.Join(dir, "col-1.json"), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	loaded, err := repo.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	body := loaded[0].Items[0].Request.Body
	if want := (domain.FileReference{Name: "a.bin", NeedsReselect: true}); body.File != want {
		t.Errorf("body file = %+v, want %+v", body.File, want)
	}
	if _, ok := body.Contents[domain.BodyTypeFile]; ok {
		t.Error("legacy file path must be removed from contents")
	}
	if body.Contents["json"] != "{}" {
		t.Error("other contents must be kept")
	}
	wantRows := []domain.FileReference{{Name: "b.txt", NeedsReselect: true}, {Name: "dir", NeedsReselect: true}, {}}
	for i, want := range wantRows {
		if body.FormData[i].File != want {
			t.Errorf("row %d file = %+v, want %+v", i, body.FormData[i].File, want)
		}
	}

	if err := repo.Save(&loaded[0]); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if raw := storedJSON(t, dir); strings.Contains(raw, "alice") || strings.Contains(raw, "filePath") {
		t.Fatalf("re-saved JSON still holds the legacy path:\n%s", raw)
	}
}

func TestLegacyBaseName(t *testing.T) {
	tests := map[string]string{
		"/home/me/a.txt":         "a.txt",
		`C:\Users\me\b.png`:      "b.png",
		`\\server\share\c`:       "c",
		"mixed/dir\\d.json":      "d.json",
		"dir/":                   "dir",
		"name-only":              "name-only",
		"":                       "",
		"/":                      "",
		`C:\Users\me\trailing\\`: "trailing",
		"../../etc/passwd":       "passwd",
	}
	for in, want := range tests {
		if got := legacyBaseName(in); got != want {
			t.Errorf("legacyBaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

// CollectionService のどの保存経路でも token が永続化されず、
// キャッシュ上の token は autosave 後も同一セッションで使える状態に残る。
func TestCollectionRepository_NoSavePathPersistsTokens(t *testing.T) {
	repo, dir := newTestRepo(t)
	layout := NewSidebarLayoutRepository(filepath.Join(t.TempDir(), "layout.json"))
	svc, err := httpapp.NewCollectionService(repo, layout, testutil.NoopLogger{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}

	withToken := func(id string) domain.HTTPRequest {
		return domain.HTTPRequest{
			ID:     id,
			Method: "POST",
			Body: domain.RequestBody{
				Type:     domain.BodyTypeFile,
				File:     domain.FileReference{Token: secretToken, Name: "a.png"},
				FormData: []domain.FormRow{{Key: "f", Kind: domain.FormRowKindFile, File: domain.FileReference{Token: secretToken, Name: "b.txt"}}},
			},
		}
	}

	col, err := svc.CreateCollection("A")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	other, err := svc.CreateCollection("B")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	steps := []struct {
		run  func() error
		name string
	}{
		{name: "AddRequest", run: func() error { _, err := svc.AddRequest(col.ID, "", withToken("r1")); return err }},
		{name: "AddRequest root", run: func() error {
			_, err := svc.AddRequest(domain.RootCollectionID, "", withToken("r2"))
			return err
		}},
		{name: "UpdateRequest", run: func() error { return svc.UpdateRequest(col.ID, withToken("r1")) }},
		{name: "RenameItem", run: func() error { return svc.RenameItem(col.ID, "r1", "renamed") }},
		{name: "RenameCollection", run: func() error { return svc.RenameCollection(col.ID, "A2") }},
		{name: "MoveItem", run: func() error { return svc.MoveItem(col.ID, "r1", other.ID, "", -1) }},
		{name: "MoveItemToSidebar", run: func() error { return svc.MoveItemToSidebar(other.ID, "r1", 0) }},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		assertNoSecrets(t, dir)
	}

	for _, item := range svc.GetRootItems() {
		if item.Request.Body.File.Token != secretToken || item.Request.Body.FormData[0].File.Token != secretToken {
			t.Fatalf("runtime token of %s was lost after saving", item.ID)
		}
	}
}

// 旧パスを持つ保存データを読み込んだ直後から、RPC 応答に旧パスが現れない。
func TestCollectionRepository_LoadedServiceHidesLegacyPaths(t *testing.T) {
	repo, dir := newTestRepo(t)
	legacy := func(id string) string {
		return `{"id":"` + id + `","name":"` + id + `","items":[{"type":"request","id":"r-` + id + `","name":"R","children":[],
			"request":{"id":"r-` + id + `","name":"R","method":"POST","url":"","doc":"","headers":[],"params":[],"auth":{},"settings":{},
			"body":{"type":"form-data","contents":{"file":"` + secretDir + `/a.bin"},
			"formData":[{"key":"f","value":"","kind":"file","filePath":"` + secretDir + `/b.txt","enabled":true}]}}}]}`
	}
	for _, id := range []string{"col-1", domain.RootCollectionID} {
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(legacy(id)), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	svc, err := httpapp.NewCollectionService(repo, NewSidebarLayoutRepository(filepath.Join(t.TempDir(), "layout.json")), testutil.NoopLogger{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}

	var items []*domain.TreeItem
	for _, c := range svc.GetCollections() {
		items = append(items, c.Items...)
	}
	items = append(items, svc.GetRootItems()...)
	if len(items) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(items))
	}
	for _, item := range items {
		body := item.Request.Body
		if _, ok := body.Contents[domain.BodyTypeFile]; ok {
			t.Fatalf("%s exposes a legacy path: %+v", item.ID, body)
		}
		if !body.File.NeedsReselect || !body.FormData[0].File.NeedsReselect {
			t.Fatalf("%s must require reselecting its files: %+v", item.ID, body)
		}
	}
}
