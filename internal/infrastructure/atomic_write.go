// Package infrastructure は共有インフラストラクチャユーティリティを提供する。
package infrastructure

import (
	"os"
	"path/filepath"
)

// AtomicWriteFile は data を path へ原子的に書き込む。
// 一時ファイルに書いてから os.Rename で置き換えるので、中断しても path は壊れない。
// 一時ファイルは path と同じディレクトリに作る (別ボリュームからは os.Rename で置き換えられない)。
// エラー時は一時ファイルをベストエフォートで削除する。
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()        //nolint:errcheck // 後始末。失敗は無視してよい
		_ = os.Remove(tmpName) //nolint:errcheck // 後始末。失敗は無視してよい
		return err
	}
	if err = tmp.Chmod(perm); err != nil {
		_ = tmp.Close()        //nolint:errcheck // 後始末。失敗は無視してよい
		_ = os.Remove(tmpName) //nolint:errcheck // 後始末。失敗は無視してよい
		return err
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName) //nolint:errcheck // 後始末。失敗は無視してよい
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName) //nolint:errcheck // 後始末。失敗は無視してよい
		return err
	}
	return nil
}
