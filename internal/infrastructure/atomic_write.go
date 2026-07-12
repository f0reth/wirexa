// Package infrastructure は共有インフラストラクチャユーティリティを提供する。
package infrastructure

import (
	"os"
	"path/filepath"
)

// AtomicWriteFile は data を path へ原子的に書き込む。
// 書き込み先と同一ディレクトリに一時ファイルを作成し、書き込み・close 後に
// os.Rename で置き換えることで、書き込み中断によるファイル破損を防ぐ。
// エラー時は一時ファイルをベストエフォートで削除する。
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()        //nolint:errcheck // best-effort cleanup
		_ = os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return err
	}
	if err = tmp.Chmod(perm); err != nil {
		_ = tmp.Close()        //nolint:errcheck // best-effort cleanup
		_ = os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return err
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return err
	}
	return os.Rename(tmpName, path)
}
