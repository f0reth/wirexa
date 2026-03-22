// Package httpinfra は HTTP インフラストラクチャ層を提供する。
package httpinfra

import (
	"encoding/json"
	"os"
	"path/filepath"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// コンパイル時に domain.CollectionRepository を満たすことを検証
var _ domain.CollectionRepository = (*JSONFileRepository)(nil)

// JSONFileRepository は JSON ファイルでコレクションを永続化するリポジトリ。
type JSONFileRepository struct {
	dir string
}

// NewJSONFileRepository はディレクトリを作成してリポジトリを返す。
func NewJSONFileRepository(dir string) (*JSONFileRepository, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &JSONFileRepository{dir: dir}, nil
}

// Load はディレクトリ内の全 JSON ファイルからコレクションを読み込む。
func (r *JSONFileRepository) Load() ([]domain.Collection, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var cols []domain.Collection
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var col domain.Collection
		if err := json.Unmarshal(data, &col); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, nil
}

// Save はコレクションを JSON ファイルに書き込む。
// シリアライズはロック外（呼び出し元が責任を持つ）。
func (r *JSONFileRepository) Save(c *domain.Collection) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.dir, c.ID+".json"), data, 0o600)
}

// Delete はコレクションの JSON ファイルを削除する。
func (r *JSONFileRepository) Delete(id string) error {
	return os.Remove(filepath.Join(r.dir, id+".json"))
}
