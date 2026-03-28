package udpapp

import (
	"fmt"
	"sync"

	"github.com/google/uuid"

	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.TargetUseCase = (*TargetService)(nil)

// TargetService はターゲット管理ユースケースの実装。
type TargetService struct {
	repo    domain.TargetRepository
	mu      sync.RWMutex
	targets []domain.UdpTarget
}

// NewTargetService は TargetService を生成する。
func NewTargetService(repo domain.TargetRepository) (*TargetService, error) {
	loaded, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load targets: %w", err)
	}
	if loaded == nil {
		loaded = []domain.UdpTarget{}
	}
	return &TargetService{repo: repo, targets: loaded}, nil
}

// GetTargets は全ターゲットのコピーを返す。
func (s *TargetService) GetTargets() []domain.UdpTarget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.UdpTarget, len(s.targets))
	copy(result, s.targets)
	return result
}

// SaveTarget はターゲットを保存する。ID が空の場合は新規生成する。
func (s *TargetService) SaveTarget(target domain.UdpTarget) (domain.UdpTarget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if target.ID == "" {
		target.ID = uuid.New().String()
	}

	if err := s.repo.Save(&target); err != nil {
		return domain.UdpTarget{}, err
	}

	for i, t := range s.targets {
		if t.ID == target.ID {
			s.targets[i] = target
			return target, nil
		}
	}
	s.targets = append(s.targets, target)
	return target, nil
}

// DeleteTarget は ID でターゲットを削除する。
func (s *TargetService) DeleteTarget(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.targets {
		if t.ID == id {
			s.targets = append(s.targets[:i], s.targets[i+1:]...)
			return s.repo.Delete(id)
		}
	}
	return &domain.NotFoundError{Resource: "target", ID: id}
}
