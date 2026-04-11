// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	"context"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// OpenAPIHandler は OpenAPI ファイル I/O を Wails RPC として公開する。
type OpenAPIHandler struct {
	ctx context.Context
}

// SetupOpenAPIHandler は既存の OpenAPIHandler インスタンスにコンテキストを注入する。
func SetupOpenAPIHandler(ctx context.Context, h *OpenAPIHandler) {
	h.ctx = ctx
}

// OpenFilePicker はネイティブのファイル選択ダイアログを開き、
// .yaml / .yml / .json フィルターを適用して選択されたパスを返す。
func (h *OpenAPIHandler) OpenFilePicker() (string, error) {
	return runtime.OpenFileDialog(h.ctx, runtime.OpenDialogOptions{
		Title: "Open OpenAPI File",
		Filters: []runtime.FileFilter{
			{DisplayName: "OpenAPI (*.yaml, *.yml, *.json)", Pattern: "*.yaml;*.yml;*.json"},
		},
	})
}

// ReadFile は指定パスのファイル内容を文字列で返す。
func (h *OpenAPIHandler) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile は指定パスにコンテンツを書き込む。
func (h *OpenAPIHandler) WriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
