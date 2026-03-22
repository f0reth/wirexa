// Package httpapp は HTTP ユースケース層を提供する。
package httpapp

import (
	"sort"
	"sync"

	"github.com/google/uuid"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// nodeEntry はツリー内のノードとその親ノードへの参照を保持する。
// parent が nil の場合はコレクションルート直下のアイテムを意味する。
type nodeEntry struct {
	node   *domain.TreeItem
	parent *domain.TreeItem
}

// CollectionService はコレクション管理ユースケースを提供する。
type CollectionService struct {
	repo      domain.CollectionRepository
	mu        sync.RWMutex
	cache     map[string]*domain.Collection
	nodeIndex map[string]map[string]*nodeEntry // collectionID → itemID → nodeEntry
}

// NewCollectionService は CollectionService を生成する。
func NewCollectionService(repo domain.CollectionRepository) *CollectionService {
	return &CollectionService{
		repo:      repo,
		cache:     make(map[string]*domain.Collection),
		nodeIndex: make(map[string]map[string]*nodeEntry),
	}
}

// Initialize はリポジトリからコレクションを読み込みキャッシュとノードインデックスを初期化する。
func (s *CollectionService) Initialize() error {
	cols, err := s.repo.Load()
	if err != nil {
		return err
	}
	s.mu.Lock()
	for i := range cols {
		c := cols[i]
		s.cache[c.ID] = &c
		idx := make(map[string]*nodeEntry)
		buildNodeIndex(idx, c.Items, nil)
		s.nodeIndex[c.ID] = idx
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
		Items: []*domain.TreeItem{},
	}
	if err := s.repo.Save(&c); err != nil {
		return domain.Collection{}, err
	}
	s.mu.Lock()
	s.cache[c.ID] = &c
	s.nodeIndex[c.ID] = make(map[string]*nodeEntry)
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
	delete(s.nodeIndex, id)
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
		s.nodeIndex[collectionID][item.ID] = &nodeEntry{node: item, parent: nil}
	} else {
		entry, ok := s.nodeIndex[collectionID][parentID]
		if !ok || entry.node.Type != domain.ItemTypeFolder {
			return nil, &domain.NotFoundError{Resource: "parent", ID: parentID}
		}
		entry.node.Children = append(entry.node.Children, item)
		s.nodeIndex[collectionID][item.ID] = &nodeEntry{node: item, parent: entry.node}
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
		s.nodeIndex[collectionID][item.ID] = &nodeEntry{node: item, parent: nil}
	} else {
		entry, ok := s.nodeIndex[collectionID][parentID]
		if !ok || entry.node.Type != domain.ItemTypeFolder {
			return nil, &domain.NotFoundError{Resource: "parent", ID: parentID}
		}
		entry.node.Children = append(entry.node.Children, item)
		s.nodeIndex[collectionID][item.ID] = &nodeEntry{node: item, parent: entry.node}
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

	entry, ok := s.nodeIndex[collectionID][req.ID]
	if !ok || entry.node.Type != domain.ItemTypeRequest {
		return &domain.NotFoundError{Resource: "request", ID: req.ID}
	}

	entry.node.Name = req.Name
	entry.node.Request = &req

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

	entry, ok := s.nodeIndex[collectionID][itemID]
	if !ok {
		return &domain.NotFoundError{Resource: "item", ID: itemID}
	}

	entry.node.Name = name
	if entry.node.Request != nil {
		entry.node.Request.Name = name
	}

	return s.repo.Save(c)
}

// DeleteItem はコレクションからアイテムを削除する。
func (s *CollectionService) DeleteItem(collectionID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &domain.NotFoundError{Resource: "collection", ID: collectionID}
	}

	entry, ok := s.nodeIndex[collectionID][itemID]
	if !ok {
		return &domain.NotFoundError{Resource: "item", ID: itemID}
	}

	if entry.parent == nil {
		for i, n := range c.Items {
			if n.ID == itemID {
				c.Items = append(c.Items[:i], c.Items[i+1:]...)
				break
			}
		}
	} else {
		for i, n := range entry.parent.Children {
			if n.ID == itemID {
				entry.parent.Children = append(entry.parent.Children[:i], entry.parent.Children[i+1:]...)
				break
			}
		}
	}

	removeSubtreeFromIndex(s.nodeIndex[collectionID], entry.node)

	return s.repo.Save(c)
}

// buildNodeIndex はツリーを1回走査してノードインデックスを構築する。
// parent が nil の場合はコレクションルート直下のアイテムを意味する。
func buildNodeIndex(index map[string]*nodeEntry, items []*domain.TreeItem, parent *domain.TreeItem) {
	for _, item := range items {
		index[item.ID] = &nodeEntry{node: item, parent: parent}
		if item.Type == domain.ItemTypeFolder {
			buildNodeIndex(index, item.Children, item)
		}
	}
}

// removeSubtreeFromIndex はノードとその全子孫をインデックスから削除する。
func removeSubtreeFromIndex(index map[string]*nodeEntry, node *domain.TreeItem) {
	delete(index, node.ID)
	for _, child := range node.Children {
		removeSubtreeFromIndex(index, child)
	}
}
