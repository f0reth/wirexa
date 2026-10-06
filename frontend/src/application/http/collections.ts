import { batch, createSignal } from "solid-js";
import { createStore, produce, reconcile } from "solid-js/store";
import type { ExpandedFoldersStorage } from "../../domain/http/ports";
import type {
  Collection,
  HttpRequest,
  SidebarEntry,
  TreeItem,
} from "../../domain/http/types";
import { ROOT_COLLECTION_ID } from "../../domain/http/types";
import type { Notifier } from "../../domain/ui/ports";
import { notifyOnError, runGuarded } from "../ui/guard";

/**
 * コレクション ID に対応するアイテムの並び。__root__ はサイドバー直下のアイテムを指す。
 * 無いコレクションは null。
 */
function itemsOf(
  collections: readonly Collection[],
  rootItems: readonly TreeItem[],
  collectionId: string,
): readonly TreeItem[] | null {
  if (collectionId === ROOT_COLLECTION_ID) return rootItems;
  return collections.find((c) => c.id === collectionId)?.items ?? null;
}

function findRequestIn(
  items: readonly TreeItem[],
  requestId: string,
): HttpRequest | null {
  for (const item of items) {
    if (item.type === "request" && item.id === requestId && item.request) {
      return item.request;
    }
    const found = findRequestIn(item.children, requestId);
    if (found) return found;
  }
  return null;
}

/**
 * 保存済みのアクティブリクエスト（id とコレクション id）をツリーから探す。
 * __root__ もフォルダを持てる（フォルダをサイドバー直下へ出せる）ので、通常のコレクションと
 * 同じく再帰的に探す。
 */
export function findRequestById(
  collections: readonly Collection[],
  rootItems: readonly TreeItem[],
  collectionId: string,
  requestId: string,
): HttpRequest | null {
  const items = itemsOf(collections, rootItems, collectionId);
  return items ? findRequestIn(items, requestId) : null;
}

/**
 * リクエストが今どのコレクションにあるかを返す。サイドバー直下（そのフォルダの中を含む）なら
 * __root__、どこにも無ければ null。
 */
export function findRequestLocation(
  collections: readonly Collection[],
  rootItems: readonly TreeItem[],
  requestId: string,
): string | null {
  if (findRequestIn(rootItems, requestId)) return ROOT_COLLECTION_ID;
  return collections.find((c) => findRequestIn(c.items, requestId))?.id ?? null;
}

/**
 * moveItem の position に渡すと、移動先の末尾に追加する。
 * Go 側の domain.PositionEnd (internal/domain/slices.go) と同じ値にする。
 */
export const APPEND_POSITION = -1;

export interface CollectionsApi {
  getCollections(): Promise<Collection[]>;
  getRootItems(): Promise<TreeItem[]>;
  createCollection(name: string): Promise<Collection>;
  deleteCollection(id: string): Promise<void>;
  renameCollection(id: string, name: string): Promise<void>;
  addFolder(
    collectionId: string,
    parentId: string,
    name: string,
  ): Promise<TreeItem>;
  addRequest(
    collectionId: string,
    parentId: string,
    req: HttpRequest,
  ): Promise<TreeItem>;
  renameItem(collectionId: string, itemId: string, name: string): Promise<void>;
  deleteItem(collectionId: string, itemId: string): Promise<void>;
  moveItem(
    sourceCollectionId: string,
    itemId: string,
    targetCollectionId: string,
    targetParentId: string,
    position: number,
  ): Promise<void>;
  getSidebarLayout(): Promise<SidebarEntry[]>;
  moveSidebarEntry(kind: string, id: string, position: number): Promise<void>;
  moveItemToSidebar(
    sourceCollectionId: string,
    itemId: string,
    sidebarPosition: number,
  ): Promise<void>;
}

/**
 * 失敗時の扱いは 2 通り。
 * - 戻り値を持たない操作: 通知して正常終了する（runGuarded）。
 * - 戻り値を持つ操作: 通知したうえで例外を再送出する（notifyOnError）。
 *   呼び出し側は戻り値の有無で成功／失敗を分岐できる。
 */
export function createCollectionsState(
  api: CollectionsApi,
  notifier: Notifier,
  expandedStorage: ExpandedFoldersStorage,
) {
  const [collections, setCollections] = createStore<Collection[]>([]);
  const [rootItems, setRootItems] = createStore<TreeItem[]>([]);
  const [sidebarLayout, setSidebarLayout] = createStore<SidebarEntry[]>([]);
  const [expandedIds, setExpandedIds] = createStore<Record<string, boolean>>(
    expandedStorage.load(),
  );
  const [collectionsLoaded, setCollectionsLoaded] = createSignal(false);

  function isExpanded(id: string, defaultValue: boolean): boolean {
    return expandedIds[id] ?? defaultValue;
  }

  function setExpanded(id: string, val: boolean): void {
    setExpandedIds(id, val);
    expandedStorage.save({ ...expandedIds, [id]: val });
  }

  // もう無いコレクション・アイテムの開閉の記録を消す。フォルダはサイドバー直下にも置けるので、
  // ルートのアイテムも有効な ID に数える。
  function pruneExpandedIds(cols: Collection[], roots: TreeItem[]): void {
    const validIds = new Set<string>();
    const walk = (items: TreeItem[]) => {
      for (const item of items) {
        validIds.add(item.id);
        if (item.children) walk(item.children);
      }
    };
    for (const col of cols) {
      validIds.add(col.id);
      walk(col.items);
    }
    walk(roots);
    const staleIds = Object.keys(expandedIds).filter((id) => !validIds.has(id));
    if (staleIds.length === 0) return;
    setExpandedIds(
      produce((draft) => {
        for (const id of staleIds) delete draft[id];
      }),
    );
    expandedStorage.save({ ...expandedIds });
  }

  // 構造を変える操作 (RPC とそのあとの再読み込み) と、外から呼ぶ再読み込みを 1 本に直列化する。
  // 3 つの取得は同じ時点のスナップショットではないので、取得の途中で構造を変える RPC が走ると、
  // 移動中のアイテムがどこにも無いツリーが組み上がる。前の操作の再読み込みが反映されるまで
  // 次の操作を始めなければ、取得の途中で構造は変わらず、反映の順も入れ替わらない。
  // 前の操作が失敗しても次は実行する。
  let queue: Promise<unknown> = Promise.resolve();

  function enqueue<T>(task: () => Promise<T>): Promise<T> {
    const run = queue.then(task);
    queue = run.catch(() => {});
    return run;
  }

  // 3 つを取得し終えてからまとめて反映する。途中まで読めた一覧は見せず、どれかの取得に
  // 失敗したら何も変えない。enqueue の中から呼ぶ (自分では積まない)。
  async function loadTree(): Promise<void> {
    const cols = await api.getCollections();
    const items = await api.getRootItems();
    const layout = await api.getSidebarLayout();
    batch(() => {
      setCollections(reconcile(cols, { key: "id" }));
      setRootItems(reconcile(items, { key: "id" }));
      setSidebarLayout(reconcile(layout));
      pruneExpandedIds(cols, items);
      setCollectionsLoaded(true);
    });
  }

  function refreshCollections(): Promise<void> {
    return enqueue(loadTree);
  }

  /** 構造を変える RPC を呼び、ツリーを読み直す。 */
  function mutate<T>(rpc: () => Promise<T>): Promise<T> {
    return enqueue(async () => {
      const result = await rpc();
      await loadTree();
      return result;
    });
  }

  async function createCollection(name: string): Promise<Collection> {
    return notifyOnError(notifier, "Failed to create collection", () =>
      mutate(() => api.createCollection(name)),
    );
  }

  async function deleteCollection(id: string): Promise<void> {
    await runGuarded(notifier, "Failed to delete collection", () =>
      mutate(() => api.deleteCollection(id)),
    );
  }

  async function renameCollection(id: string, name: string): Promise<void> {
    await runGuarded(notifier, "Failed to rename collection", () =>
      mutate(() => api.renameCollection(id, name)),
    );
  }

  async function addFolder(
    collectionId: string,
    parentId: string,
    name: string,
  ): Promise<TreeItem> {
    return notifyOnError(notifier, "Failed to add folder", () =>
      mutate(() => api.addFolder(collectionId, parentId, name)),
    );
  }

  async function addRequest(
    collectionId: string,
    parentId: string,
    req: HttpRequest,
  ): Promise<TreeItem> {
    return notifyOnError(notifier, "Failed to add request", () =>
      mutate(() => api.addRequest(collectionId, parentId, req)),
    );
  }

  async function renameItem(
    collectionId: string,
    itemId: string,
    name: string,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to rename item", () =>
      mutate(() => api.renameItem(collectionId, itemId, name)),
    );
  }

  async function deleteItem(
    collectionId: string,
    itemId: string,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to delete item", () =>
      mutate(() => api.deleteItem(collectionId, itemId)),
    );
  }

  async function moveItem(
    sourceCollectionId: string,
    itemId: string,
    targetCollectionId: string,
    targetParentId: string,
    position: number,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to move item", () =>
      mutate(() =>
        api.moveItem(
          sourceCollectionId,
          itemId,
          targetCollectionId,
          targetParentId,
          position,
        ),
      ),
    );
  }

  // 並びしか変わらないので、読み直すのはレイアウトだけ。レイアウトを書くので同じキューに通す。
  async function moveSidebarEntry(
    kind: string,
    id: string,
    position: number,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to move item", () =>
      enqueue(async () => {
        await api.moveSidebarEntry(kind, id, position);
        setSidebarLayout(reconcile(await api.getSidebarLayout()));
      }),
    );
  }

  async function moveItemToSidebar(
    sourceCollectionId: string,
    itemId: string,
    sidebarPosition: number,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to move item", () =>
      mutate(() =>
        api.moveItemToSidebar(sourceCollectionId, itemId, sidebarPosition),
      ),
    );
  }

  function patchRequest(collectionId: string, req: HttpRequest): void {
    const patch = (items: TreeItem[]): boolean => {
      for (const item of items) {
        if (item.type === "request" && item.id === req.id && item.request) {
          // name は backend が管理するため保持する
          item.request = { ...req, name: item.request.name };
          return true;
        }
        if (patch(item.children)) return true;
      }
      return false;
    };
    if (collectionId === ROOT_COLLECTION_ID) {
      setRootItems(produce((items) => void patch(items)));
      return;
    }
    setCollections(
      (col) => col.id === collectionId,
      produce((col) => void patch(col.items)),
    );
  }

  return {
    collections,
    rootItems,
    sidebarLayout,
    collectionsLoaded,
    refreshCollections,
    createCollection,
    deleteCollection,
    renameCollection,
    addFolder,
    addRequest,
    renameItem,
    deleteItem,
    moveItem,
    moveSidebarEntry,
    moveItemToSidebar,
    patchRequest,
    isExpanded,
    setExpanded,
  };
}

export type CollectionsState = ReturnType<typeof createCollectionsState>;
