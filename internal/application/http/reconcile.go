package httpapp

import (
	"sort"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// reconcileSidebarLayout は保存済みレイアウトを実データと突合して正規化する。
//  1. コレクション / __root__ 直下アイテムとして存在しないエントリを除去する
//  2. 重複エントリを除去する (最初の出現を残す)
//  3. レイアウトに無いコレクション (名前順) と __root__ 直下アイテム (ツリー順) を末尾へ追加する
//
// 既知のエントリの並び順は保存されたものを維持する。
// 空レイアウトからの初期生成も同じ規則の特殊ケースとして処理する。
//
// cols には __root__ を含めない。rootItems は __root__ の直下の子だけを渡す
// (フォルダ内にネストしたアイテムはサイドバー直下に並ばないため)。
func reconcileSidebarLayout(
	layout []domain.SidebarEntry,
	cols []*domain.Collection,
	rootItems []*domain.TreeItem,
) (next []domain.SidebarEntry, changed bool) {
	valid := make(map[domain.SidebarEntry]struct{}, len(cols)+len(rootItems))
	for _, c := range cols {
		valid[domain.SidebarEntry{Kind: sidebarKindCollection, ID: c.ID}] = struct{}{}
	}
	for _, item := range rootItems {
		valid[domain.SidebarEntry{Kind: sidebarKindItem, ID: item.ID}] = struct{}{}
	}

	next = make([]domain.SidebarEntry, 0, len(layout)+len(valid))
	kept := make(map[domain.SidebarEntry]struct{}, len(valid))
	for _, e := range layout {
		_, ok := valid[e]
		if _, dup := kept[e]; !ok || dup {
			changed = true
			continue
		}
		kept[e] = struct{}{}
		next = append(next, e)
	}

	// レイアウトに無いものを末尾へ追加する。コレクションは名前順、アイテムはツリー順。
	sorted := append([]*domain.Collection(nil), cols...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	appendMissing := func(e domain.SidebarEntry) {
		if _, ok := kept[e]; ok {
			return
		}
		kept[e] = struct{}{}
		next = append(next, e)
		changed = true
	}
	for _, c := range sorted {
		appendMissing(domain.SidebarEntry{Kind: sidebarKindCollection, ID: c.ID})
	}
	for _, item := range rootItems {
		appendMissing(domain.SidebarEntry{Kind: sidebarKindItem, ID: item.ID})
	}
	return next, changed
}
