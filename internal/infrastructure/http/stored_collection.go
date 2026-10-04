package httpinfra

import (
	"maps"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// storedCollection はコレクションの永続化 DTO。
// フィールドと JSON 名は既存ファイルの形式 (testdata/collection.golden.json) に揃える。
// file 参照は storedFileReference でしか表現できず、token と実パスを書き出せない。
type storedCollection struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Items []*storedTreeItem `json:"items"`
}

type storedTreeItem struct {
	Request  *storedRequest    `json:"request,omitempty"`
	Type     string            `json:"type"`
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Children []*storedTreeItem `json:"children"`
}

type storedRequest struct {
	Body     storedRequestBody     `json:"body"`
	Auth     storedRequestAuth     `json:"auth"`
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Method   string                `json:"method"`
	URL      string                `json:"url"`
	Doc      string                `json:"doc"`
	Headers  []storedKeyValuePair  `json:"headers"`
	Params   []storedKeyValuePair  `json:"params"`
	Settings storedRequestSettings `json:"settings"`
}

type storedRequestAuth struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

type storedKeyValuePair struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

type storedRequestSettings struct {
	ProxyMode          string `json:"proxyMode"`
	ProxyURL           string `json:"proxyURL"`
	TimeoutSec         int    `json:"timeoutSec"`
	MaxResponseBodyMB  int    `json:"maxResponseBodyMB"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	DisableRedirects   bool   `json:"disableRedirects"`
}

type storedRequestBody struct {
	// Contents は file 種別のキーを持たない (手で書き換えたファイルの読み込み時のみ存在しうる)。
	Contents       map[string]string    `json:"contents"`
	File           *storedFileReference `json:"file,omitempty"`
	Type           string               `json:"type"`
	FormData       []storedFormRow      `json:"formData,omitempty"`
	FormURLEncoded []storedFormRow      `json:"formUrlEncoded,omitempty"`
}

type storedFormRow struct {
	File        *storedFileReference `json:"file,omitempty"`
	Key         string               `json:"key"`
	Value       string               `json:"value"`
	Kind        string               `json:"kind,omitempty"`
	ContentType string               `json:"contentType,omitempty"`
	Enabled     bool                 `json:"enabled"`
}

// storedFileReference は永続化するファイル参照。token と実パスを表現できない。
// basename があれば常に再選択が必要な状態として保存する。
type storedFileReference struct {
	Name          string `json:"name,omitempty"`
	NeedsReselect bool   `json:"needsReselect,omitempty"`
}

func newStoredCollection(c *domain.Collection) storedCollection {
	return storedCollection{ID: c.ID, Name: c.Name, Items: toStoredItems(c.Items)}
}

func toStoredItems(items []*domain.TreeItem) []*storedTreeItem {
	if items == nil {
		return nil
	}
	out := make([]*storedTreeItem, 0, len(items))
	for _, it := range items {
		out = append(out, &storedTreeItem{
			Request:  toStoredRequest(it.Request),
			Type:     it.Type,
			ID:       it.ID,
			Name:     it.Name,
			Children: toStoredItems(it.Children),
		})
	}
	return out
}

func toStoredRequest(r *domain.HTTPRequest) *storedRequest {
	if r == nil {
		return nil
	}
	return &storedRequest{
		Body:     toStoredBody(&r.Body),
		Auth:     toStoredAuth(r.Auth),
		ID:       r.ID,
		Name:     r.Name,
		Method:   r.Method,
		URL:      r.URL,
		Doc:      r.Doc,
		Headers:  toStoredPairs(r.Headers),
		Params:   toStoredPairs(r.Params),
		Settings: toStoredSettings(r.Settings),
	}
}

func toStoredAuth(a domain.RequestAuth) storedRequestAuth {
	return storedRequestAuth{Type: a.Type, Username: a.Username, Password: a.Password, Token: a.Token}
}

func toStoredPairs(pairs []domain.KeyValuePair) []storedKeyValuePair {
	if pairs == nil {
		return nil
	}
	out := make([]storedKeyValuePair, len(pairs))
	for i, p := range pairs {
		out[i] = storedKeyValuePair{Key: p.Key, Value: p.Value, Enabled: p.Enabled}
	}
	return out
}

func toStoredSettings(s domain.RequestSettings) storedRequestSettings {
	return storedRequestSettings{
		ProxyMode:          s.ProxyMode,
		ProxyURL:           s.ProxyURL,
		TimeoutSec:         s.TimeoutSec,
		MaxResponseBodyMB:  s.MaxResponseBodyMB,
		InsecureSkipVerify: s.InsecureSkipVerify,
		DisableRedirects:   s.DisableRedirects,
	}
}

func toStoredBody(b *domain.RequestBody) storedRequestBody {
	contents := maps.Clone(b.Contents)
	// file ボディの送信元は File の token だけ。Contents にパスが紛れていても保存しない。
	delete(contents, domain.BodyTypeFile)
	return storedRequestBody{
		Contents:       contents,
		File:           toStoredFileRef(b.File),
		Type:           b.Type,
		FormData:       toStoredRows(b.FormData),
		FormURLEncoded: toStoredRows(b.FormURLEncoded),
	}
}

func toStoredRows(rows []domain.FormRow) []storedFormRow {
	if rows == nil {
		return nil
	}
	out := make([]storedFormRow, len(rows))
	for i := range rows {
		r := &rows[i]
		out[i] = storedFormRow{
			File:        toStoredFileRef(r.File),
			Key:         r.Key,
			Value:       r.Value,
			Kind:        r.Kind,
			ContentType: r.ContentType,
			Enabled:     r.Enabled,
		}
	}
	return out
}

// toStoredFileRef は token を捨て、basename があれば再選択が必要な参照として保存する。
// 選択中の token が有効でも、再起動後は token が無いため NeedsReselect を必ず立てる。
func toStoredFileRef(ref domain.FileReference) *storedFileReference {
	if ref.Name == "" {
		return nil
	}
	return &storedFileReference{Name: ref.Name, NeedsReselect: true}
}

func (s *storedCollection) toDomain() domain.Collection {
	return domain.Collection{ID: s.ID, Name: s.Name, Items: fromStoredItems(s.Items)}
}

func fromStoredItems(items []*storedTreeItem) []*domain.TreeItem {
	if items == nil {
		return nil
	}
	out := make([]*domain.TreeItem, 0, len(items))
	for _, it := range items {
		out = append(out, &domain.TreeItem{
			Request:  it.Request.toDomain(),
			Type:     it.Type,
			ID:       it.ID,
			Name:     it.Name,
			Children: fromStoredItems(it.Children),
		})
	}
	return out
}

func (r *storedRequest) toDomain() *domain.HTTPRequest {
	if r == nil {
		return nil
	}
	return &domain.HTTPRequest{
		Body:     r.Body.toDomain(),
		Auth:     fromStoredAuth(r.Auth),
		ID:       r.ID,
		Name:     r.Name,
		Method:   r.Method,
		URL:      r.URL,
		Doc:      r.Doc,
		Headers:  fromStoredPairs(r.Headers),
		Params:   fromStoredPairs(r.Params),
		Settings: fromStoredSettings(r.Settings),
	}
}

func fromStoredAuth(a storedRequestAuth) domain.RequestAuth {
	return domain.RequestAuth{Type: a.Type, Username: a.Username, Password: a.Password, Token: a.Token}
}

func fromStoredPairs(pairs []storedKeyValuePair) []domain.KeyValuePair {
	if pairs == nil {
		return nil
	}
	out := make([]domain.KeyValuePair, len(pairs))
	for i, p := range pairs {
		out[i] = domain.KeyValuePair{Key: p.Key, Value: p.Value, Enabled: p.Enabled}
	}
	return out
}

func fromStoredSettings(s storedRequestSettings) domain.RequestSettings {
	return domain.RequestSettings{
		ProxyMode:          s.ProxyMode,
		ProxyURL:           s.ProxyURL,
		TimeoutSec:         s.TimeoutSec,
		MaxResponseBodyMB:  s.MaxResponseBodyMB,
		InsecureSkipVerify: s.InsecureSkipVerify,
		DisableRedirects:   s.DisableRedirects,
	}
}

func (b *storedRequestBody) toDomain() domain.RequestBody {
	contents := maps.Clone(b.Contents)
	// 手で書き換えたファイルに残ったパスを RPC へ出さない。
	delete(contents, domain.BodyTypeFile)
	return domain.RequestBody{
		Contents:       contents,
		File:           fromStoredFileRef(b.File),
		Type:           b.Type,
		FormData:       fromStoredRows(b.FormData),
		FormURLEncoded: fromStoredRows(b.FormURLEncoded),
	}
}

func fromStoredRows(rows []storedFormRow) []domain.FormRow {
	if rows == nil {
		return nil
	}
	out := make([]domain.FormRow, len(rows))
	for i := range rows {
		r := &rows[i]
		out[i] = domain.FormRow{
			Key:         r.Key,
			Value:       r.Value,
			Kind:        r.Kind,
			File:        fromStoredFileRef(r.File),
			ContentType: r.ContentType,
			Enabled:     r.Enabled,
		}
	}
	return out
}

// fromStoredFileRef は保存済みの参照を token なしの再選択待ちとして復元する。
func fromStoredFileRef(ref *storedFileReference) domain.FileReference {
	if ref == nil || ref.Name == "" {
		return domain.FileReference{}
	}
	return domain.FileReference{Name: ref.Name, NeedsReselect: true}
}
