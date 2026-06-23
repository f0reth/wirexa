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

// Append はレイアウトの末尾にエントリを追加する。
func (l *SidebarLayoutService) Append(entry domain.SidebarEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	layout, err := l.repo.Load()
	if err != nil {
		return err
	}
	return l.repo.Save(append(layout, entry))
}

// Remove はレイアウトから指定 kind+ID のエントリを削除する。
func (l *SidebarLayoutService) Remove(kind, id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	layout, err := l.repo.Load()
	if err != nil {
		return err
	}
	n := 0
	for _, e := range layout {
		if e.Kind == kind && e.ID == id {
			continue
		}
		layout[n] = e
		n++
	}
	return l.repo.Save(layout[:n])
}

// Move はサイドバー上のエントリを指定位置に移動する。
func (l *SidebarLayoutService) Move(kind, id string, position int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	layout, err := l.repo.Load()
	if err != nil {
		return err
	}
	if len(layout) == 0 {
		return &cmn.NotFoundError{Resource: "sidebar entry", ID: id}
	}
	srcIdx := -1
	for i, e := range layout {
		if e.Kind == kind && e.ID == id {
			srcIdx = i
			break
		}
	}
	if srcIdx == -1 {
		return &cmn.NotFoundError{Resource: "sidebar entry", ID: id}
	}
	entry := layout[srcIdx]
	layout = append(layout[:srcIdx], layout[srcIdx+1:]...)
	return l.repo.Save(insertEntryAt(layout, entry, position))
}

// InsertItem はアイテムエントリをレイアウトの指定位置に挿入する。
func (l *SidebarLayoutService) InsertItem(itemID string, position int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	layout, err := l.repo.Load()
	if err != nil {
		return err
	}
	entry := domain.SidebarEntry{Kind: sidebarKindItem, ID: itemID}
	return l.repo.Save(insertEntryAt(layout, entry, position))
}

// GetOrInit はレイアウトを返す。空の場合は computeInitial で初期値を生成して保存する。
// computeInitial はレイアウトロックを保持していない状態で呼ばれる。呼び出し側は
// コレクションロックを取得して初期値を計算するため、ロックのネストを避けてこの順序を守る。
func (l *SidebarLayoutService) GetOrInit(computeInitial func() []domain.SidebarEntry) ([]domain.SidebarEntry, error) {
	l.mu.Lock()
	layout, err := l.repo.Load()
	if err != nil {
		l.mu.Unlock()
		return nil, fmt.Errorf("failed to load sidebar layout: %w", err)
	}
	if len(layout) > 0 {
		l.mu.Unlock()
		return layout, nil
	}
	l.mu.Unlock()

	initial := computeInitial()

	// 他ゴルーチンが先に初期化した場合はそちらを優先する（ダブルチェック）。
	l.mu.Lock()
	defer l.mu.Unlock()
	existing, err := l.repo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load sidebar layout: %w", err)
	}
	if len(existing) > 0 {
		return existing, nil
	}
	if err := l.repo.Save(initial); err != nil {
		return nil, fmt.Errorf("failed to save initial sidebar layout: %w", err)
	}
	return initial, nil
}

// insertEntryAt はエントリをスライスの指定位置に挿入する。範囲外なら末尾に追加する。
func insertEntryAt(layout []domain.SidebarEntry, entry domain.SidebarEntry, position int) []domain.SidebarEntry {
	if position < 0 || position >= len(layout) {
		return append(layout, entry)
	}
	layout = append(layout, domain.SidebarEntry{})
	copy(layout[position+1:], layout[position:])
	layout[position] = entry
	return layout
}
