// Package store は永続化リポジトリをインメモリキャッシュで包む汎用ストアを提供する。
package store

import (
	"fmt"
	"sync"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// Repository は Load/Save/Delete を持つ永続化ポート。
// domain.ProfileRepository / TargetRepository / CollectionRepository が構造的に充足する。
type Repository[T any] interface {
	Load() ([]T, error)
	Save(*T) error
	Delete(id string) error
}

// CachedStore はリポジトリから全件ロードして map にキャッシュし、
// Save / Delete でリポジトリとキャッシュの両方を更新する汎用ストア。
// getID は T から ID を取り出し、setID は T に ID を書き込む (空 ID の自動採番用)。
type CachedStore[T any] struct {
	repo     Repository[T]
	getID    func(T) string
	setID    func(*T, string)
	items    map[string]T
	resource string // NotFoundError.Resource およびエラーメッセージ用 ("profile" / "target" など)
	mu       sync.RWMutex
}

// NewCachedStore はリポジトリから全件ロードして CachedStore を生成する。
func NewCachedStore[T any](
	resource string,
	repo Repository[T],
	getID func(T) string,
	setID func(*T, string),
) (*CachedStore[T], error) {
	loaded, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", resource, err)
	}
	items := make(map[string]T, len(loaded))
	for i := range loaded {
		items[getID(loaded[i])] = loaded[i]
	}
	return &CachedStore[T]{
		repo:     repo,
		resource: resource,
		getID:    getID,
		setID:    setID,
		items:    items,
	}, nil
}

// GetAll は全アイテムのコピーを返す。
func (s *CachedStore[T]) GetAll() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]T, 0, len(s.items))
	for k := range s.items {
		result = append(result, s.items[k])
	}
	return result
}

// Save はアイテムを保存（追加または更新）する。ID が空の場合は UUID を採番する。
// 採番後のアイテムを返す。
func (s *CachedStore[T]) Save(item T) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getID(item) == "" {
		s.setID(&item, uuid.NewString())
	}
	if err := s.repo.Save(&item); err != nil {
		var zero T
		return zero, fmt.Errorf("failed to save %s: %w", s.resource, err)
	}
	s.items[s.getID(item)] = item
	return item, nil
}

// Delete は ID でアイテムを削除する。存在しない ID の場合は NotFoundError を返す。
func (s *CachedStore[T]) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return &cmn.NotFoundError{Resource: s.resource, ID: id}
	}
	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete %s: %w", s.resource, err)
	}
	delete(s.items, id)
	return nil
}
