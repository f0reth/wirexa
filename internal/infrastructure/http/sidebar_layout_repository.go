package httpinfra

import (
	"encoding/json"
	"errors"
	"os"

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

// Load はレイアウトファイルを読み込む。ファイルが存在しない場合は空スライスを返す。
func (r *SidebarLayoutRepository) Load() ([]domain.SidebarEntry, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domain.SidebarEntry{}, nil
		}
		return nil, err
	}
	var layout []domain.SidebarEntry
	if err := json.Unmarshal(data, &layout); err != nil {
		return nil, err
	}
	return layout, nil
}

// Save はレイアウトをファイルに書き込む。
// tmp ファイル経由の原子的置き換えで書き込み中断によるデータ破損を防ぐ。
func (r *SidebarLayoutRepository) Save(layout []domain.SidebarEntry) error {
	return infra.WriteJSONFile(r.path, layout, 0o600)
}
