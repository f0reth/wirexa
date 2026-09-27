//go:build integration

package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/f0reth/Wirexa/internal/adapters"
	httpapp "github.com/f0reth/Wirexa/internal/application/http"
	cmndomain "github.com/f0reth/Wirexa/internal/domain"
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

// newHTTPHandlerWithDirAndDialog は dir に保存し、ダイアログの選択結果を dialog が返す HTTPHandler を組み立てる。
// 一時ファイルの場所 (dir/http-sessions) を確かめるテストで使う。
func newHTTPHandlerWithDirAndDialog(t *testing.T, dir string, dialog adapters.FileDialog) *adapters.HTTPHandler {
	t.Helper()
	return buildHTTPHandler(t, dir, dialog)
}

// fileDialog は統合テスト用の FileDialog。OpenFile はユーザーが path を選んだものとして返し、
// SaveFile は savePath (空ならキャンセル) を返して呼び出し回数を数える。
type fileDialog struct {
	path      string
	savePath  string
	saveCalls atomic.Int32
}

func (d *fileDialog) OpenFile(context.Context, runtime.OpenDialogOptions) (string, error) {
	return d.path, nil
}

func (d *fileDialog) SaveFile(context.Context, runtime.SaveDialogOptions) (string, error) {
	d.saveCalls.Add(1)
	return d.savePath, nil
}

// httpFixture は HTTPHandler と、app.go の終了処理を再現するためのサービスをまとめる。
type httpFixture struct {
	h         *adapters.HTTPHandler
	reqSvc    *httpapp.HTTPRequestService
	netClient *httpinfra.NetClient
}

// buildHTTPHandler は HTTPHandler を組み立て、テスト終了時に一時ファイルを回収する。
func buildHTTPHandler(t *testing.T, dir string, dialog adapters.FileDialog) *adapters.HTTPHandler {
	t.Helper()
	f := buildHTTPFixture(t, dir, dialog)
	t.Cleanup(f.netClient.Cleanup)
	return f.h
}

// buildHTTPFixture は app.go の initialize と同じ依存で HTTPHandler を組み立てる。
// NetClient.Cleanup は登録しないので、終了処理の順序やクラッシュ (Cleanup が走らない終了) を
// 再現するテストで使う。回収が必要なら呼び出し側で登録する。
func buildHTTPFixture(t *testing.T, dir string, dialog adapters.FileDialog) httpFixture {
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
	return httpFixture{h: h, reqSvc: reqSvc, netClient: netClient}
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
	if err = h.RenameCollection(col.ID, "Renamed"); err != nil {
		t.Fatalf("RenameCollection: %v", err)
	}
	cols = h.GetCollections()
	if cols[0].Name != "Renamed" {
		t.Errorf("Name = %q, want Renamed", cols[0].Name)
	}

	// 削除
	if err = h.DeleteCollection(col.ID); err != nil {
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
	if req.Method != http.MethodPost {
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

	// MoveSidebarEntry: col1 (Alpha) を末尾の挿入ゾーン（移動前の一覧で position 2）へ移動 → [Beta, Alpha]
	if err = h.MoveSidebarEntry("collection", col1.ID, 2); err != nil {
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
	if err = h.MoveItemToSidebar(col2.ID, item.ID, 0); err != nil {
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
		wg.Go(func() {
			// 同じリクエストの並行送信でも execution ID は送信ごとに別になる。
			_, err := h.SendRequest(fmt.Sprintf("exec-%d", i), httpdomain.HTTPRequest{
				Method: "GET",
				URL:    srv.URL,
			})
			errs <- err
		})
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

// TestHTTP_RootCollection_DeleteAndRenameRejected は予約済みの __root__ をハンドラ経由で
// 削除・リネームできず、__root__.json とサイドバー直下のアイテムが残ることを確認する。
func TestHTTP_RootCollection_DeleteAndRenameRejected(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "collections", httpdomain.RootCollectionID+".json")

	h1 := newHTTPHandlerWithDir(t, dir)
	item, err := h1.AddRequest(httpdomain.RootCollectionID, "", httpdomain.HTTPRequest{Name: "RootReq", Method: "GET", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("AddRequest to __root__: %v", err)
	}
	if err := h1.DeleteCollection(httpdomain.RootCollectionID); err == nil {
		t.Error("DeleteCollection(__root__): expected error, got nil")
	}
	if err := h1.RenameCollection(httpdomain.RootCollectionID, "x"); err == nil {
		t.Error("RenameCollection(__root__): expected error, got nil")
	}
	if _, err := os.Stat(rootPath); err != nil {
		t.Fatalf("__root__.json should remain on disk: %v", err)
	}

	// 同一ディレクトリから読み直してもサイドバー直下のアイテムが残っている。
	h2 := newHTTPHandlerWithDir(t, dir)
	items := h2.GetRootItems()
	if len(items) != 1 || items[0].ID != item.ID {
		t.Errorf("root items after reload = %v, want [%s]", items, item.ID)
	}
}

// TestHTTP_AddRequest_AfterDeleteCollection は DeleteCollection 後に同じ collectionID で AddRequest を呼ぶと error が返ることを確認する。
func TestHTTP_AddRequest_AfterDeleteCollection(t *testing.T) {
	h := newHTTPHandler(t)

	col, err := h.CreateCollection("Temp")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if err = h.DeleteCollection(col.ID); err != nil {
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

// TestHTTP_CorruptSidebarLayout は sidebar_layout.json が壊れていても起動し、
// 壊れたファイルを .corrupt へ退避してコレクションからレイアウトを再生成することを確認する。
func TestHTTP_CorruptSidebarLayout(t *testing.T) {
	dir := t.TempDir()
	layoutPath := filepath.Join(dir, "sidebar_layout.json")

	h1 := newHTTPHandlerWithDir(t, dir)
	alpha, err := h1.CreateCollection("Alpha")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	beta, err := h1.CreateCollection("Beta")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// 名前順と逆に並べ替えておき、再生成でリセットされることを確かめる。
	if err = h1.MoveSidebarEntry("collection", alpha.ID, 2); err != nil {
		t.Fatalf("MoveSidebarEntry: %v", err)
	}

	corrupt := []byte("{ not json")
	if err = os.WriteFile(layoutPath, corrupt, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// 実リポジトリで組み立て直しても起動が止まらない (失敗すれば newHTTPHandlerWithDir が Fatal)。
	h2 := newHTTPHandlerWithDir(t, dir)

	quarantined, err := os.ReadFile(layoutPath + ".corrupt")
	if err != nil {
		t.Fatalf("sidebar_layout.json.corrupt should exist: %v", err)
	}
	if !bytes.Equal(quarantined, corrupt) {
		t.Errorf("quarantined content = %q, want %q", quarantined, corrupt)
	}
	want := []string{alpha.ID, beta.ID}
	assertLayoutIDs := func(what string, layout []httpdomain.SidebarEntry) {
		t.Helper()
		got := make([]string, 0, len(layout))
		for _, e := range layout {
			if e.Kind != "collection" {
				t.Errorf("%s: unexpected entry %+v", what, e)
			}
			got = append(got, e.ID)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}
	saved, err := httpinfra.NewSidebarLayoutRepository(layoutPath).Load()
	if err != nil {
		t.Fatalf("regenerated sidebar_layout.json should be readable: %v", err)
	}
	assertLayoutIDs("regenerated file", saved)
	layout, err := h2.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	assertLayoutIDs("GetSidebarLayout", layout)

	// 再生成したファイルは正常に読めるので、もう一度起動しても退避は起きない。
	newHTTPHandlerWithDir(t, dir)
	extra, err := filepath.Glob(layoutPath + ".corrupt.*")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(extra) != 0 {
		t.Errorf("unexpected second quarantine: %v", extra)
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

// collectionJSON は itemIDs のリクエストをルート直下に並べたコレクションの JSON を返す。
func collectionJSON(id, name string, itemIDs ...string) string {
	const itemTmpl = `{"type":"request","id":%[1]q,"name":%[1]q,"children":[],
		"request":{"id":%[1]q,"name":%[1]q,"method":"GET","url":"http://example.com","doc":"",
		"headers":[],"params":[],"auth":{"type":"none","username":"","password":"","token":""},
		"settings":{},"body":{"type":"json","contents":{}}}}`
	items := make([]string, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		items = append(items, fmt.Sprintf(itemTmpl, itemID))
	}
	return fmt.Sprintf(`{"id":%q,"name":%q,"items":[%s]}`, id, name, strings.Join(items, ","))
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

// TestHTTP_AddRequest_IgnoresCallerItemID は RPC から既存アイテムと同じ ID を渡されても
// 新しい ID を採番し、再起動時の重複回収で既存のアイテムが消えないことを確認する。
func TestHTTP_AddRequest_IgnoresCallerItemID(t *testing.T) {
	dir := t.TempDir()
	h1 := newHTTPHandlerWithDir(t, dir)
	colA, err := h1.CreateCollection("A")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	colB, err := h1.CreateCollection("B")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	existing, err := h1.AddRequest(colA.ID, "", httpdomain.HTTPRequest{Name: "Existing", Method: "GET"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	dup, err := h1.AddRequest(colB.ID, "", httpdomain.HTTPRequest{ID: existing.ID, Name: "Dup", Method: "GET"})
	if err != nil {
		t.Fatalf("AddRequest with an existing ID: %v", err)
	}
	if dup.ID == existing.ID || dup.Request.ID == existing.ID {
		t.Fatalf("AddRequest reused the caller's ID %q", existing.ID)
	}

	// 再起動後も両方のアイテムがそれぞれのコレクションに残る。
	h2 := newHTTPHandlerWithDir(t, dir)
	want := map[string]string{colA.ID: existing.ID, colB.ID: dup.ID}
	for _, c := range h2.GetCollections() {
		if len(c.Items) != 1 || c.Items[0].ID != want[c.ID] {
			t.Errorf("%s items after restart = %v, want [%s]", c.Name, c.Items, want[c.ID])
		}
	}
}

// truncatedBody は MaxResponseBodyMB: 1 で切り詰められる大きさ (1 MiB + 1 KiB) の本文を返す。
func truncatedBody() []byte {
	return bytes.Repeat([]byte("0123456789abcdef"), (1<<20+1<<10)/16)
}

// newBodyServer は body を text/plain で返すサーバーを起動する。
func newBodyServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// sendTruncated は MaxResponseBodyMB: 1 で url へ送り、本文が切り詰められたことを確かめる。
func sendTruncated(t *testing.T, h *adapters.HTTPHandler, executionID, url string) httpdomain.HTTPResponse {
	t.Helper()
	resp, err := h.SendRequest(executionID, httpdomain.HTTPRequest{
		Method:   "GET",
		URL:      url,
		Settings: httpdomain.RequestSettings{MaxResponseBodyMB: 1},
	})
	if err != nil {
		t.Fatalf("SendRequest(%s): %v", executionID, err)
	}
	if !resp.BodyTruncated {
		t.Fatalf("SendRequest(%s): BodyTruncated = false, want true", executionID)
	}
	return resp
}

// spillFiles は dir/http-sessions 配下にある、切り詰めたレスポンスの一時ファイルを返す。
func spillFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "http-sessions", "wirexa-http-*", "response-*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	return files
}

// TestHTTP_TruncatedResponse_SaveAndDiscard は、切り詰めたレスポンスの全文が一時ファイルへ退避され、
// Handler と NetClient が共有する ResponseStore を通して保存・破棄できることを確認する。
func TestHTTP_TruncatedResponse_SaveAndDiscard(t *testing.T) {
	body := truncatedBody()
	srv := newBodyServer(t, body)
	dir := t.TempDir()
	dialog := &fileDialog{}
	h := newHTTPHandlerWithDirAndDialog(t, dir, dialog)

	resp := sendTruncated(t, h, "exec-save", srv.URL)
	if len(resp.Body) != 1<<20 {
		t.Errorf("len(Body) = %d, want %d", len(resp.Body), 1<<20)
	}
	if resp.Size != int64(len(body)) {
		t.Errorf("Size = %d, want the full length %d", resp.Size, len(body))
	}
	spilled := spillFiles(t, dir)
	if len(spilled) != 1 {
		t.Fatalf("temp files = %v, want 1", spilled)
	}

	// ダイアログのキャンセルでは保存せず、一時ファイルを残して再保存できるようにする。
	saved, err := h.SaveResponseBody("exec-save")
	if err != nil || saved {
		t.Fatalf("SaveResponseBody (canceled) = (%v, %v), want (false, nil)", saved, err)
	}
	if _, err = os.Stat(spilled[0]); err != nil {
		t.Fatalf("temp file should be kept after cancel: %v", err)
	}

	// 保存すると全文が保存先へコピーされ、元の一時ファイルは削除される。
	dialog.savePath = filepath.Join(t.TempDir(), "out.txt")
	saved, err = h.SaveResponseBody("exec-save")
	if err != nil || !saved {
		t.Fatalf("SaveResponseBody = (%v, %v), want (true, nil)", saved, err)
	}
	got, err := os.ReadFile(dialog.savePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("saved %d bytes, want the full body (%d bytes)", len(got), len(body))
	}
	if _, err = os.Stat(spilled[0]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("temp file should be removed after saving: %v", err)
	}

	// 保存済みの ID はダイアログを開く前に拒否する。
	calls := dialog.saveCalls.Load()
	if _, err = h.SaveResponseBody("exec-save"); !errors.Is(err, httpdomain.ErrResponseUnavailable) {
		t.Errorf("second SaveResponseBody: want ErrResponseUnavailable, got %v", err)
	}
	if err = h.DiscardResponseBody("exec-save"); !errors.Is(err, httpdomain.ErrResponseUnavailable) {
		t.Errorf("DiscardResponseBody after saving: want ErrResponseUnavailable, got %v", err)
	}
	if got := dialog.saveCalls.Load(); got != calls {
		t.Errorf("save dialog opened %d more times for a saved response", got-calls)
	}

	// 保持中の ID では送信できず、破棄すると一時ファイルが消えて再び送信できる。
	sendTruncated(t, h, "exec-discard", srv.URL)
	if files := spillFiles(t, dir); len(files) != 1 {
		t.Fatalf("temp files = %v, want 1", files)
	}
	_, err = h.SendRequest("exec-discard", httpdomain.HTTPRequest{Method: "GET", URL: srv.URL})
	if !errors.Is(err, httpdomain.ErrResponseBusy) {
		t.Errorf("SendRequest while the response is held: want ErrResponseBusy, got %v", err)
	}
	if err := h.DiscardResponseBody("exec-discard"); err != nil {
		t.Fatalf("DiscardResponseBody: %v", err)
	}
	if files := spillFiles(t, dir); len(files) != 0 {
		t.Errorf("temp files after discard = %v, want none", files)
	}
	sendTruncated(t, h, "exec-discard", srv.URL)
}

// TestHTTP_SaveResponseBase64_BinaryBody は非 UTF-8 の本文が base64 で返り、
// SaveResponseBase64 でデコードして元のバイト列のまま保存できることを確認する。
func TestHTTP_SaveResponseBase64_BinaryBody(t *testing.T) {
	payload := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0xff, 0xfe, 0x80}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dialog := &fileDialog{savePath: filepath.Join(t.TempDir(), "out.png")}
	h := newHTTPHandlerWithDialog(t, dialog)
	resp, err := h.SendRequest("exec-bin", httpdomain.HTTPRequest{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if !resp.BodyBase64 {
		t.Fatalf("BodyBase64 = false, want true for a non UTF-8 body")
	}
	if err = h.SaveResponseBase64(resp.Body, resp.ContentType); err != nil {
		t.Fatalf("SaveResponseBase64: %v", err)
	}
	got, err := os.ReadFile(dialog.savePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("saved = %x, want %x", got, payload)
	}
}

// treeShape は木構造を名前で表した文字列にする (フォルダは name[子...])。
// ID は採番されるため、並びと入れ子は名前で比べる。
func treeShape(items []*httpdomain.TreeItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.Type == httpdomain.ItemTypeFolder {
			parts = append(parts, item.Name+"["+treeShape(item.Children)+"]")
			continue
		}
		parts = append(parts, item.Name)
	}
	return strings.Join(parts, " ")
}

// collectionShapes はコレクション ID ごとの treeShape を返す。
func collectionShapes(h *adapters.HTTPHandler) map[string]string {
	shapes := map[string]string{}
	for _, c := range h.GetCollections() {
		shapes[c.ID] = treeShape(c.Items)
	}
	return shapes
}

// TestHTTP_MoveItem_ReorderAndIntoFolder は、コレクション内の並び替え・フォルダへの移動・
// 自分のサブツリーへの移動の拒否・入れ子のフォルダのコレクション間の移動が、
// ディスクから読み直しても同じ木構造になることを確認する。
func TestHTTP_MoveItem_ReorderAndIntoFolder(t *testing.T) {
	dir := t.TempDir()
	h := newHTTPHandlerWithDir(t, dir)
	src, err := h.CreateCollection("Src")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	dst, err := h.CreateCollection("Dst")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	add := func(parentID, name string) string {
		t.Helper()
		item, aerr := h.AddRequest(src.ID, parentID, httpdomain.HTTPRequest{Name: name, Method: "GET"})
		if aerr != nil {
			t.Fatalf("AddRequest(%s): %v", name, aerr)
		}
		return item.ID
	}
	r1 := add("", "r1")
	add("", "r2")
	r3 := add("", "r3")
	folder, err := h.AddFolder(src.ID, "", "F")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}
	add(folder.ID, "fc")
	sub, err := h.AddFolder(src.ID, folder.ID, "SF")
	if err != nil {
		t.Fatalf("AddFolder: %v", err)
	}
	assertShapes := func(when string, want map[string]string) {
		t.Helper()
		if got := collectionShapes(h); !maps.Equal(got, want) {
			t.Errorf("%s: trees = %v, want %v", when, got, want)
		}
	}

	// position は移動前の並びに対する挿入位置。r1 を r2 と r3 の間 (position 2) へ。
	if err = h.MoveItem(src.ID, r1, src.ID, "", 2); err != nil {
		t.Fatalf("MoveItem (reorder): %v", err)
	}
	assertShapes("after reorder", map[string]string{src.ID: "r2 r1 r3 F[fc SF[]]", dst.ID: ""})

	if err = h.MoveItem(src.ID, r3, src.ID, folder.ID, -1); err != nil {
		t.Fatalf("MoveItem (into folder): %v", err)
	}
	assertShapes("after moving into the folder", map[string]string{src.ID: "r2 r1 F[fc SF[] r3]", dst.ID: ""})

	// 自分の子フォルダへの移動は拒否し、ファイルを書き換えない。
	srcPath := filepath.Join(dir, "collections", src.ID+".json")
	before, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var ve *cmndomain.ValidationError
	if err = h.MoveItem(src.ID, folder.ID, src.ID, sub.ID, 0); !errors.As(err, &ve) {
		t.Fatalf("MoveItem into its own subtree: want ValidationError, got %v", err)
	}
	after, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("rejected move rewrote the collection file")
	}

	// 子を持つフォルダをコレクションごと移す。
	if err := h.MoveItem(src.ID, folder.ID, dst.ID, "", -1); err != nil {
		t.Fatalf("MoveItem (across collections): %v", err)
	}
	want := map[string]string{src.ID: "r2 r1", dst.ID: "F[fc SF[] r3]"}
	assertShapes("after moving across collections", want)

	h = newHTTPHandlerWithDir(t, dir)
	assertShapes("after restart", want)
}

// TestHTTP_SidebarOperations_SurviveRestart は MoveItemToSidebar・MoveSidebarEntry・
// __root__ 直下の DeleteItem の結果が、再起動後のレイアウトとコレクションに残ることを確認する。
func TestHTTP_SidebarOperations_SurviveRestart(t *testing.T) {
	dir := t.TempDir()
	layoutPath := filepath.Join(dir, "sidebar_layout.json")
	h1 := newHTTPHandlerWithDir(t, dir)
	alpha, err := h1.CreateCollection("Alpha")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	beta, err := h1.CreateCollection("Beta")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	item, err := h1.AddRequest(alpha.ID, "", httpdomain.HTTPRequest{Name: "R", Method: "GET"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	if err = h1.MoveItemToSidebar(alpha.ID, item.ID, 0); err != nil {
		t.Fatalf("MoveItemToSidebar: %v", err)
	}
	// [item, Alpha, Beta] の Alpha を末尾の挿入ゾーン (position 3) へ。
	if err = h1.MoveSidebarEntry("collection", alpha.ID, 3); err != nil {
		t.Fatalf("MoveSidebarEntry: %v", err)
	}
	want := []httpdomain.SidebarEntry{
		{Kind: "item", ID: item.ID},
		{Kind: "collection", ID: beta.ID},
		{Kind: "collection", ID: alpha.ID},
	}

	h2 := newHTTPHandlerWithDir(t, dir)
	layout, err := h2.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	if !slices.Equal(layout, want) {
		t.Errorf("layout after restart = %v, want %v", layout, want)
	}
	if items := h2.GetRootItems(); len(items) != 1 || items[0].ID != item.ID {
		t.Errorf("root items after restart = %v, want [%s]", items, item.ID)
	}
	for _, c := range h2.GetCollections() {
		if len(c.Items) != 0 {
			t.Errorf("%s still holds %v after MoveItemToSidebar", c.Name, c.Items)
		}
	}

	if err = h2.DeleteItem(httpdomain.RootCollectionID, item.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	want = want[1:]
	h3 := newHTTPHandlerWithDir(t, dir)
	layout, err = h3.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	if !slices.Equal(layout, want) {
		t.Errorf("layout after DeleteItem and restart = %v, want %v", layout, want)
	}
	// 読み出し時の突合ではなく、ファイルからもエントリが消えている。
	saved, err := httpinfra.NewSidebarLayoutRepository(layoutPath).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !slices.Equal(saved, want) {
		t.Errorf("sidebar_layout.json = %v, want %v", saved, want)
	}
}

// TestHTTP_ReconcilesRootItemEntriesOnStartup は、__root__.json と食い違うアイテムのエントリ
// (stale・重複・欠落) が起動時に修復され、ファイルにも書き戻されることを確認する。
func TestHTTP_ReconcilesRootItemEntriesOnStartup(t *testing.T) {
	dir := t.TempDir()
	root := httpdomain.RootCollectionID
	writeCollectionJSON(t, dir, root, collectionJSON(root, root, "r1", "r2"))
	layoutPath := filepath.Join(dir, "sidebar_layout.json")
	layout := `[{"kind":"item","id":"gone"},{"kind":"item","id":"r1"},{"kind":"item","id":"r1"}]`
	if err := os.WriteFile(layoutPath, []byte(layout), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	h := newHTTPHandlerWithDir(t, dir)
	want := []httpdomain.SidebarEntry{{Kind: "item", ID: "r1"}, {Kind: "item", ID: "r2"}}
	got, err := h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("layout = %v, want %v", got, want)
	}
	saved, err := httpinfra.NewSidebarLayoutRepository(layoutPath).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !slices.Equal(saved, want) {
		t.Errorf("sidebar_layout.json = %v, want %v", saved, want)
	}
}

// TestHTTP_StaleResponseFilesSweptOnRestart は、切り詰めたまま Cleanup せずに終わった
// セッションの一時ファイルを起動時の sweep が回収し、marker の無いディレクトリは残すことと、
// 前回セッションのファイル token が再起動後に使えないことを確認する。
// sweep を app.go と同じ sessionDir で NetClient の作成前に呼ぶ配線は、このテストでは確かめない。
// 注意: SweepStaleTempFiles は os.TempDir() 直下の旧形式の一時ファイル (wirexa-response-*) も消す。
func TestHTTP_StaleResponseFilesSweptOnRestart(t *testing.T) {
	srv := newBodyServer(t, truncatedBody())
	selected := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(selected, []byte("payload"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dir := t.TempDir()
	sessions := filepath.Join(dir, "http-sessions")
	dialog := &fileDialog{path: selected}

	// 1 回目: 本文を切り詰めたまま NetClient.Cleanup を呼ばずに終わる (クラッシュ相当)。
	first := buildHTTPFixture(t, dir, dialog)
	picked, err := first.h.OpenFilePicker("")
	if err != nil {
		t.Fatalf("OpenFilePicker: %v", err)
	}
	sendTruncated(t, first.h, "exec-stale", srv.URL)
	stale, err := filepath.Glob(filepath.Join(sessions, "wirexa-http-*"))
	if err != nil || len(stale) != 1 {
		t.Fatalf("session dirs = %v (%v), want 1", stale, err)
	}
	// 接頭辞だけが一致し、Wirexa の marker を持たないディレクトリ。
	foreign := filepath.Join(sessions, "wirexa-http-foreign")
	if err = os.MkdirAll(foreign, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err = os.WriteFile(filepath.Join(foreign, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	httpinfra.SweepStaleTempFiles(sessions)

	if _, err = os.Stat(stale[0]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale session dir should be swept: %v", err)
	}
	if _, err = os.Stat(filepath.Join(foreign, "keep.txt")); err != nil {
		t.Errorf("a dir without the Wirexa marker must be kept: %v", err)
	}
	if _, err = os.Stat(filepath.Join(sessions, ".session-secret")); err != nil {
		t.Errorf(".session-secret must be kept: %v", err)
	}

	// 2 回目: ファイル token はセッション内でしか使えない。
	second := newHTTPHandlerWithDirAndDialog(t, dir, dialog)
	_, err = second.SendRequest("exec-token", httpdomain.HTTPRequest{
		Method: "POST",
		URL:    srv.URL,
		Body:   httpdomain.RequestBody{Type: httpdomain.BodyTypeFile, File: httpdomain.FileReference{Token: picked.Token}},
	})
	if !errors.Is(err, httpdomain.ErrFileAccessDenied) {
		t.Errorf("previous session's token: want ErrFileAccessDenied, got %v", err)
	}
}

// TestHTTP_Shutdown_CancelsInFlightThenCleansTempFiles は app.go の終了順序
// (HTTPRequestService.Shutdown → NetClient.Cleanup) を再現し、実行中のリクエストが止まり、
// 以降の送信がサーバーへ届かず、一時ファイルとセッションディレクトリが消えることを確認する。
func TestHTTP_Shutdown_CancelsInFlightThenCleansTempFiles(t *testing.T) {
	body := truncatedBody()
	hang := make(chan struct{})
	var afterHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/big":
			_, _ = w.Write(body)
		case "/hang":
			close(hang)
			<-r.Context().Done()
		default:
			afterHits.Add(1)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := buildHTTPFixture(t, dir, &fileDialog{})
	t.Cleanup(f.netClient.Cleanup)
	sendTruncated(t, f.h, "exec-held", srv.URL+"/big")

	done := make(chan error, 1)
	go func() {
		_, err := f.h.SendRequest("exec-hang", httpdomain.HTTPRequest{Method: "GET", URL: srv.URL + "/hang"})
		done <- err
	}()
	select {
	case <-hang:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive the in-flight request")
	}

	if !f.reqSvc.Shutdown(3 * time.Second) {
		t.Fatal("Shutdown timed out waiting for the in-flight request")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Error("in-flight SendRequest should fail after Shutdown")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight SendRequest did not return after Shutdown")
	}

	if _, err := f.h.SendRequest("exec-after", httpdomain.HTTPRequest{Method: "GET", URL: srv.URL + "/after"}); err == nil {
		t.Error("SendRequest after Shutdown should fail")
	}
	if got := afterHits.Load(); got != 0 {
		t.Errorf("request after Shutdown reached the server %d times", got)
	}
	// 終了処理中のキャンセルは何もせず、墓標も残さない。
	f.h.CancelRequest("exec-after")

	f.netClient.Cleanup()
	entries, err := os.ReadDir(filepath.Join(dir, "http-sessions"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != ".session-secret" {
			t.Errorf("left after Cleanup: %s", e.Name())
		}
	}
	if _, err := f.h.SaveResponseBody("exec-held"); !errors.Is(err, httpdomain.ErrResponseUnavailable) {
		t.Errorf("SaveResponseBody after Cleanup: want ErrResponseUnavailable, got %v", err)
	}
}

// TestHTTP_MoveItem_ThroughRootCollection は、予約済みの __root__ をサイドバーのコレクションとして
// 動かせないことと、MoveItem で __root__ に出し入れしたアイテムのエントリが、レイアウトを
// 保存せずに読み出し時の突合で現れ・消えることを確認する。
func TestHTTP_MoveItem_ThroughRootCollection(t *testing.T) {
	dir := t.TempDir()
	layoutPath := filepath.Join(dir, "sidebar_layout.json")
	h := newHTTPHandlerWithDir(t, dir)
	col, err := h.CreateCollection("C")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	item, err := h.AddRequest(col.ID, "", httpdomain.HTTPRequest{Name: "R", Method: "GET"})
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	var nf *cmndomain.NotFoundError
	if err = h.MoveSidebarEntry("collection", httpdomain.RootCollectionID, 0); !errors.As(err, &nf) {
		t.Errorf("MoveSidebarEntry(__root__): want NotFoundError, got %v", err)
	}

	if err = h.MoveItem(col.ID, item.ID, httpdomain.RootCollectionID, "", -1); err != nil {
		t.Fatalf("MoveItem into __root__: %v", err)
	}
	if items := h.GetRootItems(); len(items) != 1 || items[0].ID != item.ID {
		t.Errorf("root items = %v, want [%s]", items, item.ID)
	}
	layout, err := h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	if last := layout[len(layout)-1]; last != (httpdomain.SidebarEntry{Kind: "item", ID: item.ID}) {
		t.Errorf("last entry = %v, want the moved item", last)
	}
	raw, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), item.ID) {
		t.Errorf("MoveItem should not write the layout (the entry comes from reconciliation):\n%s", raw)
	}

	if err = h.MoveItem(httpdomain.RootCollectionID, item.ID, col.ID, "", -1); err != nil {
		t.Fatalf("MoveItem out of __root__: %v", err)
	}
	layout, err = h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	want := []httpdomain.SidebarEntry{{Kind: "collection", ID: col.ID}}
	if !slices.Equal(layout, want) {
		t.Errorf("layout = %v, want %v", layout, want)
	}
}

// TestHTTP_SendRequest_AuthOverridesAuthorizationHeader は、認証の設定がユーザー指定の
// Authorization ヘッダーより優先され、ワイヤ上に値が 1 つだけ載ることを確認する。
func TestHTTP_SendRequest_AuthOverridesAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name string
		want string
		auth httpdomain.RequestAuth
	}{
		{
			name: "basic",
			auth: httpdomain.RequestAuth{Type: "basic", Username: "user", Password: "pass"},
			want: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
		},
		{
			name: "bearer",
			auth: httpdomain.RequestAuth{Type: "bearer", Token: "mytoken"},
			want: "Bearer mytoken",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Values("Authorization")
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			h := newHTTPHandler(t)
			_, err := h.SendRequest("exec-auth", httpdomain.HTTPRequest{
				Method:  "GET",
				URL:     srv.URL,
				Headers: []httpdomain.KeyValuePair{{Key: "Authorization", Value: "Custom x", Enabled: true}},
				Auth:    tc.auth,
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if !slices.Equal(got, []string{tc.want}) {
				t.Errorf("Authorization = %q, want [%q]", got, tc.want)
			}
		})
	}
}

// TestHTTP_SendRequest_SelectedFileRemoved は、選択後・送信前に消えたファイルを送ろうとすると
// パスを含まない ErrSelectedFileUnavailable になり、本文がサーバーへ届かないことを確認する。
func TestHTTP_SendRequest_SelectedFileRemoved(t *testing.T) {
	const content = "must not be sent"
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

	tests := []struct {
		body func(token string) httpdomain.RequestBody
		name string
	}{
		{name: "file body", body: func(token string) httpdomain.RequestBody {
			return httpdomain.RequestBody{Type: httpdomain.BodyTypeFile, File: httpdomain.FileReference{Token: token}}
		}},
		{name: "form-data の file 行", body: func(token string) httpdomain.RequestBody {
			return httpdomain.RequestBody{Type: httpdomain.BodyTypeFormData, FormData: []httpdomain.FormRow{
				{Key: "f", Kind: httpdomain.FormRowKindFile, File: httpdomain.FileReference{Token: token}, Enabled: true},
			}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "gone.txt")
			if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			h := newHTTPHandlerWithDialog(t, &fileDialog{path: p})
			picked, err := h.OpenFilePicker("")
			if err != nil {
				t.Fatalf("OpenFilePicker: %v", err)
			}
			if err = os.Remove(p); err != nil {
				t.Fatalf("Remove: %v", err)
			}

			_, err = h.SendRequest("exec-gone", httpdomain.HTTPRequest{Method: "POST", URL: srv.URL, Body: tc.body(picked.Token)})
			if !errors.Is(err, httpdomain.ErrSelectedFileUnavailable) {
				t.Fatalf("SendRequest: want ErrSelectedFileUnavailable, got %v", err)
			}
			if strings.Contains(err.Error(), p) {
				t.Errorf("error leaks the path: %q", err)
			}
		})
	}

	mu.Lock()
	defer mu.Unlock()
	for _, b := range bodies {
		if strings.Contains(b, content) {
			t.Errorf("the removed file reached the server: %q", b)
		}
	}
}

// smallReadBufferListener は accept した接続の受信バッファを小さくして、
// サーバーが止まっている間にクライアントが先読みして送れる量を減らす。
type smallReadBufferListener struct{ net.Listener }

func (l smallReadBufferListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetReadBuffer(4 << 10)
	}
	return c, err
}

// TestHTTP_SendRequest_SelectedFileModifiedDuringSend は、送信中に選択ファイルが変更された場合の
// 扱いを確認する。Windows は共有モードで書き込みを拒否するので変更自体が失敗して送信が成功し、
// それ以外は ErrSelectedFileChanged で送信を打ち切る (CI の ubuntu では後者だけを検証する)。
//
// クライアントの読み取り位置を外から観測できないので、変更は書き換えではなく 0 バイトへの切り詰めにし
// (以降の読み込みがすべて短くなり、末尾の検査を待たずに気付く)、ファイルをソケットのバッファより
// 十分大きくして「変更がクライアントの末尾の読み取りより先に起きる」状況を作る。
func TestHTTP_SendRequest_SelectedFileModifiedDuringSend(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big.bin")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err = f.Truncate(64 << 20); err != nil {
		t.Fatalf("Truncate: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	started := make(chan struct{})
	resume := make(chan struct{})
	var once sync.Once
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<10)
		_, _ = io.ReadFull(r.Body, buf)
		once.Do(func() { close(started) })
		<-resume
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	srv.Listener = smallReadBufferListener{srv.Listener}
	srv.Start()
	defer srv.Close()

	h := newHTTPHandlerWithDialog(t, &fileDialog{path: p})
	picked, err := h.OpenFilePicker("")
	if err != nil {
		t.Fatalf("OpenFilePicker: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, serr := h.SendRequest("exec-modify", httpdomain.HTTPRequest{
			Method: "POST",
			URL:    srv.URL,
			Body:   httpdomain.RequestBody{Type: httpdomain.BodyTypeFile, File: httpdomain.FileReference{Token: picked.Token}},
		})
		done <- serr
	}()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		close(resume)
		t.Fatal("server did not start reading the body")
	}
	truncErr := os.Truncate(p, 0)
	close(resume)

	var sendErr error
	select {
	case sendErr = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("SendRequest did not return")
	}

	if goruntime.GOOS == "windows" {
		if truncErr == nil {
			t.Fatal("truncating the file being sent succeeded; the share mode should deny writers")
		}
		if sendErr != nil {
			t.Fatalf("SendRequest: %v", sendErr)
		}
		return
	}
	if truncErr != nil {
		t.Fatalf("Truncate: %v", truncErr)
	}
	if sendErr == nil {
		t.Fatal("前提が崩れた: 切り詰めより先にファイル全体を読み終えて送信が成功した (ファイルを大きくする必要がある)")
	}
	if !errors.Is(sendErr, httpdomain.ErrSelectedFileChanged) {
		t.Fatalf("SendRequest: want ErrSelectedFileChanged, got %v", sendErr)
	}
}

// droppingServer は Content-Length で declared バイトを宣言し、written バイトだけ書いてから
// 接続を閉じるサーバーを起動し、その URL を返す。
func droppingServer(t *testing.T, declared, written int) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				if _, rerr := http.ReadRequest(bufio.NewReader(conn)); rerr != nil {
					return
				}
				fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n", declared)
				_, _ = conn.Write(bytes.Repeat([]byte("x"), written))
			}()
		}
	}()
	return "http://" + ln.Addr().String()
}

// TestHTTP_SendRequest_ConnectionDroppedMidBody は、本文の途中で接続が切れるとエラーになり、
// 一時ファイルへ退避している最中に切れた場合も作りかけの一時ファイルを残さず、
// 同じ executionID を再利用できることを確認する。
func TestHTTP_SendRequest_ConnectionDroppedMidBody(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()

	tests := []struct {
		name    string
		written int
	}{
		{name: "閾値未満で切断", written: 512 << 10},
		{name: "閾値を超えて一時ファイルへ退避中に切断", written: 2 << 20},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := droppingServer(t, 4<<20, tc.written)
			dir := t.TempDir()
			h := newHTTPHandlerWithDir(t, dir)

			_, err := h.SendRequest("exec-drop", httpdomain.HTTPRequest{
				Method:   "GET",
				URL:      url,
				Settings: httpdomain.RequestSettings{MaxResponseBodyMB: 1},
			})
			if err == nil {
				t.Fatal("SendRequest should fail when the connection drops mid-body")
			}
			if files := spillFiles(t, dir); len(files) != 0 {
				t.Errorf("partial temp files left: %v", files)
			}
			if _, err := h.SendRequest("exec-drop", httpdomain.HTTPRequest{Method: "GET", URL: ok.URL}); err != nil {
				t.Errorf("the execution ID should be reusable after the failure: %v", err)
			}
		})
	}
}

// TestHTTP_CorruptCollectionAmongValidOnes は、壊れたコレクションと壊れた __root__.json だけを
// 退避して正常なコレクションで起動し、__root__ を空で作り直すことを確認する。
func TestHTTP_CorruptCollectionAmongValidOnes(t *testing.T) {
	dir := t.TempDir()
	collDir := filepath.Join(dir, "collections")
	root := httpdomain.RootCollectionID
	const corrupt = "{ not json"
	writeCollectionJSON(t, dir, "col-a", collectionJSON("col-a", "Alpha", "r1"))
	writeCollectionJSON(t, dir, "col-b", corrupt)
	writeCollectionJSON(t, dir, root, corrupt)

	h := newHTTPHandlerWithDir(t, dir)
	cols := h.GetCollections()
	if len(cols) != 1 || cols[0].ID != "col-a" || len(cols[0].Items) != 1 {
		t.Fatalf("collections = %v, want only col-a with its item", cols)
	}
	if items := h.GetRootItems(); len(items) != 0 {
		t.Errorf("root items = %v, want empty", items)
	}
	for _, id := range []string{"col-b", root} {
		got, err := os.ReadFile(filepath.Join(collDir, id+".json.corrupt"))
		if err != nil {
			t.Errorf("%s.json.corrupt should exist: %v", id, err)
			continue
		}
		if string(got) != corrupt {
			t.Errorf("%s.json.corrupt = %q, want the original bytes", id, got)
		}
	}
	if _, err := os.Stat(filepath.Join(collDir, "col-b.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("col-b.json should have been moved away: %v", err)
	}

	// 退避で「ファイルが無い」状態になった __root__ は作り直されて使える。
	item, err := h.AddRequest(root, "", httpdomain.HTTPRequest{Name: "R", Method: "GET"})
	if err != nil {
		t.Fatalf("AddRequest to the recreated __root__: %v", err)
	}
	h2 := newHTTPHandlerWithDir(t, dir)
	if items := h2.GetRootItems(); len(items) != 1 || items[0].ID != item.ID {
		t.Errorf("root items after restart = %v, want [%s]", items, item.ID)
	}
}

// TestHTTP_UnreadableSidebarLayout_RegeneratesWithoutQuarantine は、sidebar_layout.json が
// 破損以外の理由で読めないとき (ここではディレクトリ)、退避せずに再生成した並びで動作を続けることを確認する。
func TestHTTP_UnreadableSidebarLayout_RegeneratesWithoutQuarantine(t *testing.T) {
	dir := t.TempDir()
	// 名前順 (Alpha, Beta) と ID 順 (col-a, col-z) を逆にしておく。
	writeCollectionJSON(t, dir, "col-z", collectionJSON("col-z", "Alpha", "r1"))
	writeCollectionJSON(t, dir, "col-a", collectionJSON("col-a", "Beta", "r2"))
	layoutPath := filepath.Join(dir, "sidebar_layout.json")
	if err := os.Mkdir(layoutPath, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	h := newHTTPHandlerWithDir(t, dir)
	layout, err := h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	want := []httpdomain.SidebarEntry{{Kind: "collection", ID: "col-z"}, {Kind: "collection", ID: "col-a"}}
	if !slices.Equal(layout, want) {
		t.Errorf("layout = %v, want %v (name order)", layout, want)
	}

	if err = h.MoveSidebarEntry("collection", "col-a", 0); err == nil {
		t.Error("MoveSidebarEntry should fail while the layout cannot be read")
	}
	// レイアウトの更新は best effort なので、コレクションの作成は成功する。
	col, err := h.CreateCollection("Gamma")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	layout, err = h.GetSidebarLayout()
	if err != nil {
		t.Fatalf("GetSidebarLayout: %v", err)
	}
	if want = append(want, httpdomain.SidebarEntry{Kind: "collection", ID: col.ID}); !slices.Equal(layout, want) {
		t.Errorf("layout after CreateCollection = %v, want %v", layout, want)
	}

	if _, err := os.Stat(layoutPath + ".corrupt"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an unreadable (not corrupt) layout must not be quarantined: %v", err)
	}
	if info, err := os.Stat(layoutPath); err != nil || !info.IsDir() {
		t.Errorf("sidebar_layout.json should be left as is: %v", err)
	}
}
