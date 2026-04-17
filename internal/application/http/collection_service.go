// Package httpapp は HTTP ユースケース層を提供する。
package httpapp

import (
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
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
		return nil, fmt.Errorf("failed to load collections: %w", err)
	}
	for i := range cols {
		c := cols[i]
		svc.cache[c.ID] = &c
	}
	if _, ok := svc.cache[domain.RootCollectionID]; !ok {
		root := &domain.Collection{
			ID:    domain.RootCollectionID,
			Name:  domain.RootCollectionID,
			Items: []*domain.TreeItem{},
		}
		if err := repo.Save(root); err != nil {
			return nil, fmt.Errorf("failed to create root collection: %w", err)
		}
		svc.cache[root.ID] = root
	}
	return svc, nil
}

// GetCollections は全コレクションを名前順で返す（__root__ を除く）。
func (s *CollectionService) GetCollections() []domain.Collection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Collection, 0, len(s.cache))
	for _, c := range s.cache {
		if c.ID == domain.RootCollectionID {
			continue
		}
		result = append(result, *c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// GetRootItems はルートコレクション（__root__）のアイテム一覧を返す。
func (s *CollectionService) GetRootItems() []*domain.TreeItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	root, ok := s.cache[domain.RootCollectionID]
	if !ok {
		return []*domain.TreeItem{}
	}
	return root.Items
}

// CreateCollection は新規コレクションを作成する。
func (s *CollectionService) CreateCollection(name string) (domain.Collection, error) {
	c := domain.Collection{
		ID:    uuid.New().String(),
		Name:  name,
		Items: []*domain.TreeItem{},
	}
	if err := s.repo.Save(&c); err != nil {
		return domain.Collection{}, fmt.Errorf("failed to save collection: %w", err)
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
		return &cmn.NotFoundError{Resource: "collection", ID: id}
	}
	s.mu.RUnlock()

	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
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
		return &cmn.NotFoundError{Resource: "collection", ID: id}
	}
	c.Name = name
	if err := s.repo.Save(c); err != nil {
		return fmt.Errorf("failed to save collection: %w", err)
	}
	return nil
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
		return nil, &cmn.NotFoundError{Resource: "collection", ID: collectionID}
	}

	if parentID == "" {
		c.Items = append(c.Items, item)
	} else {
		parent, _, ok := c.FindNode(parentID)
		if !ok || parent.Type != domain.ItemTypeFolder {
			return nil, &cmn.NotFoundError{Resource: "parent", ID: parentID}
		}
		parent.Children = append(parent.Children, item)
	}

	if err := s.repo.Save(c); err != nil {
		return nil, fmt.Errorf("failed to save collection: %w", err)
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
		return nil, &cmn.NotFoundError{Resource: "collection", ID: collectionID}
	}

	if parentID == "" {
		c.Items = append(c.Items, item)
	} else {
		parent, _, ok := c.FindNode(parentID)
		if !ok || parent.Type != domain.ItemTypeFolder {
			return nil, &cmn.NotFoundError{Resource: "parent", ID: parentID}
		}
		parent.Children = append(parent.Children, item)
	}

	if err := s.repo.Save(c); err != nil {
		return nil, fmt.Errorf("failed to save collection: %w", err)
	}
	return item, nil
}

// UpdateRequest はコレクション内のリクエストを更新する。
func (s *CollectionService) UpdateRequest(collectionID string, req domain.HttpRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: "collection", ID: collectionID}
	}

	node, _, ok := c.FindNode(req.ID)
	if !ok || node.Type != domain.ItemTypeRequest {
		return &cmn.NotFoundError{Resource: "request", ID: req.ID}
	}

	// 名前の変更は RenameItem が担当するため、ここでは既存の名前を維持する
	req.Name = node.Name
	node.Request = &req

	if err := s.repo.Save(c); err != nil {
		return fmt.Errorf("failed to save collection: %w", err)
	}
	return nil
}

// RenameItem はコレクション内のアイテム名を変更する。
func (s *CollectionService) RenameItem(collectionID, itemID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: "collection", ID: collectionID}
	}

	node, _, ok := c.FindNode(itemID)
	if !ok {
		return &cmn.NotFoundError{Resource: "item", ID: itemID}
	}

	node.Name = name
	if node.Request != nil {
		node.Request.Name = name
	}

	if err := s.repo.Save(c); err != nil {
		return fmt.Errorf("failed to save collection: %w", err)
	}
	return nil
}

// MoveItem はアイテムを同一親内で位置変更する。
// targetParentID は現在の親 ID と一致している必要がある。
// position は削除後の挿入先インデックス。-1 または範囲外の場合は末尾に追加する。
func (s *CollectionService) MoveItem(collectionID, itemID, targetParentID string, position int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: "collection", ID: collectionID}
	}

	item, origParent, ok := c.FindNode(itemID)
	if !ok {
		return &cmn.NotFoundError{Resource: "item", ID: itemID}
	}

	// 同一親への移動のみ許可する。
	origParentID := ""
	if origParent != nil {
		origParentID = origParent.ID
	}
	if origParentID != targetParentID {
		return &cmn.ValidationError{Message: "cannot move item to a different folder"}
	}

	// 元インデックスが挿入位置より前の場合、削除後にインデックスがずれるので補正する。
	if position > 0 {
		var origItems []*domain.TreeItem
		if origParent == nil {
			origItems = c.Items
		} else {
			origItems = origParent.Children
		}
		for i, n := range origItems {
			if n.ID == itemID && i < position {
				position--
				break
			}
		}
	}

	c.RemoveNode(itemID)

	if targetParentID == "" {
		c.Items = insertAt(c.Items, item, position)
	} else {
		origParent.Children = insertAt(origParent.Children, item, position)
	}

	if err := s.repo.Save(c); err != nil {
		return fmt.Errorf("failed to save collection: %w", err)
	}
	return nil
}

// insertAt はスライスの指定インデックスにアイテムを挿入する。
// position が負または範囲外の場合は末尾に追加する。
func insertAt(items []*domain.TreeItem, item *domain.TreeItem, position int) []*domain.TreeItem {
	if position < 0 || position >= len(items) {
		return append(items, item)
	}
	items = append(items, nil)
	copy(items[position+1:], items[position:])
	items[position] = item
	return items
}

// DeleteItem はコレクションからアイテムをサブツリーごと削除する。
func (s *CollectionService) DeleteItem(collectionID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: "collection", ID: collectionID}
	}

	if !c.RemoveNode(itemID) {
		return &cmn.NotFoundError{Resource: "item", ID: itemID}
	}

	if err := s.repo.Save(c); err != nil {
		return fmt.Errorf("failed to save collection: %w", err)
	}
	return nil
}
