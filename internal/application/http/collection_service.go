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
	repo       domain.CollectionRepository
	layoutRepo domain.SidebarLayoutRepository
	mu         sync.RWMutex
	cache      map[string]*domain.Collection
}

// NewCollectionService は CollectionService を生成する。
// コンストラクタ内でリポジトリからコレクションを読み込む。
func NewCollectionService(repo domain.CollectionRepository, layoutRepo domain.SidebarLayoutRepository) (*CollectionService, error) {
	svc := &CollectionService{
		repo:       repo,
		layoutRepo: layoutRepo,
		cache:      make(map[string]*domain.Collection),
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

	// 既存データに Order がない場合（ゼロ値が集中する）は名前順で振り直す。
	nonRoot := make([]*domain.Collection, 0)
	for _, c := range svc.cache {
		if c.ID != domain.RootCollectionID {
			nonRoot = append(nonRoot, c)
		}
	}
	allZero := true
	for _, c := range nonRoot {
		if c.Order != 0 {
			allZero = false
			break
		}
	}
	if allZero && len(nonRoot) > 0 {
		sort.Slice(nonRoot, func(i, j int) bool { return nonRoot[i].Name < nonRoot[j].Name })
		for i, c := range nonRoot {
			c.Order = i
			if err := repo.Save(c); err != nil {
				return nil, fmt.Errorf("failed to initialize collection order: %w", err)
			}
		}
	}

	return svc, nil
}

// GetCollections は全コレクションを Order 順で返す（__root__ を除く）。
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
		if result[i].Order != result[j].Order {
			return result[i].Order < result[j].Order
		}
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
	s.mu.RLock()
	maxOrder := 0
	for _, c := range s.cache {
		if c.ID != domain.RootCollectionID && c.Order >= maxOrder {
			maxOrder = c.Order + 1
		}
	}
	s.mu.RUnlock()

	c := domain.Collection{
		ID:    uuid.New().String(),
		Name:  name,
		Items: []*domain.TreeItem{},
		Order: maxOrder,
	}
	if err := s.repo.Save(&c); err != nil {
		return domain.Collection{}, fmt.Errorf("failed to save collection: %w", err)
	}
	s.mu.Lock()
	s.cache[c.ID] = &c
	s.mu.Unlock()

	// レイアウトファイルに末尾エントリを追加する。
	if err := s.appendLayoutEntry(domain.SidebarEntry{Kind: "collection", ID: c.ID}); err != nil {
		return domain.Collection{}, fmt.Errorf("failed to update sidebar layout: %w", err)
	}
	return c, nil
}

// appendLayoutEntry はレイアウトの末尾にエントリを追加する。
// レイアウトファイルが存在しない場合は先に GetSidebarLayout で初期化する。
func (s *CollectionService) appendLayoutEntry(entry domain.SidebarEntry) error {
	layout, err := s.GetSidebarLayout()
	if err != nil {
		return err
	}
	layout = append(layout, entry)
	return s.layoutRepo.Save(layout)
}

// removeLayoutEntry はレイアウトから指定 kind+ID のエントリを削除する。
func (s *CollectionService) removeLayoutEntry(kind, id string) error {
	layout, err := s.layoutRepo.Load()
	if err != nil {
		return err
	}
	n := 0
	for _, e := range layout {
		if e.Kind == kind && e.ID == id {
			continue
		}
		layout[n] = e
		n++
	}
	return s.layoutRepo.Save(layout[:n])
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

	if err := s.removeLayoutEntry("collection", id); err != nil {
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
		return &cmn.NotFoundError{Resource: "collection", ID: id}
	}
	c.Name = name
	if err := s.repo.Save(c); err != nil {
		return fmt.Errorf("failed to save collection: %w", err)
	}
	return nil
}

// MoveCollection はコレクションを指定の位置に並び替える。
// position は 0 始まりの挿入先インデックス。
func (s *CollectionService) MoveCollection(collectionID string, position int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.cache[collectionID]; !ok {
		return &cmn.NotFoundError{Resource: "collection", ID: collectionID}
	}

	// Order 順で並べたスライスを作る。
	cols := make([]*domain.Collection, 0, len(s.cache))
	for _, c := range s.cache {
		if c.ID != domain.RootCollectionID {
			cols = append(cols, c)
		}
	}
	sort.Slice(cols, func(i, j int) bool {
		if cols[i].Order != cols[j].Order {
			return cols[i].Order < cols[j].Order
		}
		return cols[i].Name < cols[j].Name
	})

	// 対象を現在位置から削除。
	srcIdx := -1
	for i, c := range cols {
		if c.ID == collectionID {
			srcIdx = i
			break
		}
	}
	item := cols[srcIdx]
	cols = append(cols[:srcIdx], cols[srcIdx+1:]...)

	// 指定位置に挿入。
	if position < 0 || position >= len(cols) {
		cols = append(cols, item)
	} else {
		cols = append(cols, nil)
		copy(cols[position+1:], cols[position:])
		cols[position] = item
	}

	// Order を振り直して保存。
	for i, c := range cols {
		c.Order = i
		if err := s.repo.Save(c); err != nil {
			return fmt.Errorf("failed to save collection order: %w", err)
		}
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

	// root コレクションのルート直下に追加した場合、サイドバーレイアウトにも追加する。
	if collectionID == domain.RootCollectionID && parentID == "" {
		if err := s.appendLayoutEntry(domain.SidebarEntry{Kind: "item", ID: item.ID}); err != nil {
			return nil, fmt.Errorf("failed to update sidebar layout: %w", err)
		}
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

	// root コレクションのルート直下に追加した場合、サイドバーレイアウトにも追加する。
	if collectionID == domain.RootCollectionID && parentID == "" {
		if err := s.appendLayoutEntry(domain.SidebarEntry{Kind: "item", ID: item.ID}); err != nil {
			return nil, fmt.Errorf("failed to update sidebar layout: %w", err)
		}
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

// MoveItem はアイテムをコレクション内外・別の親・位置へ移動する。
// sourceCollectionID と targetCollectionID が同一の場合は同一コレクション内移動。
// position は削除後の挿入先インデックス。-1 または範囲外の場合は末尾に追加する。
func (s *CollectionService) MoveItem(sourceCollectionID, itemID, targetCollectionID, targetParentID string, position int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, ok := s.cache[sourceCollectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: "collection", ID: sourceCollectionID}
	}
	dst, ok := s.cache[targetCollectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: "collection", ID: targetCollectionID}
	}

	item, _, ok := src.FindNode(itemID)
	if !ok {
		return &cmn.NotFoundError{Resource: "item", ID: itemID}
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

	if targetParentID == "" {
		dst.Items = insertAt(dst.Items, item, position)
	} else {
		targetNode, _, ok := dst.FindNode(targetParentID)
		if !ok || targetNode.Type != domain.ItemTypeFolder {
			return &cmn.NotFoundError{Resource: "parent", ID: targetParentID}
		}
		targetNode.Children = insertAt(targetNode.Children, item, position)
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
// ファイルが存在しない場合は既存コレクションを Order 順で並べた初期値を生成して保存する。
func (s *CollectionService) GetSidebarLayout() ([]domain.SidebarEntry, error) {
	layout, err := s.layoutRepo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load sidebar layout: %w", err)
	}
	if len(layout) > 0 {
		return layout, nil
	}

	// 初回: 既存コレクションを Order 順で並べて初期レイアウトを生成する。
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
		if cols[i].Order != cols[j].Order {
			return cols[i].Order < cols[j].Order
		}
		return cols[i].Name < cols[j].Name
	})

	layout = make([]domain.SidebarEntry, 0, len(cols))
	for _, c := range cols {
		layout = append(layout, domain.SidebarEntry{Kind: "collection", ID: c.ID})
	}
	if rootItems != nil {
		for _, item := range rootItems.Items {
			layout = append(layout, domain.SidebarEntry{Kind: "item", ID: item.ID})
		}
	}

	if err := s.layoutRepo.Save(layout); err != nil {
		return nil, fmt.Errorf("failed to save initial sidebar layout: %w", err)
	}
	return layout, nil
}

// MoveSidebarEntry はサイドバー上のエントリを指定位置に移動する。
func (s *CollectionService) MoveSidebarEntry(kind, id string, position int) error {
	layout, err := s.GetSidebarLayout()
	if err != nil {
		return err
	}

	srcIdx := -1
	for i, e := range layout {
		if e.Kind == kind && e.ID == id {
			srcIdx = i
			break
		}
	}
	if srcIdx == -1 {
		return &cmn.NotFoundError{Resource: "sidebar entry", ID: id}
	}

	entry := layout[srcIdx]
	layout = append(layout[:srcIdx], layout[srcIdx+1:]...)

	if position < 0 || position >= len(layout) {
		layout = append(layout, entry)
	} else {
		layout = append(layout, domain.SidebarEntry{})
		copy(layout[position+1:], layout[position:])
		layout[position] = entry
	}

	return s.layoutRepo.Save(layout)
}

// MoveItemToSidebar はアイテムを指定コレクションから __root__ へ移動し、
// サイドバーレイアウトの指定位置に挿入する。両操作をミューテックスロック内で行う。
func (s *CollectionService) MoveItemToSidebar(sourceCollectionID, itemID string, sidebarPosition int) error {
	s.mu.Lock()

	src, ok := s.cache[sourceCollectionID]
	if !ok {
		s.mu.Unlock()
		return &cmn.NotFoundError{Resource: "collection", ID: sourceCollectionID}
	}
	root, ok := s.cache[domain.RootCollectionID]
	if !ok {
		s.mu.Unlock()
		return &cmn.NotFoundError{Resource: "collection", ID: domain.RootCollectionID}
	}

	item, _, ok := src.FindNode(itemID)
	if !ok {
		s.mu.Unlock()
		return &cmn.NotFoundError{Resource: "item", ID: itemID}
	}

	src.RemoveNode(itemID)
	root.Items = append(root.Items, item)

	if err := s.repo.Save(src); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to save source collection: %w", err)
	}
	if err := s.repo.Save(root); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to save root collection: %w", err)
	}
	s.mu.Unlock()

	// レイアウト更新: エントリを追加して指定位置に移動する。
	layout, err := s.GetSidebarLayout()
	if err != nil {
		return err
	}
	layout = append(layout, domain.SidebarEntry{Kind: "item", ID: itemID})
	entry := layout[len(layout)-1]
	layout = layout[:len(layout)-1]

	if sidebarPosition < 0 || sidebarPosition >= len(layout) {
		layout = append(layout, entry)
	} else {
		layout = append(layout, domain.SidebarEntry{})
		copy(layout[sidebarPosition+1:], layout[sidebarPosition:])
		layout[sidebarPosition] = entry
	}
	return s.layoutRepo.Save(layout)
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

	// root コレクションのアイテムはサイドバーレイアウトからも削除する。
	if collectionID == domain.RootCollectionID {
		if err := s.removeLayoutEntry("item", itemID); err != nil {
			return fmt.Errorf("failed to update sidebar layout: %w", err)
		}
	}
	return nil
}
