// Package infrastructure は共有インフラストラクチャユーティリティを提供する。
package infrastructure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/f0reth/Wirexa/internal/domain"
)

// JSONStore は JSON ファイルによる汎用永続化ストア。
// T は保存するドメイン型、getID は T から一意な ID を取得する関数。
type JSONStore[T any] struct {
	getID  func(*T) string
	dir    string
	logger domain.Logger
}

// SetLogger は破損ファイル退避などの記録に使うロガーを注入する。
// 未設定 (nil) の場合はログ出力をスキップする。
func (s *JSONStore[T]) SetLogger(l domain.Logger) {
	s.logger = l
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
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			// 読めないファイルは触らずスキップして起動を継続する。
			s.logf("json_store: failed to read file, skipping", "file", e.Name(), "error", err)
			continue
		}
		var item T
		if err := json.Unmarshal(data, &item); err != nil {
			// 破損ファイルは .corrupt へ退避してスキップし、読めた分だけで継続する。
			dest, rerr := s.quarantine(path)
			if rerr != nil {
				s.logf("json_store: failed to quarantine corrupt file", "file", e.Name(), "error", rerr)
			} else {
				s.logf("json_store: quarantined corrupt file", "file", e.Name(), "quarantined", filepath.Base(dest), "error", err)
			}
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// quarantine は破損ファイルを "<path>.corrupt" へリネーム退避する。
// 退避先が既存の場合は "<path>.corrupt.<unixnano>" へフォールバックする。
// 退避後のパスを返す。
func (s *JSONStore[T]) quarantine(path string) (string, error) {
	dest := path + ".corrupt"
	if _, err := os.Stat(dest); err == nil {
		dest = fmt.Sprintf("%s.corrupt.%d", path, time.Now().UnixNano())
	}
	if err := os.Rename(path, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// logf は logger が設定されていればエラーとして記録する。
func (s *JSONStore[T]) logf(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Error(msg, args...)
	}
}

// Save はアイテムを JSON ファイルに書き込む。
// tmp ファイル経由の原子的置き換えで書き込み中断によるデータ破損を防ぐ。
func (s *JSONStore[T]) Save(item *T) error {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	dest := filepath.Join(s.dir, s.getID(item)+".json")
	return AtomicWriteFile(dest, data, 0o600)
}

// Delete はアイテムの JSON ファイルを削除する。
func (s *JSONStore[T]) Delete(id string) error {
	return os.Remove(filepath.Join(s.dir, id+".json"))
}
