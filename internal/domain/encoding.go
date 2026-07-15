package domain

import (
	"encoding/base64"
	"unicode/utf8"
)

// EncodeMaybeBase64 は b が有効な UTF-8 ならそのまま文字列化し、
// そうでなければ base64 化する。2 つ目の戻り値は base64 化したかどうか。
// 非 UTF-8 のバイナリを string 変換で壊さずフロントへ渡すために使う。
func EncodeMaybeBase64(b []byte) (string, bool) {
	if utf8.Valid(b) {
		return string(b), false
	}
	return base64.StdEncoding.EncodeToString(b), true
}
