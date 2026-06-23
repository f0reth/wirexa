//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/f0reth/Wirexa/internal/adapters"
	httpapp "github.com/f0reth/Wirexa/internal/application/http"
	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
	httpinfra "github.com/f0reth/Wirexa/internal/infrastructure/http"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// newHTTPHandlerWithDir は指定ディレクトリから HttpHandler を組み立てる（永続化テスト用）。
// コレクションは dir/collections/ サブディレクトリに保存し、sidebar_layout.json と混在させない。
func newHTTPHandlerWithDir(t *testing.T, dir string) *adapters.HttpHandler {
	t.Helper()
	collDir := filepath.Join(dir, "collections")
	repo, err := infra.NewJSONStore(collDir, func(c *httpdomain.Collection) string { return c.ID })
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	layoutRepo := httpinfra.NewSidebarLayoutRepository(filepath.Join(dir, "sidebar_layout.json"))
	collSvc, err := httpapp.NewCollectionService(repo, layoutRepo)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	reqSvc := httpapp.NewHTTPRequestService(httpinfra.NewNetClient(), testutil.NoopLogger{})
	h := &adapters.HttpHandler{}
	adapters.SetupHTTPHandler(context.Background(), h, reqSvc, collSvc, collSvc, nil)
	return h
}

// newHTTPHandler は統合テスト用に HttpHandler を DI で組み立てる。
func newHTTPHandler(t *testing.T) *adapters.HttpHandler {
	t.Helper()
	return newHTTPHandlerWithDir(t, t.TempDir())
}

// TestHTTP_CollectionCRUD はコレクションの作成・取得・名前変更・削除を通しでテストする。
func TestHTTP_CollectionCRUD(t *testing.T) {
	h := newHTTPHandler(t)

	// 作成
	col, err := h.CreateCollection("MyCollection")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if col.Name != "MyCollection" {
		t.Errorf("Name = %q, want MyCollection", col.Name)
	}

	// 取得
	cols := h.GetCollections()
	if len(cols) != 1 || cols[0].ID != col.ID {
		t.Fatalf("GetCollections: got %d items, want 1", len(cols))
	}

	// 名前変更
	if err := h.RenameCollection(col.ID, "Renamed"); err != nil {
		t.Fatalf("RenameCollection: %v", err)
	}
	cols = h.GetCollections()
	if cols[0].Name != "Renamed" {
		t.Errorf("Name = %q, want Renamed", cols[0].Name)
	}

	// 削除
	if err := h.DeleteCollection(col.ID); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
	if cols := h.GetCollections(); len(cols) != 0 {
		t.Errorf("expected 0 collections after delete, got %d", len(cols))
	}
}

// TestHTTP_FolderAndRequestTree はフォルダ・リクエスト追加とネスト構造の保存・取得をテストする。
func TestHTTP_FolderAndRequestTree(t *testing.T) {
	h := newHTTPHandler(t)

	col, _ := h.CreateCollection("Tree")

	// ルートにフォルダを追加
	folder, err := h.AddFolder(col.ID, "", "FolderA")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}

	// フォルダ内にリクエストを追加
	req := httpdomain.HttpRequest{Name: "GET example", Method: "GET", URL: "http://example.com"}
	item, err := h.AddRequest(col.ID, folder.ID, req)
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	if item.Request.URL != "http://example.com" {
		t.Errorf("URL = %q, want http://example.com", item.Request.URL)
	}

	// 永続化確認: GetCollections で取得して構造を検証
	cols := h.GetCollections()
	if len(cols) != 1 {
		t.Fatalf("expected 1 collection, got %d", len(cols))
	}
	if len(cols[0].Items) != 1 {
		t.Fatalf("expected 1 root item, got %d", len(cols[0].Items))
	}
	rootFolder := cols[0].Items[0]
	if rootFolder.Type != httpdomain.ItemTypeFolder {
		t.Errorf("root item type = %q, want folder", rootFolder.Type)
	}
	if len(rootFolder.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(rootFolder.Children))
	}
	if rootFolder.Children[0].Type != httpdomain.ItemTypeRequest {
		t.Errorf("child type = %q, want request", rootFolder.Children[0].Type)
	}
}

// TestHTTP_SendRequest_2xx は httptest.Server に GET/POST を送り 2xx 応答をマッピングする。
func TestHTTP_SendRequest_2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true}`)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)

	for _, method := range []string{"GET", "POST"} {
		t.Run(method, func(t *testing.T) {
			resp, err := h.SendRequest(httpdomain.HttpRequest{
				Method: method,
				URL:    srv.URL,
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
			}
			if !strings.Contains(resp.Body, `"ok":true`) {
				t.Errorf("Body = %q, want to contain ok:true", resp.Body)
			}
		})
	}
}

// TestHTTP_SendRequest_4xx5xx はエラーステータスが HttpResponse.StatusCode に反映されることを確認する。
func TestHTTP_SendRequest_4xx5xx(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"404", http.StatusNotFound},
		{"500", http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			h := newHTTPHandler(t)
			resp, err := h.SendRequest(httpdomain.HttpRequest{
				Method: "GET",
				URL:    srv.URL,
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if resp.StatusCode != tc.statusCode {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tc.statusCode)
			}
		})
	}
}

// TestHTTP_SendRequest_HeadersAndParams はヘッダー・クエリパラメータが実際のリクエストに含まれることを確認する。
func TestHTTP_SendRequest_HeadersAndParams(t *testing.T) {
	var gotHeader, gotParam string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Test")
		gotParam = r.URL.Query().Get("q")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest(httpdomain.HttpRequest{
		Method: "GET",
		URL:    srv.URL,
		Headers: []httpdomain.KeyValuePair{
			{Key: "X-Test", Value: "hello", Enabled: true},
		},
		Params: []httpdomain.KeyValuePair{
			{Key: "q", Value: "world", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if gotHeader != "hello" {
		t.Errorf("X-Test header = %q, want hello", gotHeader)
	}
	if gotParam != "world" {
		t.Errorf("query param q = %q, want world", gotParam)
	}
}

// TestHTTP_CancelRequest は CancelRequest でコンテキストがキャンセルされることを確認する。
func TestHTTP_CancelRequest(t *testing.T) {
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(ready)
		// リクエストがキャンセルされるまでブロック
		<-r.Context().Done()
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	done := make(chan error, 1)
	go func() {
		_, err := h.SendRequest(httpdomain.HttpRequest{
			Method: "GET",
			URL:    srv.URL,
		})
		done <- err
	}()

	// サーバーがリクエストを受け取ってからキャンセル
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive request in time")
	}
	h.CancelRequest("")

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error after cancel, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SendRequest did not return after cancel")
	}
}

// TestHTTP_UpdateRequest は AddRequest → UpdateRequest → GetCollections でリクエスト内容が更新されることを確認する。
func TestHTTP_UpdateRequest(t *testing.T) {
	h := newHTTPHandler(t)
	col, _ := h.CreateCollection("C")

	item, err := h.AddRequest(col.ID, "", httpdomain.HttpRequest{Name: "Req", Method: "GET", URL: "http://old.example.com"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	if err := h.UpdateRequest(col.ID, httpdomain.HttpRequest{ID: item.ID, Name: "Req", Method: "POST", URL: "http://new.example.com"}); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}

	cols := h.GetCollections()
	if len(cols) != 1 || len(cols[0].Items) != 1 {
		t.Fatalf("unexpected collection structure after update")
	}
	req := cols[0].Items[0].Request
	if req.URL != "http://new.example.com" {
		t.Errorf("URL = %q, want http://new.example.com", req.URL)
	}
	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST", req.Method)
	}
}

// TestHTTP_RenameItem は AddRequest → RenameItem → GetCollections で名前変更が反映されることを確認する。
func TestHTTP_RenameItem(t *testing.T) {
	h := newHTTPHandler(t)
	col, _ := h.CreateCollection("C")

	item, err := h.AddRequest(col.ID, "", httpdomain.HttpRequest{Name: "OldName", Method: "GET", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	if err := h.RenameItem(col.ID, item.ID, "NewName"); err != nil {
		t.Fatalf("RenameItem: %v", err)
	}

	cols := h.GetCollections()
	if len(cols) != 1 || len(cols[0].Items) != 1 {
		t.Fatalf("unexpected collection structure after rename")
	}
	if cols[0].Items[0].Name != "NewName" {
		t.Errorf("item Name = %q, want NewName", cols[0].Items[0].Name)
	}
}

// TestHTTP_DeleteItem は AddRequest → DeleteItem → GetCollections でアイテムが削除されることを確認する。
func TestHTTP_DeleteItem(t *testing.T) {
	h := newHTTPHandler(t)
	col, _ := h.CreateCollection("C")

	item, err := h.AddRequest(col.ID, "", httpdomain.HttpRequest{Name: "Req", Method: "GET", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	if err := h.DeleteItem(col.ID, item.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	cols := h.GetCollections()
	if len(cols) != 1 {
		t.Fatalf("expected 1 collection, got %d", len(cols))
	}
	if len(cols[0].Items) != 0 {
		t.Errorf("expected 0 items after delete, got %d", len(cols[0].Items))
	}
}

// TestHTTP_GetRootItems は __root__ へのアイテム追加後に GetRootItems が正しく返ることを確認する。
func TestHTTP_GetRootItems(t *testing.T) {
	h := newHTTPHandler(t)

	// 初期状態は空
	if items := h.GetRootItems(); len(items) != 0 {
		t.Fatalf("expected 0 root items initially, got %d", len(items))
	}

	// __root__ コレクションにフォルダを追加
	folder, err := h.AddFolder(httpdomain.RootCollectionID, "", "RootFolder")
	if err != nil {
		t.Fatalf("AddFolder to __root__: %v", err)
	}

	items := h.GetRootItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 root item, got %d", len(items))
	}
	if items[0].ID != folder.ID {
		t.Errorf("root item ID = %q, want %q", items[0].ID, folder.ID)
	}
	if items[0].Name != "RootFolder" {
		t.Errorf("root item Name = %q, want RootFolder", items[0].Name)
	}
}

// TestHTTP_MoveCollection は 2 コレクション作成後に MoveCollection で Order が変わることを確認する。
func TestHTTP_MoveCollection(t *testing.T) {
	h := newHTTPHandler(t)

	col1, _ := h.CreateCollection("Alpha")
	col2, _ := h.CreateCollection("Beta")

	// 名前順: [Alpha, Beta] が初期順序
	cols := h.GetCollections()
	if len(cols) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(cols))
	}

	// Beta を position 0 に移動 → [Beta, Alpha]
	if err := h.MoveCollection(col2.ID, 0); err != nil {
		t.Fatalf("MoveCollection: %v", err)
	}

	cols = h.GetCollections()
	if len(cols) != 2 {
		t.Fatalf("expected 2 collections after move, got %d", len(cols))
	}
	if cols[0].ID != col2.ID {
		t.Errorf("first collection = %q, want col2 (%q)", cols[0].ID, col2.ID)
	}
	if cols[1].ID != col1.ID {
		t.Errorf("second collection = %q, want col1 (%q)", cols[1].ID, col1.ID)
	}
}

// TestHTTP_MoveItem は AddRequest → MoveItem（コレクション間移動）が正しく動くことを確認する。
func TestHTTP_MoveItem(t *testing.T) {
	h := newHTTPHandler(t)

	col1, _ := h.CreateCollection("Source")
	col2, _ := h.CreateCollection("Target")

	item, err := h.AddRequest(col1.ID, "", httpdomain.HttpRequest{Name: "Req", Method: "GET", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	// col1 → col2 へ移動
	if err := h.MoveItem(col1.ID, item.ID, col2.ID, "", -1); err != nil {
		t.Fatalf("MoveItem: %v", err)
	}

	cols := h.GetCollections()
	for _, c := range cols {
		switch c.ID {
		case col1.ID:
			if len(c.Items) != 0 {
				t.Errorf("col1 should have 0 items after move, got %d", len(c.Items))
			}
		case col2.ID:
			if len(c.Items) != 1 {
				t.Errorf("col2 should have 1 item after move, got %d", len(c.Items))
			} else if c.Items[0].ID != item.ID {
				t.Errorf("col2 item ID = %q, want %q", c.Items[0].ID, item.ID)
			}
		}
	}
}

// TestHTTP_SidebarLayout はサイドバーレイアウト関連 3 メソッドを E2E で検証する。
func TestHTTP_SidebarLayout(t *testing.T) {
	h := newHTTPHandler(t)

	col1, _ := h.CreateCollection("Alpha")
	col2, _ := h.CreateCollection("Beta")

	// GetSidebarLayout: 2 コレクションエントリが存在する
	layout, err := h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	if len(layout) != 2 {
		t.Fatalf("expected 2 sidebar entries, got %d", len(layout))
	}

	// MoveSidebarEntry: col1 (Alpha) を末尾（position 1）へ移動 → [Beta, Alpha]
	if err := h.MoveSidebarEntry("collection", col1.ID, 1); err != nil {
		t.Fatalf("MoveSidebarEntry: %v", err)
	}
	layout, err = h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout after MoveSidebarEntry: %v", err)
	}
	if len(layout) != 2 {
		t.Fatalf("expected 2 entries after move, got %d", len(layout))
	}
	if layout[0].ID != col2.ID {
		t.Errorf("layout[0] = %q, want col2 (%q)", layout[0].ID, col2.ID)
	}
	if layout[1].ID != col1.ID {
		t.Errorf("layout[1] = %q, want col1 (%q)", layout[1].ID, col1.ID)
	}

	// MoveItemToSidebar: col2 にリクエストを追加し、サイドバー先頭に移動
	item, err := h.AddRequest(col2.ID, "", httpdomain.HttpRequest{Name: "R", Method: "GET", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	if err := h.MoveItemToSidebar(col2.ID, item.ID, 0); err != nil {
		t.Fatalf("MoveItemToSidebar: %v", err)
	}

	layout, err = h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout after MoveItemToSidebar: %v", err)
	}
	// [item, col2, col1] (item が position 0 に挿入)
	if len(layout) != 3 {
		t.Fatalf("expected 3 entries after MoveItemToSidebar, got %d", len(layout))
	}
	if layout[0].ID != item.ID || layout[0].Kind != "item" {
		t.Errorf("layout[0] = {Kind:%q ID:%q}, want {Kind:item ID:%q}", layout[0].Kind, layout[0].ID, item.ID)
	}

	// MoveItemToSidebar 後、アイテムは GetRootItems にも現れる
	rootItems := h.GetRootItems()
	found := false
	for _, ri := range rootItems {
		if ri.ID == item.ID {
			found = true
		}
	}
	if !found {
		t.Error("item not found in GetRootItems after MoveItemToSidebar")
	}
}

// TestHTTP_PersistenceRoundTrip は同一ディレクトリで 2 回目の Handler 作成後にデータが復元されることを確認する。
func TestHTTP_PersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// 1 回目: コレクション作成・名前変更・リクエスト追加
	h1 := newHTTPHandlerWithDir(t, dir)
	col, err := h1.CreateCollection("PersistCol")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if err := h1.RenameCollection(col.ID, "RenamedCol"); err != nil {
		t.Fatalf("RenameCollection: %v", err)
	}
	if _, err := h1.AddRequest(col.ID, "", httpdomain.HttpRequest{Name: "Req", Method: "GET", URL: "http://example.com"}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	// 2 回目: 同一ディレクトリから Handler を再作成してデータを確認
	h2 := newHTTPHandlerWithDir(t, dir)
	cols := h2.GetCollections()

	found := false
	for _, c := range cols {
		if c.ID == col.ID {
			found = true
			if c.Name != "RenamedCol" {
				t.Errorf("persisted Name = %q, want RenamedCol", c.Name)
			}
			if len(c.Items) != 1 {
				t.Errorf("expected 1 item after reload, got %d", len(c.Items))
			}
		}
	}
	if !found {
		t.Errorf("collection %q not found after reload", col.ID)
	}
}

// TestHTTP_SendRequest_DisabledHeaderExcluded は Enabled:false のヘッダーがリクエストに含まれないことを確認する。
func TestHTTP_SendRequest_DisabledHeaderExcluded(t *testing.T) {
	var gotDisabled, gotEnabled string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDisabled = r.Header.Get("X-Disabled")
		gotEnabled = r.Header.Get("X-Enabled")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest(httpdomain.HttpRequest{
		Method: "GET",
		URL:    srv.URL,
		Headers: []httpdomain.KeyValuePair{
			{Key: "X-Disabled", Value: "should-not-appear", Enabled: false},
			{Key: "X-Enabled", Value: "should-appear", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if gotDisabled != "" {
		t.Errorf("disabled header X-Disabled was sent with value %q", gotDisabled)
	}
	if gotEnabled != "should-appear" {
		t.Errorf("enabled header X-Enabled = %q, want should-appear", gotEnabled)
	}
}

// TestHTTP_SendRequest_DisabledParamExcluded は Enabled:false のパラメータが URL クエリに含まれないことを確認する。
func TestHTTP_SendRequest_DisabledParamExcluded(t *testing.T) {
	var gotDisabled, gotEnabled string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDisabled = r.URL.Query().Get("disabled")
		gotEnabled = r.URL.Query().Get("enabled")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest(httpdomain.HttpRequest{
		Method: "GET",
		URL:    srv.URL,
		Params: []httpdomain.KeyValuePair{
			{Key: "disabled", Value: "should-not-appear", Enabled: false},
			{Key: "enabled", Value: "should-appear", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if gotDisabled != "" {
		t.Errorf("disabled param was sent with value %q", gotDisabled)
	}
	if gotEnabled != "should-appear" {
		t.Errorf("enabled param = %q, want should-appear", gotEnabled)
	}
}

// TestHTTP_SendRequest_InvalidMethod は無効な HTTP メソッドで ValidationError が返ることを確認する。
func TestHTTP_SendRequest_InvalidMethod(t *testing.T) {
	h := newHTTPHandler(t)
	_, err := h.SendRequest(httpdomain.HttpRequest{
		Method: "INVALID",
		URL:    "http://example.com",
	})
	if err == nil {
		t.Error("expected error for invalid HTTP method, got nil")
	}
}

// TestHTTP_SendRequest_Auth は Basic 認証・Bearer トークン認証がリクエストヘッダーに正しく設定されることを確認する。
func TestHTTP_SendRequest_Auth(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		var gotUser, gotPass string
		var gotOK bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser, gotPass, gotOK = r.BasicAuth()
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		h := newHTTPHandler(t)
		_, err := h.SendRequest(httpdomain.HttpRequest{
			Method: "GET",
			URL:    srv.URL,
			Auth:   httpdomain.RequestAuth{Type: "basic", Username: "user", Password: "pass"},
		})
		if err != nil {
			t.Fatalf("SendRequest: %v", err)
		}
		if !gotOK {
			t.Error("BasicAuth not present in request")
		}
		if gotUser != "user" || gotPass != "pass" {
			t.Errorf("BasicAuth = (%q, %q), want (user, pass)", gotUser, gotPass)
		}
	})

	t.Run("bearer", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		h := newHTTPHandler(t)
		_, err := h.SendRequest(httpdomain.HttpRequest{
			Method: "GET",
			URL:    srv.URL,
			Auth:   httpdomain.RequestAuth{Type: "bearer", Token: "mytoken123"},
		})
		if err != nil {
			t.Fatalf("SendRequest: %v", err)
		}
		if gotAuth != "Bearer mytoken123" {
			t.Errorf("Authorization = %q, want Bearer mytoken123", gotAuth)
		}
	})
}

// TestHTTP_SendRequest_BodyTypes は各ボディタイプで Content-Type ヘッダーが自動付与されることを確認する。
func TestHTTP_SendRequest_BodyTypes(t *testing.T) {
	tests := []struct {
		bodyType    string
		content     string
		wantCT      string
	}{
		{"json", `{"k":"v"}`, "application/json"},
		{"text", "hello text", "text/plain"},
		{"form-urlencoded", "k=v", "application/x-www-form-urlencoded"},
	}

	for _, tc := range tests {
		t.Run(tc.bodyType, func(t *testing.T) {
			var gotCT string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotCT = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			h := newHTTPHandler(t)
			_, err := h.SendRequest(httpdomain.HttpRequest{
				Method: "POST",
				URL:    srv.URL,
				Body: httpdomain.RequestBody{
					Type:     tc.bodyType,
					Contents: map[string]string{tc.bodyType: tc.content},
				},
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if !strings.HasPrefix(gotCT, tc.wantCT) {
				t.Errorf("Content-Type = %q, want prefix %q", gotCT, tc.wantCT)
			}
		})
	}
}

// TestHTTP_SendRequest_UnreachableServer は閉じた httptest.Server へのリクエストで error が返ることを確認する。
func TestHTTP_SendRequest_UnreachableServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Close() // サーバーを先に閉じる

	h := newHTTPHandler(t)
	_, err := h.SendRequest(httpdomain.HttpRequest{
		Method: "GET",
		URL:    srv.URL,
	})
	if err == nil {
		t.Error("expected error when server is closed, got nil")
	}
}

// TestHTTP_SendRequest_Timeout は TimeoutSec が短い場合に応答が遅いサーバーへのリクエストがタイムアウトすることを確認する。
func TestHTTP_SendRequest_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// リクエストがタイムアウトするまでブロック
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest(httpdomain.HttpRequest{
		Method:   "GET",
		URL:      srv.URL,
		Settings: httpdomain.RequestSettings{TimeoutSec: 1},
	})
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// TestHTTP_SendRequest_Concurrent は複数 goroutine から並行して SendRequest を呼んでも安全であることを確認する。
func TestHTTP_SendRequest_Concurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	const n = 5
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.SendRequest(httpdomain.HttpRequest{
				Method: "GET",
				URL:    srv.URL,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("SendRequest concurrent error: %v", err)
		}
	}
}

// TestHTTP_DeleteCollection_NotFound は存在しない collectionID で NotFoundError が返ることを確認する。
func TestHTTP_DeleteCollection_NotFound(t *testing.T) {
	h := newHTTPHandler(t)
	if err := h.DeleteCollection("nonexistent-id"); err == nil {
		t.Error("expected error for nonexistent collection, got nil")
	}
}

// TestHTTP_AddRequest_AfterDeleteCollection は DeleteCollection 後に同じ collectionID で AddRequest を呼ぶと error が返ることを確認する。
func TestHTTP_AddRequest_AfterDeleteCollection(t *testing.T) {
	h := newHTTPHandler(t)

	col, err := h.CreateCollection("Temp")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if err := h.DeleteCollection(col.ID); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}

	_, err = h.AddRequest(col.ID, "", httpdomain.HttpRequest{Name: "R", Method: "GET", URL: "http://example.com"})
	if err == nil {
		t.Error("expected error after collection deleted, got nil")
	}
}

// TestHTTP_CorruptStorage はストレージの JSON ファイルが不正な状態で newHTTPHandlerWithDir を呼ぶとエラーになることを確認する。
func TestHTTP_CorruptStorage(t *testing.T) {
	dir := t.TempDir()

	// 壊れた JSON ファイルをストレージディレクトリに配置する
	corruptFile := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corruptFile, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// NewCollectionService の Load() が失敗することを確認する（DI を手動で組み立てる）
	repo, err := infra.NewJSONStore(dir, func(c *httpdomain.Collection) string { return c.ID })
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	layoutRepo := httpinfra.NewSidebarLayoutRepository(filepath.Join(dir, "layout.json"))
	_, err = httpapp.NewCollectionService(repo, layoutRepo)
	if err == nil {
		t.Error("expected error for corrupt storage, got nil")
	}
}
