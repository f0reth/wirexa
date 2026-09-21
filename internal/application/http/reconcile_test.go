package httpapp

import (
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// reconcileCols は名前付きコレクションのスライスを組み立てる。
func reconcileCols(pairs ...string) []*domain.Collection {
	cols := make([]*domain.Collection, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		cols = append(cols, &domain.Collection{ID: pairs[i], Name: pairs[i+1], Items: []*domain.TreeItem{}})
	}
	return cols
}

// rootItems は __root__ 直下のアイテムを組み立てる。
func rootItems(ids ...string) []*domain.TreeItem {
	items := make([]*domain.TreeItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, &domain.TreeItem{Type: domain.ItemTypeRequest, ID: id, Name: id})
	}
	return items
}

func TestReconcileSidebarLayout(t *testing.T) {
	tests := []struct {
		name        string
		layout      []string
		cols        []*domain.Collection
		items       []*domain.TreeItem
		want        []string
		wantChanged bool
	}{
		{
			name:        "変更が無ければ changed は偽",
			layout:      []string{"c:c1", "i:r1"},
			cols:        reconcileCols("c1", "A"),
			items:       rootItems("r1"),
			want:        []string{"c:c1", "i:r1"},
			wantChanged: false,
		},
		{
			name:        "存在しないコレクション ID のエントリを除去する",
			layout:      []string{"c:gone", "c:c1"},
			cols:        reconcileCols("c1", "A"),
			want:        []string{"c:c1"},
			wantChanged: true,
		},
		{
			name:        "存在しないアイテム ID のエントリを除去する",
			layout:      []string{"i:gone", "i:r1"},
			items:       rootItems("r1"),
			want:        []string{"i:r1"},
			wantChanged: true,
		},
		{
			name:        "kind が食い違うエントリを除去する",
			layout:      []string{"i:c1"},
			cols:        reconcileCols("c1", "A"),
			want:        []string{"c:c1"},
			wantChanged: true,
		},
		{
			name:        "重複エントリを除去し最初の出現の位置を保つ",
			layout:      []string{"c:c1", "c:c2", "c:c1"},
			cols:        reconcileCols("c1", "B", "c2", "A"),
			want:        []string{"c:c1", "c:c2"},
			wantChanged: true,
		},
		{
			name:        "レイアウトに無いコレクションを名前順で末尾へ追加する",
			layout:      []string{"c:c3"},
			cols:        reconcileCols("c3", "C", "c1", "Zebra", "c2", "Apple"),
			want:        []string{"c:c3", "c:c2", "c:c1"},
			wantChanged: true,
		},
		{
			name:        "レイアウトに無い __root__ 直下アイテムをツリー順で末尾へ追加する",
			layout:      []string{"i:r2"},
			items:       rootItems("r1", "r2", "r3"),
			want:        []string{"i:r2", "i:r1", "i:r3"},
			wantChanged: true,
		},
		{
			name:        "空レイアウトからはコレクション(名前順)→アイテム(ツリー順)の並びを生成する",
			layout:      nil,
			cols:        reconcileCols("c1", "B", "c2", "A"),
			items:       rootItems("r1", "r2"),
			want:        []string{"c:c2", "c:c1", "i:r1", "i:r2"},
			wantChanged: true,
		},
		{
			name:        "既知エントリの並び順は保存されたものを維持する",
			layout:      []string{"i:r1", "c:c1"},
			cols:        reconcileCols("c1", "A"),
			items:       rootItems("r1"),
			want:        []string{"i:r1", "c:c1"},
			wantChanged: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := reconcileSidebarLayout(entries(tc.layout...), tc.cols, tc.items)
			assertLayout(t, got, tc.want...)
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
		})
	}
}

// __root__ のフォルダ内にネストしたアイテムはサイドバー直下に並ばないため、
// そのエントリは stale として除去される。
func TestReconcileSidebarLayout_NestedRootItemIsNotASidebarEntry(t *testing.T) {
	nested := &domain.TreeItem{Type: domain.ItemTypeRequest, ID: "nested", Name: "Nested"}
	folder := &domain.TreeItem{Type: domain.ItemTypeFolder, ID: "f1", Name: "F1", Children: []*domain.TreeItem{nested}}

	got, changed := reconcileSidebarLayout(entries("i:f1", "i:nested"), nil, []*domain.TreeItem{folder})
	assertLayout(t, got, "i:f1")
	if !changed {
		t.Error("changed = false, want true")
	}
}

func TestReconcileSidebarLayout_DoesNotMutateInput(t *testing.T) {
	layout := entries("c:gone", "c:c1")
	before := entryIDs(layout)

	reconcileSidebarLayout(layout, reconcileCols("c1", "A"), nil)

	after := entryIDs(layout)
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("input layout was mutated: %v → %v", before, after)
		}
	}
}
