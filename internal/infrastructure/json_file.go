package infrastructure

import (
	"encoding/json"
	"os"
)

// WriteJSONFile は v を JSON へマーシャルし path へ原子的に書き込む。
// インデント形式を 1 箇所に集約するため、設定ファイルの書き出しはすべてここを通す。
func WriteJSONFile(path string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(path, data, perm)
}
