// Package mqttinfra は MQTT インフラストラクチャ層を提供する。
package mqttinfra

import (
	infra "github.com/f0reth/Wirexa/internal/infrastructure"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// コンパイル時に domain.ProfileRepository を満たすことを検証
var _ domain.ProfileRepository = (*JSONProfileRepository)(nil)

// JSONProfileRepository は JSON ファイルで MQTT プロファイルを永続化するリポジトリ。
type JSONProfileRepository struct {
	store *infra.JSONStore[domain.BrokerProfile]
}

// NewJSONProfileRepository はディレクトリを作成してリポジトリを返す。
func NewJSONProfileRepository(dir string) (*JSONProfileRepository, error) {
	store, err := infra.NewJSONStore(dir, func(p *domain.BrokerProfile) string { return p.ID })
	if err != nil {
		return nil, err
	}
	return &JSONProfileRepository{store: store}, nil
}

// Load はディレクトリ内の全 JSON ファイルからプロファイルを読み込む。
func (r *JSONProfileRepository) Load() ([]domain.BrokerProfile, error) { return r.store.Load() }

// Save はプロファイルを JSON ファイルに書き込む。
func (r *JSONProfileRepository) Save(p *domain.BrokerProfile) error { return r.store.Save(p) }

// Delete はプロファイルの JSON ファイルを削除する。
func (r *JSONProfileRepository) Delete(id string) error { return r.store.Delete(id) }
