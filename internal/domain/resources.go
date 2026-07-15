package domain

// リソース名定数。NotFoundError.Resource に用いる。
// 自由文字列の散在を防ぎ、エラーメッセージの表記を一元管理する。
const (
	ResourceConnection   = "connection"
	ResourceSession      = "session"
	ResourceProfile      = "profile"
	ResourceTarget       = "target"
	ResourceCollection   = "collection"
	ResourceItem         = "item"
	ResourceRequest      = "request"
	ResourceParent       = "parent"
	ResourceSidebarEntry = "sidebar entry"
)

// MsgRequired は「必須項目が未入力」を表す共有バリデーションメッセージ。
// 同じ条件が "is required" / "required" に割れていたのを統一する。
const MsgRequired = "is required"
