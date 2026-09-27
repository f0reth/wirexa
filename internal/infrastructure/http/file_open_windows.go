package httpinfra

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// openSelectedFile は送信するファイルを、読み取りだけを共有する共有モード (FILE_SHARE_READ) で開く。
// 書き込みと削除を共有しないので、開いている間は他のプロセスが書き込めず、削除もリネームもできない。
// 送る内容は開いた時点のまま変わらない。
// 他のプロセスが書き込み用に開いているファイルは開けないので ErrSelectedFileInUse を返す。
// それ以外の失敗は ErrSelectedFileUnavailable で、どちらもパスを含めない。
func openSelectedFile(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(longPath(path))
	if err != nil {
		return nil, domain.ErrSelectedFileUnavailable
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, domain.ErrSelectedFileInUse
		}
		return nil, domain.ErrSelectedFileUnavailable
	}
	return os.NewFile(uintptr(h), path), nil
}
