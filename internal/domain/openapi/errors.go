package openapidomain

import "errors"

// ErrFileAccessDenied はファイルダイアログで許可されていないパスへのアクセスを示す。
var ErrFileAccessDenied = errors.New("access denied: path was not granted via a file dialog")
