package httpinfra

import (
	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
)

var _ domain.CollectionRepository = (*CollectionRepository)(nil)

// CollectionRepository は HTTP コレクションを JSON ファイルへ永続化する。
//
// ディスクへは永続化 DTO (storedCollection) を書く。
// storedCollection は session token と実パスを表現できないので、どの保存経路からも永続化されない。
// 変換は deep copy で、呼び出し元の domain.Collection は変更しない。
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

// Exists はコレクションのファイルが存在するかを返す。読み込みで読み飛ばしたファイルも含む。
func (r *CollectionRepository) Exists(id string) (bool, error) {
	return r.store.Exists(id)
}
