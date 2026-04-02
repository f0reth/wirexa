// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// HttpHandler は Wails RPC アダプターとして HTTP ユースケースを公開する。
type HttpHandler struct {
	reqSvc  domain.RequestUseCase
	collSvc domain.CollectionUseCase
}

// NewHTTPHandler は HttpHandler を生成する。
func NewHTTPHandler(reqSvc domain.RequestUseCase, collSvc domain.CollectionUseCase) *HttpHandler {
	return &HttpHandler{reqSvc: reqSvc, collSvc: collSvc}
}

// SetupHTTPHandler は既存の HttpHandler インスタンスにサービスを注入する。
// Wails の Bind に渡す前に事前確保した空ハンドラーを startup() で初期化する際に使用する。
func SetupHTTPHandler(h *HttpHandler, reqSvc domain.RequestUseCase, collSvc domain.CollectionUseCase) {
	h.reqSvc = reqSvc
	h.collSvc = collSvc
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
func (h *HttpHandler) SendRequest(req domain.HttpRequest) (domain.HttpResponse, error) {
	return h.reqSvc.SendRequest(req)
}

// CancelRequest は実行中の HTTP リクエストをキャンセルする。
func (h *HttpHandler) CancelRequest() {
	h.reqSvc.CancelRequest()
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
