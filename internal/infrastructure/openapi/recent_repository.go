package openapiinfra

import (
	domain "github.com/f0reth/Wirexa/internal/domain/openapi"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

// コンパイル時に domain.RecentRepository を満たすことを検証
var _ domain.RecentRepository = (*RecentRepository)(nil)

// RecentRepository は openapi-recents.json の読み書きリポジトリ。
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
	items, _, err := infra.ReadJSONFile[[]domain.OpenAPIRecent](r.path)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.OpenAPIRecent{}
	}
	return items, nil
}

// Save は一覧をファイルへ原子的に書き込む。
func (r *RecentRepository) Save(items []domain.OpenAPIRecent) error {
	return infra.WriteJSONFile(r.path, items, 0o600)
}

// Quarantine は壊れた recents ファイルを退避し、退避先のパスを返す。
func (r *RecentRepository) Quarantine() (string, error) {
	return infra.QuarantineFile(r.path)
}
