import { createSignal } from "solid-js";
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

  function pruneExpandedIds(cols: Collection[]): void {
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
    const staleIds = Object.keys(expandedIds).filter((id) => !validIds.has(id));
    if (staleIds.length === 0) return;
    setExpandedIds(
      produce((draft) => {
        for (const id of staleIds) delete draft[id];
      }),
    );
    expandedStorage.save({ ...expandedIds });
  }

  async function refreshRootItems(): Promise<void> {
    const items = await api.getRootItems();
    setRootItems(reconcile(items, { key: "id" }));
  }

  async function refreshSidebarLayout(): Promise<void> {
    const layout = await api.getSidebarLayout();
    setSidebarLayout(reconcile(layout));
  }

  async function refreshCollections(): Promise<void> {
    const cols = await api.getCollections();
    setCollections(reconcile(cols, { key: "id" }));
    pruneExpandedIds(cols);
    await refreshRootItems();
    await refreshSidebarLayout();
    setCollectionsLoaded(true);
  }

  async function createCollection(name: string): Promise<Collection> {
    return notifyOnError(notifier, "Failed to create collection", async () => {
      const collection = await api.createCollection(name);
      await refreshCollections();
      return collection;
    });
  }

  async function deleteCollection(id: string): Promise<void> {
    await runGuarded(notifier, "Failed to delete collection", async () => {
      await api.deleteCollection(id);
      await refreshCollections();
    });
  }

  async function renameCollection(id: string, name: string): Promise<void> {
    await runGuarded(notifier, "Failed to rename collection", async () => {
      await api.renameCollection(id, name);
      await refreshCollections();
    });
  }

  async function addFolder(
    collectionId: string,
    parentId: string,
    name: string,
  ): Promise<TreeItem> {
    return notifyOnError(notifier, "Failed to add folder", async () => {
      const item = await api.addFolder(collectionId, parentId, name);
      await refreshCollections();
      return item;
    });
  }

  async function addRequest(
    collectionId: string,
    parentId: string,
    req: HttpRequest,
  ): Promise<TreeItem> {
    return notifyOnError(notifier, "Failed to add request", async () => {
      const item = await api.addRequest(collectionId, parentId, req);
      await refreshCollections();
      return item;
    });
  }

  async function renameItem(
    collectionId: string,
    itemId: string,
    name: string,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to rename item", async () => {
      await api.renameItem(collectionId, itemId, name);
      await refreshCollections();
    });
  }

  async function deleteItem(
    collectionId: string,
    itemId: string,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to delete item", async () => {
      await api.deleteItem(collectionId, itemId);
      await refreshCollections();
    });
  }

  async function moveItem(
    sourceCollectionId: string,
    itemId: string,
    targetCollectionId: string,
    targetParentId: string,
    position: number,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to move item", async () => {
      await api.moveItem(
        sourceCollectionId,
        itemId,
        targetCollectionId,
        targetParentId,
        position,
      );
      await refreshCollections();
    });
  }

  async function moveSidebarEntry(
    kind: string,
    id: string,
    position: number,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to move item", async () => {
      await api.moveSidebarEntry(kind, id, position);
      await refreshSidebarLayout();
    });
  }

  async function moveItemToSidebar(
    sourceCollectionId: string,
    itemId: string,
    sidebarPosition: number,
  ): Promise<void> {
    await runGuarded(notifier, "Failed to move item", async () => {
      await api.moveItemToSidebar(sourceCollectionId, itemId, sidebarPosition);
      await refreshCollections();
    });
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
