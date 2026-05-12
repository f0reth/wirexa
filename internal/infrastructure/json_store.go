// Package infrastructure は共有インフラストラクチャユーティリティを提供する。
package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// JSONStore は JSON ファイルによる汎用永続化ストア。
// T は保存するドメイン型、getID は T から一意な ID を取得する関数。
type JSONStore[T any] struct {
	dir   string
	getID func(*T) string
}

// NewJSONStore はディレクトリを作成して JSONStore を返す。
func NewJSONStore[T any](dir string, getID func(*T) string) (*JSONStore[T], error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &JSONStore[T]{dir: dir, getID: getID}, nil
}

// Load はディレクトリ内の全 JSON ファイルからアイテムを読み込む。
func (s *JSONStore[T]) Load() ([]T, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var items []T
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var item T
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// Save はアイテムを JSON ファイルに書き込む。
// tmp ファイル経由の原子的置き換えで書き込み中断によるデータ破損を防ぐ。
func (s *JSONStore[T]) Save(item *T) error {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	dest := filepath.Join(s.dir, s.getID(item)+".json")
	tmp, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dest)
}

// Delete はアイテムの JSON ファイルを削除する。
func (s *JSONStore[T]) Delete(id string) error {
	return os.Remove(filepath.Join(s.dir, id+".json"))
}
