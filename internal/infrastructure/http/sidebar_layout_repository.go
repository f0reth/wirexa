package httpinfra

import (
	domain "github.com/f0reth/Wirexa/internal/domain/http"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

// コンパイル時に domain.SidebarLayoutRepository を満たすことを検証
var _ domain.SidebarLayoutRepository = (*SidebarLayoutRepository)(nil)

// SidebarLayoutRepository は sidebar_layout.json の読み書きリポジトリ。
type SidebarLayoutRepository struct {
	path string
}

// NewSidebarLayoutRepository は指定パスの SidebarLayoutRepository を返す。
func NewSidebarLayoutRepository(path string) *SidebarLayoutRepository {
	return &SidebarLayoutRepository{path: path}
}

// Load はレイアウトファイルを読み込む。ファイルが存在しない場合や null の場合は空スライスを返す。
// JSON として解釈できない場合は cmn.ErrCorruptData を wrap して返し、
// 読み込み自体の失敗と区別できるようにする。
func (r *SidebarLayoutRepository) Load() ([]domain.SidebarEntry, error) {
	layout, _, err := infra.ReadJSONFile[[]domain.SidebarEntry](r.path)
	if err != nil {
		return nil, err
	}
	if layout == nil {
		layout = []domain.SidebarEntry{}
	}
	return layout, nil
}

// Save はレイアウトをファイルに書き込む。
// tmp ファイル経由の原子的置き換えで書き込み中断によるデータ破損を防ぐ。
func (r *SidebarLayoutRepository) Save(layout []domain.SidebarEntry) error {
	return infra.WriteJSONFile(r.path, layout, 0o600)
}

// Quarantine は壊れたレイアウトファイルを退避し、退避先のパスを返す。
func (r *SidebarLayoutRepository) Quarantine() (string, error) {
	return infra.QuarantineFile(r.path)
}
