// Package mqttinfra は MQTT インフラストラクチャ層を提供する。
package mqttinfra

import (
	"encoding/json"
	"os"
	"path/filepath"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// コンパイル時に domain.ProfileRepository を満たすことを検証
var _ domain.ProfileRepository = (*JSONProfileRepository)(nil)

// JSONProfileRepository は JSON ファイルで MQTT プロファイルを永続化するリポジトリ。
type JSONProfileRepository struct {
	dir string
}

// NewJSONProfileRepository はディレクトリを作成してリポジトリを返す。
func NewJSONProfileRepository(dir string) (*JSONProfileRepository, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &JSONProfileRepository{dir: dir}, nil
}

// Load はディレクトリ内の全 JSON ファイルからプロファイルを読み込む。
func (r *JSONProfileRepository) Load() ([]domain.BrokerProfile, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var profiles []domain.BrokerProfile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var p domain.BrokerProfile
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// Save はプロファイルを JSON ファイルに書き込む。
func (r *JSONProfileRepository) Save(p *domain.BrokerProfile) error {
	data, err := json.MarshalIndent(p, "", "  ") //nolint:gosec // password フィールドは接続設定として意図的に保存する
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.dir, p.ID+".json"), data, 0o600)
}

// Delete はプロファイルの JSON ファイルを削除する。
func (r *JSONProfileRepository) Delete(id string) error {
	return os.Remove(filepath.Join(r.dir, id+".json"))
}
