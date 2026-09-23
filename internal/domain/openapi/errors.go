package openapidomain

import "errors"

var (
	// ErrFileAccessDenied はファイルダイアログで許可されていないパスへのアクセスを示す。
	ErrFileAccessDenied = errors.New("access denied: path was not granted via a file dialog")
	// ErrRecentsCorrupt は recents ファイルは読めたが JSON として解釈できないことを示す。
	// 読み込み自体の失敗 (権限・ロック等) とは区別する。
	ErrRecentsCorrupt = errors.New("openapi recents file is corrupt")
)
