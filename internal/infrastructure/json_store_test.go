package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type testItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func newTestStore(t *testing.T) *JSONStore[testItem] {
	t.Helper()
	dir := t.TempDir()
	store, err := NewJSONStore(dir, func(item *testItem) string { return item.ID })
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	return store
}

func TestJSONStore_Load_EmptyDir(t *testing.T) {
	store := newTestStore(t)
	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestJSONStore_SaveAndLoad(t *testing.T) {
	store := newTestStore(t)
	item := testItem{ID: "item1", Name: "Test Item"}
	if err := store.Save(&item); err != nil {
		t.Fatalf("Save: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "item1" || items[0].Name != "Test Item" {
		t.Errorf("loaded item = %+v, want {ID:item1, Name:Test Item}", items[0])
	}
}

func TestJSONStore_Save_MultipleItems(t *testing.T) {
	store := newTestStore(t)
	for _, item := range []testItem{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Beta"},
		{ID: "c", Name: "Gamma"},
	} {
		if err := store.Save(&item); err != nil {
			t.Fatalf("Save(%q): %v", item.ID, err)
		}
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestJSONStore_Save_OverwritesExisting(t *testing.T) {
	store := newTestStore(t)
	original := testItem{ID: "x", Name: "Original"}
	store.Save(&original)

	updated := testItem{ID: "x", Name: "Updated"}
	if err := store.Save(&updated); err != nil {
		t.Fatalf("Save updated: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "Updated" {
		t.Errorf("Name = %q, want Updated", items[0].Name)
	}
}

func TestJSONStore_Delete_RemovesFile(t *testing.T) {
	store := newTestStore(t)
	item := testItem{ID: "del1", Name: "ToDelete"}
	store.Save(&item)

	if err := store.Delete("del1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load after delete: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items after delete, got %d", len(items))
	}
}

func TestJSONStore_Delete_NonExistentFile(t *testing.T) {
	store := newTestStore(t)
	// 存在しないIDを削除しようとするとエラーを返す
	err := store.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestJSONStore_Load_SkipsNonJSONFiles(t *testing.T) {
	store := newTestStore(t)
	// JSONでないファイルを直接作成
	dir := store.dir
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("text"), 0o600); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("readme"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}

	// JSONファイルも1つ追加
	item := testItem{ID: "j1", Name: "JSON"}
	store.Save(&item)

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (non-JSON skipped), got %d", len(items))
	}
}

func TestJSONStore_Load_SkipsSubdirectories(t *testing.T) {
	store := newTestStore(t)
	// サブディレクトリを作成
	dir := store.dir
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	item := testItem{ID: "k1", Name: "Key"}
	store.Save(&item)

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (subdirs skipped), got %d", len(items))
	}
}

func TestJSONStore_Load_CorruptedJSON(t *testing.T) {
	store := newTestStore(t)
	dir := store.dir
	// 不正なJSONファイルを作成
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not valid json {{{"), 0o600); err != nil {
		t.Fatalf("write bad.json: %v", err)
	}

	// 破損ファイルはエラーにせず退避してスキップし、起動を継続する
	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load should not fail on corrupt file: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items (corrupt skipped), got %d", len(items))
	}

	// 元の bad.json は退避されている
	if _, statErr := os.Stat(filepath.Join(dir, "bad.json")); !os.IsNotExist(statErr) {
		t.Error("bad.json should have been renamed away")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "bad.json.corrupt")); statErr != nil {
		t.Errorf("bad.json.corrupt should exist: %v", statErr)
	}
}

func TestJSONStore_Load_SkipsCorruptKeepsValid(t *testing.T) {
	store := newTestStore(t)
	dir := store.dir

	// 有効なアイテムを1つ保存
	valid := testItem{ID: "good", Name: "Valid"}
	if err := store.Save(&valid); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// 破損ファイルを混在させる
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{{{"), 0o600); err != nil {
		t.Fatalf("write broken.json: %v", err)
	}

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 valid item, got %d", len(items))
	}
	if items[0].ID != "good" {
		t.Errorf("loaded item ID = %q, want good", items[0].ID)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "broken.json.corrupt")); statErr != nil {
		t.Errorf("broken.json.corrupt should exist: %v", statErr)
	}
	// 退避後の再ロードは .corrupt を無視して同じ結果になる
	items2, err := store.Load()
	if err != nil {
		t.Fatalf("Load (second): %v", err)
	}
	if len(items2) != 1 {
		t.Errorf("expected 1 item on reload, got %d", len(items2))
	}
}

func TestJSONStore_Save_CreatesValidJSON(t *testing.T) {
	store := newTestStore(t)
	item := testItem{ID: "json1", Name: "JSON Test"}
	store.Save(&item)

	dir := store.dir
	data, err := os.ReadFile(filepath.Join(dir, "json1.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var loaded testItem
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if loaded.ID != "json1" || loaded.Name != "JSON Test" {
		t.Errorf("loaded = %+v", loaded)
	}
}

func TestJSONStore_NewJSONStore_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "path", "store")

	store, err := NewJSONStore(dir, func(item *testItem) string { return item.ID })
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}

	if _, statErr := os.Stat(store.dir); os.IsNotExist(statErr) {
		t.Error("directory was not created")
	}
}
