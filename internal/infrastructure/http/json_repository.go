// Package httpinfra は HTTP インフラストラクチャ層を提供する。
package httpinfra

import (
	infra "github.com/f0reth/Wirexa/internal/infrastructure"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// コンパイル時に domain.CollectionRepository を満たすことを検証
var _ domain.CollectionRepository = (*JSONFileRepository)(nil)

// JSONFileRepository は JSON ファイルでコレクションを永続化するリポジトリ。
type JSONFileRepository struct {
	store *infra.JSONStore[domain.Collection]
}

// NewJSONFileRepository はディレクトリを作成してリポジトリを返す。
func NewJSONFileRepository(dir string) (*JSONFileRepository, error) {
	store, err := infra.NewJSONStore(dir, func(c *domain.Collection) string { return c.ID })
	if err != nil {
		return nil, err
	}
	return &JSONFileRepository{store: store}, nil
}

// Load はディレクトリ内の全 JSON ファイルからコレクションを読み込む。
func (r *JSONFileRepository) Load() ([]domain.Collection, error) { return r.store.Load() }

// Save はコレクションを JSON ファイルに書き込む。
func (r *JSONFileRepository) Save(c *domain.Collection) error { return r.store.Save(c) }

// Delete はコレクションの JSON ファイルを削除する。
func (r *JSONFileRepository) Delete(id string) error { return r.store.Delete(id) }
