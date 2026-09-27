package httpapp

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// entries はテスト用のレイアウトを短く書くためのヘルパー。
// "c:id" はコレクション、"i:id" はアイテムを表す。
func entries(specs ...string) []domain.SidebarEntry {
	out := make([]domain.SidebarEntry, 0, len(specs))
	for _, spec := range specs {
		kind := sidebarKindCollection
		if spec[0] == 'i' {
			kind = sidebarKindItem
		}
		out = append(out, domain.SidebarEntry{Kind: kind, ID: spec[2:]})
	}
	return out
}

// entryIDs は比較しやすいようにエントリを "c:id" / "i:id" の形へ戻す。
func entryIDs(layout []domain.SidebarEntry) []string {
	out := make([]string, 0, len(layout))
	for _, e := range layout {
		prefix := "c:"
		if e.Kind == sidebarKindItem {
			prefix = "i:"
		}
		out = append(out, prefix+e.ID)
	}
	return out
}

func assertLayout(t *testing.T, got []domain.SidebarEntry, want ...string) {
	t.Helper()
	gotIDs := entryIDs(got)
	if len(gotIDs) != len(want) {
		t.Fatalf("layout = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("layout = %v, want %v", gotIDs, want)
		}
	}
}

func TestLayoutAppend(t *testing.T) {
	got, err := layoutAppend(domain.SidebarEntry{Kind: sidebarKindItem, ID: "r1"})(entries("c:c1"))
	if err != nil {
		t.Fatalf("layoutAppend: %v", err)
	}
	assertLayout(t, got, "c:c1", "i:r1")
}

func TestLayoutRemove(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		id    string
		start []string
		want  []string
	}{
		{"先頭を削除", sidebarKindCollection, "c1", []string{"c:c1", "c:c2"}, []string{"c:c2"}},
		{"中間を削除", sidebarKindCollection, "c2", []string{"c:c1", "c:c2", "c:c3"}, []string{"c:c1", "c:c3"}},
		{"kind が違えば消さない", sidebarKindItem, "c1", []string{"c:c1"}, []string{"c:c1"}},
		{"存在しなくてもエラーにしない", sidebarKindCollection, "none", []string{"c:c1"}, []string{"c:c1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := layoutRemove(tc.kind, tc.id)(entries(tc.start...))
			if err != nil {
				t.Fatalf("layoutRemove: %v", err)
			}
			assertLayout(t, got, tc.want...)
		})
	}
}

func TestLayoutMove(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		position int
		want     []string
	}{
		// position は移動前の一覧に対するインデックス（UI の挿入ゾーン）。
		{"末尾から先頭へ", "c3", 0, []string{"c:c3", "c:c1", "c:c2"}},
		{"末尾から中間へ", "c3", 1, []string{"c:c1", "c:c3", "c:c2"}},
		{"先頭を c2 と c3 の間へ", "c1", 2, []string{"c:c2", "c:c1", "c:c3"}},
		{"先頭から末尾の挿入ゾーンへ", "c1", 3, []string{"c:c2", "c:c3", "c:c1"}},
		{"自分の直前の挿入ゾーンは動かさない", "c2", 1, []string{"c:c1", "c:c2", "c:c3"}},
		{"自分の直後の挿入ゾーンも動かさない", "c2", 2, []string{"c:c1", "c:c2", "c:c3"}},
		{"負は末尾", "c1", -1, []string{"c:c2", "c:c3", "c:c1"}},
		{"範囲外は末尾", "c1", 10, []string{"c:c2", "c:c3", "c:c1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := layoutMove(sidebarKindCollection, tc.id, tc.position)(entries("c:c1", "c:c2", "c:c3"))
			if err != nil {
				t.Fatalf("layoutMove: %v", err)
			}
			assertLayout(t, got, tc.want...)
		})
	}
}

// TestLayoutMove_MovesDownUsesPreRemovalIndex はルートのアイテムも、コレクションと同じく
// 移動前のインデックスで後方へ移動することを確認する。
func TestLayoutMove_MovesDownUsesPreRemovalIndex(t *testing.T) {
	got, err := layoutMove(sidebarKindItem, "r1", 2)(entries("i:r1", "c:c1", "c:c2"))
	if err != nil {
		t.Fatalf("layoutMove: %v", err)
	}
	assertLayout(t, got, "c:c1", "i:r1", "c:c2")
}

func TestLayoutMove_NotFound(t *testing.T) {
	for _, start := range [][]string{{}, {"c:c1"}} {
		_, err := layoutMove(sidebarKindCollection, "nonexistent", 0)(entries(start...))
		if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
			t.Errorf("layoutMove(%v) error = %T (%v), want NotFoundError", start, err, err)
		}
	}
}

func TestLayoutInsertItem(t *testing.T) {
	got, err := layoutInsertItem("r1", 1)(entries("c:c1", "c:c2"))
	if err != nil {
		t.Fatalf("layoutInsertItem: %v", err)
	}
	assertLayout(t, got, "c:c1", "i:r1", "c:c2")
}

func TestSidebarLayoutService_Update_MutatorErrorLeavesFileUnchanged(t *testing.T) {
	repo := &inMemoryLayoutRepo{layout: entries("c:c1")}
	svc := NewSidebarLayoutService(repo, nil)

	err := svc.Update(layoutMove(sidebarKindCollection, "nonexistent", 0))
	if err == nil {
		t.Fatal("expected error from mutator, got nil")
	}
	assertLayout(t, repo.snapshot(), "c:c1")
}

func TestSidebarLayoutService_Update_SaveErrorLeavesFileUnchanged(t *testing.T) {
	repo := &inMemoryLayoutRepo{layout: entries("c:c1")}
	svc := NewSidebarLayoutService(repo, nil)
	repo.saveErr = errors.New("save error")

	if err := svc.Update(layoutAppend(domain.SidebarEntry{Kind: sidebarKindCollection, ID: "c2"})); err == nil {
		t.Fatal("expected error from repo.Save, got nil")
	}
	repo.saveErr = nil
	assertLayout(t, repo.snapshot(), "c:c1")
}

func TestSidebarLayoutService_Update_CorruptQuarantinesAndSavesFromEmpty(t *testing.T) {
	repo := &inMemoryLayoutRepo{loadErr: errCorruptLayout, layout: entries("c:stale")}
	svc := NewSidebarLayoutService(repo, nil)

	if err := svc.Update(layoutAppend(domain.SidebarEntry{Kind: sidebarKindCollection, ID: "c1"})); err != nil {
		t.Fatalf("Update on corrupt layout: %v", err)
	}
	if n := repo.quarantineCount(); n != 1 {
		t.Errorf("Quarantine calls = %d, want 1", n)
	}
	assertLayout(t, repo.snapshot(), "c:c1")
}

func TestSidebarLayoutService_Update_CorruptQuarantineFailsStillSaves(t *testing.T) {
	repo := &inMemoryLayoutRepo{
		loadErr:       errCorruptLayout,
		quarantineErr: errors.New("rename failed"),
		layout:        entries("c:stale"),
	}
	logger := &recordingLogger{}
	svc := NewSidebarLayoutService(repo, logger)

	if err := svc.Update(layoutAppend(domain.SidebarEntry{Kind: sidebarKindCollection, ID: "c1"})); err != nil {
		t.Fatalf("Update on corrupt layout: %v", err)
	}
	if logger.errors == 0 {
		t.Error("quarantine failure should be logged")
	}
	// 再生成可能データなので、退避できなくても上書きする。
	assertLayout(t, repo.snapshot(), "c:c1")
}

func TestSidebarLayoutService_Update_ReadErrorDoesNotSave(t *testing.T) {
	repo := &inMemoryLayoutRepo{loadErr: errors.New("permission denied"), layout: entries("c:c1")}
	svc := NewSidebarLayoutService(repo, nil)

	if err := svc.Update(layoutAppend(domain.SidebarEntry{Kind: sidebarKindCollection, ID: "c2"})); err == nil {
		t.Fatal("Update should fail when the layout cannot be read")
	}
	if n := repo.quarantineCount(); n != 0 {
		t.Errorf("Quarantine must not be called on read errors (calls = %d)", n)
	}
	assertLayout(t, repo.snapshot(), "c:c1")
}
