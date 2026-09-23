package openapidomain

import cmn "github.com/f0reth/Wirexa/internal/domain"

// FileAccess は許可確認済みのパスに対するネイティブファイル I/O の出力ポート。
type FileAccess interface {
	ReadFile(path string) ([]byte, error)
	// WriteFile は data を path へ原子的に書き込む。
	WriteFile(path string, data []byte) error
}

// RecentRepository は recents 一覧の永続化抽象。一覧全体を 1 単位で読み書きする。
type RecentRepository interface {
	// Load は一覧を読む。未作成なら空スライスと nil を返す。
	// JSON として壊れている場合は cmn.ErrCorruptData を wrap して返し、それ以外の失敗はそのまま返す。
	Load() ([]OpenAPIRecent, error)
	Save(items []OpenAPIRecent) error
	cmn.Quarantiner
}
