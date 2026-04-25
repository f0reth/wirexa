import {
  type Accessor,
  createContext,
  createEffect,
  type JSX,
  onCleanup,
  useContext,
} from "solid-js";
import { createCollectionsState } from "../../application/http/collections";
import { createRequestState } from "../../application/http/request";
import type {
  Collection,
  HttpMethod,
  HttpRequest,
  HttpResponse,
  KeyValuePair,
  RequestAuth,
  RequestBody,
  RequestSettings,
  SidebarEntry,
  TreeItem,
} from "../../domain/http/types";
import { ROOT_COLLECTION_ID } from "../../domain/http/types";
import * as httpClient from "../../infrastructure/http/client";
import { createLogger } from "../../infrastructure/logger/client";
import { createActiveRequestStorage } from "../../infrastructure/storage/local-storage";

export interface RequestContextValue {
  method: Accessor<HttpMethod>;
  setMethod: (val: HttpMethod) => void;
  url: Accessor<string>;
  setUrl: (val: string) => void;
  headers: Accessor<KeyValuePair[]>;
  setHeaders: (val: KeyValuePair[]) => void;
  params: Accessor<KeyValuePair[]>;
  setParams: (val: KeyValuePair[]) => void;
  body: Accessor<RequestBody>;
  setBody: (val: RequestBody) => void;
  auth: Accessor<RequestAuth>;
  setAuth: (val: RequestAuth) => void;
  settings: Accessor<RequestSettings>;
  setSettings: (val: RequestSettings) => void;
  response: Accessor<HttpResponse | null>;
  loading: Accessor<boolean>;
  activeRequestId: Accessor<string | null>;
  activeCollectionId: Accessor<string | null>;
  sendRequest: () => Promise<void>;
  cancelRequest: () => Promise<void>;
  loadRequest: (req: HttpRequest, collectionId: string) => void;
  newRequest: () => void;
  saveCurrentRequest: () => Promise<void>;
  restoreActiveRequest: () => void;
}

export interface CollectionsContextValue {
  collections: Collection[];
  rootItems: TreeItem[];
  sidebarLayout: SidebarEntry[];
  refreshCollections: () => Promise<void>;
  createCollection: (name: string) => Promise<Collection>;
  deleteCollection: (id: string) => Promise<void>;
  renameCollection: (id: string, name: string) => Promise<void>;
  addFolder: (
    collectionId: string,
    parentId: string,
    name: string,
  ) => Promise<TreeItem>;
  addRequest: (
    collectionId: string,
    parentId: string,
    req: HttpRequest,
  ) => Promise<TreeItem>;
  renameItem: (
    collectionId: string,
    itemId: string,
    name: string,
  ) => Promise<void>;
  deleteItem: (collectionId: string, itemId: string) => Promise<void>;
  moveCollection: (collectionId: string, position: number) => Promise<void>;
  moveItem: (
    sourceCollectionId: string,
    itemId: string,
    targetCollectionId: string,
    targetParentId: string,
    position: number,
  ) => Promise<void>;
  moveSidebarEntry: (
    kind: string,
    id: string,
    position: number,
  ) => Promise<void>;
  moveItemToSidebar: (
    sourceCollectionId: string,
    itemId: string,
    sidebarPosition: number,
  ) => Promise<void>;
  isExpanded: (id: string, defaultValue: boolean) => boolean;
  setExpanded: (id: string, val: boolean) => void;
}

const HttpRequestContext = createContext<RequestContextValue>();
const HttpCollectionsContext = createContext<CollectionsContextValue>();

export function HttpProvider(props: { children: JSX.Element }) {
  const collectionsState = createCollectionsState(httpClient);
  const requestState = createRequestState(
    {
      sendRequest: httpClient.sendRequest,
      cancelRequest: httpClient.cancelRequest,
      updateRequest: httpClient.updateRequest,
      afterSave: (colId, req) => collectionsState.patchRequest(colId, req),
    },
    createLogger("frontend:http"),
  );

  // 自動保存: 変更から 500ms 後にバックエンドへ保存
  // saveVersion で世代管理し、古い保存が実行されるのを防ぐ
  let saveVersion = 0;

  createEffect(() => {
    requestState.method();
    requestState.url();
    requestState.headers();
    requestState.params();
    requestState.body();
    requestState.auth();
    requestState.settings();

    const id = requestState.activeRequestId();
    if (!id) return;

    const version = ++saveVersion;

    const timer = setTimeout(() => {
      if (version !== saveVersion) return;
      requestState.saveCurrentRequest().catch((err) => {
        console.error("Auto-save failed:", err);
      });
    }, 500);

    onCleanup(() => clearTimeout(timer));
  });

  // アクティブリクエストをlocalStorageに永続化する
  const activeRequestStorage = createActiveRequestStorage();
  // 初期null（未選択）とユーザーによる明示的なクリアを区別するフラグ
  let activeRequestWasSet = false;

  createEffect(() => {
    const id = requestState.activeRequestId();
    const colId = requestState.activeCollectionId();
    if (id && colId) {
      activeRequestWasSet = true;
      activeRequestStorage.save(id, colId);
    } else if (!id && activeRequestWasSet) {
      activeRequestStorage.clear();
    }
  });

  // コレクションロード後にアクティブリクエストを復元する（CollectionTreeから呼ばれる）
  const restoreActiveRequest = () => {
    if (requestState.activeRequestId()) return;

    const saved = activeRequestStorage.load();
    if (!saved) return;

    const { requestId, collectionId } = saved;

    const findInTree = (items: TreeItem[]): HttpRequest | null => {
      for (const item of items) {
        if (item.type === "request" && item.id === requestId && item.request) {
          return item.request;
        }
        const found = findInTree(item.children);
        if (found) return found;
      }
      return null;
    };

    if (collectionId === ROOT_COLLECTION_ID) {
      const item = collectionsState.rootItems.find(
        (i) => i.id === requestId && i.type === "request",
      );
      if (item?.request) {
        requestState.loadRequest(item.request, ROOT_COLLECTION_ID);
      }
      return;
    }

    const col = collectionsState.collections.find((c) => c.id === collectionId);
    if (!col) return;
    const req = findInTree(col.items);
    if (req) {
      requestState.loadRequest(req, collectionId);
    }
  };

  const contextValue: RequestContextValue = {
    ...requestState,
    restoreActiveRequest,
  };

  return (
    <HttpRequestContext.Provider value={contextValue}>
      <HttpCollectionsContext.Provider value={collectionsState}>
        {props.children}
      </HttpCollectionsContext.Provider>
    </HttpRequestContext.Provider>
  );
}

export function useHttpRequest(): RequestContextValue {
  const ctx = useContext(HttpRequestContext);
  if (!ctx) throw new Error("useHttpRequest must be used within HttpProvider");
  return ctx;
}

export function useHttpCollections(): CollectionsContextValue {
  const ctx = useContext(HttpCollectionsContext);
  if (!ctx)
    throw new Error("useHttpCollections must be used within HttpProvider");
  return ctx;
}
