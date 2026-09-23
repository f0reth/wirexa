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
//
// 変更系メソッドは copy-on-write で動く。キャッシュ上のコレクションを直接
// 書き換えず、Clone したコピーへ変更を適用し、永続化がすべて成功してから
// キャッシュのエントリを丸ごと差し替える。これによりエラーを返した操作は
// メモリ上にも何も残さない。
//
// 境界では所有権を切る。公開メソッドの戻り値はキャッシュを Clone したもので、
// 引数で受け取った可変データも Clone してから取り込む。呼び出し側（adapter や
// Wails の JSON 化）がロック外で戻り値を読み書きしても、キャッシュには触れない。
//
// ロック順序は CollectionService.mu → SidebarLayoutService.mu の一方向とする。
// 逆向き（レイアウトロックを保持したままコレクションロックを取る）経路を
// 作ってはならない。レイアウト突合に必要なコレクション情報は、レイアウト操作を
// 始める前に読み出しておく。
type CollectionService struct {
	repo   domain.CollectionRepository
	layout *SidebarLayoutService
	logger cmn.Logger
	// cache はコレクション ID → コレクション。
	// 不変条件: キャッシュに載せた（公開した）コレクションとその配下は以後変更しない。
	// 変更は Clone したコピーに対して行い、エントリごと差し替える。snapshotForLayout は
	// この不変条件を前提にロック外で読むため、直接書き換える変更を入れてはならない。
	// 例外は NewCollectionService 内の正規化・重複回収で、サービスを返す前
	// （どのゴルーチンにも公開される前）なので不変条件に反しない。
	cache map[string]*domain.Collection
	mu    sync.RWMutex
}

// NewCollectionService は CollectionService を生成する。
// コンストラクタ内でリポジトリからコレクションを読み込む。
// 失敗として返すのはコレクションの読み込み自体の失敗と、__root__ の新規作成の失敗だけで、
// サイドバーレイアウトの破損・読み込み失敗では起動を止めない（reconcileLayoutAtStartup を参照）。
// logger は nil を許容し、その場合 best-effort な処理の失敗記録をスキップする。
func NewCollectionService(
	repo domain.CollectionRepository,
	layoutRepo domain.SidebarLayoutRepository,
	logger cmn.Logger,
) (*CollectionService, error) {
	svc := &CollectionService{
		repo:   repo,
		layout: NewSidebarLayoutService(layoutRepo, logger),
		logger: logger,
		cache:  make(map[string]*domain.Collection),
	}
	cols, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load collections: %w", err)
	}
	// ここと recoverDuplicateItems はキャッシュを直接変更するが、
	// サービスを返す前なので cache の不変条件に反しない。
	for i := range cols {
		c := cols[i]
		normalizeItemForms(c.Items)
		svc.cache[c.ID] = &c
	}
	if err := svc.ensureRootCollection(); err != nil {
		return nil, err
	}

	svc.recoverDuplicateItems()
	svc.reconcileLayoutAtStartup()

	return svc, nil
}

// ensureRootCollection は __root__ がキャッシュに無いとき、ファイルが本当に無い場合に限って作成する。
// ファイルが残っている（読み込みで読み飛ばされた）か、有無を確認できない場合は作らない。
// 空の root で上書きすると、読めなかった中身（サイドバー直下のリクエスト）が失われるため。
// その場合 root はキャッシュに載らず、root への書き込み操作はすべて NotFound で拒否される。
// 次回起動でファイルを読めれば root は通常どおり読み込まれる。
func (s *CollectionService) ensureRootCollection() error {
	if _, ok := s.cache[domain.RootCollectionID]; ok {
		return nil
	}
	exists, err := s.repo.Exists(domain.RootCollectionID)
	if err != nil {
		s.logError("failed to check root collection file; running without it this session", err)
		return nil
	}
	if exists {
		s.logError("root collection file exists but could not be loaded; running without it this session",
			&cmn.NotFoundError{Resource: sidebarKindCollection, ID: domain.RootCollectionID})
		return nil
	}
	// ファイルが本当に無い: 初回起動、または破損ファイルの退避に成功した後。
	root := &domain.Collection{
		ID:    domain.RootCollectionID,
		Name:  domain.RootCollectionID,
		Items: []*domain.TreeItem{},
	}
	if err := s.repo.Save(root); err != nil {
		return fmt.Errorf("failed to create root collection: %w", err)
	}
	s.cache[root.ID] = root
	return nil
}

// recoverDuplicateItems は「追加してから削除」の途中でプロセスが消えた場合に残る
// 重複アイテムを回収し、内容が変わったコレクションを保存する。
// 保存失敗は best-effort。メモリ上は回収済みなので動作は整合し、次回起動で再試行される。
func (s *CollectionService) recoverDuplicateItems() {
	for id, next := range dropDuplicateItems(s.cache) {
		s.cache[id] = next
		if err := s.repo.Save(next); err != nil {
			s.logError("failed to persist duplicate item recovery", err)
		}
	}
}

// reconcileLayoutAtStartup は保存済みレイアウトを実データと突合し、変化していれば保存する。
// レイアウトはコレクションから再生成できる導出データなので、ここでは起動を止めない。
//   - 壊れていた場合: s.layout.Load が退避してから空レイアウトを返すので、通常の突合から
//     保存までの流れで再生成したファイルが書かれる。
//   - 破損以外の読み込み失敗: ログに残して何もしない。読めないファイルを上書きしないよう保存はせず、
//     並びは GetSidebarLayout の読み出し時の突合に任せる。
//   - 保存失敗: best-effort。読み出し時にも突合されるため、読み取り専用ディレクトリや
//     ディスクフルでも整合した並びを返せる。
func (s *CollectionService) reconcileLayoutAtStartup() {
	layout, err := s.layout.Load()
	if err != nil {
		s.logError("failed to load sidebar layout at startup; it will not be saved", err)
		return
	}
	cols, rootItems := s.snapshotForLayout()
	next, changed := reconcileSidebarLayout(layout, cols, rootItems)
	if !changed {
		return
	}
	if err := s.layout.Save(next); err != nil {
		s.logError("failed to persist sidebar layout reconciliation", err)
	}
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
// 戻り値はキャッシュと可変状態を共有しないディープコピー。
func (s *CollectionService) GetCollections() []domain.Collection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Collection, 0, len(s.cache))
	for _, c := range s.cache {
		if c.ID == domain.RootCollectionID {
			continue
		}
		result = append(result, *c.Clone())
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// GetRootItems はルートコレクション（__root__）のアイテム一覧を返す。
// 戻り値はキャッシュと可変状態を共有しないディープコピー。
func (s *CollectionService) GetRootItems() []*domain.TreeItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	root, ok := s.cache[domain.RootCollectionID]
	if !ok {
		return []*domain.TreeItem{}
	}
	return root.Clone().Items
}

// CreateCollection は新規コレクションを作成する。名前が空の場合は ValidationError を返す。
func (s *CollectionService) CreateCollection(name string) (domain.Collection, error) {
	if strings.TrimSpace(name) == "" {
		return domain.Collection{}, &cmn.ValidationError{Field: "name", Message: cmn.MsgRequired}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	c := &domain.Collection{
		ID:    uuid.NewString(),
		Name:  name,
		Items: []*domain.TreeItem{},
	}
	uow := s.begin()
	uow.SaveCollection(nil, c)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return domain.Collection{}, err
	}
	s.cache[c.ID] = c

	// レイアウトファイルに末尾エントリを追加する。
	s.applyLayoutBestEffort(layoutAppend(domain.SidebarEntry{Kind: sidebarKindCollection, ID: c.ID}))
	return *c.Clone(), nil
}

// DeleteCollection は ID でコレクションを削除する。
func (s *CollectionService) DeleteCollection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[id]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: id}
	}
	uow := s.begin()
	uow.DeleteCollection(c)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return err
	}
	delete(s.cache, id)

	s.applyLayoutBestEffort(layoutRemove(sidebarKindCollection, id))
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
	next := c.Clone()
	next.Name = name

	uow := s.begin()
	uow.SaveCollection(c, next)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return err
	}
	s.cache[id] = next
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
	return s.addItem(collectionID, parentID, item)
}

// AddRequest はコレクションにリクエストを追加する。
// req は複製してから取り込むため、呼び出し側の req（Contents map など）は書き換えない。
func (s *CollectionService) AddRequest(collectionID, parentID string, req domain.HTTPRequest) (*domain.TreeItem, error) {
	r := req.Clone()
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	r.Body.DropFileContents()
	item := &domain.TreeItem{
		Type:     domain.ItemTypeRequest,
		ID:       r.ID,
		Name:     r.Name,
		Request:  r,
		Children: []*domain.TreeItem{},
	}
	return s.addItem(collectionID, parentID, item)
}

// addItem は組み立て済みの TreeItem をコレクションへ追加し、必要ならサイドバーにも反映する。
// item の所有権はキャッシュへ移るため、呼び出し側へは挿入したノードの複製を返す。
func (s *CollectionService) addItem(collectionID, parentID string, item *domain.TreeItem) (*domain.TreeItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return nil, &cmn.NotFoundError{Resource: sidebarKindCollection, ID: collectionID}
	}
	next := c.Clone()
	if !next.AppendItem(parentID, item) {
		return nil, &cmn.NotFoundError{Resource: cmn.ResourceParent, ID: parentID}
	}

	uow := s.begin()
	uow.SaveCollection(c, next)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return nil, err
	}
	s.cache[collectionID] = next

	// root コレクションのルート直下に追加した場合、サイドバーレイアウトにも追加する。
	if collectionID == domain.RootCollectionID && parentID == "" {
		s.applyLayoutBestEffort(layoutAppend(domain.SidebarEntry{Kind: sidebarKindItem, ID: item.ID}))
	}
	return item.Clone(), nil
}

// UpdateRequest はコレクション内のリクエストを更新する。
// req は複製してから取り込むため、呼び出し側の req（Contents map など）は書き換えない。
func (s *CollectionService) UpdateRequest(collectionID string, req domain.HTTPRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: collectionID}
	}

	next := c.Clone()
	node, _, ok := next.FindNode(req.ID)
	if !ok || node.Type != domain.ItemTypeRequest {
		return &cmn.NotFoundError{Resource: cmn.ResourceRequest, ID: req.ID}
	}

	r := req.Clone()
	r.Name = node.Name
	r.Body.DropFileContents()
	node.Request = r

	uow := s.begin()
	uow.SaveCollection(c, next)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return err
	}
	s.cache[collectionID] = next
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

	next := c.Clone()
	node, _, ok := next.FindNode(itemID)
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindItem, ID: itemID}
	}

	node.Name = name
	if node.Request != nil {
		node.Request.Name = name
	}

	uow := s.begin()
	uow.SaveCollection(c, next)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return err
	}
	s.cache[collectionID] = next
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

	// 同一コレクション内移動では1つのクローンを src/dst 兼用にする。
	// コレクションを跨ぐ場合は両方をクローンし、src のクローンから外したノードを
	// dst のクローンへ挿入する。
	sameCollection := sourceCollectionID == targetCollectionID
	srcNext := src.Clone()
	dstNext := srcNext
	if !sameCollection {
		dstNext = dst.Clone()
	}

	item, _, ok := srcNext.FindNode(itemID)
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindItem, ID: itemID}
	}

	// 挿入先を RemoveNode の前に検証する。失敗時はソースを一切変更しない (#6)。
	if targetParentID != "" {
		parent, _, ok := dstNext.FindNode(targetParentID)
		if !ok || parent.Type != domain.ItemTypeFolder {
			return &cmn.NotFoundError{Resource: cmn.ResourceParent, ID: targetParentID}
		}
		// 自身のサブツリー内へは移動できない（RemoveNode で親ごと消えるため）。
		if item.Contains(targetParentID) {
			return &cmn.ValidationError{Field: cmn.ResourceParent, Message: "cannot move an item into its own subtree"}
		}
	}

	// 同一コレクション内移動の場合、削除前に挿入先インデックスを補正する。
	if sameCollection && position > 0 {
		var targetItems []*domain.TreeItem
		if targetParentID == "" {
			targetItems = dstNext.Items
		} else {
			targetNode, _, ok := dstNext.FindNode(targetParentID)
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

	srcNext.RemoveNode(itemID)

	if !dstNext.InsertItem(targetParentID, item, position) {
		return &cmn.NotFoundError{Resource: cmn.ResourceParent, ID: targetParentID}
	}

	// 移動先を先に書いてから移動元を書く。途中でプロセスが消えると巻き戻しは
	// 走らないが、この順序なら残骸は「両方に存在する」重複であり、起動時に回収できる。
	// 先に移動元を書くとアイテムの喪失になり、これは検出も回収もできない。
	uow := s.begin()
	if !sameCollection {
		uow.SaveCollection(dst, dstNext)
	}
	uow.SaveCollection(src, srcNext)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return err
	}
	s.cache[sourceCollectionID] = srcNext
	s.cache[targetCollectionID] = dstNext
	return nil
}

// GetSidebarLayout はサイドバーレイアウトを返す。
// 保存済みレイアウトをその時点のコレクションと突合して正規化してから返す（保存はしない）。
// レイアウトは並び順のヒントであって存在の正ではないため、レイアウト書き込みが
// 失敗していても、frontend は常に実データと整合した並びを受け取る。
// 同じ理由で読み込みに失敗した場合もエラーにせず、ログに残して空レイアウトとして突合する。
func (s *CollectionService) GetSidebarLayout() ([]domain.SidebarEntry, error) {
	cols, rootItems := s.snapshotForLayout()
	layout, err := s.layout.Load()
	if err != nil {
		s.logError("failed to load sidebar layout; returning reconciled layout", err)
		layout = []domain.SidebarEntry{}
	}
	next, _ := reconcileSidebarLayout(layout, cols, rootItems)
	return next, nil
}

// snapshotForLayout は突合に必要なコレクション一覧（__root__ を除く）と
// __root__ 直下アイテムを読み出す。レイアウトロックを取る前に呼び、
// 突合の入力をあらかじめ確定させることでロックのネストを避ける。
//
// 戻り値はキャッシュ上のポインターをそのまま共有し、呼び出し側はロック外で読む。
// 公開済みエントリは不変（cache の不変条件）なので安全だが、キャッシュを直接
// 書き換える変更を入れるとこの前提が崩れる。application 層の外へは渡さないこと。
func (s *CollectionService) snapshotForLayout() ([]*domain.Collection, []*domain.TreeItem) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cols := make([]*domain.Collection, 0, len(s.cache))
	for _, c := range s.cache {
		if c.ID != domain.RootCollectionID {
			cols = append(cols, c)
		}
	}
	var rootItems []*domain.TreeItem
	if root, ok := s.cache[domain.RootCollectionID]; ok {
		rootItems = root.Items
	}
	return cols, rootItems
}

// MoveSidebarEntry はサイドバー上のエントリを指定位置に移動する。
// 並び替えそのものが目的なので、保存失敗はエラーとして呼び出し側へ返す。
// 移動前に突合するのは、frontend が GetSidebarLayout の（突合済みの）並びを見て
// 移動先を決めるため。ファイルにまだ無いエントリの移動を取りこぼさない。
func (s *CollectionService) MoveSidebarEntry(kind, id string, position int) error {
	cols, rootItems := s.snapshotForLayout()
	return s.layout.Update(func(layout []domain.SidebarEntry) ([]domain.SidebarEntry, error) {
		reconciled, _ := reconcileSidebarLayout(layout, cols, rootItems)
		return layoutMove(kind, id, position)(reconciled)
	})
}

// applyLayoutBestEffort はレイアウトを更新し、失敗してもログに残して続行する。
// レイアウトは導出値で、欠落・stale は読み出し時の突合で修復されるため、
// コレクション本体の変更をレイアウトの保存失敗で巻き戻さない。
func (s *CollectionService) applyLayoutBestEffort(mutate layoutMutator) {
	if err := s.layout.Update(mutate); err != nil {
		s.logError("failed to update sidebar layout", err)
	}
}

// logError は logger が設定されている場合だけエラーを記録する。
func (s *CollectionService) logError(msg string, err error) {
	if s.logger == nil {
		return
	}
	s.logger.Error(msg, "source", "http", "error", err)
}

// MoveItemToSidebar はアイテムを指定コレクションから __root__ へ移動し、
// サイドバーレイアウトの指定位置に挿入する。
func (s *CollectionService) MoveItemToSidebar(sourceCollectionID, itemID string, sidebarPosition int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, ok := s.cache[sourceCollectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: sourceCollectionID}
	}
	root, ok := s.cache[domain.RootCollectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: domain.RootCollectionID}
	}

	// 移動元が __root__ 自身の場合は1つのクローンを src/root 兼用にする。
	fromRoot := sourceCollectionID == domain.RootCollectionID
	srcNext := src.Clone()
	rootNext := srcNext
	if !fromRoot {
		rootNext = root.Clone()
	}

	item, _, ok := srcNext.FindNode(itemID)
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindItem, ID: itemID}
	}

	srcNext.RemoveNode(itemID)
	rootNext.AppendItem("", item)

	// MoveItem と同じ理由で __root__ を先に書いてから移動元を書く。
	uow := s.begin()
	if !fromRoot {
		uow.SaveCollection(root, rootNext)
	}
	uow.SaveCollection(src, srcNext)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return err
	}
	s.cache[sourceCollectionID] = srcNext
	s.cache[domain.RootCollectionID] = rootNext

	// レイアウトへの挿入は best-effort。失敗した場合アイテムはサイドバー末尾に現れる
	// (指定位置には入らないが、読み出し時の突合で並び自体は整合する)。
	s.applyLayoutBestEffort(layoutInsertItem(itemID, sidebarPosition))
	return nil
}

// DeleteItem はコレクションからアイテムをサブツリーごと削除する。
func (s *CollectionService) DeleteItem(collectionID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.cache[collectionID]
	if !ok {
		return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: collectionID}
	}

	next := c.Clone()
	if !next.RemoveNode(itemID) {
		return &cmn.NotFoundError{Resource: sidebarKindItem, ID: itemID}
	}

	uow := s.begin()
	uow.SaveCollection(c, next)
	if err := uow.Err(); err != nil {
		uow.Rollback()
		return err
	}
	s.cache[collectionID] = next

	// root コレクションのアイテムはサイドバーレイアウトからも削除する。
	if collectionID == domain.RootCollectionID {
		s.applyLayoutBestEffort(layoutRemove(sidebarKindItem, itemID))
	}
	return nil
}
