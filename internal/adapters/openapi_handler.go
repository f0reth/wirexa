// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

// errAccessDenied は許可されていないパスへのアクセス時に返す。
var errAccessDenied = errors.New("access denied: path was not granted via a file dialog")

// OpenAPIHandler は OpenAPI ファイル I/O を Wails RPC として公開する。
//
// セキュリティ: Wails RPC は webview 上の JS から誰でも呼べるため、ReadFile /
// WriteFile が受理するパスは「OS ダイアログ (OpenFileDialog / SaveFileDialog) の
// 戻り値」に由来するもの (= 許可リスト granted に登録済み) のみに限定する。
// 許可リストへの登録は Go 内で完結するダイアログ / 保存ハンドラ、および起動時の
// recents seed 経由でしか行われず、JS 入力からは汚染できない。
type OpenAPIHandler struct {
	ctx     context.Context
	granted map[string]struct{}
	recents *openapiRecentStore
	mu      sync.Mutex
}

// SetupOpenAPIHandler は既存の OpenAPIHandler インスタンスに ctx と recents ストアを
// 注入し、永続化された recents のパスを許可リストへ seed する。
func SetupOpenAPIHandler(ctx context.Context, h *OpenAPIHandler, recentsPath string, logger cmn.Logger) {
	h.ctx = ctx
	h.granted = make(map[string]struct{})
	h.recents = newOpenapiRecentStore(recentsPath, logger)
	for _, p := range h.recents.paths() {
		h.granted[filepath.Clean(p)] = struct{}{}
	}
}

// grant はパスを許可リストへ登録する。cleaned なパスを返す。
func (h *OpenAPIHandler) grant(path string) string {
	cleaned := filepath.Clean(path)
	h.mu.Lock()
	h.granted[cleaned] = struct{}{}
	h.mu.Unlock()
	return cleaned
}

// isGranted はパスが許可リストに登録済みかを返す。
func (h *OpenAPIHandler) isGranted(path string) bool {
	cleaned := filepath.Clean(path)
	h.mu.Lock()
	_, ok := h.granted[cleaned]
	h.mu.Unlock()
	return ok
}

// OpenFilePicker はネイティブのファイル選択ダイアログを開き、選択されたパスを
// 許可リストと recents に登録してから返す。キャンセル時は ("", nil)。
func (h *OpenAPIHandler) OpenFilePicker() (string, error) {
	path, err := runtime.OpenFileDialog(h.ctx, runtime.OpenDialogOptions{
		Title: "Open OpenAPI File",
		Filters: []runtime.FileFilter{
			{DisplayName: "OpenAPI (*.yaml, *.yml, *.json)", Pattern: "*.yaml;*.yml;*.json"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	cleaned := h.grant(path)
	_ = h.recents.add(cleaned, filepath.Base(cleaned)) //nolint:errcheck // recents 永続化失敗は致命的でない
	return cleaned, nil
}

// SaveFileAs はネイティブの保存ダイアログを開き、選択先へ content を原子的に書き込む。
// 選択先を許可リストと recents に登録し、保存パスを返す。キャンセル時は ("", nil)。
// D&D / ペーストで開いた無題文書の初回保存に使う。
func (h *OpenAPIHandler) SaveFileAs(defaultName, content string) (string, error) {
	path, err := runtime.SaveFileDialog(h.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "OpenAPI (*.yaml, *.yml, *.json)", Pattern: "*.yaml;*.yml;*.json"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	cleaned := h.grant(path)
	if err := infra.AtomicWriteFile(cleaned, []byte(content), 0o600); err != nil {
		return "", err
	}
	_ = h.recents.add(cleaned, filepath.Base(cleaned)) //nolint:errcheck // recents 永続化失敗は致命的でない
	return cleaned, nil
}

// ReadFile は許可リストに登録済みのパスの内容を文字列で返す。未許可なら拒否する。
func (h *OpenAPIHandler) ReadFile(path string) (string, error) {
	if !h.isGranted(path) {
		return "", errAccessDenied
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile は許可リストに登録済みのパスへ content を原子的に書き込む。未許可なら拒否する。
func (h *OpenAPIHandler) WriteFile(path, content string) error {
	if !h.isGranted(path) {
		return errAccessDenied
	}
	return infra.AtomicWriteFile(filepath.Clean(path), []byte(content), 0o600)
}

// GetRecents は最近使ったファイル一覧を order 昇順で返す。
func (h *OpenAPIHandler) GetRecents() []OpenAPIRecent {
	return h.recents.list()
}

// RemoveRecent は recents と許可リストからパスを削除する。
func (h *OpenAPIHandler) RemoveRecent(path string) error {
	cleaned := filepath.Clean(path)
	h.mu.Lock()
	delete(h.granted, cleaned)
	h.mu.Unlock()
	return h.recents.remove(path)
}

// MoveRecent は recents 内でパスを指定位置へ並び替える。
func (h *OpenAPIHandler) MoveRecent(path string, index int) error {
	return h.recents.move(path, index)
}
