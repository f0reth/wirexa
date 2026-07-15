// Package mqttapp は MQTT アプリケーション層を提供する。
package mqttapp

import (
	"github.com/f0reth/Wirexa/internal/application/store"
	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// コンパイル時に domain.ProfileUseCase を満たすことを検証
var _ domain.ProfileUseCase = (*ProfileService)(nil)

// ProfileService は MQTT ブローカープロファイルの CRUD を管理するアプリケーションサービス。
type ProfileService struct {
	store *store.CachedStore[domain.BrokerProfile]
}

// NewProfileService はリポジトリからプロファイルをロードして ProfileService を生成する。
func NewProfileService(repo domain.ProfileRepository) (*ProfileService, error) {
	cs, err := store.NewCachedStore[domain.BrokerProfile](
		cmn.ResourceProfile, repo,
		func(p domain.BrokerProfile) string { return p.ID },
		func(p *domain.BrokerProfile, id string) { p.ID = id },
	)
	if err != nil {
		return nil, err
	}
	return &ProfileService{store: cs}, nil
}

// GetProfiles は全プロファイルのコピーを返す。
func (s *ProfileService) GetProfiles() []domain.BrokerProfile {
	return s.store.GetAll()
}

// SaveProfile はプロファイルを保存（追加または更新）する。
func (s *ProfileService) SaveProfile(profile domain.BrokerProfile) error {
	_, err := s.store.Save(profile)
	return err
}

// DeleteProfile はプロファイルを削除する。存在しない ID の場合は NotFoundError を返す。
func (s *ProfileService) DeleteProfile(id string) error {
	return s.store.Delete(id)
}
