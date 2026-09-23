//go:build !windows

package httpinfra

import (
	"os"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// openSelectedFile は送信するファイルを開く。Windows 以外のファイルロックは協調型で、
// 他のプロセスの書き込みを OS が止めないため、送信中の変更は CheckUnchanged で検出する。
// 失敗は ErrSelectedFileUnavailable で、パスを含めない。
func openSelectedFile(path string) (*os.File, error) {
	f, err := os.Open(path) //nolint:gosec // path はファイルダイアログの戻り値で、登録済み token からしか引けない
	if err != nil {
		return nil, domain.ErrSelectedFileUnavailable
	}
	return f, nil
}
