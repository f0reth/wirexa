package httpinfra

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

var _ domain.CollectionRepository = (*CollectionRepository)(nil)

// CollectionRepository は HTTP コレクションを JSON ファイルへ永続化する。
//
// runtime model (domain.Collection) は session token を持つが、ディスクへは
// 永続化 DTO (storedCollection) を書く。変換をこの境界 1 か所で行うため、
// CollectionService のどの保存経路 (追加・更新・名称変更・移動) からも
// token と実パスは永続化されない。変換は deep copy で、呼び出し元の runtime model は変更しない。
type CollectionRepository struct {
	store *infra.JSONStore[storedCollection]
}

// NewCollectionRepository はディレクトリを作成して CollectionRepository を返す。
// logger は破損ファイルの退避記録に使う。nil の場合は記録しない。
func NewCollectionRepository(dir string, logger cmn.Logger) (*CollectionRepository, error) {
	store, err := infra.NewJSONStore(dir, func(c *storedCollection) string { return c.ID })
	if err != nil {
		return nil, err
	}
	if logger != nil {
		store.SetLogger(logger)
	}
	return &CollectionRepository{store: store}, nil
}

// Load は全コレクションを読み込み、runtime model へ変換して返す。
// 旧形式のファイルパスは basename と再選択状態へ移行し、パス自体は返さない。
func (r *CollectionRepository) Load() ([]domain.Collection, error) {
	stored, err := r.store.Load()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Collection, 0, len(stored))
	for i := range stored {
		out = append(out, stored[i].toDomain())
	}
	return out, nil
}

// Save はコレクションを永続化 DTO へ変換して保存する。
func (r *CollectionRepository) Save(c *domain.Collection) error {
	s := newStoredCollection(c)
	return r.store.Save(&s)
}

// Delete はコレクションのファイルを削除する。
func (r *CollectionRepository) Delete(id string) error {
	return r.store.Delete(id)
}

// ── 永続化 DTO ────────────────────────────────────────────────────────────────
// フィールドと JSON 名は domain 型と揃え、既存ファイルとの互換を保つ。
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
	Body     storedRequestBody      `json:"body"`
	Auth     domain.RequestAuth     `json:"auth"`
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Method   string                 `json:"method"`
	URL      string                 `json:"url"`
	Doc      string                 `json:"doc"`
	Headers  []domain.KeyValuePair  `json:"headers"`
	Params   []domain.KeyValuePair  `json:"params"`
	Settings domain.RequestSettings `json:"settings"`
}

type storedRequestBody struct {
	// Contents は file 種別のキーを持たない (旧データの読み込み時のみ存在しうる)。
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
	// legacyFilePath は旧形式の "filePath"。読み込み時の移行にだけ使い、書き出さない。
	legacyFilePath string
	Enabled        bool `json:"enabled"`
}

// UnmarshalJSON は旧形式の "filePath" を書き出し不能な非公開フィールドへ読み込む。
func (r *storedFormRow) UnmarshalJSON(data []byte) error {
	type plain storedFormRow
	var aux struct {
		FilePath string `json:"filePath"`
		plain
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = storedFormRow(aux.plain)
	r.legacyFilePath = aux.FilePath
	return nil
}

// storedFileReference は永続化するファイル参照。token と実パスを表現できない。
// basename があれば常に再選択が必要な状態として保存する。
type storedFileReference struct {
	Name          string `json:"name,omitempty"`
	NeedsReselect bool   `json:"needsReselect,omitempty"`
}

// ── runtime model → 永続化 DTO ───────────────────────────────────────────────

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
		Auth:     r.Auth,
		ID:       r.ID,
		Name:     r.Name,
		Method:   r.Method,
		URL:      r.URL,
		Doc:      r.Doc,
		Headers:  slices.Clone(r.Headers),
		Params:   slices.Clone(r.Params),
		Settings: r.Settings,
	}
}

func toStoredBody(b *domain.RequestBody) storedRequestBody {
	contents := maps.Clone(b.Contents)
	// runtime に残る旧形式のパス (file 種別の本文) は basename だけを残して捨てる。
	legacyPath := contents[domain.BodyTypeFile]
	delete(contents, domain.BodyTypeFile)
	return storedRequestBody{
		Contents:       contents,
		File:           toStoredFileRef(b.File, legacyPath),
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
			File:        toStoredFileRef(r.File, r.FilePath),
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
func toStoredFileRef(ref domain.FileReference, legacyPath string) *storedFileReference {
	name := ref.Name
	if name == "" {
		name = legacyBaseName(legacyPath)
	}
	if name == "" {
		return nil
	}
	return &storedFileReference{Name: name, NeedsReselect: true}
}

// ── 永続化 DTO → runtime model ───────────────────────────────────────────────

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
		Auth:     r.Auth,
		ID:       r.ID,
		Name:     r.Name,
		Method:   r.Method,
		URL:      r.URL,
		Doc:      r.Doc,
		Headers:  slices.Clone(r.Headers),
		Params:   slices.Clone(r.Params),
		Settings: r.Settings,
	}
}

func (b *storedRequestBody) toDomain() domain.RequestBody {
	contents := maps.Clone(b.Contents)
	legacyPath := contents[domain.BodyTypeFile]
	delete(contents, domain.BodyTypeFile)
	return domain.RequestBody{
		Contents:       contents,
		File:           fromStoredFileRef(b.File, legacyPath),
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
			File:        fromStoredFileRef(r.File, r.legacyFilePath),
			ContentType: r.ContentType,
			Enabled:     r.Enabled,
		}
	}
	return out
}

// fromStoredFileRef は保存済みの参照を token なしの再選択待ちとして復元する。
// 旧形式のパスは basename にだけ変換し、許可として扱わない (registry へ登録しない)。
func fromStoredFileRef(ref *storedFileReference, legacyPath string) domain.FileReference {
	name := ""
	if ref != nil {
		name = ref.Name
	}
	if name == "" {
		name = legacyBaseName(legacyPath)
	}
	if name == "" {
		return domain.FileReference{}
	}
	return domain.FileReference{Name: name, NeedsReselect: true}
}

// legacyBaseName は旧データのパスから表示用の basename を取り出す。
// 保存時と現在で OS が異なりうるため、/ と \ の両方を区切りとして扱う。
// 文字列処理だけで、パスが指すファイルにはアクセスしない。
func legacyBaseName(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		p = p[i+1:]
	}
	return p
}
