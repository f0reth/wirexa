package httpapp

import (
	"fmt"
	"sync"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// SidebarLayoutService はサイドバーレイアウト（コレクション/ルートアイテムの並び順）の
// 永続化と操作を担う。CollectionService から分離することで、コレクションキャッシュ用の
// ロックとレイアウト用のロックを別構造体に分け、1構造体に二重ミューテックスを抱える
// 状態（ロック順序依存によるデッドロック危険）を構造的に排除する。
type SidebarLayoutService struct {
	repo domain.SidebarLayoutRepository
	mu   sync.Mutex
}

// NewSidebarLayoutService は SidebarLayoutService を生成する。
func NewSidebarLayoutService(repo domain.SidebarLayoutRepository) *SidebarLayoutService {
	return &SidebarLayoutService{repo: repo}
}

// layoutMutator は読み込み済みレイアウトから保存すべきレイアウトを組み立てる。
// エントリが見つからないなどの業務エラーはここで返し、ファイルは書き換えない。
// 純関数としてロックの外でも単体で検証できるようにし、ロード〜保存の I/O は
// Update 側にだけ置く。
type layoutMutator func([]domain.SidebarEntry) ([]domain.SidebarEntry, error)

// layoutAppend はレイアウトの末尾にエントリを追加する。
func layoutAppend(entry domain.SidebarEntry) layoutMutator {
	return func(layout []domain.SidebarEntry) ([]domain.SidebarEntry, error) {
		return append(layout, entry), nil
	}
}

// layoutRemove は指定 kind+ID のエントリを取り除く。
// 対象が無い場合もエラーにしない（既に整合しているため）。
func layoutRemove(kind, id string) layoutMutator {
	return func(layout []domain.SidebarEntry) ([]domain.SidebarEntry, error) {
		n := 0
		for _, e := range layout {
			if e.Kind == kind && e.ID == id {
				continue
			}
			layout[n] = e
			n++
		}
		return layout[:n], nil
	}
}

// layoutMove は既存エントリを position へ移動する。見つからない場合は NotFoundError を返す。
func layoutMove(kind, id string, position int) layoutMutator {
	return func(layout []domain.SidebarEntry) ([]domain.SidebarEntry, error) {
		srcIdx := -1
		for i, e := range layout {
			if e.Kind == kind && e.ID == id {
				srcIdx = i
				break
			}
		}
		if srcIdx == -1 {
			return nil, &cmn.NotFoundError{Resource: cmn.ResourceSidebarEntry, ID: id}
		}
		entry := layout[srcIdx]
		layout = append(layout[:srcIdx], layout[srcIdx+1:]...)
		return cmn.InsertAt(layout, entry, position), nil
	}
}

// layoutInsertItem はアイテムエントリを指定位置に挿入する。
func layoutInsertItem(itemID string, position int) layoutMutator {
	return func(layout []domain.SidebarEntry) ([]domain.SidebarEntry, error) {
		entry := domain.SidebarEntry{Kind: sidebarKindItem, ID: itemID}
		return cmn.InsertAt(layout, entry, position), nil
	}
}

// Update はレイアウトを読み込み、mutate の結果を保存する。
// mutate がエラーを返した場合はファイルを書き換えない。
func (l *SidebarLayoutService) Update(mutate layoutMutator) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	layout, err := l.repo.Load()
	if err != nil {
		return fmt.Errorf("failed to load sidebar layout: %w", err)
	}
	next, err := mutate(layout)
	if err != nil {
		return err
	}
	if err := l.repo.Save(next); err != nil {
		return fmt.Errorf("failed to save sidebar layout: %w", err)
	}
	return nil
}

// Load は保存済みレイアウトをそのまま読み込む。突合は呼び出し側が行う。
func (l *SidebarLayoutService) Load() ([]domain.SidebarEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	layout, err := l.repo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load sidebar layout: %w", err)
	}
	return layout, nil
}

// Save はレイアウトをそのまま保存する。
func (l *SidebarLayoutService) Save(layout []domain.SidebarEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.repo.Save(layout); err != nil {
		return fmt.Errorf("failed to save sidebar layout: %w", err)
	}
	return nil
}
