// Package udpinfra は UDP インフラストラクチャ層を提供する。
package udpinfra

import (
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

var _ domain.TargetRepository = (*JSONTargetRepository)(nil)

// JSONTargetRepository は JSON ファイルによるターゲット永続化実装。
type JSONTargetRepository struct {
	store *infra.JSONStore[domain.UdpTarget]
}

// NewJSONTargetRepository は指定ディレクトリに JSONTargetRepository を生成する。
func NewJSONTargetRepository(dir string) (*JSONTargetRepository, error) {
	store, err := infra.NewJSONStore(dir, func(t *domain.UdpTarget) string { return t.ID })
	if err != nil {
		return nil, err
	}
	return &JSONTargetRepository{store: store}, nil
}

// Load は全ターゲットを読み込む。
func (r *JSONTargetRepository) Load() ([]domain.UdpTarget, error) { return r.store.Load() }

// Save はターゲットを保存する。
func (r *JSONTargetRepository) Save(t *domain.UdpTarget) error { return r.store.Save(t) }

// Delete は ID でターゲットを削除する。
func (r *JSONTargetRepository) Delete(id string) error { return r.store.Delete(id) }
