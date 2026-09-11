package adapters

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// FileDialog はネイティブのファイル選択・保存ダイアログを表す。
// ハンドラーが Wails runtime を直接呼ばないようにし、ダイアログ前の拒否や
// キャンセル時の挙動を unit test で決定的に検証できるようにする。
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
