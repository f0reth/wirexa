import {
  type Accessor,
  createContext,
  createMemo,
  createSignal,
  type JSX,
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
  TreeItem,
} from "../../domain/http/types";
import * as httpClient from "../../infrastructure/http/client";

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
  dirty: Accessor<boolean>;
  activeRequestId: Accessor<string | null>;
  activeCollectionId: Accessor<string | null>;
  sendRequest: () => Promise<void>;
  cancelRequest: () => Promise<void>;
  loadRequest: (req: HttpRequest, collectionId: string) => void;
  newRequest: () => void;
  saveCurrentRequest: () => Promise<void>;
  formatJsonBody: (content: string) => string;
}

export interface CollectionsContextValue {
  collections: Collection[];
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
}

const HttpRequestContext = createContext<RequestContextValue>();
const HttpCollectionsContext = createContext<CollectionsContextValue>();

type SavedSnapshot = {
  method: HttpMethod;
  url: string;
  headers: KeyValuePair[];
  params: KeyValuePair[];
  body: RequestBody;
  auth: RequestAuth;
  settings: RequestSettings;
};

export function HttpProvider(props: { children: JSX.Element }) {
  const collectionsState = createCollectionsState(httpClient);
  const requestState = createRequestState({
    sendRequest: httpClient.sendRequest,
    cancelRequest: httpClient.cancelRequest,
    updateRequest: httpClient.updateRequest,
    afterSave: () => collectionsState.refreshCollections(),
  });

  // dirty 管理はプレゼンテーション層の責務
  const [savedSnapshot, setSavedSnapshot] = createSignal<SavedSnapshot | null>(
    null,
  );

  function loadRequest(req: HttpRequest, collectionId: string): void {
    requestState.loadRequest(req, collectionId);
    setSavedSnapshot({
      method: req.method,
      url: req.url,
      headers: req.headers,
      params: req.params,
      body: req.body,
      auth: req.auth,
      settings: req.settings,
    });
  }

  function newRequest(): void {
    requestState.newRequest();
    setSavedSnapshot(null);
  }

  async function saveCurrentRequest(): Promise<void> {
    await requestState.saveCurrentRequest();
    setSavedSnapshot({
      method: requestState.method(),
      url: requestState.url(),
      headers: requestState.headers(),
      params: requestState.params(),
      body: requestState.body(),
      auth: requestState.auth(),
      settings: requestState.settings(),
    });
  }

  const dirty = createMemo(() => {
    const snap = savedSnapshot();
    if (snap === null || requestState.activeRequestId() === null) return false;
    return (
      requestState.method() !== snap.method ||
      requestState.url() !== snap.url ||
      JSON.stringify(requestState.headers()) !== JSON.stringify(snap.headers) ||
      JSON.stringify(requestState.params()) !== JSON.stringify(snap.params) ||
      JSON.stringify(requestState.body()) !== JSON.stringify(snap.body) ||
      JSON.stringify(requestState.auth()) !== JSON.stringify(snap.auth) ||
      JSON.stringify(requestState.settings()) !== JSON.stringify(snap.settings)
    );
  });

  const contextValue: RequestContextValue = {
    ...requestState,
    dirty,
    loadRequest,
    newRequest,
    saveCurrentRequest,
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
