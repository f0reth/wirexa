import type { httpdomain } from "../models";
import { store } from "../../store";

export function AddFolder(
  collectionId: string,
  _parentId: string,
  name: string,
): Promise<httpdomain.TreeItem> {
  const item: httpdomain.TreeItem = {
    type: "folder",
    id: crypto.randomUUID(),
    name,
    children: [],
  };
  const col = store.collections.find((c) => c.id === collectionId);
  if (col) {
    col.items.push(item);
  }
  return Promise.resolve(item);
}

export function AddRequest(
  collectionId: string,
  _parentId: string,
  request: httpdomain.HttpRequest,
): Promise<httpdomain.TreeItem> {
  const item: httpdomain.TreeItem = {
    type: "request",
    id: crypto.randomUUID(),
    name: request.name,
    children: [],
    request,
  };
  const col = store.collections.find((c) => c.id === collectionId);
  if (col) {
    col.items.push(item);
  }
  return Promise.resolve(item);
}

export function CancelRequest(): Promise<void> {
  return Promise.resolve();
}

export function CreateCollection(name: string): Promise<httpdomain.Collection> {
  const collection: httpdomain.Collection = {
    id: crypto.randomUUID(),
    name,
    items: [],
  };
  store.collections.push(collection);
  return Promise.resolve(collection);
}

export function DeleteCollection(id: string): Promise<void> {
  store.collections = store.collections.filter((c) => c.id !== id);
  return Promise.resolve();
}

export function DeleteItem(collectionId: string, itemId: string): Promise<void> {
  const col = store.collections.find((c) => c.id === collectionId);
  if (col) {
    col.items = col.items.filter((i) => i.id !== itemId);
  }
  return Promise.resolve();
}

export function GetCollections(): Promise<Array<httpdomain.Collection>> {
  return Promise.resolve([...store.collections]);
}

export function RenameCollection(id: string, name: string): Promise<void> {
  const col = store.collections.find((c) => c.id === id);
  if (col) {
    col.name = name;
  }
  return Promise.resolve();
}

export function RenameItem(
  _collectionId: string,
  itemId: string,
  name: string,
): Promise<void> {
  for (const col of store.collections) {
    const item = col.items.find((i) => i.id === itemId);
    if (item) {
      item.name = name;
      break;
    }
  }
  return Promise.resolve();
}

export function SendRequest(
  _request: httpdomain.HttpRequest,
): Promise<httpdomain.HttpResponse> {
  return Promise.resolve(
    store.httpResponse ?? {
      statusCode: 200,
      statusText: "OK",
      headers: {},
      body: "",
      contentType: "text/plain",
      size: 0,
      timingMs: 0,
      error: "",
    },
  );
}

export function UpdateRequest(
  _collectionId: string,
  _request: httpdomain.HttpRequest,
): Promise<void> {
  return Promise.resolve();
}
