// Package httpapp は HTTP ユースケース層を提供する。
package httpapp

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// サイドバーレイアウトのエントリ種別。cmn.Resource* と値は同じだが、
// レイアウトの Kind フィールド用途を明示するため別名で保持する。
const (
	sidebarKindCollection = cmn.ResourceCollection
	sidebarKindItem       = cmn.ResourceItem
)

// CollectionService はコレクション管理ユースケースを提供する。
// サイドバーレイアウトの操作は SidebarLayoutService に委譲し、
// 自身はコレクションキャッシュ用のロック(mu)のみを保持する。
type CollectionService struct {
	repo   domain.CollectionRepository
	layout *SidebarLayoutService
	cache  map[string]*domain.Collection
	mu     sync.RWMutex
}

// NewCollectionService は CollectionService を生成する。
// コンストラクタ内でリポジトリからコレクションを読み込む。
func NewCollectionService(repo domain.CollectionRepository, layoutRepo domain.SidebarLayoutRepository) (*CollectionService, error) {
	svc := &CollectionService{
		repo:   repo,
		layout: NewSidebarLayoutService(layoutRepo),
		cache:  make(map[string]*domain.Collection),
	}
	cols, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load collections: %w", err)
	}
	for i := range cols {
		c := cols[i]
		normalizeItemForms(c.Items)
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

	if _, err := svc.GetSidebarLayout(); err != nil {
		return nil, fmt.Errorf("failed to initialize sidebar layout: %w", err)
	}

	return svc, nil
}

// normalizeItemForms はツリーを再帰的に走査し、各リクエストの form 系ボディを移行する。
// 読み込み直後にキャッシュ全体へ適用することで、GetCollections / GetRootItems /
// 送信のいずれの経路でも行が復元済みであることを保証する。
// __root__ もキャッシュに載るため、ルート直下のリクエストも対象になる。
func normalizeItemForms(items []*domain.TreeItem) {
	for _, item := range items {
		if item.Request != nil {
			item.Request.Body.NormalizeForms()
		}
		normalizeItemForms(item.Children)
	}
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

// CreateCollection は新規コレクションを作成する。名前が空の場合は ValidationError を返す。
func (s *CollectionService) CreateCollection(name string) (domain.Collection, error) {
	if strings.TrimSpace(name) == "" {
		return domain.Collection{}, &cmn.ValidationError{Field: "name", Message: cmn.MsgRequired}
	}
	c := domain.Collection{
		ID:    uuid.NewString(),
		Name:  name,
		Items: []*domain.TreeItem{},
	}
	if err := s.repo.Save(&c); err != nil {
		return domain.Collection{}, fmt.Errorf("failed to save collection: %w", err)
	}
	s.mu.Lock()
	s.cache[c.ID] = &c
	s.mu.Unlock()

	// レイアウトファイルに末尾エントリを追加する。
	if err := s.layout.Append(domain.SidebarEntry{Kind: sidebarKindCollection, ID: c.ID}); err != nil {
		return domain.Collection{}, fmt.Errorf("failed to update sidebar layout: %w", err)
	}
	return c, nil
}

// DeleteCollection は ID でコレクションを削除する。
func (s *CollectionService) DeleteCollection(id string) error {
	s.mu.RLock()
	if _, ok := s.cache[id]; !ok {
		s.mu.RUnlock()
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: id}
	}
	s.mu.RUnlock()

	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()

	if err := s.layout.Remove(sidebarKindCollection, id); err != nil {
		return fmt.Errorf("failed to update sidebar layout: %w", err)
	}
	return nil
}

// RenameCollection はコレクション名を変更する。
func (s *CollectionService) RenameCollection(id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cache[id]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: id}
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
		ID:       uuid.NewString(),
		Name:     name,
		Children: []*domain.TreeItem{},
	}
	if err := s.addItem(collectionID, parentID, item); err != nil {
		return nil, err
	}
	return item, nil
}

// AddRequest はコレクションにリクエストを追加する。
func (s *CollectionService) AddRequest(collectionID, parentID string, req domain.HTTPRequest) (*domain.TreeItem, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	item := &domain.TreeItem{
		Type:     domain.ItemTypeRequest,
		ID:       req.ID,
		Name:     req.Name,
		Request:  &req,
		Children: []*domain.TreeItem{},
	}
	if err := s.addItem(collectionID, parentID, item); err != nil {
		return nil, err
	}
	return item, nil
}

// addItem は組み立て済みの TreeItem をコレクションへ追加し、必要ならサイドバーにも反映する。
// キャッシュ更新はロック区間内で完結させ、レイアウト追加はロック解放後に委譲する
// (レイアウトロックとのネストによるデッドロックを避けるため)。
func (s *CollectionService) addItem(collectionID, parentID string, item *domain.TreeItem) error {
	if err := s.appendItemToCache(collectionID, parentID, item); err != nil {
		return err
	}
	// root コレクションのルート直下に追加した場合、サイドバーレイアウトにも追加する。
	if collectionID == domain.RootCollectionID && parentID == "" {
		if err := s.layout.Append(domain.SidebarEntry{Kind: sidebarKindItem, ID: item.ID}); err != nil {
			return fmt.Errorf("failed to update sidebar layout: %w", err)
		}
	}
	return nil
}

// appendItemToCache はロックを取り、キャッシュへ item を追加して永続化する。
func (s *CollectionService) appendItemToCache(collectionID, parentID string, item *domain.TreeItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: collectionID}
	}
	if !c.AppendItem(parentID, item) {
		return &cmn.NotFoundError{Resource: cmn.ResourceParent, ID: parentID}
	}
	if err := s.repo.Save(c); err != nil {
		return fmt.Errorf("failed to save collection: %w", err)
	}
	return nil
}

// UpdateRequest はコレクション内のリクエストを更新する。
func (s *CollectionService) UpdateRequest(collectionID string, req domain.HTTPRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: collectionID}
	}

	node, _, ok := c.FindNode(req.ID)
	if !ok || node.Type != domain.ItemTypeRequest {
		return &cmn.NotFoundError{Resource: cmn.ResourceRequest, ID: req.ID}
	}

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
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: collectionID}
	}

	node, _, ok := c.FindNode(itemID)
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindItem, ID: itemID}
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

// MoveItem はアイテムをコレクション内外・別の親・位置へ移動する。
// sourceCollectionID と targetCollectionID が同一の場合は同一コレクション内移動。
// position は削除後の挿入先インデックス。-1 または範囲外の場合は末尾に追加する。
func (s *CollectionService) MoveItem(sourceCollectionID, itemID, targetCollectionID, targetParentID string, position int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, ok := s.cache[sourceCollectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: sourceCollectionID}
	}
	dst, ok := s.cache[targetCollectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: targetCollectionID}
	}

	item, _, ok := src.FindNode(itemID)
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindItem, ID: itemID}
	}

	// 挿入先を RemoveNode の前に検証する。失敗時はソースを一切変更しない (#6)。
	if targetParentID != "" {
		parent, _, ok := dst.FindNode(targetParentID)
		if !ok || parent.Type != domain.ItemTypeFolder {
			return &cmn.NotFoundError{Resource: cmn.ResourceParent, ID: targetParentID}
		}
		// 自身のサブツリー内へは移動できない（RemoveNode で親ごと消えるため）。
		if item.Contains(targetParentID) {
			return &cmn.ValidationError{Field: cmn.ResourceParent, Message: "cannot move an item into its own subtree"}
		}
	}

	// 同一コレクション内移動の場合、削除前に挿入先インデックスを補正する。
	if sourceCollectionID == targetCollectionID && position > 0 {
		var targetItems []*domain.TreeItem
		if targetParentID == "" {
			targetItems = dst.Items
		} else {
			targetNode, _, ok := dst.FindNode(targetParentID)
			if ok {
				targetItems = targetNode.Children
			}
		}
		for i, n := range targetItems {
			if n.ID == itemID && i < position {
				position--
				break
			}
		}
	}

	src.RemoveNode(itemID)

	if !dst.InsertItem(targetParentID, item, position) {
		return &cmn.NotFoundError{Resource: cmn.ResourceParent, ID: targetParentID}
	}

	if sourceCollectionID != targetCollectionID {
		if err := s.repo.Save(src); err != nil {
			return fmt.Errorf("failed to save source collection: %w", err)
		}
	}
	if err := s.repo.Save(dst); err != nil {
		return fmt.Errorf("failed to save target collection: %w", err)
	}
	return nil
}

// GetSidebarLayout はサイドバーレイアウトを返す。
// ファイルが存在しない場合は既存コレクションを名前順で並べた初期値を生成して保存する。
// 初期値計算（mu.RLock）はレイアウトロックを保持していない状態で行われるため、
// 2つのロックがネストせずデッドロックの危険がない。
func (s *CollectionService) GetSidebarLayout() ([]domain.SidebarEntry, error) {
	return s.layout.GetOrInit(s.computeInitialLayout)
}

// computeInitialLayout はコレクション順とルートアイテムからレイアウト初期値を生成する。
func (s *CollectionService) computeInitialLayout() []domain.SidebarEntry {
	s.mu.RLock()
	cols := make([]*domain.Collection, 0, len(s.cache))
	for _, c := range s.cache {
		if c.ID != domain.RootCollectionID {
			cols = append(cols, c)
		}
	}
	rootItems := s.cache[domain.RootCollectionID]
	s.mu.RUnlock()

	sort.Slice(cols, func(i, j int) bool {
		return cols[i].Name < cols[j].Name
	})

	initial := make([]domain.SidebarEntry, 0, len(cols))
	for _, c := range cols {
		initial = append(initial, domain.SidebarEntry{Kind: sidebarKindCollection, ID: c.ID})
	}
	if rootItems != nil {
		for _, item := range rootItems.Items {
			initial = append(initial, domain.SidebarEntry{Kind: sidebarKindItem, ID: item.ID})
		}
	}
	return initial
}

// MoveSidebarEntry はサイドバー上のエントリを指定位置に移動する。
func (s *CollectionService) MoveSidebarEntry(kind, id string, position int) error {
	return s.layout.Move(kind, id, position)
}

// MoveItemToSidebar はアイテムを指定コレクションから __root__ へ移動し、
// サイドバーレイアウトの指定位置に挿入する。
// コレクション更新（mu）を終えてロックを解放した後にレイアウト挿入を委譲する。
// 2つの操作の間に他ゴルーチンの割り込みが入り得る点は許容している。
func (s *CollectionService) MoveItemToSidebar(sourceCollectionID, itemID string, sidebarPosition int) error {
	s.mu.Lock()

	src, ok := s.cache[sourceCollectionID]
	if !ok {
		s.mu.Unlock()
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: sourceCollectionID}
	}
	root, ok := s.cache[domain.RootCollectionID]
	if !ok {
		s.mu.Unlock()
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: domain.RootCollectionID}
	}

	item, _, ok := src.FindNode(itemID)
	if !ok {
		s.mu.Unlock()
		return &cmn.NotFoundError{Resource: sidebarKindItem, ID: itemID}
	}

	src.RemoveNode(itemID)
	root.AppendItem("", item)

	if err := s.repo.Save(src); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to save source collection: %w", err)
	}
	if err := s.repo.Save(root); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to save root collection: %w", err)
	}
	s.mu.Unlock()

	return s.layout.InsertItem(itemID, sidebarPosition)
}

// DeleteItem はコレクションからアイテムをサブツリーごと削除する。
func (s *CollectionService) DeleteItem(collectionID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: collectionID}
	}

	if !c.RemoveNode(itemID) {
		return &cmn.NotFoundError{Resource: sidebarKindItem, ID: itemID}
	}

	if err := s.repo.Save(c); err != nil {
		return fmt.Errorf("failed to save collection: %w", err)
	}

	// root コレクションのアイテムはサイドバーレイアウトからも削除する。
	if collectionID == domain.RootCollectionID {
		if err := s.layout.Remove(sidebarKindItem, itemID); err != nil {
			return fmt.Errorf("failed to update sidebar layout: %w", err)
		}
	}
	return nil
}
