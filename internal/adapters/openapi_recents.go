package adapters

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"

	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

// openapiMaxRecents は保持する最近使ったファイルの最大件数。
const openapiMaxRecents = 50

// OpenApiRecent は最近開いた OpenAPI ファイルの参照。
// path を一意キーとして扱う (id は持たない)。
type OpenApiRecent struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Order        int    `json:"order"`
	LastOpenedAt string `json:"lastOpenedAt"`
}

// openapiRecentStore は最近使ったファイル一覧を設定ディレクトリの JSON に永続化する。
// 追加はダイアログ / 保存ハンドラ経由でのみ行われ、JS からは汚染できない。
type openapiRecentStore struct {
	mu    sync.Mutex
	path  string
	items []OpenApiRecent
}

// newOpenapiRecentStore は指定パスの JSON をロードしてストアを返す。
// 読み込み・パースに失敗した場合は空で開始する (起動は止めない)。
func newOpenapiRecentStore(path string) *openapiRecentStore {
	s := &openapiRecentStore{path: path}
	if data, err := os.ReadFile(path); err == nil {
		var items []OpenApiRecent
		if err := json.Unmarshal(data, &items); err == nil {
			s.items = items
		}
	}
	return s
}

// list は order 昇順でコピーを返す。
func (s *openapiRecentStore) list() []OpenApiRecent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]OpenApiRecent, len(s.items))
	copy(out, s.items)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// paths は全 recents のパスを返す (許可リスト seed 用)。
func (s *openapiRecentStore) paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it.Path)
	}
	return out
}

// add はパスを追加する。既存なら lastOpenedAt を更新する。
// 上限超過時は lastOpenedAt が古いものから間引く。
func (s *openapiRecentStore) add(path, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	found := false
	for i := range s.items {
		if s.items[i].Path == path {
			s.items[i].LastOpenedAt = now
			found = true
			break
		}
	}
	if !found {
		s.items = append(s.items, OpenApiRecent{
			Path:         path,
			Name:         name,
			Order:        len(s.items),
			LastOpenedAt: now,
		})
	}

	if len(s.items) > openapiMaxRecents {
		sort.SliceStable(s.items, func(i, j int) bool {
			return s.items[i].LastOpenedAt > s.items[j].LastOpenedAt
		})
		s.items = s.items[:openapiMaxRecents]
	}
	s.reindexLocked()
	return s.persistLocked()
}

// remove はパスを一覧から削除する。
func (s *openapiRecentStore) remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.items[:0]
	for _, it := range s.items {
		if it.Path != path {
			filtered = append(filtered, it)
		}
	}
	s.items = filtered
	s.reindexLocked()
	return s.persistLocked()
}

// move は path のエントリを index の位置へ並び替える。
func (s *openapiRecentStore) move(path string, index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sort.SliceStable(s.items, func(i, j int) bool { return s.items[i].Order < s.items[j].Order })
	from := -1
	for i := range s.items {
		if s.items[i].Path == path {
			from = i
			break
		}
	}
	if from == -1 {
		return nil
	}
	item := s.items[from]
	s.items = append(s.items[:from], s.items[from+1:]...)
	if index < 0 {
		index = 0
	}
	if index > len(s.items) {
		index = len(s.items)
	}
	s.items = append(s.items, OpenApiRecent{})
	copy(s.items[index+1:], s.items[index:])
	s.items[index] = item
	s.reindexLocked()
	return s.persistLocked()
}

// reindexLocked は現在の並び順に沿って Order を 0..n へ振り直す。呼び出し側で lock 済み。
func (s *openapiRecentStore) reindexLocked() {
	for i := range s.items {
		s.items[i].Order = i
	}
}

// persistLocked は現在の一覧を JSON へ原子的に書き出す。呼び出し側で lock 済み。
func (s *openapiRecentStore) persistLocked() error {
	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return infra.AtomicWriteFile(s.path, data, 0o600)
}
