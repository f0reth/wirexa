// Package httpdomain は HTTP ドメイン層のポートインターフェースを定義する。
package httpdomain

import "context"

// HTTPTransport はHTTPリクエスト実行を担うポート。
// Application層はこのインターフェースを通じてネットワークI/Oを行う。
type HTTPTransport interface {
	Do(ctx context.Context, req HTTPRequest) (HTTPResponse, error)
}

// SelectedFileReader は file token を解決し、ファイルダイアログで選択されたファイルを読むポート。
// 空・未登録の token では ErrFileAccessDenied を返し、ファイルを開かない。
type SelectedFileReader interface {
	ReadSelectedFile(token string) (SelectedFileContent, error)
}

// SelectedFileContent は送信用に読み込んだ選択ファイル。
// Name と ContentType は registry が選択時に決めた値で、frontend から渡された値ではない。
type SelectedFileContent struct {
	Name        string
	ContentType string
	Data        []byte
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
	Load() ([]Collection, error)
	Save(c *Collection) error
	Delete(id string) error
}

// SidebarLayoutRepository はサイドバーレイアウトの永続化抽象。
type SidebarLayoutRepository interface {
	Load() ([]SidebarEntry, error)
	Save(layout []SidebarEntry) error
}

// RequestUseCase は HTTP リクエスト送信のユースケース入力ポート。
type RequestUseCase interface {
	SendRequest(req HTTPRequest) (HTTPResponse, error)
	CancelRequest(id string)
}

// CollectionUseCase はコレクション自体のCRUDユースケース入力ポート。
type CollectionUseCase interface {
	GetCollections() []Collection
	GetRootItems() []*TreeItem
	CreateCollection(name string) (Collection, error)
	DeleteCollection(id string) error
	RenameCollection(id, name string) error
	GetSidebarLayout() ([]SidebarEntry, error)
	MoveSidebarEntry(kind, id string, position int) error
	MoveItemToSidebar(sourceCollectionID, itemID string, sidebarPosition int) error
}

// CollectionItemUseCase はコレクション内ツリーアイテム管理のユースケース入力ポート。
type CollectionItemUseCase interface {
	AddFolder(collectionID, parentID, name string) (*TreeItem, error)
	AddRequest(collectionID, parentID string, req HTTPRequest) (*TreeItem, error)
	UpdateRequest(collectionID string, req HTTPRequest) error
	RenameItem(collectionID, itemID, name string) error
	DeleteItem(collectionID, itemID string) error
	// MoveItem はアイテムをコレクション内外・別の親・位置へ移動する。
	// targetParentID が空の場合はターゲットコレクションルートへ移動する。
	// position は挿入先インデックス（削除後の配列に対する）。-1 の場合は末尾に追加。
	MoveItem(sourceCollectionID, itemID, targetCollectionID, targetParentID string, position int) error
}
