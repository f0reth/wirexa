package domain

import "slices"

// InsertAt は s の pos の位置に v を挿入したスライスを返す。
// pos が負または len(s) を超える場合は末尾に追加する。
// UI からのドラッグ&ドロップ位置は範囲外になりうるため、
// panic する slices.Insert をそのまま使わずここでクランプする。
func InsertAt[T any](s []T, v T, pos int) []T {
	if pos < 0 || pos > len(s) {
		pos = len(s)
	}
	return slices.Insert(s, pos, v)
}
