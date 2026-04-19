package httpdomain

// RequestAuth はリクエスト認証情報を表す。
type RequestAuth struct {
	Type     string `json:"type"`     // "none" | "basic" | "bearer"
	Username string `json:"username"` // Basic認証用
	Password string `json:"password"` // Basic認証用
	Token    string `json:"token"`    // Bearerトークン用
}

// RequestSettings はリクエストごとの HTTP クライアント設定を表す。
// ゼロ値はすべてデフォルト動作を意味する。
type RequestSettings struct {
	TimeoutSec         int    `json:"timeoutSec"`         // 0 = デフォルト (30s)
	ProxyMode          string `json:"proxyMode"`          // "" | "system" | "none" | "custom"
	ProxyURL           string `json:"proxyURL"`           // ProxyMode == "custom" のときに使用
	InsecureSkipVerify bool   `json:"insecureSkipVerify"` // TLS 証明書検証をスキップ
	DisableRedirects   bool   `json:"disableRedirects"`   // リダイレクトを無効化
	MaxResponseBodyMB  int    `json:"maxResponseBodyMB"`  // 0 = デフォルト (10MB)
}

// HttpRequest は HTTP リクエストを表す。
type HttpRequest struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Headers  []KeyValuePair  `json:"headers"`
	Params   []KeyValuePair  `json:"params"`
	Body     RequestBody     `json:"body"`
	Auth     RequestAuth     `json:"auth"`
	Settings RequestSettings `json:"settings"`
}

// KeyValuePair はヘッダーやパラメータのキーバリューペアを表す。
type KeyValuePair struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

// RequestBody はリクエストボディを表す。
type RequestBody struct {
	Type     string            `json:"type"`     // "none"|"json"|"text"|"form-urlencoded"|"form-data"|"file"
	Contents map[string]string `json:"contents"` // ボディタイプごとのコンテンツ
}

// HttpResponse は HTTP レスポンスを表す。
type HttpResponse struct {
	StatusCode    int               `json:"statusCode"`
	StatusText    string            `json:"statusText"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	ContentType   string            `json:"contentType"`
	Size          int64             `json:"size"`
	TimingMs      int64             `json:"timingMs"`
	Error         string            `json:"error"`
	BodyTruncated bool              `json:"bodyTruncated"` // ボディが上限で切り捨てられた場合 true
}

// Collection はリクエストコレクションを表す。
type Collection struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Items []*TreeItem `json:"items"`
	Order int         `json:"order"`
}

// TreeItem はコレクション内のフォルダまたはリクエストアイテムを表す。
type TreeItem struct {
	Type     string       `json:"type"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Children []*TreeItem  `json:"children"`
	Request  *HttpRequest `json:"request,omitempty"`
}

// ItemType 定数はツリーアイテムの種別を定義する。
const (
	ItemTypeFolder  = "folder"
	ItemTypeRequest = "request"
)

// RootCollectionID はルートリクエスト置き場として使用する予約済みコレクション ID。
const RootCollectionID = "__root__"

// SidebarEntry はサイドバーレイアウトの1エントリを表す。
type SidebarEntry struct {
	Kind string `json:"kind"` // "collection" | "item"
	ID   string `json:"id"`
}

// findNode はツリーを再帰的に走査してIDに一致するノードとその親を返す。
func findNode(id string, items []*TreeItem, parent *TreeItem) (*TreeItem, *TreeItem, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, parent, true
		}
		if item.Type == ItemTypeFolder {
			if node, par, ok := findNode(id, item.Children, item); ok {
				return node, par, ok
			}
		}
	}
	return nil, nil, false
}

// FindNode はコレクション内のノードをIDで検索し、ノードとその親ノードを返す。
// parent が nil の場合はルート直下のアイテムを意味する。
func (c *Collection) FindNode(id string) (*TreeItem, *TreeItem, bool) {
	return findNode(id, c.Items, nil)
}

// RemoveNode はコレクションからIDに対応するノードをサブツリーごと削除する。
// 削除に成功した場合は true を返す。
func (c *Collection) RemoveNode(id string) bool {
	_, parent, ok := findNode(id, c.Items, nil)
	if !ok {
		return false
	}
	if parent == nil {
		for i, n := range c.Items {
			if n.ID == id {
				c.Items = append(c.Items[:i], c.Items[i+1:]...)
				return true
			}
		}
	} else {
		for i, n := range parent.Children {
			if n.ID == id {
				parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
				return true
			}
		}
	}
	return false
}
