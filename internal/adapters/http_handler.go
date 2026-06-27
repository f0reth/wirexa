// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
)

// TempFileProvider はテンポラリファイルパスを取得・消費するインターフェース。
// インフラ詳細を adapter 層に伝達するために使用する。
type TempFileProvider interface {
	ConsumeTempFilePath(requestID string) string
}

// HttpHandler は Wails RPC アダプターとして HTTP ユースケースを公開する。
type HttpHandler struct {
	ctx       context.Context
	reqSvc    httpdomain.RequestUseCase
	collSvc   httpdomain.CollectionUseCase
	itemSvc   httpdomain.CollectionItemUseCase
	tempFiles TempFileProvider
}

// SetupHTTPHandler は既存の HttpHandler インスタンスにサービスを注入する。
// Wails の Bind に渡す前に事前確保した空ハンドラーを startup() で初期化する際に使用する。
func SetupHTTPHandler(ctx context.Context, h *HttpHandler, reqSvc httpdomain.RequestUseCase, collSvc httpdomain.CollectionUseCase, itemSvc httpdomain.CollectionItemUseCase, tempFiles TempFileProvider) {
	h.ctx = ctx
	h.reqSvc = reqSvc
	h.collSvc = collSvc
	h.itemSvc = itemSvc
	h.tempFiles = tempFiles
}

// OpenFilePicker はネイティブのファイル選択ダイアログを開き、選択されたファイルパスを返す。
func (h *HttpHandler) OpenFilePicker() (string, error) {
	return runtime.OpenFileDialog(h.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
	})
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
func (h *HttpHandler) SendRequest(req httpdomain.HttpRequest) (httpdomain.HttpResponse, error) {
	res, err := h.reqSvc.SendRequest(req)
	if err != nil {
		return httpdomain.HttpResponse{}, err
	}
	if res.BodyTruncated && h.tempFiles != nil {
		res.TempFilePath = h.tempFiles.ConsumeTempFilePath(req.ID)
	}
	return res, nil
}

// CancelRequest は指定 ID の実行中 HTTP リクエストをキャンセルする。
func (h *HttpHandler) CancelRequest(id string) {
	h.reqSvc.CancelRequest(id)
}

// SaveResponseBody はテンポラリファイルをOSのファイル保存ダイアログで指定先に保存する。
// 保存後にテンポラリファイルを削除する。キャンセル時は何もしない。
func (h *HttpHandler) SaveResponseBody(tempFilePath, contentType string) error {
	ext := contentTypeToExtension(contentType)
	savePath, err := runtime.SaveFileDialog(h.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "response" + ext,
		Filters: []runtime.FileFilter{{
			DisplayName: contentType,
			Pattern:     "*" + ext,
		}},
	})
	if err != nil || savePath == "" {
		return err
	}
	if err := copyFile(tempFilePath, savePath); err != nil {
		return err
	}
	return os.Remove(tempFilePath)
}

// GetRootItems はルートコレクションのアイテム一覧を返す。
func (h *HttpHandler) GetRootItems() []*httpdomain.TreeItem {
	return h.collSvc.GetRootItems()
}

// GetCollections は全コレクションを返す。
func (h *HttpHandler) GetCollections() []httpdomain.Collection {
	return h.collSvc.GetCollections()
}

// CreateCollection は新規コレクションを作成する。
func (h *HttpHandler) CreateCollection(name string) (httpdomain.Collection, error) {
	return h.collSvc.CreateCollection(name)
}

// DeleteCollection は ID でコレクションを削除する。
func (h *HttpHandler) DeleteCollection(id string) error {
	return h.collSvc.DeleteCollection(id)
}

// RenameCollection はコレクション名を変更する。
func (h *HttpHandler) RenameCollection(id, name string) error {
	return h.collSvc.RenameCollection(id, name)
}

// AddFolder はコレクションにフォルダを追加する。
func (h *HttpHandler) AddFolder(collectionID, parentID, name string) (*httpdomain.TreeItem, error) {
	return h.itemSvc.AddFolder(collectionID, parentID, name)
}

// AddRequest はコレクションにリクエストを追加する。
func (h *HttpHandler) AddRequest(collectionID, parentID string, req httpdomain.HttpRequest) (*httpdomain.TreeItem, error) {
	return h.itemSvc.AddRequest(collectionID, parentID, req)
}

// UpdateRequest はコレクション内のリクエストを更新する。
func (h *HttpHandler) UpdateRequest(collectionID string, req httpdomain.HttpRequest) error {
	return h.itemSvc.UpdateRequest(collectionID, req)
}

// RenameItem はコレクション内のアイテム名を変更する。
func (h *HttpHandler) RenameItem(collectionID, itemID, name string) error {
	return h.itemSvc.RenameItem(collectionID, itemID, name)
}

// DeleteItem はコレクションからアイテムを削除する。
func (h *HttpHandler) DeleteItem(collectionID, itemID string) error {
	return h.itemSvc.DeleteItem(collectionID, itemID)
}

// MoveCollection はコレクションを指定の位置に並び替える。
func (h *HttpHandler) MoveCollection(collectionID string, position int) error {
	return h.collSvc.MoveCollection(collectionID, position)
}

// MoveItem はアイテムをコレクション内外・別の親・位置へ移動する。
// targetParentID が空文字の場合はターゲットコレクションルートへ移動する。
// position は挿入先インデックス（削除後）。-1 の場合は末尾に追加する。
func (h *HttpHandler) MoveItem(sourceCollectionID, itemID, targetCollectionID, targetParentID string, position int) error {
	return h.itemSvc.MoveItem(sourceCollectionID, itemID, targetCollectionID, targetParentID, position)
}

// GetSidebarLayout はサイドバーレイアウトを返す。
func (h *HttpHandler) GetSidebarLayout() ([]httpdomain.SidebarEntry, error) {
	return h.collSvc.GetSidebarLayout()
}

// MoveSidebarEntry はサイドバー上のエントリを指定位置に移動する。
func (h *HttpHandler) MoveSidebarEntry(kind, id string, position int) error {
	return h.collSvc.MoveSidebarEntry(kind, id, position)
}

// MoveItemToSidebar はアイテムを指定コレクションから __root__ へ移動し、
// サイドバーレイアウトの指定位置に挿入する。
func (h *HttpHandler) MoveItemToSidebar(sourceCollectionID, itemID string, sidebarPosition int) error {
	return h.collSvc.MoveItemToSidebar(sourceCollectionID, itemID, sidebarPosition)
}

func contentTypeToExtension(contentType string) string {
	ct := strings.TrimSpace(strings.ToLower(strings.Split(contentType, ";")[0]))
	switch ct {
	case "application/json":
		return ".json"
	case "text/html":
		return ".html"
	case "text/plain":
		return ".txt"
	case "text/xml", "application/xml":
		return ".xml"
	case "text/csv":
		return ".csv"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // path comes from OS save dialog
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() //nolint:errcheck // best-effort cleanup
	out, err := os.Create(dst)        //nolint:gosec // path comes from OS save dialog
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }() //nolint:errcheck // best-effort cleanup
	_, err = io.Copy(out, in)
	return err
}
