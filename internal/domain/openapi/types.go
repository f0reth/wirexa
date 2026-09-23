// Package openapidomain は OpenAPI ファイル操作のドメイン型とポートを定義する。
package openapidomain

// このパッケージの型は業務概念と不変条件を表し、Wails RPC とイベントの配線型を兼ねる
// (adapters に RPC 用の DTO は作らない)。json タグは RPC とイベントの配線形式だけを表す。
// 永続化形式は infrastructure のリポジトリにある storedXxx DTO が持つので、タグを変えても
// 保存形式は変わらない。保存対象のフィールドを足すときは stored DTO と変換関数にも足す。
// ドメイン型に載せてよいのは RPC で受け渡す業務上の値だけで、パスや一時的なハンドルのような
// infrastructure 内部の状態は載せない。

// OpenAPIRecent は最近開いた OpenAPI ファイルの参照。
// path を一意キーとして扱う (id は持たない)。
type OpenAPIRecent struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	LastOpenedAt string `json:"lastOpenedAt"`
	Order        int    `json:"order"`
}
