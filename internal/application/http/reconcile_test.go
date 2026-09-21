package httpapp

import (
	"errors"
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

// --- dropDuplicateItems ---

// dupCollections は ID をキーにしたコレクションのマップを組み立てる。
func dupCollections(cols ...*domain.Collection) map[string]*domain.Collection {
	m := make(map[string]*domain.Collection, len(cols))
	for _, c := range cols {
		m[c.ID] = c
	}
	return m
}

// colWith は ID とツリーからコレクションを組み立てる。
func colWith(id string, items ...*domain.TreeItem) *domain.Collection {
	return &domain.Collection{ID: id, Name: id, Items: items}
}

// req はリクエストアイテムを組み立てる。
func req(id string) *domain.TreeItem {
	return &domain.TreeItem{Type: domain.ItemTypeRequest, ID: id, Name: id, Children: []*domain.TreeItem{}}
}

// folder はフォルダアイテムを組み立てる。
func folder(id string, children ...*domain.TreeItem) *domain.TreeItem {
	return &domain.TreeItem{Type: domain.ItemTypeFolder, ID: id, Name: id, Children: children}
}

// itemIDs はツリーを深さ優先で走査して ID を並べる。
func itemIDs(items []*domain.TreeItem) []string {
	var ids []string
	for _, item := range items {
		ids = append(ids, item.ID)
		ids = append(ids, itemIDs(item.Children)...)
	}
	return ids
}

func TestDropDuplicateItems_NoDuplicates(t *testing.T) {
	cols := dupCollections(colWith("c1", req("r1")), colWith("c2", req("r2")))
	if changed := dropDuplicateItems(cols); len(changed) != 0 {
		t.Errorf("changed = %v, want empty", changed)
	}
}

func TestDropDuplicateItems_KeepsRootOccurrence(t *testing.T) {
	// __root__ を先頭に走査するので、残るのは __root__ 側。
	cols := dupCollections(
		colWith(domain.RootCollectionID, req("r1")),
		colWith("c1", req("r1"), req("r2")),
	)
	changed := dropDuplicateItems(cols)
	if len(changed) != 1 {
		t.Fatalf("changed = %v, want only c1", changed)
	}
	got := itemIDs(changed["c1"].Items)
	if len(got) != 1 || got[0] != "r2" {
		t.Errorf("c1 items = %v, want [r2]", got)
	}
	// 入力は変更されない。
	if len(cols["c1"].Items) != 2 {
		t.Error("dropDuplicateItems mutated its input")
	}
}

func TestDropDuplicateItems_KeepsLowestCollectionID(t *testing.T) {
	// __root__ が絡まない場合はコレクション ID の昇順で最初の出現を残す。
	cols := dupCollections(colWith("cb", req("r1")), colWith("ca", req("r1")))
	changed := dropDuplicateItems(cols)
	if len(changed) != 1 {
		t.Fatalf("changed = %v, want only cb", changed)
	}
	if _, ok := changed["cb"]; !ok {
		t.Errorf("changed = %v, want the higher ID (cb) to lose the item", changed)
	}
	if len(changed["cb"].Items) != 0 {
		t.Errorf("cb items = %v, want empty", itemIDs(changed["cb"].Items))
	}
}

func TestDropDuplicateItems_RemovesDuplicatedSubtree(t *testing.T) {
	// フォルダごと重複した場合は子孫ごと消え、子孫は別の重複として扱われない。
	cols := dupCollections(
		colWith("ca", folder("f1", req("r1"))),
		colWith("cb", folder("f1", req("r1")), req("r2")),
	)
	changed := dropDuplicateItems(cols)
	got := itemIDs(changed["cb"].Items)
	if len(got) != 1 || got[0] != "r2" {
		t.Errorf("cb items = %v, want [r2]", got)
	}
}

func TestDropDuplicateItems_RemovesNestedDuplicate(t *testing.T) {
	// 移動先のフォルダ内に入ったアイテムが移動元にも残っているケース。
	cols := dupCollections(
		colWith("ca", folder("f1", req("r1"))),
		colWith("cb", req("r1")),
	)
	changed := dropDuplicateItems(cols)
	if len(changed) != 1 {
		t.Fatalf("changed = %v, want only cb", changed)
	}
	if got := itemIDs(changed["cb"].Items); len(got) != 0 {
		t.Errorf("cb items = %v, want empty", got)
	}
}

// --- 起動時の回収 ---

func TestNewCollectionService_RecoversDuplicateItemsOnStartup(t *testing.T) {
	repo := newFakeRepo(
		colWith("ca", req("r1")),
		colWith("cb", req("r1")),
	)
	svc := newSvcWithRepo(t, repo, &inMemoryLayoutRepo{})

	// キャッシュ・ディスクの双方で1件だけになる。
	if got := itemIDs(cachedCollection(t, svc, "ca").Items); len(got) != 1 {
		t.Errorf("ca items = %v, want [r1]", got)
	}
	if got := itemIDs(cachedCollection(t, svc, "cb").Items); len(got) != 0 {
		t.Errorf("cb items = %v, want empty", got)
	}
	if got := itemIDs(repo.snapshot("cb").Items); len(got) != 0 {
		t.Errorf("persisted cb items = %v, want empty", got)
	}
}

func TestNewCollectionService_DuplicateRecoverySaveError_StillStarts(t *testing.T) {
	repo := newFakeRepo(
		colWith("ca", req("r1")),
		colWith("cb", req("r1")),
	)
	repo.failSaveAlways("cb")
	logger := &recordingLogger{}

	svc, err := NewCollectionService(repo, &inMemoryLayoutRepo{}, logger)
	if err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	if logger.errors == 0 {
		t.Error("the failed recovery save was not logged")
	}
	// メモリ上は回収済みなので動作は整合する。
	if got := itemIDs(cachedCollection(t, svc, "cb").Items); len(got) != 0 {
		t.Errorf("cb items = %v, want empty", got)
	}
}

func TestNewCollectionService_PersistsLayoutReconciliation(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{layout: entries("c:gone", "c:c1")}
	repo := newFakeRepo(colWith("c1"), colWith("c2"))

	if _, err := NewCollectionService(repo, layoutRepo, nil); err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	// stale な gone が消え、レイアウトに無かった c2 が末尾に足されて保存される。
	assertLayout(t, layoutRepo.snapshot(), "c:c1", "c:c2")
}

func TestNewCollectionService_UnchangedLayoutIsNotRewritten(t *testing.T) {
	layoutRepo := &inMemoryLayoutRepo{layout: entries("c:c1")}
	layoutRepo.saveErr = errors.New("layout save error")

	// 突合で変化しないなら保存しないので、保存が失敗する状態でも起動できる。
	if _, err := NewCollectionService(newFakeRepo(colWith("c1")), layoutRepo, nil); err != nil {
		t.Fatalf("NewCollectionService: %v", err)
	}
	layoutRepo.saveErr = nil
	assertLayout(t, layoutRepo.snapshot(), "c:c1")
}
