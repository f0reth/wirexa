// Package openapiinfra は OpenAPI ファイル操作の出力ポート実装を提供する。
package openapiinfra

import (
	"os"

	domain "github.com/f0reth/Wirexa/internal/domain/openapi"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

// コンパイル時に domain.FileAccess を満たすことを検証
var _ domain.FileAccess = (*NativeFileAccess)(nil)

// NativeFileAccess は OS のファイルシステムへ直接読み書きする FileAccess 実装。
// 許可の確認は呼び出し側 (application) の責務で、ここではパスを検証しない。
type NativeFileAccess struct{}

// NewNativeFileAccess は NativeFileAccess を返す。
func NewNativeFileAccess() *NativeFileAccess {
	return &NativeFileAccess{}
}

// ReadFile は path の内容を返す。
func (*NativeFileAccess) ReadFile(path string) ([]byte, error) {
	// #nosec G304 -- application が許可リスト (ダイアログで選ばれたパス) で確認したパスだけが渡る。
	return os.ReadFile(path)
}

// WriteFile は data を path へ原子的に書き込む。
func (*NativeFileAccess) WriteFile(path string, data []byte) error {
	return infra.AtomicWriteFile(path, data, 0o600)
}
