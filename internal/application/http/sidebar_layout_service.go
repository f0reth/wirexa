package httpapp

import (
	"fmt"
	"sync"

	"github.com/f0reth/Wirexa/internal/application/store"
	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// SidebarLayoutService はサイドバーレイアウト（コレクション/ルートアイテムの並び順）の
// 永続化と操作を担う。CollectionService から分離することで、コレクションキャッシュ用の
// ロックとレイアウト用のロックを別構造体に分け、1構造体に二重ミューテックスを抱える
// 状態（ロック順序依存によるデッドロック危険）を構造的に排除する。
//
// レイアウトはコレクションから再生成できるデータ（store.PolicyRegenerable）として扱う。
// ファイルが壊れていれば退避してから空レイアウトとして扱い、次の保存で再生成した内容に置き換える。
// 退避に失敗しても上書きしてよい。破損以外の読み込み失敗はそのまま返し、ファイルには触れない。
type SidebarLayoutService struct {
	repo   domain.SidebarLayoutRepository
	logger cmn.Logger
	mu     sync.Mutex
}

// NewSidebarLayoutService は SidebarLayoutService を生成する。
// logger は破損ファイルの退避記録に使う。nil の場合は記録しない。
func NewSidebarLayoutService(repo domain.SidebarLayoutRepository, logger cmn.Logger) *SidebarLayoutService {
	return &SidebarLayoutService{repo: repo, logger: logger}
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

// loadLocked はレイアウトを読み込む。壊れていれば退避してから空レイアウトを返す
// （起動後に外部から壊された場合も、次の操作で同じように復旧する）。
// 破損以外の読み込み失敗はエラーとして返す。呼び出し側で l.mu をロック済み。
func (l *SidebarLayoutService) loadLocked() ([]domain.SidebarEntry, error) {
	layout, _, err := store.LoadSingleFile(l.repo, store.PolicyRegenerable, l.logger, "sidebar_layout")
	if err != nil {
		return nil, fmt.Errorf("failed to load sidebar layout: %w", err)
	}
	if layout == nil {
		layout = []domain.SidebarEntry{}
	}
	return layout, nil
}

// Update はレイアウトを読み込み、mutate の結果を保存する。
// mutate がエラーを返した場合はファイルを書き換えない。
// 読み込みに失敗した場合（破損以外）は既存の並びが分からないため、保存せずにエラーを返す。
func (l *SidebarLayoutService) Update(mutate layoutMutator) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	layout, err := l.loadLocked()
	if err != nil {
		return err
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

// Load は保存済みレイアウトを読み込む。突合は呼び出し側が行う。
// 壊れていた場合は退避したうえで空レイアウトを返す。
func (l *SidebarLayoutService) Load() ([]domain.SidebarEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loadLocked()
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
