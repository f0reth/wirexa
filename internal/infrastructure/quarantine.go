package infrastructure

import (
	"fmt"
	"os"
	"time"
)

// QuarantineFile は破損ファイルを "<path>.corrupt" へリネーム退避する。
// 退避先が既存の場合は "<path>.corrupt.<unixnano>" へフォールバックし、
// 既存の退避ファイルは上書きしない。退避後のパスを返す。
func QuarantineFile(path string) (string, error) {
	dest := path + ".corrupt"
	if _, err := os.Stat(dest); err == nil {
		dest = fmt.Sprintf("%s.corrupt.%d", path, time.Now().UnixNano())
	}
	if err := os.Rename(path, dest); err != nil {
		return "", err
	}
	return dest, nil
}
