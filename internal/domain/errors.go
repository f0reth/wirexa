// Package domain はドメイン層の共通型を提供する。
package domain

import (
	"errors"
	"fmt"
)

// ErrCorruptData は保存データを読めたが JSON として解釈できないことを示す。
// 読み込み自体の失敗 (権限・ロック等) とは区別し、退避の対象はこのエラーのファイルだけとする。
// 各リポジトリは parse 失敗をこれで wrap して返し、呼び出し側は errors.Is で判定する。
var ErrCorruptData = errors.New("stored data is corrupt")

// NotFoundError はリソースが見つからない場合のエラー。
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// ValidationError は入力値検証エラー。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
	}
	return e.Message
}
