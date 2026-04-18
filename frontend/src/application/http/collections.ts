import { createStore, produce, reconcile } from "solid-js/store";
import type {
  Collection,
  HttpRequest,
  SidebarEntry,
  TreeItem,
} from "../../domain/http/types";
import { ROOT_COLLECTION_ID } from "../../domain/http/types";

const STORAGE_KEY = "wirexa:http:expandedFolders";

function loadExpandedIds(): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {
    // ignore
  }
  return {};
}

function saveExpandedIds(ids: Record<string, boolean>): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(ids));
  } catch {
    // ignore
  }
}

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
  moveCollection(collectionId: string, position: number): Promise<void>;
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

export function createCollectionsState(api: CollectionsApi) {
  const [collections, setCollections] = createStore<Collection[]>([]);
  const [rootItems, setRootItems] = createStore<TreeItem[]>([]);
  const [sidebarLayout, setSidebarLayout] = createStore<SidebarEntry[]>([]);
  const [expandedIds, setExpandedIds] = createStore<Record<string, boolean>>(
    loadExpandedIds(),
  );

  function isExpanded(id: string, defaultValue: boolean): boolean {
    return expandedIds[id] ?? defaultValue;
  }

  function setExpanded(id: string, val: boolean): void {
    setExpandedIds(id, val);
    saveExpandedIds({ ...expandedIds, [id]: val });
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
    saveExpandedIds({ ...expandedIds });
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
  }

  async function createCollection(name: string): Promise<Collection> {
    const collection = await api.createCollection(name);
    await refreshCollections();
    return collection;
  }

  async function deleteCollection(id: string): Promise<void> {
    await api.deleteCollection(id);
    await refreshCollections();
  }

  async function renameCollection(id: string, name: string): Promise<void> {
    await api.renameCollection(id, name);
    await refreshCollections();
  }

  async function addFolder(
    collectionId: string,
    parentId: string,
    name: string,
  ): Promise<TreeItem> {
    const item = await api.addFolder(collectionId, parentId, name);
    await refreshCollections();
    return item;
  }

  async function addRequest(
    collectionId: string,
    parentId: string,
    req: HttpRequest,
  ): Promise<TreeItem> {
    const item = await api.addRequest(collectionId, parentId, req);
    await refreshCollections();
    return item;
  }

  async function renameItem(
    collectionId: string,
    itemId: string,
    name: string,
  ): Promise<void> {
    await api.renameItem(collectionId, itemId, name);
    await refreshCollections();
  }

  async function deleteItem(
    collectionId: string,
    itemId: string,
  ): Promise<void> {
    await api.deleteItem(collectionId, itemId);
    await refreshCollections();
  }

  async function moveCollection(
    collectionId: string,
    position: number,
  ): Promise<void> {
    await api.moveCollection(collectionId, position);
    await refreshCollections();
  }

  async function moveItem(
    sourceCollectionId: string,
    itemId: string,
    targetCollectionId: string,
    targetParentId: string,
    position: number,
  ): Promise<void> {
    await api.moveItem(
      sourceCollectionId,
      itemId,
      targetCollectionId,
      targetParentId,
      position,
    );
    await refreshCollections();
  }

  async function moveSidebarEntry(
    kind: string,
    id: string,
    position: number,
  ): Promise<void> {
    await api.moveSidebarEntry(kind, id, position);
    await refreshSidebarLayout();
  }

  async function moveItemToSidebar(
    sourceCollectionId: string,
    itemId: string,
    sidebarPosition: number,
  ): Promise<void> {
    await api.moveItemToSidebar(sourceCollectionId, itemId, sidebarPosition);
    await refreshCollections();
  }

  function patchRequest(collectionId: string, req: HttpRequest): void {
    if (collectionId === ROOT_COLLECTION_ID) {
      setRootItems(
        produce((items) => {
          for (const item of items) {
            if (item.type === "request" && item.id === req.id && item.request) {
              item.request = { ...req, name: item.request.name };
            }
          }
        }),
      );
      return;
    }
    setCollections(
      (col) => col.id === collectionId,
      produce((col) => {
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
        patch(col.items);
      }),
    );
  }

  return {
    collections,
    rootItems,
    sidebarLayout,
    refreshCollections,
    createCollection,
    deleteCollection,
    renameCollection,
    addFolder,
    addRequest,
    renameItem,
    deleteItem,
    moveCollection,
    moveItem,
    moveSidebarEntry,
    moveItemToSidebar,
    patchRequest,
    isExpanded,
    setExpanded,
  };
}
