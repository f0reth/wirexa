package httpapp

import (
	"fmt"
	"slices"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// unitOfWork は複数のコレクションファイルへの書き込みを1単位にまとめる。
// 各操作は成功したときだけ「直前の内容へ戻す」undo をスタックへ積み、
// 途中で失敗すると以降の操作を飛ばして最初のエラーを保持する。
// 呼び出し側は Err() を見て、失敗していれば Rollback() でディスクを戻し、
// 成功していればキャッシュを差し替える。
//
// 巻き戻せるのはプロセスが生きている間の保存エラーだけで、プロセスが消えた場合は
// undo が走らない。そのためクラッシュ耐性は書き込み順序 (追加してから削除) と
// 起動時の回収で担保し、この型には負わせない。
type unitOfWork struct {
	repo   domain.CollectionRepository
	logger cmn.Logger
	undos  []func() error
	err    error
}

// begin は CollectionService のリポジトリとロガーを束ねた unitOfWork を開始する。
func (s *CollectionService) begin() *unitOfWork {
	return &unitOfWork{repo: s.repo, logger: s.logger}
}

// SaveCollection は next を保存する。prev は保存前の内容で、nil の場合 (新規作成) の
// 巻き戻しは削除になる。prev は巻き戻しでそのまま書き戻すため、呼び出し側は以後
// prev を変更してはならない (キャッシュ上の現行オブジェクトをそのまま渡す)。
func (u *unitOfWork) SaveCollection(prev, next *domain.Collection) {
	if u.err != nil {
		return
	}
	if err := u.repo.Save(next); err != nil {
		u.err = fmt.Errorf("failed to save collection: %w", err)
		return
	}
	if prev == nil {
		id := next.ID
		u.undos = append(u.undos, func() error { return u.repo.Delete(id) })
		return
	}
	u.undos = append(u.undos, func() error { return u.repo.Save(prev) })
}

// DeleteCollection は prev.ID のファイルを削除する。巻き戻しは prev の再保存。
func (u *unitOfWork) DeleteCollection(prev *domain.Collection) {
	if u.err != nil {
		return
	}
	if err := u.repo.Delete(prev.ID); err != nil {
		u.err = fmt.Errorf("failed to delete collection: %w", err)
		return
	}
	u.undos = append(u.undos, func() error { return u.repo.Save(prev) })
}

// Err は最初に発生したエラーを返す。
func (u *unitOfWork) Err() error { return u.err }

// Rollback は積まれた undo を逆順に実行する。
// undo 自体の失敗はこれ以上ディスクを戻す手段が無いためログに記録して続行し、
// 呼び出し側には元のエラーを返す。残った不整合は起動時の回収で拾う。
func (u *unitOfWork) Rollback() {
	for _, undo := range slices.Backward(u.undos) {
		if err := undo(); err != nil && u.logger != nil {
			u.logger.Error("failed to roll back collection write", "source", "http", "error", err)
		}
	}
	u.undos = nil
}
