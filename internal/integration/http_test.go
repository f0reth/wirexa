//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/f0reth/Wirexa/internal/adapters"
	httpapp "github.com/f0reth/Wirexa/internal/application/http"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
	httpinfra "github.com/f0reth/Wirexa/internal/infrastructure/http"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// newHTTPHandler は統合テスト用に HttpHandler を DI で組み立てる。
func newHTTPHandler(t *testing.T) *adapters.HttpHandler {
	t.Helper()
	repo, err := httpinfra.NewJSONFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONFileRepository: %v", err)
	}
	collSvc, err := httpapp.NewCollectionService(repo)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	reqSvc := httpapp.NewHTTPRequestService(httpinfra.NewNetClient(), testutil.NoopLogger{})
	return adapters.NewHTTPHandler(reqSvc, collSvc)
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
	req := domain.HttpRequest{Name: "GET example", Method: "GET", URL: "http://example.com"}
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
	if rootFolder.Type != domain.ItemTypeFolder {
		t.Errorf("root item type = %q, want folder", rootFolder.Type)
	}
	if len(rootFolder.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(rootFolder.Children))
	}
	if rootFolder.Children[0].Type != domain.ItemTypeRequest {
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
			resp, err := h.SendRequest(domain.HttpRequest{
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
			resp, err := h.SendRequest(domain.HttpRequest{
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
	_, err := h.SendRequest(domain.HttpRequest{
		Method: "GET",
		URL:    srv.URL,
		Headers: []domain.KeyValuePair{
			{Key: "X-Test", Value: "hello", Enabled: true},
		},
		Params: []domain.KeyValuePair{
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
		_, err := h.SendRequest(domain.HttpRequest{
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
	h.CancelRequest()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error after cancel, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SendRequest did not return after cancel")
	}
}
