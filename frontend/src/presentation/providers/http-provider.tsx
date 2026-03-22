import {
  type Accessor,
  createContext,
  type JSX,
  type Setter,
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
  RequestBody,
  TreeItem,
} from "../../domain/http/types";
import * as httpClient from "../../infrastructure/http/client";

export interface RequestContextValue {
  method: Accessor<HttpMethod>;
  setMethod: Setter<HttpMethod>;
  url: Accessor<string>;
  setUrl: Setter<string>;
  headers: Accessor<KeyValuePair[]>;
  setHeaders: Setter<KeyValuePair[]>;
  params: Accessor<KeyValuePair[]>;
  setParams: Setter<KeyValuePair[]>;
  body: Accessor<RequestBody>;
  setBody: Setter<RequestBody>;
  response: Accessor<HttpResponse | null>;
  loading: Accessor<boolean>;
  dirty: Accessor<boolean>;
  activeRequestId: Accessor<string | null>;
  activeCollectionId: Accessor<string | null>;
  sendRequest: () => Promise<void>;
  loadRequest: (req: HttpRequest, collectionId: string) => void;
  newRequest: () => void;
  saveCurrentRequest: () => Promise<void>;
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

export function HttpProvider(props: { children: JSX.Element }) {
  const requestState = createRequestState({
    sendRequest: httpClient.sendRequest,
    updateRequest: httpClient.updateRequest,
  });
  const collectionsState = createCollectionsState(httpClient);

  return (
    <HttpRequestContext.Provider value={requestState}>
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
