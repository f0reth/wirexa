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
	ProxyMode          string `json:"proxyMode"`
	ProxyURL           string `json:"proxyURL"`
	TimeoutSec         int    `json:"timeoutSec"`
	MaxResponseBodyMB  int    `json:"maxResponseBodyMB"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	DisableRedirects   bool   `json:"disableRedirects"`
}

// HttpRequest は HTTP リクエストを表す。
type HttpRequest struct {
	Body     RequestBody     `json:"body"`
	Auth     RequestAuth     `json:"auth"`
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Doc      string          `json:"doc"`
	Headers  []KeyValuePair  `json:"headers"`
	Params   []KeyValuePair  `json:"params"`
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
	Contents map[string]string `json:"contents"`
	Type     string            `json:"type"`
}

// HttpResponse は HTTP レスポンスを表す。
type HttpResponse struct {
	Headers       map[string]string `json:"headers"`
	StatusText    string            `json:"statusText"`
	Body          string            `json:"body"`
	ContentType   string            `json:"contentType"`
	Error         string            `json:"error"`
	StatusCode    int               `json:"statusCode"`
	Size          int64             `json:"size"`
	TimingMs      int64             `json:"timingMs"`
	BodyTruncated bool              `json:"bodyTruncated"`
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
	Request  *HttpRequest `json:"request,omitempty"`
	Type     string       `json:"type"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Children []*TreeItem  `json:"children"`
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

// AppendItem はアイテムをコレクションに追加する。
// parentID が空の場合はルート直下に、非空の場合は指定フォルダの末尾に追加する。
// parentID が見つからない、またはフォルダでない場合は false を返す。
func (c *Collection) AppendItem(parentID string, item *TreeItem) bool {
	if parentID == "" {
		c.Items = append(c.Items, item)
		return true
	}
	parent, _, ok := findNode(parentID, c.Items, nil)
	if !ok || parent.Type != ItemTypeFolder {
		return false
	}
	parent.Children = append(parent.Children, item)
	return true
}

// InsertItem はアイテムをコレクションの指定位置に挿入する。
// parentID が空の場合はルート直下に、非空の場合は指定フォルダ内に挿入する。
// position が負または範囲外の場合は末尾に追加する。
// parentID が見つからない、またはフォルダでない場合は false を返す。
func (c *Collection) InsertItem(parentID string, item *TreeItem, position int) bool {
	if parentID == "" {
		c.Items = insertAt(c.Items, item, position)
		return true
	}
	parent, _, ok := findNode(parentID, c.Items, nil)
	if !ok || parent.Type != ItemTypeFolder {
		return false
	}
	parent.Children = insertAt(parent.Children, item, position)
	return true
}

// insertAt はスライスの指定インデックスにアイテムを挿入する。
// position が負または範囲外の場合は末尾に追加する。
func insertAt(items []*TreeItem, item *TreeItem, position int) []*TreeItem {
	if position < 0 || position >= len(items) {
		return append(items, item)
	}
	items = append(items, nil)
	copy(items[position+1:], items[position:])
	items[position] = item
	return items
}
