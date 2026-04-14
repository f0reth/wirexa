// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// HttpHandler は Wails RPC アダプターとして HTTP ユースケースを公開する。
type HttpHandler struct {
	ctx     context.Context
	reqSvc  domain.RequestUseCase
	collSvc domain.CollectionUseCase
}

// SetupHTTPHandler は既存の HttpHandler インスタンスにサービスを注入する。
// Wails の Bind に渡す前に事前確保した空ハンドラーを startup() で初期化する際に使用する。
func SetupHTTPHandler(ctx context.Context, h *HttpHandler, reqSvc domain.RequestUseCase, collSvc domain.CollectionUseCase) {
	h.ctx = ctx
	h.reqSvc = reqSvc
	h.collSvc = collSvc
}

// OpenFilePicker はネイティブのファイル選択ダイアログを開き、選択されたファイルパスを返す。
func (h *HttpHandler) OpenFilePicker() (string, error) {
	return runtime.OpenFileDialog(h.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
	})
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
func (h *HttpHandler) SendRequest(req domain.HttpRequest) (domain.HttpResponse, error) {
	return h.reqSvc.SendRequest(req)
}

// CancelRequest は指定 ID の実行中 HTTP リクエストをキャンセルする。
func (h *HttpHandler) CancelRequest(id string) {
	h.reqSvc.CancelRequest(id)
}

// GetCollections は全コレクションを返す。
func (h *HttpHandler) GetCollections() []domain.Collection {
	return h.collSvc.GetCollections()
}

// CreateCollection は新規コレクションを作成する。
func (h *HttpHandler) CreateCollection(name string) (domain.Collection, error) {
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
func (h *HttpHandler) AddFolder(collectionID, parentID, name string) (*domain.TreeItem, error) {
	return h.collSvc.AddFolder(collectionID, parentID, name)
}

// AddRequest はコレクションにリクエストを追加する。
func (h *HttpHandler) AddRequest(collectionID, parentID string, req domain.HttpRequest) (*domain.TreeItem, error) {
	return h.collSvc.AddRequest(collectionID, parentID, req)
}

// UpdateRequest はコレクション内のリクエストを更新する。
func (h *HttpHandler) UpdateRequest(collectionID string, req domain.HttpRequest) error {
	return h.collSvc.UpdateRequest(collectionID, req)
}

// RenameItem はコレクション内のアイテム名を変更する。
func (h *HttpHandler) RenameItem(collectionID, itemID, name string) error {
	return h.collSvc.RenameItem(collectionID, itemID, name)
}

// DeleteItem はコレクションからアイテムを削除する。
func (h *HttpHandler) DeleteItem(collectionID, itemID string) error {
	return h.collSvc.DeleteItem(collectionID, itemID)
}

// MoveItem はアイテムを同一コレクション内の別の親・位置へ移動する。
// targetParentID が空文字の場合はコレクションルートへ移動する。
// position は挿入先インデックス（削除後）。-1 の場合は末尾に追加する。
func (h *HttpHandler) MoveItem(collectionID, itemID, targetParentID string, position int) error {
	return h.collSvc.MoveItem(collectionID, itemID, targetParentID, position)
}
