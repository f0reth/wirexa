import {
  AddFolder,
  AddRequest,
  CancelRequest,
  CreateCollection,
  DeleteCollection,
  DeleteItem,
  GetCollections,
  GetRootItems,
  GetSidebarLayout,
  MoveCollection,
  MoveItem,
  MoveItemToSidebar,
  MoveSidebarEntry,
  RenameCollection,
  RenameItem,
  SaveResponseBody,
  SendRequest,
  UpdateRequest,
} from "../../../wailsjs/go/adapters/HttpHandler";
import { adapters } from "../../../wailsjs/go/models";
import {
  type Collection,
  DEFAULT_SETTINGS,
  type HttpRequest,
  type HttpResponse,
  isAuthType,
  isBodyType,
  isHttpMethod,
  type KeyValuePair,
  type RequestAuth,
  type RequestBody,
  type RequestSettings,
  type SidebarEntry,
  type TreeItem,
} from "../../domain/http/types";

// domain → Wails
function toWailsRequest(req: HttpRequest): adapters.HttpRequest {
  return adapters.HttpRequest.createFrom(req);
}

// Wails → domain
function fromWailsKeyValuePair(kv: adapters.KeyValuePair): KeyValuePair {
  return { key: kv.key, value: kv.value, enabled: kv.enabled };
}

function fromWailsRequestSettings(
  settings: adapters.RequestSettings | undefined | null,
): RequestSettings {
  if (!settings) return { ...DEFAULT_SETTINGS };
  return {
    timeoutSec: settings.timeoutSec ?? 0,
    proxyMode:
      settings.proxyMode === "none" || settings.proxyMode === "custom"
        ? settings.proxyMode
        : "system",
    proxyURL: settings.proxyURL ?? "",
    insecureSkipVerify: settings.insecureSkipVerify ?? false,
    disableRedirects: settings.disableRedirects ?? false,
    maxResponseBodyMB: settings.maxResponseBodyMB ?? 0,
  };
}

function fromWailsRequestAuth(auth: adapters.RequestAuth): RequestAuth {
  return {
    type: auth && isAuthType(auth.type) ? auth.type : "none",
    username: auth?.username ?? "",
    password: auth?.password ?? "",
    token: auth?.token ?? "",
  };
}

function fromWailsRequestBody(body: adapters.RequestBody): RequestBody {
  if (!isBodyType(body.type)) {
    throw new Error(`Unknown body type: ${body.type}`);
  }
  return {
    type: body.type,
    contents: (body.contents ?? {}) as RequestBody["contents"],
  };
}

function fromWailsHttpRequest(req: adapters.HttpRequest): HttpRequest {
  if (!isHttpMethod(req.method)) {
    throw new Error(`Unknown HTTP method: ${req.method}`);
  }
  return {
    id: req.id,
    name: req.name,
    method: req.method,
    url: req.url,
    headers: req.headers.map(fromWailsKeyValuePair),
    params: req.params.map(fromWailsKeyValuePair),
    body: fromWailsRequestBody(req.body),
    auth: fromWailsRequestAuth(req.auth),
    settings: fromWailsRequestSettings(req.settings),
  };
}

function fromWailsHttpResponse(res: adapters.HttpResponse): HttpResponse {
  return {
    statusCode: res.statusCode,
    statusText: res.statusText,
    headers: res.headers,
    body: res.body,
    contentType: res.contentType,
    size: res.size,
    timingMs: res.timingMs,
    error: res.error,
    bodyTruncated: res.bodyTruncated ?? false,
    tempFilePath: res.tempFilePath ?? "",
  };
}

function fromWailsTreeItem(item: adapters.TreeItem): TreeItem {
  if (item.type !== "folder" && item.type !== "request") {
    throw new Error(`Unknown tree item type: ${item.type}`);
  }
  return {
    type: item.type,
    id: item.id,
    name: item.name,
    children: (item.children ?? []).map(fromWailsTreeItem),
    request: item.request ? fromWailsHttpRequest(item.request) : undefined,
  };
}

function fromWailsCollection(col: adapters.Collection): Collection {
  return {
    id: col.id,
    name: col.name,
    items: col.items.map(fromWailsTreeItem),
    order: col.order ?? 0,
  };
}

export async function sendRequest(req: HttpRequest): Promise<HttpResponse> {
  const result = await SendRequest(toWailsRequest(req));
  return fromWailsHttpResponse(result);
}

export async function cancelRequest(id: string): Promise<void> {
  return CancelRequest(id);
}

export async function getCollections(): Promise<Collection[]> {
  const result = await GetCollections();
  return result.map(fromWailsCollection);
}

export async function getRootItems(): Promise<TreeItem[]> {
  const result = await GetRootItems();
  return result.map(fromWailsTreeItem);
}

export async function createCollection(name: string): Promise<Collection> {
  const result = await CreateCollection(name);
  return fromWailsCollection(result);
}

export async function deleteCollection(id: string): Promise<void> {
  return DeleteCollection(id);
}

export async function renameCollection(
  id: string,
  name: string,
): Promise<void> {
  return RenameCollection(id, name);
}

export async function addFolder(
  collectionId: string,
  parentId: string,
  name: string,
): Promise<TreeItem> {
  const result = await AddFolder(collectionId, parentId, name);
  return fromWailsTreeItem(result);
}

export async function addRequest(
  collectionId: string,
  parentId: string,
  req: HttpRequest,
): Promise<TreeItem> {
  const result = await AddRequest(collectionId, parentId, toWailsRequest(req));
  return fromWailsTreeItem(result);
}

export async function updateRequest(
  collectionId: string,
  req: HttpRequest,
): Promise<void> {
  return UpdateRequest(collectionId, toWailsRequest(req));
}

export async function renameItem(
  collectionId: string,
  itemId: string,
  name: string,
): Promise<void> {
  return RenameItem(collectionId, itemId, name);
}

export async function deleteItem(
  collectionId: string,
  itemId: string,
): Promise<void> {
  return DeleteItem(collectionId, itemId);
}

export async function moveCollection(
  collectionId: string,
  position: number,
): Promise<void> {
  return MoveCollection(collectionId, position);
}

export async function moveItem(
  sourceCollectionId: string,
  itemId: string,
  targetCollectionId: string,
  targetParentId: string,
  position: number,
): Promise<void> {
  return MoveItem(
    sourceCollectionId,
    itemId,
    targetCollectionId,
    targetParentId,
    position,
  );
}

export async function getSidebarLayout(): Promise<SidebarEntry[]> {
  const result = await GetSidebarLayout();
  return result.map((e) => ({
    kind: e.kind as SidebarEntry["kind"],
    id: e.id,
  }));
}

export async function moveSidebarEntry(
  kind: string,
  id: string,
  position: number,
): Promise<void> {
  return MoveSidebarEntry(kind, id, position);
}

export async function moveItemToSidebar(
  sourceCollectionId: string,
  itemId: string,
  sidebarPosition: number,
): Promise<void> {
  return MoveItemToSidebar(sourceCollectionId, itemId, sidebarPosition);
}

export async function saveResponseBody(
  tempFilePath: string,
  contentType: string,
): Promise<void> {
  return SaveResponseBody(tempFilePath, contentType);
}
