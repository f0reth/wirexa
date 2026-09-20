package domain

import "strings"

// maxIDLength は永続化ファイル名として使う ID の最大長。
const maxIDLength = 128

// windowsReservedNames は Windows が通常ファイルとして扱えない予約デバイス名。
// 拡張子が付いていてもデバイスとして解決されるため、先頭セグメントで判定する。
var windowsReservedNames = map[string]struct{}{
	"con": {}, "prn": {}, "aux": {}, "nul": {},
	"com1": {}, "com2": {}, "com3": {}, "com4": {}, "com5": {},
	"com6": {}, "com7": {}, "com8": {}, "com9": {},
	"lpt1": {}, "lpt2": {}, "lpt3": {}, "lpt4": {}, "lpt5": {},
	"lpt6": {}, "lpt7": {}, "lpt8": {}, "lpt9": {},
}

// ValidateID は永続化ファイル名として使う ID を検証する。
// 許可するのは ASCII の英数字と "." "_" "-" のみで、パス区切り・ドライブ指定・
// 制御文字・Unicode の別表現をまとめて排除する。
func ValidateID(id string) error {
	if id == "" {
		return &ValidationError{Field: "id", Message: "must not be empty"}
	}
	if len(id) > maxIDLength {
		return &ValidationError{Field: "id", Message: "must be at most 128 characters"}
	}
	for i := range len(id) {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-':
		default:
			return &ValidationError{Field: "id", Message: "must contain only ASCII letters, digits, '.', '_' and '-'"}
		}
	}
	// "." ".." はディレクトリ自身・親を指す。先頭の "." は隠しファイル、
	// 末尾の "." は Windows が除去するため、いずれも 1 対 1 のファイル名にならない。
	if strings.HasPrefix(id, ".") || strings.HasSuffix(id, ".") {
		return &ValidationError{Field: "id", Message: "must not start or end with '.'"}
	}
	// 予約デバイス名は拡張子が付いても解決されるので、最初の "." より前で判定する。
	head, _, _ := strings.Cut(id, ".")
	if _, reserved := windowsReservedNames[strings.ToLower(head)]; reserved {
		return &ValidationError{Field: "id", Message: "must not be a reserved device name"}
	}
	return nil
}
