package openapidomain

// FileUseCase は OpenAPI ファイル操作のユースケース入力ポート。
//
// ReadFile / WriteFile が受理するのは許可リストに登録済みのパスだけで、
// 許可リストへの登録経路は OpenSelected / SaveSelected と起動時の recents seed に限る。
type FileUseCase interface {
	// OpenSelected はダイアログで選ばれたパスを許可リストと recents へ登録し、正規化したパスを返す。
	// 呼んでよいのは adapter のダイアログ処理だけで、RPC 引数を渡してはならない。
	OpenSelected(path string) string
	// SaveSelected はダイアログで選ばれたパスを許可リストへ登録し、content を書き込んでから recents へ登録する。
	// 呼んでよいのは adapter のダイアログ処理だけで、RPC 引数を渡してはならない。
	SaveSelected(path, content string) (string, error)
	// ReadFile は許可済みパスの内容を返す。未許可なら ErrFileAccessDenied。
	ReadFile(path string) (string, error)
	// WriteFile は許可済みパスへ content を書き込む。未許可なら ErrFileAccessDenied。
	WriteFile(path, content string) error
	// GetRecents は recents を Order 昇順で返す。
	GetRecents() []OpenAPIRecent
	// RemoveRecent は recents と許可リストからパスを削除する。
	RemoveRecent(path string) error
	// MoveRecent は recents 内でパスを index の位置へ並び替える。
	MoveRecent(path string, index int) error
}

// FileAccess は許可確認済みのパスに対するネイティブファイル I/O の出力ポート。
type FileAccess interface {
	ReadFile(path string) ([]byte, error)
	// WriteFile は data を path へ原子的に書き込む。
	WriteFile(path string, data []byte) error
}

// RecentRepository は recents 一覧の永続化抽象。一覧全体を 1 単位で読み書きする。
type RecentRepository interface {
	// Load は一覧を読む。未作成なら空スライスと nil を返す。
	// JSON として壊れている場合は ErrRecentsCorrupt を wrap して返し、それ以外の失敗はそのまま返す。
	Load() ([]OpenAPIRecent, error)
	Save(items []OpenAPIRecent) error
	// Quarantine は壊れたファイルを上書きされない場所へ退避し、退避先のパスを返す。
	Quarantine() (string, error)
}
