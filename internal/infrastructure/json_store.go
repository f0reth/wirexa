// Package infrastructure は共有インフラストラクチャユーティリティを提供する。
package infrastructure

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/f0reth/Wirexa/internal/domain"
)

// JSONStore は JSON ファイルによる汎用永続化ストア。
// T は保存するドメイン型、getID は T から一意な ID を取得する関数。
type JSONStore[T any] struct {
	logger domain.Logger
	getID  func(*T) string
	dir    string
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
		// path は ReadDir が返した s.dir 直下のエントリ名で、外部入力ではない。
		path := filepath.Join(s.dir, e.Name())
		item, found, err := ReadJSONFile[T](path)
		switch {
		case errors.Is(err, domain.ErrCorruptData):
			// 破損ファイルは .corrupt へ退避してスキップし、読めた分だけで継続する。
			dest, rerr := QuarantineFile(path)
			if rerr != nil {
				s.logf("json_store: failed to quarantine corrupt file", "file", e.Name(), "error", err, "quarantineError", rerr)
			} else {
				s.logf("json_store: quarantined corrupt file", "file", e.Name(), "quarantined", filepath.Base(dest), "error", err)
			}
			continue
		case err != nil:
			// 読めないファイルは触らずスキップして起動を継続する。
			s.logf("json_store: failed to read file, skipping", "file", e.Name(), "error", err)
			continue
		case !found:
			// ReadDir の後に消えたファイルは読み込み対象から外す。
			continue
		}
		// ファイルの中身に書かれた ID はそのままドメイン型とキャッシュのキーになる。
		// ストア外を指す ID でキャッシュを汚さないよう、規則に反する項目はスキップする
		// (JSON としては妥当なので破損ファイルと違い .corrupt へは退避しない)。
		if err := domain.ValidateID(s.getID(&item)); err != nil {
			s.logf("json_store: skipping item with unsafe id", "file", e.Name(), "error", err)
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// logf は logger が設定されていればエラーとして記録する。
func (s *JSONStore[T]) logf(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Error(msg, args...)
	}
}

// resolve は ID からストア内のファイルパスを組み立てる。
// ID 規則の検証とパス封じ込めの 2 段構えで、規則の取りこぼしがあっても
// ストア外を指すパスを返さないようにする。
func (s *JSONStore[T]) resolve(id string) (string, error) {
	if err := domain.ValidateID(id); err != nil {
		return "", err
	}
	name := id + ".json"
	dest := filepath.Join(s.dir, name)
	rel, err := filepath.Rel(s.dir, dest)
	if err != nil || rel != name {
		return "", &domain.ValidationError{Field: "id", Message: "resolves outside the store directory"}
	}
	return dest, nil
}

// Save はアイテムを JSON ファイルに書き込む。
// tmp ファイル経由の原子的置き換えで書き込み中断によるデータ破損を防ぐ。
func (s *JSONStore[T]) Save(item *T) error {
	dest, err := s.resolve(s.getID(item))
	if err != nil {
		return err
	}
	return WriteJSONFile(dest, item, 0o600)
}

// Exists は id のファイルが存在するかを返す。Load で読み飛ばしたファイルも存在として扱う。
// 存在しない場合は false と nil、有無を確認できない場合はエラーを返す。
func (s *JSONStore[T]) Exists(id string) (bool, error) {
	dest, err := s.resolve(id)
	if err != nil {
		return false, err
	}
	if _, err := os.Lstat(dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete はアイテムの JSON ファイルを削除する。
func (s *JSONStore[T]) Delete(id string) error {
	dest, err := s.resolve(id)
	if err != nil {
		return err
	}
	return os.Remove(dest)
}
