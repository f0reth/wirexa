package udpapp

import (
	"github.com/f0reth/Wirexa/internal/application/store"
	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.TargetUseCase = (*TargetService)(nil)

// TargetService はターゲット管理ユースケースの実装。
type TargetService struct {
	store *store.CachedStore[domain.UDPTarget]
}

// NewTargetService は TargetService を生成する。
func NewTargetService(repo domain.TargetRepository) (*TargetService, error) {
	cs, err := store.NewCachedStore[domain.UDPTarget](
		cmn.ResourceTarget, repo,
		func(t domain.UDPTarget) string { return t.ID },
		func(t *domain.UDPTarget, id string) { t.ID = id },
	)
	if err != nil {
		return nil, err
	}
	return &TargetService{store: cs}, nil
}

// GetTargets は全ターゲットのコピーを返す。
func (s *TargetService) GetTargets() []domain.UDPTarget {
	return s.store.GetAll()
}

// SaveTarget はターゲットを保存する。ID が空の場合は新規生成する。
// host/port が不正な場合は永続化せず ValidationError を返す。
func (s *TargetService) SaveTarget(target domain.UDPTarget) (domain.UDPTarget, error) {
	if err := target.Validate(); err != nil {
		return domain.UDPTarget{}, err
	}
	return s.store.Save(target)
}

// DeleteTarget は ID でターゲットを削除する。存在しない ID の場合は NotFoundError を返す。
func (s *TargetService) DeleteTarget(id string) error {
	return s.store.Delete(id)
}
