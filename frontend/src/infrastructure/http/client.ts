import {
  AddFolder,
  AddRequest,
  CancelRequest,
  CreateCollection,
  DeleteCollection,
  DeleteItem,
  DiscardResponseBody,
  GetCollections,
  GetRootItems,
  GetSidebarLayout,
  MoveItem,
  MoveItemToSidebar,
  MoveSidebarEntry,
  OpenFilePicker,
  RenameCollection,
  RenameItem,
  SaveResponseBase64,
  SaveResponseBody,
  SendRequest,
  UpdateRequest,
} from "../../../wailsjs/go/adapters/HTTPHandler";
import { httpdomain } from "../../../wailsjs/go/models";
import {
  type Collection,
  DEFAULT_SETTINGS,
  type FileReference,
  type FormRow,
  type HttpRequest,
  type HttpResponse,
  isAuthType,
  isBodyType,
  isFormRowKind,
  isHttpMethod,
  type KeyValuePair,
  type RequestAuth,
  type RequestBody,
  type RequestSettings,
  type SidebarEntry,
  type TreeItem,
} from "../../domain/http/types";

// domain → Wails
function toWailsRequest(req: HttpRequest): httpdomain.HTTPRequest {
  return httpdomain.HTTPRequest.createFrom(req);
}

// Wails → domain
function fromWailsKeyValuePair(kv: httpdomain.KeyValuePair): KeyValuePair {
  return { key: kv.key, value: kv.value, enabled: kv.enabled };
}

function fromWailsRequestSettings(
  settings: httpdomain.RequestSettings | undefined | null,
): RequestSettings {
  if (!settings) return { ...DEFAULT_SETTINGS };
  // スプレッドで素通しし、Go 側がプリミティブ項目を足しても黙って落ちないようにする。
  // ユニオン型の proxyMode のみ明示的に絞り込む。
  return {
    ...settings,
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

// ファイル参照が空（未選択）なら undefined にし、UI が未選択と参照ありを区別できるようにする。
function fromWailsFileReference(
  ref: httpdomain.FileReference | undefined | null,
): FileReference | undefined {
  if (!ref || (!ref.token && !ref.name)) return undefined;
  return {
    token: ref.token ?? "",
    name: ref.name ?? "",
    contentType: ref.contentType ?? "",
    needsReselect: ref.needsReselect ?? false,
  };
}

// Go の kind は string なのでユニオンへ絞り込む。未設定（kind 導入前の行）と
// 未知の値はどちらも text 相当として扱い、行を捨てない。
function fromWailsFormRow(row: httpdomain.FormRow): FormRow {
  return {
    ...row,
    kind: row.kind && isFormRowKind(row.kind) ? row.kind : "text",
    file: fromWailsFileReference(row.file),
    contentType: row.contentType ?? "",
  };
}

function fromWailsRequestBody(body: httpdomain.RequestBody): RequestBody {
  if (!isBodyType(body.type)) {
    throw new Error(`Unknown body type: ${body.type}`);
  }
  // 旧データの行復元は Go 側のロード時に済んでいるため、ここでは素通しする。
  return {
    ...body,
    type: body.type,
    contents: (body.contents ?? {}) as RequestBody["contents"],
    formData: body.formData?.map(fromWailsFormRow),
    formUrlEncoded: body.formUrlEncoded?.map(fromWailsFormRow),
    file: fromWailsFileReference(body.file),
  };
}

function fromWailsHttpRequest(req: httpdomain.HTTPRequest): HttpRequest {
  if (!isHttpMethod(req.method)) {
    throw new Error(`Unknown HTTP method: ${req.method}`);
  }
  // スプレッドで素通しし、新規プリミティブ項目が黙って落ちないようにする。
  // ユニオン型・入れ子のフィールドのみ明示的に変換/絞り込む。
  return {
    ...req,
    method: req.method,
    headers: req.headers.map(fromWailsKeyValuePair),
    params: req.params.map(fromWailsKeyValuePair),
    body: fromWailsRequestBody(req.body),
    auth: fromWailsRequestAuth(req.auth),
    settings: fromWailsRequestSettings(req.settings),
    doc: req.doc ?? "",
  };
}

function fromWailsHttpResponse(res: httpdomain.HTTPResponse): HttpResponse {
  return {
    ...res,
    headers: res.headers ?? {},
    bodyTruncated: res.bodyTruncated ?? false,
    bodyBase64: res.bodyBase64 ?? false,
    bodyCapped: res.bodyCapped ?? false,
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

// openFilePicker はネイティブのファイル選択ダイアログを開き、選択されたファイルの参照を返す。
// hint は入力欄の文字列で、ダイアログの初期位置にだけ使われる（許可にはならない）。
// token はダイアログの選択結果からだけ発行され、キャンセル時は undefined を返す。
// presentation 層が wailsjs バインディングを直接叩かないようインフラ層でラップする。
export async function openFilePicker(
  hint: string,
): Promise<FileReference | undefined> {
  const selected = await OpenFilePicker(hint);
  if (!selected?.token) return undefined;
  return {
    token: selected.token,
    name: selected.name,
    contentType: selected.contentType,
  };
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

// saveResponseBody は切り詰められたレスポンスの全文を、送信時の execution ID で保存する。
// 保存元の一時ファイルは backend が追跡しており、パスはここを通らない。
// 保存したら true、保存ダイアログをキャンセルしたら false（再度保存できる）。
export async function saveResponseBody(executionId: string): Promise<boolean> {
  return SaveResponseBody(executionId);
}

// discardResponseBody は不要になった切り詰めレスポンスの一時ファイルを破棄させる。
export async function discardResponseBody(executionId: string): Promise<void> {
  return DiscardResponseBody(executionId);
}

// saveResponseBinary はメモリ上の base64 ボディを保存ダイアログの選択先へ書き出す。
// 切り詰められていない非 UTF-8 レスポンスの保存に使う (temp ファイルを介さない)。
export async function saveResponseBinary(
  base64Content: string,
  contentType: string,
): Promise<void> {
  return SaveResponseBase64(base64Content, contentType);
}
