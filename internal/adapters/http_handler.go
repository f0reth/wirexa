// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	"context"
	"encoding/base64"
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

// HTTPHandler は Wails RPC アダプターとして HTTP ユースケースを公開する。
type HTTPHandler struct {
	ctx       context.Context
	reqSvc    httpdomain.RequestUseCase
	collSvc   httpdomain.CollectionUseCase
	itemSvc   httpdomain.CollectionItemUseCase
	tempFiles TempFileProvider
	dialog    FileDialog
}

// HTTPHandlerDeps は HTTPHandler に注入する依存をまとめる。
// Dialog が nil の場合は Wails runtime のダイアログを使う。
type HTTPHandlerDeps struct {
	ReqSvc    httpdomain.RequestUseCase
	CollSvc   httpdomain.CollectionUseCase
	ItemSvc   httpdomain.CollectionItemUseCase
	TempFiles TempFileProvider
	Dialog    FileDialog
}

// SetupHTTPHandler は既存の HTTPHandler インスタンスにサービスを注入する。
// Wails の Bind に渡す前に事前確保した空ハンドラーを startup() で初期化する際に使用する。
func SetupHTTPHandler(ctx context.Context, h *HTTPHandler, deps HTTPHandlerDeps) {
	h.ctx = ctx
	h.reqSvc = deps.ReqSvc
	h.collSvc = deps.CollSvc
	h.itemSvc = deps.ItemSvc
	h.tempFiles = deps.TempFiles
	h.dialog = deps.Dialog
	if h.dialog == nil {
		h.dialog = wailsFileDialog{}
	}
}

// OpenFilePicker はネイティブのファイル選択ダイアログを開き、選択されたファイルパスを返す。
func (h *HTTPHandler) OpenFilePicker() (string, error) {
	return h.dialog.OpenFile(h.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
	})
}

// GuessFormPartContentType は form-data の file 行で自動付与される Content-Type を返す。
// UI のヒント表示と送信時の判定を同じ実装に寄せるために公開する
// （拡張子の判定は OS 依存で、フロント側に再実装すると表示と実際の送信内容がずれる）。
// パスは受け取るだけでファイルは読まないため、読み取り許可の検査は不要。
func (h *HTTPHandler) GuessFormPartContentType(path string) string {
	return httpdomain.GuessFileContentType(path)
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
func (h *HTTPHandler) SendRequest(req httpdomain.HTTPRequest) (httpdomain.HTTPResponse, error) { //nolint:gocritic // hugeParam: preserve the Wails RPC DTO's value semantics.
	res, err := h.reqSvc.SendRequest(req)
	if err != nil {
		return httpdomain.HTTPResponse{}, err
	}
	if res.BodyTruncated && h.tempFiles != nil {
		res.TempFilePath = h.tempFiles.ConsumeTempFilePath(req.ID)
	}
	return res, nil
}

// CancelRequest は指定 ID の実行中 HTTP リクエストをキャンセルする。
func (h *HTTPHandler) CancelRequest(id string) {
	h.reqSvc.CancelRequest(id)
}

// SaveResponseBody はテンポラリファイルをOSのファイル保存ダイアログで指定先に保存する。
// 保存後にテンポラリファイルを削除する。キャンセル時は何もしない。
func (h *HTTPHandler) SaveResponseBody(tempFilePath, contentType string) error {
	savePath, err := h.dialog.SaveFile(h.ctx, saveDialogOptions(contentType))
	if err != nil || savePath == "" {
		return err
	}
	if err := copyFile(tempFilePath, savePath); err != nil {
		return err
	}
	return os.Remove(tempFilePath)
}

// SaveResponseBase64 は base64 エンコードされたバイナリボディをデコードし、
// OSのファイル保存ダイアログで指定された先へ書き出す。キャンセル時は何もしない。
// 切り詰められていない非 UTF-8 レスポンスの保存に使う（temp ファイルを介さない）。
func (h *HTTPHandler) SaveResponseBase64(base64Content, contentType string) error {
	data, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		return err
	}
	savePath, err := h.dialog.SaveFile(h.ctx, saveDialogOptions(contentType))
	if err != nil || savePath == "" {
		return err
	}
	return os.WriteFile(savePath, data, 0o600)
}

// saveDialogOptions は Content-Type から保存ダイアログの既定ファイル名と filter を組み立てる。
func saveDialogOptions(contentType string) runtime.SaveDialogOptions {
	ext := contentTypeToExtension(contentType)
	return runtime.SaveDialogOptions{
		DefaultFilename: "response" + ext,
		Filters: []runtime.FileFilter{{
			DisplayName: contentType,
			Pattern:     "*" + ext,
		}},
	}
}

// GetRootItems はルートコレクションのアイテム一覧を返す。
func (h *HTTPHandler) GetRootItems() []*httpdomain.TreeItem {
	return h.collSvc.GetRootItems()
}

// GetCollections は全コレクションを返す。
func (h *HTTPHandler) GetCollections() []httpdomain.Collection {
	return h.collSvc.GetCollections()
}

// CreateCollection は新規コレクションを作成する。
func (h *HTTPHandler) CreateCollection(name string) (httpdomain.Collection, error) {
	return h.collSvc.CreateCollection(name)
}

// DeleteCollection は ID でコレクションを削除する。
func (h *HTTPHandler) DeleteCollection(id string) error {
	return h.collSvc.DeleteCollection(id)
}

// RenameCollection はコレクション名を変更する。
func (h *HTTPHandler) RenameCollection(id, name string) error {
	return h.collSvc.RenameCollection(id, name)
}

// AddFolder はコレクションにフォルダを追加する。
func (h *HTTPHandler) AddFolder(collectionID, parentID, name string) (*httpdomain.TreeItem, error) {
	return h.itemSvc.AddFolder(collectionID, parentID, name)
}

// AddRequest はコレクションにリクエストを追加する。
func (h *HTTPHandler) AddRequest(collectionID, parentID string, req httpdomain.HTTPRequest) (*httpdomain.TreeItem, error) { //nolint:gocritic // hugeParam: preserve the Wails RPC DTO's value semantics.
	return h.itemSvc.AddRequest(collectionID, parentID, req)
}

// UpdateRequest はコレクション内のリクエストを更新する。
func (h *HTTPHandler) UpdateRequest(collectionID string, req httpdomain.HTTPRequest) error { //nolint:gocritic // hugeParam: preserve the Wails RPC DTO's value semantics.
	return h.itemSvc.UpdateRequest(collectionID, req)
}

// RenameItem はコレクション内のアイテム名を変更する。
func (h *HTTPHandler) RenameItem(collectionID, itemID, name string) error {
	return h.itemSvc.RenameItem(collectionID, itemID, name)
}

// DeleteItem はコレクションからアイテムを削除する。
func (h *HTTPHandler) DeleteItem(collectionID, itemID string) error {
	return h.itemSvc.DeleteItem(collectionID, itemID)
}

// MoveItem はアイテムをコレクション内外・別の親・位置へ移動する。
// targetParentID が空文字の場合はターゲットコレクションルートへ移動する。
// position は挿入先インデックス（削除後）。-1 の場合は末尾に追加する。
func (h *HTTPHandler) MoveItem(sourceCollectionID, itemID, targetCollectionID, targetParentID string, position int) error {
	return h.itemSvc.MoveItem(sourceCollectionID, itemID, targetCollectionID, targetParentID, position)
}

// GetSidebarLayout はサイドバーレイアウトを返す。
func (h *HTTPHandler) GetSidebarLayout() ([]httpdomain.SidebarEntry, error) {
	return h.collSvc.GetSidebarLayout()
}

// MoveSidebarEntry はサイドバー上のエントリを指定位置に移動する。
func (h *HTTPHandler) MoveSidebarEntry(kind, id string, position int) error {
	return h.collSvc.MoveSidebarEntry(kind, id, position)
}

// MoveItemToSidebar はアイテムを指定コレクションから __root__ へ移動し、
// サイドバーレイアウトの指定位置に挿入する。
func (h *HTTPHandler) MoveItemToSidebar(sourceCollectionID, itemID string, sidebarPosition int) error {
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
