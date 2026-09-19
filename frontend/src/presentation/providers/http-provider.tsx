import {
  type Accessor,
  createContext,
  createEffect,
  type JSX,
  onMount,
  useContext,
} from "solid-js";
import {
  createCollectionsState,
  findRequestById,
} from "../../application/http/collections";
import {
  createAutoSaveEffect,
  createRequestState,
  type ResponseSaveState,
} from "../../application/http/request";
import { notify } from "../../application/ui/notifications";
import type {
  Collection,
  FileReference,
  FormBodyType,
  FormRow,
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
  bodyContent: Accessor<string>;
  setBodyContent: (content: string) => void;
  formBodyType: Accessor<FormBodyType | null>;
  formPairs: Accessor<FormRow[]>;
  setFormPairs: (rows: FormRow[]) => void;
  auth: Accessor<RequestAuth>;
  setAuth: (val: RequestAuth) => void;
  settings: Accessor<RequestSettings>;
  setSettings: (val: RequestSettings) => void;
  doc: Accessor<string>;
  setDoc: (val: string) => void;
  response: Accessor<HttpResponse | null>;
  responseSaveState: Accessor<ResponseSaveState>;
  saveResponseBody: () => Promise<void>;
  loading: Accessor<boolean>;
  activeRequestId: Accessor<string | null>;
  activeCollectionId: Accessor<string | null>;
  saveError: Accessor<string | null>;
  clearSaveError: () => void;
  sendRequest: () => Promise<void>;
  cancelRequest: () => Promise<void>;
  pickFile: (hint: string) => Promise<FileReference | undefined>;
  loadRequest: (req: HttpRequest, collectionId: string) => void;
  newRequest: () => void;
  saveCurrentRequest: () => Promise<void>;
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
  const collectionsState = createCollectionsState(httpClient, notify);
  const requestState = createRequestState(
    {
      sendRequest: httpClient.sendRequest,
      cancelRequest: httpClient.cancelRequest,
      updateRequest: httpClient.updateRequest,
      openFilePicker: httpClient.openFilePicker,
      saveResponseBody: httpClient.saveResponseBody,
      saveResponseBinary: httpClient.saveResponseBinary,
      discardResponseBody: httpClient.discardResponseBody,
      afterSave: (colId, req) => collectionsState.patchRequest(colId, req),
    },
    createLogger("frontend:http"),
    notify,
  );

  createAutoSaveEffect(requestState, notify);

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

  // コレクションロード後にアクティブリクエストを復元する。
  const restoreActiveRequest = () => {
    if (requestState.activeRequestId()) return;

    const saved = activeRequestStorage.load();
    if (!saved) return;

    const req = findRequestById(
      collectionsState.collections,
      collectionsState.rootItems,
      saved.collectionId,
      saved.requestId,
    );
    if (req) requestState.loadRequest(req, saved.collectionId);
  };

  // 起動時のシーケンスはここに集約する（表示中のサイドバーに依存させない）。
  onMount(async () => {
    await collectionsState.refreshCollections();
    restoreActiveRequest();
  });

  const contextValue: RequestContextValue = { ...requestState };

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
