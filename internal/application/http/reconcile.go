package httpapp

import (
	"sort"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// dropDuplicateItems は複数のコレクションに現れるアイテム ID を1つだけ残して除去する。
// 「追加してから削除」の途中でプロセスが消えた場合の残骸を回収する。
// 残す側は __root__ を先頭、以降はコレクション ID の昇順で走査した最初の出現とする。
// クラッシュ後にどちらが移動先だったかは判別できないため、この規則は
// 「決定的であること」だけを保証し、移動の意図までは復元しない。
//
// 入力は変更せず、内容が変わったコレクションだけをコピーとして返す。
func dropDuplicateItems(cols map[string]*domain.Collection) map[string]*domain.Collection {
	seen := make(map[string]struct{})
	changed := make(map[string]*domain.Collection)
	for _, id := range duplicateScanOrder(cols) {
		dup := collectDuplicateIDs(cols[id].Items, seen)
		if len(dup) == 0 {
			continue
		}
		next := cols[id].Clone()
		for _, itemID := range dup {
			next.RemoveNode(itemID)
		}
		changed[id] = next
	}
	return changed
}

// duplicateScanOrder は重複回収の走査順を返す。__root__ を先頭、以降は ID の昇順。
func duplicateScanOrder(cols map[string]*domain.Collection) []string {
	ids := make([]string, 0, len(cols))
	for id := range cols {
		if id != domain.RootCollectionID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if _, ok := cols[domain.RootCollectionID]; ok {
		ids = append([]string{domain.RootCollectionID}, ids...)
	}
	return ids
}

// collectDuplicateIDs はツリーを走査し、seen に未登録の ID を登録しつつ、
// 既に登録済みだったノードの ID を返す。
// 重複ノードの子孫は親ごと消えるため走査しない。
func collectDuplicateIDs(items []*domain.TreeItem, seen map[string]struct{}) []string {
	var dup []string
	for _, item := range items {
		if _, ok := seen[item.ID]; ok {
			dup = append(dup, item.ID)
			continue
		}
		seen[item.ID] = struct{}{}
		dup = append(dup, collectDuplicateIDs(item.Children, seen)...)
	}
	return dup
}

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
