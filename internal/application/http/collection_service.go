// Package httpapp は HTTP ユースケース層を提供する。
package httpapp

import (
	"sort"
	"sync"

	"github.com/google/uuid"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// CollectionService はコレクション管理ユースケースを提供する。
type CollectionService struct {
	repo  domain.CollectionRepository
	mu    sync.RWMutex
	cache map[string]*domain.Collection
}

// NewCollectionService は CollectionService を生成する。
// コンストラクタ内でリポジトリからコレクションを読み込む。
func NewCollectionService(repo domain.CollectionRepository) (*CollectionService, error) {
	svc := &CollectionService{
		repo:  repo,
		cache: make(map[string]*domain.Collection),
	}
	cols, err := repo.Load()
	if err != nil {
		return nil, err
	}
	for i := range cols {
		c := cols[i]
		svc.cache[c.ID] = &c
	}
	return svc, nil
}

// GetCollections は全コレクションを名前順で返す。
func (s *CollectionService) GetCollections() []domain.Collection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Collection, 0, len(s.cache))
	for _, c := range s.cache {
		result = append(result, *c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// CreateCollection は新規コレクションを作成する。
func (s *CollectionService) CreateCollection(name string) (domain.Collection, error) {
	c := domain.Collection{
		ID:    uuid.New().String(),
		Name:  name,
		Items: []*domain.TreeItem{},
	}
	if err := s.repo.Save(&c); err != nil {
		return domain.Collection{}, err
	}
	s.mu.Lock()
	s.cache[c.ID] = &c
	s.mu.Unlock()
	return c, nil
}

// DeleteCollection は ID でコレクションを削除する。
func (s *CollectionService) DeleteCollection(id string) error {
	s.mu.RLock()
	if _, ok := s.cache[id]; !ok {
		s.mu.RUnlock()
		return &domain.NotFoundError{Resource: "collection", ID: id}
	}
	s.mu.RUnlock()

	if err := s.repo.Delete(id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()
	return nil
}

// RenameCollection はコレクション名を変更する。
func (s *CollectionService) RenameCollection(id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cache[id]
	if !ok {
		return &domain.NotFoundError{Resource: "collection", ID: id}
	}
	c.Name = name
	return s.repo.Save(c)
}

// AddFolder はコレクションにフォルダを追加する。
func (s *CollectionService) AddFolder(collectionID, parentID, name string) (*domain.TreeItem, error) {
	item := &domain.TreeItem{
		Type:     domain.ItemTypeFolder,
		ID:       uuid.New().String(),
		Name:     name,
		Children: []*domain.TreeItem{},
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return nil, &domain.NotFoundError{Resource: "collection", ID: collectionID}
	}

	if parentID == "" {
		c.Items = append(c.Items, item)
	} else {
		parent, _, ok := c.FindNode(parentID)
		if !ok || parent.Type != domain.ItemTypeFolder {
			return nil, &domain.NotFoundError{Resource: "parent", ID: parentID}
		}
		parent.Children = append(parent.Children, item)
	}

	if err := s.repo.Save(c); err != nil {
		return nil, err
	}
	return item, nil
}

// AddRequest はコレクションにリクエストを追加する。
func (s *CollectionService) AddRequest(collectionID, parentID string, req domain.HttpRequest) (*domain.TreeItem, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	item := &domain.TreeItem{
		Type:     domain.ItemTypeRequest,
		ID:       req.ID,
		Name:     req.Name,
		Request:  &req,
		Children: []*domain.TreeItem{},
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return nil, &domain.NotFoundError{Resource: "collection", ID: collectionID}
	}

	if parentID == "" {
		c.Items = append(c.Items, item)
	} else {
		parent, _, ok := c.FindNode(parentID)
		if !ok || parent.Type != domain.ItemTypeFolder {
			return nil, &domain.NotFoundError{Resource: "parent", ID: parentID}
		}
		parent.Children = append(parent.Children, item)
	}

	if err := s.repo.Save(c); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateRequest はコレクション内のリクエストを更新する。
func (s *CollectionService) UpdateRequest(collectionID string, req domain.HttpRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &domain.NotFoundError{Resource: "collection", ID: collectionID}
	}

	node, _, ok := c.FindNode(req.ID)
	if !ok || node.Type != domain.ItemTypeRequest {
		return &domain.NotFoundError{Resource: "request", ID: req.ID}
	}

	node.Name = req.Name
	node.Request = &req

	return s.repo.Save(c)
}

// RenameItem はコレクション内のアイテム名を変更する。
func (s *CollectionService) RenameItem(collectionID, itemID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &domain.NotFoundError{Resource: "collection", ID: collectionID}
	}

	node, _, ok := c.FindNode(itemID)
	if !ok {
		return &domain.NotFoundError{Resource: "item", ID: itemID}
	}

	node.Name = name
	if node.Request != nil {
		node.Request.Name = name
	}

	return s.repo.Save(c)
}

// DeleteItem はコレクションからアイテムをサブツリーごと削除する。
func (s *CollectionService) DeleteItem(collectionID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &domain.NotFoundError{Resource: "collection", ID: collectionID}
	}

	if !c.RemoveNode(itemID) {
		return &domain.NotFoundError{Resource: "item", ID: itemID}
	}

	return s.repo.Save(c)
}
