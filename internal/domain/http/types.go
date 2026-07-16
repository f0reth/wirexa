package httpdomain

import (
	"net/url"
	"strings"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// BodyType 定数のうち、行編集 UI を持つ form 系ボディの種別。
const (
	BodyTypeFormData       = "form-data"
	BodyTypeFormURLEncoded = "form-urlencoded"
)

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

// HTTPRequest は HTTP リクエストを表す。
type HTTPRequest struct {
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
// FormData / FormURLEncoded は form 系ボディの編集状態（行）を保持し、
// ワイヤ形式は送信時に生成する。Contents は form 系以外のボディ本文と、
// Forms 導入前に保存された旧データの復元元を兼ねる。
//
// 行を map[string][]KeyValuePair でまとめないのは Wails のバインディング生成が
// map[string][]Struct を壊すため（asMap 経路が `new Array(rows)` で行配列を
// 二重配列にする）。body type ごとに独立したスライスとして持つ。
type RequestBody struct {
	Contents       map[string]string `json:"contents"`
	FormData       []KeyValuePair    `json:"formData,omitempty"`
	FormURLEncoded []KeyValuePair    `json:"formUrlEncoded,omitempty"`
	Type           string            `json:"type"`
}

// formPairsFor は body type に対応する行スライスへのポインタを返す。
// form 系以外の body type では nil を返す。
func (b *RequestBody) formPairsFor(bodyType string) *[]KeyValuePair {
	switch bodyType {
	case BodyTypeFormData:
		return &b.FormData
	case BodyTypeFormURLEncoded:
		return &b.FormURLEncoded
	}
	return nil
}

// FormPairs は現在の Type に対応するフォーム行を返す。form 系以外では nil。
func (b *RequestBody) FormPairs() []KeyValuePair {
	if dst := b.formPairsFor(b.Type); dst != nil {
		return *dst
	}
	return nil
}

// NormalizeForms は行フィールド導入前に保存されたリクエストを移行する。
// 行が未設定で Contents に旧 urlencoded 文字列が残っている場合のみ復元し、
// 移行後は Contents 側の form 系キーを削除して正を一本化する。
// キーを消さないと「全行を削除した」状態が行なしと区別できず、
// 再読み込み時に旧文字列から行が復活してしまう。
func (b *RequestBody) NormalizeForms() {
	for _, bodyType := range []string{BodyTypeFormData, BodyTypeFormURLEncoded} {
		dst := b.formPairsFor(bodyType)
		if content := b.Contents[bodyType]; *dst == nil && content != "" {
			*dst = ParseFormPairs(content)
		}
		delete(b.Contents, bodyType)
	}
}

// ParseFormPairs は application/x-www-form-urlencoded 文字列を行へ復元する。
// url.ParseQuery と違い map を経由しないため、入力の行順と重複キーをそのまま保つ。
func ParseFormPairs(content string) []KeyValuePair {
	if content == "" {
		return nil
	}
	segments := strings.Split(content, "&")
	pairs := make([]KeyValuePair, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		key, value, _ := strings.Cut(segment, "=")
		pairs = append(pairs, KeyValuePair{
			Key:     unescapeFormComponent(key),
			Value:   unescapeFormComponent(value),
			Enabled: true,
		})
	}
	return pairs
}

// EncodeFormPairs は有効な行を application/x-www-form-urlencoded 文字列へ変換する。
// url.Values.Encode() を使わないのはキー名でソートされ UI の行順が失われるため。
func EncodeFormPairs(pairs []KeyValuePair) string {
	var sb strings.Builder
	for _, p := range pairs {
		if !p.Enabled || p.Key == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(url.QueryEscape(p.Key))
		sb.WriteByte('=')
		sb.WriteString(url.QueryEscape(p.Value))
	}
	return sb.String()
}

// unescapeFormComponent は urlencoded の 1 要素をデコードする。
// 不正なエスケープを含む入力でも移行を失敗させず、生の文字列として扱う。
func unescapeFormComponent(s string) string {
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}

// HTTPResponse は HTTP レスポンスを表す。
type HTTPResponse struct {
	// Headers は Set-Cookie などの複数値ヘッダを落とさないよう、キーごとに全値を保持する。
	Headers     map[string][]string `json:"headers"`
	StatusText  string              `json:"statusText"`
	Body        string              `json:"body"`
	ContentType string              `json:"contentType"`
	Error       string              `json:"error"`
	// TempFilePath はボディが大きく切り詰められた際に全文を保存したテンポラリファイルのパス。
	// adapter 層が ConsumeTempFilePath で設定する出力用フィールド（infra は設定しない）。
	TempFilePath string `json:"tempFilePath"`
	StatusCode   int    `json:"statusCode"`
	// Size は実際に受信したバイト数。BodyCapped が true の場合は絶対上限で頭打ちになるため、
	// レスポンスの全長とは一致しない。
	Size          int64 `json:"size"`
	TimingMs      int64 `json:"timingMs"`
	BodyTruncated bool  `json:"bodyTruncated"`
	// BodyBase64 は Body が非 UTF-8 バイナリのため base64 エンコードされていることを示す。
	BodyBase64 bool `json:"bodyBase64"`
	// BodyCapped は絶対上限に達して受信を打ち切ったことを示す。
	// このとき TempFilePath のファイルもレスポンスの全文ではない。
	BodyCapped bool `json:"bodyCapped"`
}

// Collection はリクエストコレクションを表す。
type Collection struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Items []*TreeItem `json:"items"`
}

// TreeItem はコレクション内のフォルダまたはリクエストアイテムを表す。
type TreeItem struct {
	Request  *HTTPRequest `json:"request,omitempty"`
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

// Contains は t 自身または子孫に id のノードが存在するか返す。
func (t *TreeItem) Contains(id string) bool {
	if t.ID == id {
		return true
	}
	_, _, ok := findNode(id, t.Children, nil)
	return ok
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
		c.Items = cmn.InsertAt(c.Items, item, position)
		return true
	}
	parent, _, ok := findNode(parentID, c.Items, nil)
	if !ok || parent.Type != ItemTypeFolder {
		return false
	}
	parent.Children = cmn.InsertAt(parent.Children, item, position)
	return true
}
