package domain

// リソース名定数。NotFoundError.Resource に用いる。
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
const MsgRequired = "is required"
