import { createStore, produce, reconcile } from "solid-js/store";
import type {
  Collection,
  HttpRequest,
  TreeItem,
} from "../../domain/http/types";

export interface CollectionsApi {
  getCollections(): Promise<Collection[]>;
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
    collectionId: string,
    itemId: string,
    targetParentId: string,
    position: number,
  ): Promise<void>;
}

export function createCollectionsState(api: CollectionsApi) {
  const [collections, setCollections] = createStore<Collection[]>([]);

  async function refreshCollections(): Promise<void> {
    const cols = await api.getCollections();
    setCollections(reconcile(cols, { key: "id" }));
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

  async function moveItem(
    collectionId: string,
    itemId: string,
    targetParentId: string,
    position: number,
  ): Promise<void> {
    await api.moveItem(collectionId, itemId, targetParentId, position);
    await refreshCollections();
  }

  function patchRequest(collectionId: string, req: HttpRequest): void {
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
    refreshCollections,
    createCollection,
    deleteCollection,
    renameCollection,
    addFolder,
    addRequest,
    renameItem,
    deleteItem,
    moveItem,
    patchRequest,
  };
}
