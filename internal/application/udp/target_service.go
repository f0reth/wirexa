package udpapp

import (
	"fmt"
	"sync"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.TargetUseCase = (*TargetService)(nil)

// TargetService はターゲット管理ユースケースの実装。
type TargetService struct {
	repo    domain.TargetRepository
	mu      sync.RWMutex
	targets map[string]domain.UdpTarget
}

// NewTargetService は TargetService を生成する。
func NewTargetService(repo domain.TargetRepository) (*TargetService, error) {
	loaded, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load targets: %w", err)
	}
	targets := make(map[string]domain.UdpTarget, len(loaded))
	for _, t := range loaded {
		targets[t.ID] = t
	}
	return &TargetService{repo: repo, targets: targets}, nil
}

// GetTargets は全ターゲットのコピーを返す。
func (s *TargetService) GetTargets() []domain.UdpTarget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.UdpTarget, 0, len(s.targets))
	for _, t := range s.targets {
		result = append(result, t)
	}
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

	s.targets[target.ID] = target
	return target, nil
}

// DeleteTarget は ID でターゲットを削除する。
func (s *TargetService) DeleteTarget(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.targets[id]; !ok {
		return &cmn.NotFoundError{Resource: "target", ID: id}
	}
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	delete(s.targets, id)
	return nil
}
