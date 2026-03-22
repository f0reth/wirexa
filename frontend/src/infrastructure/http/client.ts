import {
  AddFolder,
  AddRequest,
  CreateCollection,
  DeleteCollection,
  DeleteItem,
  GetCollections,
  RenameCollection,
  RenameItem,
  SendRequest,
  UpdateRequest,
} from "../../../wailsjs/go/adapters/HttpHandler";
import { httpdomain } from "../../../wailsjs/go/models";
import type {
  Collection,
  HttpMethod,
  HttpRequest,
  HttpResponse,
  KeyValuePair,
  RequestBody,
  TreeItem,
} from "../../domain/http/types";

// domain → Wails
function toWailsRequest(req: HttpRequest): httpdomain.HttpRequest {
  return httpdomain.HttpRequest.createFrom(req);
}

// Wails → domain
function fromWailsKeyValuePair(kv: httpdomain.KeyValuePair): KeyValuePair {
  return { key: kv.key, value: kv.value, enabled: kv.enabled };
}

function fromWailsRequestBody(body: httpdomain.RequestBody): RequestBody {
  return {
    type: body.type as RequestBody["type"],
    content: body.content,
  };
}

function fromWailsHttpRequest(req: httpdomain.HttpRequest): HttpRequest {
  return {
    id: req.id,
    name: req.name,
    method: req.method as HttpMethod,
    url: req.url,
    headers: req.headers.map(fromWailsKeyValuePair),
    params: req.params.map(fromWailsKeyValuePair),
    body: fromWailsRequestBody(req.body),
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
  return {
    type: item.type as "folder" | "request",
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
