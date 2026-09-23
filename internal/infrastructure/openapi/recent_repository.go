package openapiinfra

import (
	domain "github.com/f0reth/Wirexa/internal/domain/openapi"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

// コンパイル時に domain.RecentRepository を満たすことを検証
var _ domain.RecentRepository = (*RecentRepository)(nil)

// RecentRepository は openapi-recents.json の読み書きリポジトリ。
// ディスクへは永続化 DTO (storedRecent) を書き、domain 型の json タグ (RPC の配線形式) を
// 変えても保存形式が変わらないようにする。
type RecentRepository struct {
	path string
}

// NewRecentRepository は指定パスの RecentRepository を返す。
func NewRecentRepository(path string) *RecentRepository {
	return &RecentRepository{path: path}
}

// Load は recents ファイルを読み込む。ファイルが存在しない場合は空スライスを返す。
// JSON として解釈できない場合は cmn.ErrCorruptData を wrap して返し、
// 読み込み自体の失敗と区別できるようにする。
func (r *RecentRepository) Load() ([]domain.OpenAPIRecent, error) {
	stored, _, err := infra.ReadJSONFile[[]storedRecent](r.path)
	if err != nil {
		return nil, err
	}
	items := make([]domain.OpenAPIRecent, len(stored))
	for i, s := range stored {
		items[i] = domain.OpenAPIRecent{Path: s.Path, Name: s.Name, LastOpenedAt: s.LastOpenedAt, Order: s.Order}
	}
	return items, nil
}

// Save は一覧をファイルへ原子的に書き込む。
func (r *RecentRepository) Save(items []domain.OpenAPIRecent) error {
	stored := make([]storedRecent, len(items))
	for i, it := range items {
		stored[i] = storedRecent{Path: it.Path, Name: it.Name, LastOpenedAt: it.LastOpenedAt, Order: it.Order}
	}
	return infra.WriteJSONFile(r.path, stored, 0o600)
}

// Quarantine は壊れた recents ファイルを退避し、退避先のパスを返す。
func (r *RecentRepository) Quarantine() (string, error) {
	return infra.QuarantineFile(r.path)
}

// storedRecent は recents 1 件の永続化 DTO。
// フィールドと JSON 名は既存ファイルの形式 (testdata/recents.golden.json) に揃える。
// domain 型は埋め込まない。
type storedRecent struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	LastOpenedAt string `json:"lastOpenedAt"`
	Order        int    `json:"order"`
}
