package adapters

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// FileDialog はネイティブのファイル選択・保存ダイアログを表す。
// テストで差し替えられるよう、ハンドラーは Wails runtime を直接呼ばずにこれを使う。
type FileDialog interface {
	OpenFile(ctx context.Context, options runtime.OpenDialogOptions) (string, error)
	SaveFile(ctx context.Context, options runtime.SaveDialogOptions) (string, error)
}

// wailsFileDialog は Wails runtime を使う本番用の FileDialog。
type wailsFileDialog struct{}

func (wailsFileDialog) OpenFile(ctx context.Context, options runtime.OpenDialogOptions) (string, error) {
	return runtime.OpenFileDialog(ctx, options)
}

func (wailsFileDialog) SaveFile(ctx context.Context, options runtime.SaveDialogOptions) (string, error) {
	return runtime.SaveFileDialog(ctx, options)
}
