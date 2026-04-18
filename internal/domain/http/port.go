// Package httpdomain は HTTP ドメイン層のポートインターフェースを定義する。
package httpdomain

import "context"

// HttpTransport はHTTPリクエスト実行を担うポート。
// Application層はこのインターフェースを通じてネットワークI/Oを行う。
type HttpTransport interface {
	Do(ctx context.Context, req HttpRequest) (HttpResponse, error)
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
	SendRequest(req HttpRequest) (HttpResponse, error)
	CancelRequest(id string)
}

// CollectionUseCase はコレクション自体のCRUDユースケース入力ポート。
type CollectionUseCase interface {
	GetCollections() []Collection
	GetRootItems() []*TreeItem
	CreateCollection(name string) (Collection, error)
	DeleteCollection(id string) error
	RenameCollection(id, name string) error
	MoveCollection(collectionID string, position int) error
	GetSidebarLayout() ([]SidebarEntry, error)
	MoveSidebarEntry(kind, id string, position int) error
	MoveItemToSidebar(sourceCollectionID, itemID string, sidebarPosition int) error
}

// CollectionItemUseCase はコレクション内ツリーアイテム管理のユースケース入力ポート。
type CollectionItemUseCase interface {
	AddFolder(collectionID, parentID, name string) (*TreeItem, error)
	AddRequest(collectionID, parentID string, req HttpRequest) (*TreeItem, error)
	UpdateRequest(collectionID string, req HttpRequest) error
	RenameItem(collectionID, itemID, name string) error
	DeleteItem(collectionID, itemID string) error
	// MoveItem はアイテムをコレクション内外・別の親・位置へ移動する。
	// targetParentID が空の場合はターゲットコレクションルートへ移動する。
	// position は挿入先インデックス（削除後の配列に対する）。-1 の場合は末尾に追加。
	MoveItem(sourceCollectionID, itemID, targetCollectionID, targetParentID string, position int) error
}
