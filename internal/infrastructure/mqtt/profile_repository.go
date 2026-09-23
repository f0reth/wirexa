package mqttinfra

import (
	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

var _ domain.ProfileRepository = (*ProfileRepository)(nil)

// ProfileRepository は MQTT ブローカープロファイルを JSON ファイルへ永続化する。
//
// ディスクへは永続化 DTO (storedBrokerProfile) を書き、domain 型の json タグ (RPC の配線形式) を
// 変えても保存形式が変わらないようにする。変換はこの境界 1 か所で行う。
type ProfileRepository struct {
	store *infra.JSONStore[storedBrokerProfile]
}

// NewProfileRepository はディレクトリを作成して ProfileRepository を返す。
// logger は破損ファイルの退避記録に使う。nil の場合は記録しない。
func NewProfileRepository(dir string, logger cmn.Logger) (*ProfileRepository, error) {
	store, err := infra.NewJSONStore(dir, func(p *storedBrokerProfile) string { return p.ID })
	if err != nil {
		return nil, err
	}
	if logger != nil {
		store.SetLogger(logger)
	}
	return &ProfileRepository{store: store}, nil
}

// Load は全プロファイルを読み込み、domain 型へ変換して返す。
func (r *ProfileRepository) Load() ([]domain.BrokerProfile, error) {
	stored, err := r.store.Load()
	if err != nil {
		return nil, err
	}
	out := make([]domain.BrokerProfile, 0, len(stored))
	for i := range stored {
		out = append(out, stored[i].toDomain())
	}
	return out, nil
}

// Save はプロファイルを永続化 DTO へ変換して保存する。
func (r *ProfileRepository) Save(p *domain.BrokerProfile) error {
	s := newStoredBrokerProfile(p)
	return r.store.Save(&s)
}

// Delete はプロファイルのファイルを削除する。
func (r *ProfileRepository) Delete(id string) error {
	return r.store.Delete(id)
}

// storedBrokerProfile は MQTT ブローカープロファイルの永続化 DTO。
// フィールドと JSON 名は既存ファイルの形式 (testdata/profile.golden.json) に揃える。
// domain 型は埋め込まない。
type storedBrokerProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"useTls"`
}

func newStoredBrokerProfile(p *domain.BrokerProfile) storedBrokerProfile {
	return storedBrokerProfile{
		ID:       p.ID,
		Name:     p.Name,
		Broker:   p.Broker,
		ClientID: p.ClientID,
		Username: p.Username,
		Password: p.Password,
		UseTLS:   p.UseTLS,
	}
}

func (s *storedBrokerProfile) toDomain() domain.BrokerProfile {
	return domain.BrokerProfile{
		ID:       s.ID,
		Name:     s.Name,
		Broker:   s.Broker,
		ClientID: s.ClientID,
		Username: s.Username,
		Password: s.Password,
		UseTLS:   s.UseTLS,
	}
}
