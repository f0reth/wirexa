package udpapp

import (
	"github.com/f0reth/Wirexa/internal/application/store"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.TargetUseCase = (*TargetService)(nil)

// TargetService はターゲット管理ユースケースの実装。
type TargetService struct {
	store *store.CachedStore[domain.UdpTarget]
}

// NewTargetService は TargetService を生成する。
func NewTargetService(repo domain.TargetRepository) (*TargetService, error) {
	cs, err := store.NewCachedStore[domain.UdpTarget](
		"target", repo,
		func(t domain.UdpTarget) string { return t.ID },
		func(t *domain.UdpTarget, id string) { t.ID = id },
	)
	if err != nil {
		return nil, err
	}
	return &TargetService{store: cs}, nil
}

// GetTargets は全ターゲットのコピーを返す。
func (s *TargetService) GetTargets() []domain.UdpTarget {
	return s.store.GetAll()
}

// SaveTarget はターゲットを保存する。ID が空の場合は新規生成する。
func (s *TargetService) SaveTarget(target domain.UdpTarget) (domain.UdpTarget, error) {
	return s.store.Save(target)
}

// DeleteTarget は ID でターゲットを削除する。存在しない ID の場合は NotFoundError を返す。
func (s *TargetService) DeleteTarget(id string) error {
	return s.store.Delete(id)
}
