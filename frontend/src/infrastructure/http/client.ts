import {
  AddFolder,
  AddRequest,
  CancelRequest,
  CreateCollection,
  DeleteCollection,
  DeleteItem,
  GetCollections,
  MoveItem,
  RenameCollection,
  RenameItem,
  SendRequest,
  UpdateRequest,
} from "../../../wailsjs/go/adapters/HttpHandler";
import { httpdomain } from "../../../wailsjs/go/models";
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
  type TreeItem,
} from "../../domain/http/types";

// domain → Wails
function toWailsRequest(req: HttpRequest): httpdomain.HttpRequest {
  return httpdomain.HttpRequest.createFrom(req);
}

// Wails → domain
function fromWailsKeyValuePair(kv: httpdomain.KeyValuePair): KeyValuePair {
  return { key: kv.key, value: kv.value, enabled: kv.enabled };
}

function fromWailsRequestSettings(
  settings: httpdomain.RequestSettings | undefined | null,
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

function fromWailsRequestAuth(auth: httpdomain.RequestAuth): RequestAuth {
  return {
    type: auth && isAuthType(auth.type) ? auth.type : "none",
    username: auth?.username ?? "",
    password: auth?.password ?? "",
    token: auth?.token ?? "",
  };
}

function fromWailsRequestBody(body: httpdomain.RequestBody): RequestBody {
  if (!isBodyType(body.type)) {
    throw new Error(`Unknown body type: ${body.type}`);
  }
  return {
    type: body.type,
    contents: (body.contents ?? {}) as RequestBody["contents"],
  };
}

function fromWailsHttpRequest(req: httpdomain.HttpRequest): HttpRequest {
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

function fromWailsHttpResponse(res: httpdomain.HttpResponse): HttpResponse {
  return {
    statusCode: res.statusCode,
    statusText: res.statusText,
    headers: res.headers,
    body: res.body,
    contentType: res.contentType,
    size: res.size,
    timingMs: res.timingMs,
    error: res.error,
  };
}

function fromWailsTreeItem(item: httpdomain.TreeItem): TreeItem {
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

function fromWailsCollection(col: httpdomain.Collection): Collection {
  return {
    id: col.id,
    name: col.name,
    items: col.items.map(fromWailsTreeItem),
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

export async function moveItem(
  collectionId: string,
  itemId: string,
  targetParentId: string,
  position: number,
): Promise<void> {
  return MoveItem(collectionId, itemId, targetParentId, position);
}
