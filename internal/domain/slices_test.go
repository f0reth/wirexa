package domain

import (
	"slices"
	"testing"
)

func TestInsertAt(t *testing.T) {
	tests := []struct {
		name string
		src  []string
		want []string
		pos  int
	}{
		{name: "先頭", src: []string{"a", "b", "c"}, pos: 0, want: []string{"x", "a", "b", "c"}},
		{name: "中間", src: []string{"a", "b", "c"}, pos: 1, want: []string{"a", "x", "b", "c"}},
		{name: "末尾 (pos == len)", src: []string{"a", "b", "c"}, pos: 3, want: []string{"a", "b", "c", "x"}},
		{name: "len 超過は末尾", src: []string{"a", "b", "c"}, pos: 99, want: []string{"a", "b", "c", "x"}},
		{name: "負は末尾", src: []string{"a", "b", "c"}, pos: -1, want: []string{"a", "b", "c", "x"}},
		{name: "空スライス", src: []string{}, pos: 0, want: []string{"x"}},
		{name: "空スライスに負", src: []string{}, pos: -5, want: []string{"x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InsertAt(tt.src, "x", tt.pos)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("InsertAt(%v, x, %d) = %v, want %v", tt.src, tt.pos, got, tt.want)
			}
		})
	}
}

// 呼び出し側は返り値を使う契約なので、元のスライス変数の長さが変わらないことを確認する。
func TestInsertAt_DoesNotGrowSource(t *testing.T) {
	src := []string{"a", "b", "c"}
	InsertAt(src, "x", 1)
	if len(src) != 3 {
		t.Fatalf("len(src) = %d, want 3", len(src))
	}
}

func TestInsertAt_Nil(t *testing.T) {
	var src []int
	got := InsertAt(src, 7, 3)
	if !slices.Equal(got, []int{7}) {
		t.Fatalf("InsertAt(nil, 7, 3) = %v, want [7]", got)
	}
}
