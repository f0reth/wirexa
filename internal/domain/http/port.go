// Package httpdomain は HTTP ドメイン層のポートインターフェースを定義する。
package httpdomain

import (
	"context"
	"io"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// HTTPTransport はHTTPリクエスト実行を担うポート。
// Application層はこのインターフェースを通じてネットワークI/Oを行う。
// executionID は送信ごとの実行 ID で、打ち切り時の一時ファイルはこの ID で追跡する。
// req.ID は保存済みリクエストの永続 ID であり、実行の識別には使わない。
type HTTPTransport interface {
	Do(ctx context.Context, executionID string, req HTTPRequest) (HTTPResponse, error)
}

// SelectedFileOpener は file token を解決し、ファイルダイアログで選択されたファイルを開くポート。
// 空・未登録の token では ErrFileAccessDenied を返し、ファイルを開かない。
// パスを受け取る経路は無く、返すエラーにパスや token を含めない。
type SelectedFileOpener interface {
	OpenSelectedFile(token string) (OpenedSelectedFile, error)
}

// OpenedSelectedFile は送信用に開いた選択ファイル。呼び出し側が File を必ず Close する。
// Name と ContentType は registry が選択時に決めた値で、frontend から渡された値ではない。
// Size は開いた時点のサイズで、送信ボディの長さはこの値で決める。
type OpenedSelectedFile struct {
	File        SelectedFileHandle
	Name        string
	ContentType string
	Size        int64
}

// SelectedFileHandle は開いた選択ファイルのハンドル。
// ReadAt は読み位置を共有しないため、複数のリーダーから並行して読める。
// ReadAt と CheckUnchanged が返すのは io.EOF か、パスを含まない domain エラー
// (ErrSelectedFileUnavailable / ErrSelectedFileChanged) だけ。
type SelectedFileHandle interface {
	io.ReaderAt
	io.Closer
	// CheckUnchanged は開いた時点からサイズと更新時刻が変わっていないかを確かめ、
	// 変わっていれば ErrSelectedFileChanged を返す。
	CheckUnchanged() error
}

// ResponseBodyStore は切り詰められたレスポンス全文の一時ファイルを execution ID で管理するポート。
// 一時ファイルのパスは外へ出さず、保存は lease を通してのみ行う。
type ResponseBodyStore interface {
	// AcquireSave は ready 状態の一時ファイルを saving にして lease を返す。
	AcquireSave(executionID string) (ResponseBodyLease, error)
	// Discard は ready 状態の一時ファイルを削除して追跡を終える。
	Discard(executionID string) error
}

// ResponseBodyLease は保存中の一時ファイルへの排他的な参照。
// SaveTo か Release のどちらかを必ず 1 回呼ぶ。
type ResponseBodyLease interface {
	ContentType() string
	// SaveTo は一時ファイルを dst へコピーし、成功したら一時ファイルと追跡を削除する。
	SaveTo(dst string) error
	// Release は保存を中止し、再保存できる ready 状態へ戻す。
	Release()
}

// CollectionRepository はコレクションの永続化抽象。
type CollectionRepository interface {
	// Load は読めたコレクションだけを返す。読めないファイルや壊れたファイルは読み飛ばす。
	Load() ([]Collection, error)
	Save(c *Collection) error
	Delete(id string) error
	// Exists は id のファイルが存在するかを返す。Load で読み飛ばされたファイルも存在として扱う。
	// 有無を確認できなかった場合はエラーを返す。
	Exists(id string) (bool, error)
}

// SidebarLayoutRepository はサイドバーレイアウトの永続化抽象。一覧全体を 1 単位で読み書きする。
type SidebarLayoutRepository interface {
	// Load はレイアウトを読む。未作成なら空スライスと nil を返す。
	// JSON として壊れている場合は cmn.ErrCorruptData を wrap して返し、それ以外の失敗はそのまま返す。
	Load() ([]SidebarEntry, error)
	Save(layout []SidebarEntry) error
	cmn.Quarantiner
}
