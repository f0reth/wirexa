// Package mqttapp は MQTT アプリケーション層を提供する。
package mqttapp

import (
	"fmt"
	"sync"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// コンパイル時に domain.ProfileUseCase を満たすことを検証
var _ domain.ProfileUseCase = (*ProfileService)(nil)

// ProfileService は MQTT ブローカープロファイルの CRUD を管理するアプリケーションサービス。
type ProfileService struct {
	repo     domain.ProfileRepository
	mu       sync.RWMutex
	profiles map[string]domain.BrokerProfile
}

// NewProfileService はリポジトリからプロファイルをロードして ProfileService を生成する。
func NewProfileService(repo domain.ProfileRepository) (*ProfileService, error) {
	loaded, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load profiles: %w", err)
	}
	profiles := make(map[string]domain.BrokerProfile, len(loaded))
	for _, p := range loaded {
		profiles[p.ID] = p
	}
	return &ProfileService{repo: repo, profiles: profiles}, nil
}

// GetProfiles は全プロファイルのコピーを返す。
func (s *ProfileService) GetProfiles() []domain.BrokerProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.BrokerProfile, 0, len(s.profiles))
	for _, p := range s.profiles {
		result = append(result, p)
	}
	return result
}

// SaveProfile はプロファイルを保存（追加または更新）する。
func (s *ProfileService) SaveProfile(profile domain.BrokerProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.repo.Save(&profile); err != nil {
		return err
	}
	s.profiles[profile.ID] = profile
	return nil
}

// DeleteProfile はプロファイルを削除する。
func (s *ProfileService) DeleteProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	delete(s.profiles, id)
	return nil
}
