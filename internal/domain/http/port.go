// Package httpdomain は HTTP ドメイン層のポートインターフェースを定義する。
package httpdomain

// CollectionRepository はコレクションの永続化抽象。
type CollectionRepository interface {
	Load() ([]Collection, error)
	Save(c *Collection) error
	Delete(id string) error
}

// RequestUseCase は HTTP リクエスト送信のユースケース入力ポート。
type RequestUseCase interface {
	SendRequest(req HttpRequest) (HttpResponse, error)
}

// CollectionUseCase はコレクション管理のユースケース入力ポート。
// Wails RPC アダプター (ports 層) はこのインターフェースのみに依存する。
type CollectionUseCase interface {
	GetCollections() []Collection
	CreateCollection(name string) (Collection, error)
	DeleteCollection(id string) error
	RenameCollection(id, name string) error
	AddFolder(collectionID, parentID, name string) (*TreeItem, error)
	AddRequest(collectionID, parentID string, req HttpRequest) (*TreeItem, error)
	UpdateRequest(collectionID string, req HttpRequest) error
	RenameItem(collectionID, itemID, name string) error
	DeleteItem(collectionID, itemID string) error
}
