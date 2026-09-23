//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/f0reth/Wirexa/internal/adapters"
	httpapp "github.com/f0reth/Wirexa/internal/application/http"
	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
	httpinfra "github.com/f0reth/Wirexa/internal/infrastructure/http"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// newHTTPHandlerWithDir は指定ディレクトリから HTTPHandler を組み立てる（永続化テスト用）。
// コレクションは dir/collections/ サブディレクトリに保存し、sidebar_layout.json と混在させない。
func newHTTPHandlerWithDir(t *testing.T, dir string) *adapters.HTTPHandler {
	t.Helper()
	return buildHTTPHandler(t, dir, nil)
}

// newHTTPHandlerWithDialog は OpenFilePicker が dialog の選択結果を返す HTTPHandler を組み立てる。
func newHTTPHandlerWithDialog(t *testing.T, dialog adapters.FileDialog) *adapters.HTTPHandler {
	t.Helper()
	return buildHTTPHandler(t, t.TempDir(), dialog)
}

// fileDialog は統合テスト用の FileDialog。OpenFile はユーザーが path を選んだものとして返す。
type fileDialog struct{ path string }

func (d *fileDialog) OpenFile(context.Context, runtime.OpenDialogOptions) (string, error) {
	return d.path, nil
}

func (d *fileDialog) SaveFile(context.Context, runtime.SaveDialogOptions) (string, error) {
	return "", nil
}

func buildHTTPHandler(t *testing.T, dir string, dialog adapters.FileDialog) *adapters.HTTPHandler {
	t.Helper()
	collDir := filepath.Join(dir, "collections")
	repo, err := httpinfra.NewCollectionRepository(collDir, nil)
	if err != nil {
		t.Fatalf("NewCollectionRepository: %v", err)
	}
	layoutRepo := httpinfra.NewSidebarLayoutRepository(filepath.Join(dir, "sidebar_layout.json"))
	collSvc, err := httpapp.NewCollectionService(repo, layoutRepo, testutil.NoopLogger{})
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	files := httpinfra.NewFileRegistry()
	netClient := httpinfra.NewNetClient(files, filepath.Join(dir, "http-sessions"))
	t.Cleanup(netClient.Cleanup)
	reqSvc := httpapp.NewHTTPRequestService(context.Background(), netClient, testutil.NoopLogger{})
	h := &adapters.HTTPHandler{}
	adapters.SetupHTTPHandler(context.Background(), h, adapters.HTTPHandlerDeps{
		ReqSvc:    reqSvc,
		CollSvc:   collSvc,
		ItemSvc:   collSvc,
		Responses: netClient.Responses(),
		Files:     files,
		Dialog:    dialog,
	})
	return h
}

// newHTTPHandler は統合テスト用に HTTPHandler を DI で組み立てる。
func newHTTPHandler(t *testing.T) *adapters.HTTPHandler {
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
	req := httpdomain.HTTPRequest{Name: "GET example", Method: "GET", URL: "http://example.com"}
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
			resp, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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

// TestHTTP_SendRequest_4xx5xx はエラーステータスが HTTPResponse.StatusCode に反映されることを確認する。
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
			resp, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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

// TestHTTP_SendRequest_DuplicateRequestHeaders は同名ヘッダーが複数指定された場合に全て送信されることを確認する。
func TestHTTP_SendRequest_DuplicateRequestHeaders(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Values("X-Dup")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
		Method: "GET",
		URL:    srv.URL,
		Headers: []httpdomain.KeyValuePair{
			{Key: "X-Dup", Value: "one", Enabled: true},
			{Key: "X-Dup", Value: "two", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	want := []string{"one", "two"}
	if !slices.Equal(got, want) {
		t.Errorf("X-Dup headers = %v, want %v", got, want)
	}
}

// TestHTTP_SendRequest_MultiValueResponseHeaders は Set-Cookie のような複数値レスポンスヘッダーが
// 先頭 1 件に潰されず全て保持されることを確認する。
func TestHTTP_SendRequest_MultiValueResponseHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "a=1; Path=/")
		w.Header().Add("Set-Cookie", "b=2; Path=/")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	resp, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	want := []string{"a=1; Path=/", "b=2; Path=/"}
	if !slices.Equal(resp.Headers["Set-Cookie"], want) {
		t.Errorf("Set-Cookie = %v, want %v", resp.Headers["Set-Cookie"], want)
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
	// キャンセルは送信時の execution ID で行う。保存済みリクエストの ID とは無関係。
	const executionID = "exec-cancel"
	done := make(chan error, 1)
	go func() {
		_, err := h.SendRequest(executionID, httpdomain.HTTPRequest{
			ID:     "saved-1",
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
	h.CancelRequest(executionID)

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error after cancel, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SendRequest did not return after cancel")
	}
}

// TestHTTP_CancelRequest_SameSavedRequestInParallel は同じ保存済みリクエストを 2 本並行実行し、
// 片方の execution ID だけをキャンセルしても他方は完走することを確認する。
func TestHTTP_CancelRequest_SameSavedRequestInParallel(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("hang") == "1" {
			close(blocked)
			<-r.Context().Done() // キャンセルされるまでブロック
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	// 2 本とも同じ保存済みリクエスト (同じ ID) を送る。
	saved := httpdomain.HTTPRequest{ID: "saved-1", Method: "GET", URL: srv.URL}

	canceled := make(chan error, 1)
	go func() {
		hang := saved
		hang.URL = srv.URL + "?hang=1"
		_, err := h.SendRequest("exec-hang", hang)
		canceled <- err
	}()

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive the first request in time")
	}

	// 同じ保存済みリクエストでも execution ID が違えば拒否されず、独立して完走する。
	resp, err := h.SendRequest("exec-ok", saved)
	if err != nil {
		t.Fatalf("parallel send of the same saved request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	h.CancelRequest("exec-hang")
	select {
	case err := <-canceled:
		if err == nil {
			t.Fatal("expected the canceled execution to fail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceling one execution ID did not reach its request")
	}
}

// TestHTTP_UpdateRequest は AddRequest → UpdateRequest → GetCollections でリクエスト内容が更新されることを確認する。
func TestHTTP_UpdateRequest(t *testing.T) {
	h := newHTTPHandler(t)
	col, _ := h.CreateCollection("C")

	item, err := h.AddRequest(col.ID, "", httpdomain.HTTPRequest{Name: "Req", Method: "GET", URL: "http://old.example.com"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	if err := h.UpdateRequest(col.ID, httpdomain.HTTPRequest{ID: item.ID, Name: "Req", Method: "POST", URL: "http://new.example.com"}); err != nil {
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

	item, err := h.AddRequest(col.ID, "", httpdomain.HTTPRequest{Name: "OldName", Method: "GET", URL: "http://example.com"})
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

	item, err := h.AddRequest(col.ID, "", httpdomain.HTTPRequest{Name: "Req", Method: "GET", URL: "http://example.com"})
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

// TestHTTP_MoveItem は AddRequest → MoveItem（コレクション間移動）が正しく動くことを確認する。
func TestHTTP_MoveItem(t *testing.T) {
	h := newHTTPHandler(t)

	col1, _ := h.CreateCollection("Source")
	col2, _ := h.CreateCollection("Target")

	item, err := h.AddRequest(col1.ID, "", httpdomain.HTTPRequest{Name: "Req", Method: "GET", URL: "http://example.com"})
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
	item, err := h.AddRequest(col2.ID, "", httpdomain.HTTPRequest{Name: "R", Method: "GET", URL: "http://example.com"})
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
	if _, err := h1.AddRequest(col.ID, "", httpdomain.HTTPRequest{Name: "Req", Method: "GET", URL: "http://example.com"}); err != nil {
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
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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
		_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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
		_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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
		name   string
		body   httpdomain.RequestBody
		wantCT string
	}{
		{
			name:   "json",
			body:   contentsBody("json", `{"k":"v"}`),
			wantCT: "application/json",
		},
		{
			name:   "text",
			body:   contentsBody("text", "hello text"),
			wantCT: "text/plain",
		},
		{
			name: "form-urlencoded",
			body: httpdomain.RequestBody{
				Type:           httpdomain.BodyTypeFormURLEncoded,
				FormURLEncoded: []httpdomain.FormRow{{Key: "k", Value: "v", Enabled: true}},
			},
			wantCT: "application/x-www-form-urlencoded",
		},
		{
			name: "form-data",
			body: httpdomain.RequestBody{
				Type:     httpdomain.BodyTypeFormData,
				FormData: []httpdomain.FormRow{{Key: "k", Value: "v", Enabled: true}},
			},
			wantCT: "multipart/form-data; boundary=",
		},
		// 行フィールド導入前に保存されたリクエストは Contents の文字列から移行される。
		{
			name:   "form-urlencoded (旧データ)",
			body:   contentsBody(httpdomain.BodyTypeFormURLEncoded, "k=v"),
			wantCT: "application/x-www-form-urlencoded",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotCT string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotCT = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			h := newHTTPHandler(t)
			_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
				Method: "POST",
				URL:    srv.URL,
				Body:   tc.body,
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

func contentsBody(bodyType, content string) httpdomain.RequestBody {
	return httpdomain.RequestBody{
		Type:     bodyType,
		Contents: map[string]string{bodyType: content},
	}
}

// formRows は送信ボディの検証に使う代表的な行を返す。
// 無効行と空キー行はワイヤに載ってはならない。
func formRows() []httpdomain.FormRow {
	return []httpdomain.FormRow{
		{Key: "z", Value: "first", Enabled: true},
		{Key: "a", Value: "second", Enabled: true},
		{Key: "disabled", Value: "no", Enabled: false},
		{Key: "", Value: "orphan", Enabled: true},
	}
}

// TestHTTP_SendRequest_FormURLEncodedBody は urlencoded ボディが行順を保ち、
// 無効行・空キー行を除外して送信されることを確認する。
func TestHTTP_SendRequest_FormURLEncodedBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
		Method: "POST",
		URL:    srv.URL,
		Body: httpdomain.RequestBody{
			Type:           httpdomain.BodyTypeFormURLEncoded,
			FormURLEncoded: formRows(),
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	// url.Values.Encode() を使うとキー名でソートされ "a=second&z=first" になる。
	// UI の行順が送信順であることを保証する。
	if want := "z=first&a=second"; gotBody != want {
		t.Errorf("body = %q, want %q", gotBody, want)
	}
}

// TestHTTP_SendRequest_FormDataBody は form-data が multipart として組み立てられ、
// サーバー側で各フィールドが読めることを確認する。
func TestHTTP_SendRequest_FormDataBody(t *testing.T) {
	var gotForm map[string][]string
	var parseErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if parseErr = r.ParseMultipartForm(1 << 20); parseErr == nil {
			gotForm = r.MultipartForm.Value
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
		Method: "POST",
		URL:    srv.URL,
		Body: httpdomain.RequestBody{
			Type:     httpdomain.BodyTypeFormData,
			FormData: formRows(),
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if parseErr != nil {
		t.Fatalf("ParseMultipartForm: %v", parseErr)
	}

	if got := gotForm["z"]; len(got) != 1 || got[0] != "first" {
		t.Errorf(`field "z" = %v, want ["first"]`, got)
	}
	if got := gotForm["a"]; len(got) != 1 || got[0] != "second" {
		t.Errorf(`field "a" = %v, want ["second"]`, got)
	}
	if _, ok := gotForm["disabled"]; ok {
		t.Error("無効行が送信されている")
	}
	if _, ok := gotForm[""]; ok {
		t.Error("空キー行が送信されている")
	}
}

// TestHTTP_SendRequest_FormDataKinds は json / file 行が multipart のパートとして
// 送られ、Content-Type 未指定なら kind から自動付与されることを確認する。
func TestHTTP_SendRequest_FormDataKinds(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "upload.json")
	if err := os.WriteFile(filePath, []byte(`{"from":"file"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var gotForm *multipart.Form
	var parseErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if parseErr = r.ParseMultipartForm(1 << 20); parseErr == nil {
			gotForm = r.MultipartForm
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// ファイルはダイアログで選択されたものとして token を得る。パスは RPC に渡らない。
	h := newHTTPHandlerWithDialog(t, &fileDialog{path: filePath})
	picked, err := h.OpenFilePicker("")
	if err != nil {
		t.Fatalf("OpenFilePicker: %v", err)
	}
	_, err = h.SendRequest("exec-1", httpdomain.HTTPRequest{
		Method: "POST",
		URL:    srv.URL,
		Body: httpdomain.RequestBody{
			Type: httpdomain.BodyTypeFormData,
			FormData: []httpdomain.FormRow{
				{Key: "plain", Value: "text value", Kind: httpdomain.FormRowKindText, Enabled: true},
				{Key: "meta", Value: `{"k":"v"}`, Kind: httpdomain.FormRowKindJSON, Enabled: true},
				{Key: "custom", Value: "a,b", Kind: httpdomain.FormRowKindText, ContentType: "text/csv", Enabled: true},
				{Key: "doc", File: httpdomain.FileReference{Token: picked.Token}, Kind: httpdomain.FormRowKindFile, Enabled: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if parseErr != nil {
		t.Fatalf("ParseMultipartForm: %v", parseErr)
	}

	if got := gotForm.Value["plain"]; len(got) != 1 || got[0] != "text value" {
		t.Errorf(`field "plain" = %v, want ["text value"]`, got)
	}
	if got := gotForm.Value["meta"]; len(got) != 1 || got[0] != `{"k":"v"}` {
		t.Errorf(`field "meta" = %v, want [{"k":"v"}]`, got)
	}
	if got := gotForm.Value["custom"]; len(got) != 1 || got[0] != "a,b" {
		t.Errorf(`field "custom" = %v, want ["a,b"]`, got)
	}

	// file 行はフィールドではなくファイルパートとして届く。
	files := gotForm.File["doc"]
	if len(files) != 1 {
		t.Fatalf(`file "doc" = %v, want 1 part`, files)
	}
	if files[0].Filename != "upload.json" {
		t.Errorf("filename = %q, want upload.json", files[0].Filename)
	}
	f, err := files[0].Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(content) != `{"from":"file"}` {
		t.Errorf("file content = %q, want the file on disk", content)
	}

	// 具体的な MIME は OS 依存なので、拡張子から判定できたことだけを見る。
	if ct := files[0].Header.Get("Content-Type"); ct == "" || ct == "application/octet-stream" {
		t.Errorf("file Content-Type = %q, want a type guessed from .json", ct)
	}
}

// TestHTTP_SendRequest_FormDataOverridesUserContentType は、ユーザーが Content-Type を
// 指定しても multipart の boundary 付きヘッダーが優先されることを確認する。
// boundary は送信時に採番するためユーザーには書けず、尊重するとボディが解釈不能になる。
func TestHTTP_SendRequest_FormDataOverridesUserContentType(t *testing.T) {
	var gotCT string
	var parseErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		parseErr = r.ParseMultipartForm(1 << 20)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
		Method:  "POST",
		URL:     srv.URL,
		Headers: []httpdomain.KeyValuePair{{Key: "Content-Type", Value: "text/plain", Enabled: true}},
		Body: httpdomain.RequestBody{
			Type:     httpdomain.BodyTypeFormData,
			FormData: []httpdomain.FormRow{{Key: "k", Value: "v", Enabled: true}},
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data; boundary=") {
		t.Errorf("Content-Type = %q, want multipart with boundary", gotCT)
	}
	if parseErr != nil {
		t.Fatalf("ParseMultipartForm: %v", parseErr)
	}
}

// TestHTTP_SendRequest_FormURLEncodedKeepsUserContentType は urlencoded では
// ユーザー指定の Content-Type を尊重する（form-data のような上書きをしない）ことを確認する。
func TestHTTP_SendRequest_FormURLEncodedKeepsUserContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
		Method:  "POST",
		URL:     srv.URL,
		Headers: []httpdomain.KeyValuePair{{Key: "Content-Type", Value: "application/custom", Enabled: true}},
		Body: httpdomain.RequestBody{
			Type:           httpdomain.BodyTypeFormURLEncoded,
			FormURLEncoded: []httpdomain.FormRow{{Key: "k", Value: "v", Enabled: true}},
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if gotCT != "application/custom" {
		t.Errorf("Content-Type = %q, want %q", gotCT, "application/custom")
	}
}

// TestHTTP_SendRequest_UnreachableServer は閉じた httptest.Server へのリクエストで error が返ることを確認する。
func TestHTTP_SendRequest_UnreachableServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Close() // サーバーを先に閉じる

	h := newHTTPHandler(t)
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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
	_, err := h.SendRequest("exec-1", httpdomain.HTTPRequest{
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

	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 同じリクエストの並行送信でも execution ID は送信ごとに別になる。
			_, err := h.SendRequest(fmt.Sprintf("exec-%d", i), httpdomain.HTTPRequest{
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

	_, err = h.AddRequest(col.ID, "", httpdomain.HTTPRequest{Name: "R", Method: "GET", URL: "http://example.com"})
	if err == nil {
		t.Error("expected error after collection deleted, got nil")
	}
}

// TestHTTP_CorruptStorage はストレージの JSON ファイルが不正でも起動を継続し、
// 破損ファイルが退避されることを確認する。
func TestHTTP_CorruptStorage(t *testing.T) {
	dir := t.TempDir()

	// 壊れた JSON ファイルをストレージディレクトリに配置する
	corruptFile := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corruptFile, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// 破損ファイルは退避・スキップされ、NewCollectionService は成功する（DI を手動で組み立てる）
	repo, err := httpinfra.NewCollectionRepository(dir, nil)
	if err != nil {
		t.Fatalf("NewCollectionRepository: %v", err)
	}
	layoutRepo := httpinfra.NewSidebarLayoutRepository(filepath.Join(dir, "layout.json"))
	if _, err = httpapp.NewCollectionService(repo, layoutRepo, testutil.NoopLogger{}); err != nil {
		t.Fatalf("NewCollectionService should tolerate corrupt storage: %v", err)
	}

	// 破損ファイルは .corrupt へ退避されている
	if _, statErr := os.Stat(corruptFile); !os.IsNotExist(statErr) {
		t.Error("corrupt.json should have been quarantined")
	}
	if _, statErr := os.Stat(corruptFile + ".corrupt"); statErr != nil {
		t.Errorf("corrupt.json.corrupt should exist: %v", statErr)
	}
}

// TestHTTP_UnloadableRootIsNotOverwritten は __root__.json が存在するのに読み込めなかった場合、
// 空の root で作り直して上書きせずに起動することを確認する。
// 権限やロックで読めない状態は Windows で安定して作れないため、JSONStore.Load が
// 退避せずに読み飛ばす「JSON としては正しいが中身の ID が不正」なファイルで代用する。
func TestHTTP_UnloadableRootIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "collections", httpdomain.RootCollectionID+".json")
	writeCollectionJSON(t, dir, httpdomain.RootCollectionID, `{"id":"../escape","name":"x","items":[]}`)
	original, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	assertRootUntouched := func(when string) {
		t.Helper()
		got, rerr := os.ReadFile(rootPath)
		if rerr != nil {
			t.Fatalf("%s: ReadFile: %v", when, rerr)
		}
		if !bytes.Equal(got, original) {
			t.Errorf("%s: __root__.json was overwritten: %s", when, got)
		}
	}

	h := newHTTPHandlerWithDir(t, dir)
	assertRootUntouched("after startup")
	if items := h.GetRootItems(); len(items) != 0 {
		t.Errorf("GetRootItems = %v, want empty", items)
	}
	if _, err = h.AddRequest(httpdomain.RootCollectionID, "", httpdomain.HTTPRequest{Name: "R", Method: "GET"}); err == nil {
		t.Error("AddRequest to the unavailable root should fail")
	}
	assertRootUntouched("after AddRequest")

	// 他のコレクションは通常どおり作成でき、再起動後も読み込める。
	col, err := h.CreateCollection("Other")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	h2 := newHTTPHandlerWithDir(t, dir)
	if cols := h2.GetCollections(); len(cols) != 1 || cols[0].ID != col.ID {
		t.Errorf("collections after restart = %v, want [%s]", cols, col.ID)
	}
	assertRootUntouched("after restart")
}

// TestHTTP_SendRequest_FileBodyViaDialogToken はダイアログで選んだファイルが token 経由で送れ、
// filename と Content-Type も frontend の値ではなく選択時の値になることを確認する。
func TestHTTP_SendRequest_FileBodyViaDialogToken(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(filePath, []byte("file bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var gotBody, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody, gotType = string(data), r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandlerWithDialog(t, &fileDialog{path: filePath})
	picked, err := h.OpenFilePicker("")
	if err != nil {
		t.Fatalf("OpenFilePicker: %v", err)
	}
	if picked.Name != "payload.txt" || strings.Contains(picked.Token, filePath) {
		t.Fatalf("selected = %+v, want the basename and an opaque token", picked)
	}

	_, err = h.SendRequest("exec-file", httpdomain.HTTPRequest{
		ID:     "saved-file",
		Method: "POST",
		URL:    srv.URL,
		Body: httpdomain.RequestBody{
			Type: httpdomain.BodyTypeFile,
			File: httpdomain.FileReference{Token: picked.Token, Name: "spoofed.exe", ContentType: "application/x-spoofed"},
		},
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if gotBody != "file bytes" {
		t.Errorf("body = %q, want the selected file", gotBody)
	}
	if gotType != picked.ContentType {
		t.Errorf("Content-Type = %q, want %q from the selection", gotType, picked.ContentType)
	}
}

// TestHTTP_SendRequest_RawPathsAreNeverRead は RPC 引数に任意の絶対パスや偽の token を渡しても、
// そのファイルが読まれて外部へ送られないことを確認する。
func TestHTTP_SendRequest_RawPathsAreNeverRead(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(data))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const forged = "00112233445566778899aabbccddeeff"
	fileRow := func(ref httpdomain.FileReference) []httpdomain.FormRow {
		return []httpdomain.FormRow{{Key: "f", Kind: httpdomain.FormRowKindFile, File: ref, Enabled: true}}
	}
	tests := []struct {
		wantErr error
		name    string
		body    httpdomain.RequestBody
	}{
		{name: "file body の Contents に生のパス", body: httpdomain.RequestBody{Type: httpdomain.BodyTypeFile, Contents: map[string]string{"file": secret}}},
		{name: "form-data 行に名前だけの参照", body: httpdomain.RequestBody{Type: httpdomain.BodyTypeFormData, FormData: fileRow(httpdomain.FileReference{Name: "secret.txt"})}, wantErr: httpdomain.ErrFileAccessDenied},
		{name: "file body に偽の token", body: httpdomain.RequestBody{Type: httpdomain.BodyTypeFile, File: httpdomain.FileReference{Token: forged}}, wantErr: httpdomain.ErrFileAccessDenied},
		{name: "form-data 行に偽の token", body: httpdomain.RequestBody{Type: httpdomain.BodyTypeFormData, FormData: fileRow(httpdomain.FileReference{Token: forged})}, wantErr: httpdomain.ErrFileAccessDenied},
		{name: "token の無い保存済み参照", body: httpdomain.RequestBody{Type: httpdomain.BodyTypeFile, File: httpdomain.FileReference{Name: "secret.txt", NeedsReselect: true}}, wantErr: httpdomain.ErrFileAccessDenied},
	}

	h := newHTTPHandler(t)
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.SendRequest(fmt.Sprintf("exec-%d", i), httpdomain.HTTPRequest{ID: "saved-1", Method: "POST", URL: srv.URL, Body: tc.body})
			if tc.wantErr == nil && err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("SendRequest error = %v, want %v", err, tc.wantErr)
				}
				if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), forged) {
					t.Fatalf("error leaks the path or token: %q", err)
				}
			}
		})
	}

	mu.Lock()
	defer mu.Unlock()
	for _, b := range bodies {
		if strings.Contains(b, "top secret") {
			t.Fatalf("the secret file was sent: %q", b)
		}
	}
}

// writeCollectionJSON は collections/ に手書きの JSON を置く。
// クラッシュが残す状態をディスク上に直接作るために使う。
func writeCollectionJSON(t *testing.T, dir, id, body string) {
	t.Helper()
	collDir := filepath.Join(dir, "collections")
	if err := os.MkdirAll(collDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(collDir, id+".json"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// collectionJSON は1件のリクエストだけを持つコレクションの JSON を返す。
func collectionJSON(id, name, itemID string) string {
	const tmpl = `{"id":%[1]q,"name":%[2]q,"items":[{"type":"request","id":%[3]q,"name":%[3]q,"children":[],
		"request":{"id":%[3]q,"name":%[3]q,"method":"GET","url":"http://example.com","doc":"",
		"headers":[],"params":[],"auth":{"type":"none","username":"","password":"","token":""},
		"settings":{},"body":{"type":"json","contents":{}}}}]}`
	return fmt.Sprintf(tmpl, id, name, itemID)
}

// TestHTTP_RecoversDuplicateItemsOnStartup は、移動の途中でプロセスが消えて
// アイテムが2つのコレクションに残った状態から起動すると、1つへ回収されることを
// 確認する。プロセスを実際に強制終了させる代わりに、クラッシュが残す状態を
// ディスク上に直接作る (この方が OS を問わず決定的に回せる)。
func TestHTTP_RecoversDuplicateItemsOnStartup(t *testing.T) {
	dir := t.TempDir()
	writeCollectionJSON(t, dir, "col-a", collectionJSON("col-a", "Alpha", "dup-item"))
	writeCollectionJSON(t, dir, "col-b", collectionJSON("col-b", "Beta", "dup-item"))

	h := newHTTPHandlerWithDir(t, dir)

	// 残るのはコレクション ID の昇順で最初に現れた col-a 側。
	for _, c := range h.GetCollections() {
		want := 0
		if c.ID == "col-a" {
			want = 1
		}
		if len(c.Items) != want {
			t.Errorf("%s items = %d, want %d", c.ID, len(c.Items), want)
		}
	}

	// ディスク上のコレクションファイルも回収済みになっている。
	h2 := newHTTPHandlerWithDir(t, dir)
	total := 0
	for _, c := range h2.GetCollections() {
		total += len(c.Items)
	}
	if total != 1 {
		t.Errorf("items after reload = %d, want 1 (回収がディスクに永続化されていない)", total)
	}
}

// TestHTTP_ReconcilesSidebarLayoutOnStartup は、実データと食い違うレイアウトが
// 起動時に修復され、ディスク上のファイルも書き換わることを確認する。
func TestHTTP_ReconcilesSidebarLayoutOnStartup(t *testing.T) {
	dir := t.TempDir()
	writeCollectionJSON(t, dir, "col-a", collectionJSON("col-a", "Alpha", "r1"))
	writeCollectionJSON(t, dir, "col-b", collectionJSON("col-b", "Beta", "r2"))
	// 存在しないコレクション ID と重複エントリを含み、col-b が欠けたレイアウト。
	layoutPath := filepath.Join(dir, "sidebar_layout.json")
	layout := `[{"kind":"collection","id":"gone"},{"kind":"collection","id":"col-a"},
		{"kind":"collection","id":"col-a"}]`
	if err := os.WriteFile(layoutPath, []byte(layout), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	h := newHTTPHandlerWithDir(t, dir)
	got, err := h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	want := []httpdomain.SidebarEntry{
		{Kind: "collection", ID: "col-a"},
		{Kind: "collection", ID: "col-b"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("layout = %v, want %v", got, want)
	}

	// ディスク上の sidebar_layout.json も書き換わっている。
	raw, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "gone") {
		t.Errorf("sidebar_layout.json still holds the stale entry:\n%s", raw)
	}
	if !strings.Contains(string(raw), "col-b") {
		t.Errorf("sidebar_layout.json is missing the added collection:\n%s", raw)
	}
}

// TestHTTP_CancelRequest_BeforeSend は送信の登録より前に届いたキャンセルを取りこぼさず、
// リクエストがサーバーへ 1 件も届かないことを確認する。
func TestHTTP_CancelRequest_BeforeSend(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newHTTPHandler(t)
	const executionID = "exec-early-cancel"
	h.CancelRequest(executionID)

	_, err := h.SendRequest(executionID, httpdomain.HTTPRequest{ID: "saved-1", Method: "GET", URL: srv.URL})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendRequest after an early cancel: want context.Canceled, got %v", err)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("canceled request reached the server %d times", got)
	}

	// 墓標は 1 回で消費されるので、同じ execution ID での再送は通常どおり成功する。
	if _, err := h.SendRequest(executionID, httpdomain.HTTPRequest{ID: "saved-1", Method: "GET", URL: srv.URL}); err != nil {
		t.Fatalf("re-send after the tombstone was consumed: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("server hits = %d, want 1", got)
	}
}
