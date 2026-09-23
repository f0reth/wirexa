package adapters

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	openapidomain "github.com/f0reth/Wirexa/internal/domain/openapi"
)

// openapiFileFilters は OpenAPI ファイルのダイアログで使うフィルタ。
var openapiFileFilters = []runtime.FileFilter{
	{DisplayName: "OpenAPI (*.yaml, *.yml, *.json)", Pattern: "*.yaml;*.yml;*.json"},
}

// OpenAPIHandler は OpenAPI ファイル I/O を Wails RPC として公開する。
//
// セキュリティ: Wails RPC は webview 上の JS から誰でも呼べるため、ReadFile /
// WriteFile が受理するパスはユースケース側の許可リストに登録済みのものに限られる。
// 許可リストへ登録する OpenSelected / SaveSelected へ渡すのは OS ダイアログの
// 戻り値だけとし、RPC 引数のパスを許可する経路は作らないこと。
type OpenAPIHandler struct {
	ctx    context.Context
	files  openapidomain.FileUseCase
	dialog FileDialog
}

// OpenAPIHandlerDeps は OpenAPIHandler に注入する依存をまとめる。
// Dialog が nil の場合は Wails runtime のダイアログを使う。
type OpenAPIHandlerDeps struct {
	Files  openapidomain.FileUseCase
	Dialog FileDialog
}

// SetupOpenAPIHandler は既存の OpenAPIHandler インスタンスに ctx とユースケースを注入する。
// Wails の Bind に渡す前に事前確保した空ハンドラーを startup() で初期化する際に使用する。
func SetupOpenAPIHandler(ctx context.Context, h *OpenAPIHandler, deps OpenAPIHandlerDeps) {
	h.ctx = ctx
	h.files = deps.Files
	h.dialog = deps.Dialog
	if h.dialog == nil {
		h.dialog = wailsFileDialog{}
	}
}

// OpenFilePicker はネイティブのファイル選択ダイアログを開き、選択されたパスを
// 許可リストと recents に登録してから返す。キャンセル時は ("", nil)。
func (h *OpenAPIHandler) OpenFilePicker() (string, error) {
	path, err := h.dialog.OpenFile(h.ctx, runtime.OpenDialogOptions{
		Title:   "Open OpenAPI File",
		Filters: openapiFileFilters,
	})
	if err != nil || path == "" {
		return "", err
	}
	return h.files.OpenSelected(path), nil
}

// SaveFileAs はネイティブの保存ダイアログを開き、選択先へ content を書き込む。
// 選択先を許可リストと recents に登録し、保存パスを返す。キャンセル時は ("", nil)。
// D&D / ペーストで開いた無題文書の初回保存に使う。
func (h *OpenAPIHandler) SaveFileAs(defaultName, content string) (string, error) {
	path, err := h.dialog.SaveFile(h.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Filters:         openapiFileFilters,
	})
	if err != nil || path == "" {
		return "", err
	}
	return h.files.SaveSelected(path, content)
}

// ReadFile は許可リストに登録済みのパスの内容を文字列で返す。未許可なら拒否する。
func (h *OpenAPIHandler) ReadFile(path string) (string, error) {
	return h.files.ReadFile(path)
}

// WriteFile は許可リストに登録済みのパスへ content を書き込む。未許可なら拒否する。
func (h *OpenAPIHandler) WriteFile(path, content string) error {
	return h.files.WriteFile(path, content)
}

// GetRecents は最近使ったファイル一覧を order 昇順で返す。
func (h *OpenAPIHandler) GetRecents() []openapidomain.OpenAPIRecent {
	return h.files.GetRecents()
}

// RemoveRecent は recents と許可リストからパスを削除する。
func (h *OpenAPIHandler) RemoveRecent(path string) error {
	return h.files.RemoveRecent(path)
}

// MoveRecent は recents 内でパスを指定位置へ並び替える。
func (h *OpenAPIHandler) MoveRecent(path string, index int) error {
	return h.files.MoveRecent(path, index)
}
