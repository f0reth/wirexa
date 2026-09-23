package udpinfra

import (
	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

var _ domain.TargetRepository = (*TargetRepository)(nil)

// TargetRepository は UDP 送信先プリセットを JSON ファイルへ永続化する。
//
// ディスクへは永続化 DTO (storedUDPTarget) を書き、domain 型の json タグ (RPC の配線形式) を
// 変えても保存形式が変わらないようにする。変換はこの境界 1 か所で行う。
type TargetRepository struct {
	store *infra.JSONStore[storedUDPTarget]
}

// NewTargetRepository はディレクトリを作成して TargetRepository を返す。
// logger は破損ファイルの退避記録に使う。nil の場合は記録しない。
func NewTargetRepository(dir string, logger cmn.Logger) (*TargetRepository, error) {
	store, err := infra.NewJSONStore(dir, func(t *storedUDPTarget) string { return t.ID })
	if err != nil {
		return nil, err
	}
	if logger != nil {
		store.SetLogger(logger)
	}
	return &TargetRepository{store: store}, nil
}

// Load は全ターゲットを読み込み、domain 型へ変換して返す。
func (r *TargetRepository) Load() ([]domain.UDPTarget, error) {
	stored, err := r.store.Load()
	if err != nil {
		return nil, err
	}
	out := make([]domain.UDPTarget, 0, len(stored))
	for i := range stored {
		out = append(out, stored[i].toDomain())
	}
	return out, nil
}

// Save はターゲットを永続化 DTO へ変換して保存する。
func (r *TargetRepository) Save(t *domain.UDPTarget) error {
	s := newStoredUDPTarget(t)
	return r.store.Save(&s)
}

// Delete はターゲットのファイルを削除する。
func (r *TargetRepository) Delete(id string) error {
	return r.store.Delete(id)
}

// storedUDPTarget は UDP 送信先プリセットの永続化 DTO。
// フィールドと JSON 名は既存ファイルの形式 (testdata/target.golden.json) に揃える。
// domain 型は埋め込まない。
type storedUDPTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

func newStoredUDPTarget(t *domain.UDPTarget) storedUDPTarget {
	return storedUDPTarget{ID: t.ID, Name: t.Name, Host: t.Host, Port: t.Port}
}

func (s *storedUDPTarget) toDomain() domain.UDPTarget {
	return domain.UDPTarget{ID: s.ID, Name: s.Name, Host: s.Host, Port: s.Port}
}
