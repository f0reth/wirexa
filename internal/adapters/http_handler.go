// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
)

// HttpHandler は Wails RPC アダプターとして HTTP ユースケースを公開する。
type HttpHandler struct {
	ctx     context.Context
	reqSvc  httpdomain.RequestUseCase
	collSvc httpdomain.CollectionUseCase
	itemSvc httpdomain.CollectionItemUseCase
}

// SetupHTTPHandler は既存の HttpHandler インスタンスにサービスを注入する。
// Wails の Bind に渡す前に事前確保した空ハンドラーを startup() で初期化する際に使用する。
func SetupHTTPHandler(ctx context.Context, h *HttpHandler, reqSvc httpdomain.RequestUseCase, collSvc httpdomain.CollectionUseCase, itemSvc httpdomain.CollectionItemUseCase) {
	h.ctx = ctx
	h.reqSvc = reqSvc
	h.collSvc = collSvc
	h.itemSvc = itemSvc
}

// OpenFilePicker はネイティブのファイル選択ダイアログを開き、選択されたファイルパスを返す。
func (h *HttpHandler) OpenFilePicker() (string, error) {
	return runtime.OpenFileDialog(h.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
	})
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
func (h *HttpHandler) SendRequest(req HttpRequest) (HttpResponse, error) {
	res, err := h.reqSvc.SendRequest(fromHTTPRequestDTO(req))
	if err != nil {
		return HttpResponse{}, err
	}
	return toHTTPResponseDTO(res), nil
}

// CancelRequest は指定 ID の実行中 HTTP リクエストをキャンセルする。
func (h *HttpHandler) CancelRequest(id string) {
	h.reqSvc.CancelRequest(id)
}

// GetRootItems はルートコレクションのアイテム一覧を返す。
func (h *HttpHandler) GetRootItems() []*TreeItem {
	items := h.collSvc.GetRootItems()
	result := make([]*TreeItem, len(items))
	for i, item := range items {
		result[i] = toTreeItemDTO(item)
	}
	return result
}

// GetCollections は全コレクションを返す。
func (h *HttpHandler) GetCollections() []Collection {
	cols := h.collSvc.GetCollections()
	result := make([]Collection, len(cols))
	for i, c := range cols {
		result[i] = toCollectionDTO(c)
	}
	return result
}

// CreateCollection は新規コレクションを作成する。
func (h *HttpHandler) CreateCollection(name string) (Collection, error) {
	col, err := h.collSvc.CreateCollection(name)
	if err != nil {
		return Collection{}, err
	}
	return toCollectionDTO(col), nil
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
func (h *HttpHandler) AddFolder(collectionID, parentID, name string) (*TreeItem, error) {
	item, err := h.itemSvc.AddFolder(collectionID, parentID, name)
	if err != nil {
		return nil, err
	}
	return toTreeItemDTO(item), nil
}

// AddRequest はコレクションにリクエストを追加する。
func (h *HttpHandler) AddRequest(collectionID, parentID string, req HttpRequest) (*TreeItem, error) {
	item, err := h.itemSvc.AddRequest(collectionID, parentID, fromHTTPRequestDTO(req))
	if err != nil {
		return nil, err
	}
	return toTreeItemDTO(item), nil
}

// UpdateRequest はコレクション内のリクエストを更新する。
func (h *HttpHandler) UpdateRequest(collectionID string, req HttpRequest) error {
	return h.itemSvc.UpdateRequest(collectionID, fromHTTPRequestDTO(req))
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
func (h *HttpHandler) GetSidebarLayout() ([]SidebarEntryDTO, error) {
	layout, err := h.collSvc.GetSidebarLayout()
	if err != nil {
		return nil, err
	}
	result := make([]SidebarEntryDTO, len(layout))
	for i, e := range layout {
		result[i] = toSidebarEntryDTO(e)
	}
	return result, nil
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
