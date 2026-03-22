// Package httpapp は HTTP ユースケース層を提供する。
package httpapp

import (
	"fmt"
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
func NewCollectionService(repo domain.CollectionRepository) *CollectionService {
	return &CollectionService{
		repo:  repo,
		cache: make(map[string]*domain.Collection),
	}
}

// Initialize はリポジトリからコレクションを読み込みキャッシュを初期化する。
func (s *CollectionService) Initialize() error {
	cols, err := s.repo.Load()
	if err != nil {
		return err
	}
	s.mu.Lock()
	for i := range cols {
		c := cols[i]
		s.cache[c.ID] = &c
	}
	s.mu.Unlock()
	return nil
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
		Items: []domain.TreeItem{},
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
		return fmt.Errorf("collection not found: %s", id)
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
	s.mu.RLock()
	c, ok := s.cache[id]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("collection not found: %s", id)
	}
	snapshot := *c
	s.mu.RUnlock()

	snapshot.Name = name
	if err := s.repo.Save(&snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[id] = &snapshot
	s.mu.Unlock()
	return nil
}

// AddFolder はコレクションにフォルダを追加する。
func (s *CollectionService) AddFolder(collectionID, parentID, name string) (*domain.TreeItem, error) {
	item := domain.TreeItem{
		Type:     domain.ItemTypeFolder,
		ID:       uuid.New().String(),
		Name:     name,
		Children: []domain.TreeItem{},
	}

	s.mu.RLock()
	c, ok := s.cache[collectionID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("collection not found: %s", collectionID)
	}
	snapshot := *c
	s.mu.RUnlock()

	if parentID == "" {
		snapshot.Items = append(snapshot.Items, item)
	} else if !addToParent(&snapshot.Items, parentID, item) {
		return nil, fmt.Errorf("parent not found: %s", parentID)
	}

	if err := s.repo.Save(&snapshot); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[collectionID] = &snapshot
	s.mu.Unlock()
	return &item, nil
}

// AddRequest はコレクションにリクエストを追加する。
func (s *CollectionService) AddRequest(collectionID, parentID string, req domain.HttpRequest) (*domain.TreeItem, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	item := domain.TreeItem{
		Type:     domain.ItemTypeRequest,
		ID:       req.ID,
		Name:     req.Name,
		Request:  &req,
		Children: []domain.TreeItem{},
	}

	s.mu.RLock()
	c, ok := s.cache[collectionID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("collection not found: %s", collectionID)
	}
	snapshot := *c
	s.mu.RUnlock()

	if parentID == "" {
		snapshot.Items = append(snapshot.Items, item)
	} else if !addToParent(&snapshot.Items, parentID, item) {
		return nil, fmt.Errorf("parent not found: %s", parentID)
	}

	if err := s.repo.Save(&snapshot); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[collectionID] = &snapshot
	s.mu.Unlock()
	return &item, nil
}

// UpdateRequest はコレクション内のリクエストを更新する。
func (s *CollectionService) UpdateRequest(collectionID string, req domain.HttpRequest) error {
	s.mu.RLock()
	c, ok := s.cache[collectionID]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("collection not found: %s", collectionID)
	}
	snapshot := *c
	s.mu.RUnlock()

	if !updateRequestInTree(&snapshot.Items, req) {
		return fmt.Errorf("request not found: %s", req.ID)
	}

	if err := s.repo.Save(&snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[collectionID] = &snapshot
	s.mu.Unlock()
	return nil
}

// RenameItem はコレクション内のアイテム名を変更する。
func (s *CollectionService) RenameItem(collectionID, itemID, name string) error {
	s.mu.RLock()
	c, ok := s.cache[collectionID]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("collection not found: %s", collectionID)
	}
	snapshot := *c
	s.mu.RUnlock()

	if !renameInTree(&snapshot.Items, itemID, name) {
		return fmt.Errorf("item not found: %s", itemID)
	}

	if err := s.repo.Save(&snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[collectionID] = &snapshot
	s.mu.Unlock()
	return nil
}

// DeleteItem はコレクションからアイテムを削除する。
func (s *CollectionService) DeleteItem(collectionID, itemID string) error {
	s.mu.RLock()
	c, ok := s.cache[collectionID]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("collection not found: %s", collectionID)
	}
	snapshot := *c
	s.mu.RUnlock()

	if !removeFromTree(&snapshot.Items, itemID) {
		return fmt.Errorf("item not found: %s", itemID)
	}

	if err := s.repo.Save(&snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[collectionID] = &snapshot
	s.mu.Unlock()
	return nil
}

// Tree helpers

func addToParent(items *[]domain.TreeItem, parentID string, child domain.TreeItem) bool {
	for i := range *items {
		if (*items)[i].ID == parentID && (*items)[i].Type == domain.ItemTypeFolder {
			(*items)[i].Children = append((*items)[i].Children, child)
			return true
		}
		if (*items)[i].Type == domain.ItemTypeFolder {
			if addToParent(&(*items)[i].Children, parentID, child) {
				return true
			}
		}
	}
	return false
}

func renameInTree(items *[]domain.TreeItem, id, name string) bool {
	for i := range *items {
		if (*items)[i].ID == id {
			(*items)[i].Name = name
			if (*items)[i].Request != nil {
				(*items)[i].Request.Name = name
			}
			return true
		}
		if (*items)[i].Type == domain.ItemTypeFolder {
			if renameInTree(&(*items)[i].Children, id, name) {
				return true
			}
		}
	}
	return false
}

func removeFromTree(items *[]domain.TreeItem, id string) bool {
	for i := range *items {
		if (*items)[i].ID == id {
			*items = append((*items)[:i], (*items)[i+1:]...)
			return true
		}
		if (*items)[i].Type == domain.ItemTypeFolder {
			if removeFromTree(&(*items)[i].Children, id) {
				return true
			}
		}
	}
	return false
}

func updateRequestInTree(items *[]domain.TreeItem, req domain.HttpRequest) bool {
	for i := range *items {
		if (*items)[i].ID == req.ID && (*items)[i].Type == domain.ItemTypeRequest {
			(*items)[i].Name = req.Name
			(*items)[i].Request = &req
			return true
		}
		if (*items)[i].Type == domain.ItemTypeFolder {
			if updateRequestInTree(&(*items)[i].Children, req) {
				return true
			}
		}
	}
	return false
}
