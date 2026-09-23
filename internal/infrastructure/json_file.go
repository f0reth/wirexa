package infrastructure

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"

	"github.com/f0reth/Wirexa/internal/domain"
)

// ReadJSONFile は path の JSON を T へ読み込む。設定ファイルの読み込みはすべてここを通し、
// 「未作成」「破損」「読み込み失敗」の分類をこの 1 か所で決める。
//   - ファイルが存在しない: found=false, err=nil
//   - JSON として解釈できない: domain.ErrCorruptData を wrap したエラー
//   - それ以外の I/O エラー (権限・ロック等): そのまま返す (破損とは扱わない)
func ReadJSONFile[T any](path string) (v T, found bool, err error) {
	// #nosec G304 -- path はアプリが決める設定ファイルのパスで、外部入力ではない。
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return v, false, nil
		}
		return v, false, err
	}
	if err := json.Unmarshal(data, &v); err != nil {
		var zero T
		return zero, true, fmt.Errorf("%w: %w", domain.ErrCorruptData, err)
	}
	return v, true, nil
}

// WriteJSONFile は v を JSON へマーシャルし path へ原子的に書き込む。
// インデント形式を 1 箇所に集約するため、設定ファイルの書き出しはすべてここを通す。
func WriteJSONFile(path string, v any, perm os.FileMode) error {
	data, err := json.Marshal(v, jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	return AtomicWriteFile(path, data, perm)
}
