// UI e2e 用の偽バックエンド。Wails が注入する window.go / window.runtime を、メモリ上に状態を
// 持つ実装で置き換える。Go の CollectionService (internal/application/http/collection_service.go)
// の意味論に合わせてある: __root__ 予約コレクション、サイドバーレイアウトへの追従など。
//
// 状態は sessionStorage に載せてある。Playwright はテストごとに新しいブラウザコンテキストを作る
// ので、これで「リロードを跨いで残るが、テストは跨がない」隔離になる。ディスクにも実バックエンド
// にも触れないため fullyParallel で安全に並列実行でき、後始末も要らない。
import {
  DEFAULT_SETTINGS,
  type Collection,
  type FileReference,
  type HttpRequest,
  type HttpResponse,
  ROOT_COLLECTION_ID,
  type SidebarEntry,
  type TreeItem,
} from "../../src/domain/http/types";
import type {
  BrokerProfile,
  ConnectionStatus,
} from "../../src/domain/mqtt/types";
import type { OpenApiFile } from "../../src/domain/openapi/types";
import type {
  UdpListenSession,
  UdpSendRequest,
  UdpSendResult,
  UdpTarget,
} from "../../src/domain/udp/types";
import { WailsEvents } from "../../src/shared/wails-events";
import type { FakeSeed } from "./types";

const clone = <T>(v: T): T => structuredClone(v);

// ── 永続化 (sessionStorage) ───────────────────────────────────────────────────

const STORAGE_KEY = "__wirexa_fake_backend";

interface Db {
  nextId: number;
  collections: Collection[];
  sidebar: SidebarEntry[];
  udpTargets: UdpTarget[];
  listeners: UdpListenSession[];
  mqttProfiles: BrokerProfile[];
  /** バックエンドが持つ MQTT 接続。Go と同じくページのリロードを跨いで残る。 */
  mqttConnections: ConnectionStatus[];
  /** OpenAPI のファイルパス → 内容。ディスク上のファイルを模す。 */
  files: Record<string, string>;
  /** OpenAPI の最近使ったファイル。Go の FileService と同じく order 昇順で持つ。 */
  openApiRecents: OpenApiFile[];
}

function newId(prefix: string): string {
  return `${prefix}-${++db.nextId}`;
}

function blankRequest(id: string, name: string, url: string): HttpRequest {
  return {
    id,
    name,
    method: "GET",
    url,
    headers: [],
    params: [],
    body: { type: "none", contents: {} },
    auth: { type: "none", username: "", password: "", token: "" },
    settings: { ...DEFAULT_SETTINGS },
    doc: "",
  };
}

function seeded(seed: FakeSeed): Db {
  const fresh: Db = {
    nextId: 0,
    collections: [
      { id: ROOT_COLLECTION_ID, name: ROOT_COLLECTION_ID, items: [] },
    ],
    sidebar: [],
    udpTargets: [],
    listeners: [],
    mqttProfiles: [],
    mqttConnections: [],
    files: {},
    openApiRecents: [],
  };
  db = fresh;

  for (const c of seed.collections ?? []) {
    const col: Collection = {
      id: c.id ?? newId("col"),
      name: c.name,
      items: [],
    };
    for (const r of c.requests ?? []) {
      const id = r.id ?? newId("req");
      const request = blankRequest(id, r.name, r.url ?? "");
      col.items.push({
        type: "request",
        id,
        name: r.name,
        children: [],
        request: r.body ? { ...request, body: r.body } : request,
      });
    }
    fresh.collections.push(col);
    fresh.sidebar.push({ kind: "collection", id: col.id });
  }
  for (const t of seed.udpTargets ?? []) {
    fresh.udpTargets.push({ ...t, id: t.id ?? newId("target") });
  }
  for (const p of seed.mqttProfiles ?? []) {
    fresh.mqttProfiles.push({
      id: p.id ?? newId("profile"),
      name: p.name,
      broker: p.broker ?? "tcp://localhost:1883",
      clientId: "",
      username: "",
      password: "",
      useTls: false,
    });
  }
  for (const f of seed.openApiFiles ?? []) {
    fresh.files[f.path] = f.content;
    addOpenApiRecent(f.path);
  }
  return fresh;
}

const seed: FakeSeed = window.__wirexaSeed ?? {};
const stored = sessionStorage.getItem(STORAGE_KEY);

// seeded() が newId 経由で db を参照するため、宣言を済ませてから代入する。
let db: Db;
db = stored ? (JSON.parse(stored) as Db) : seeded(seed);

function save(): void {
  sessionStorage.setItem(STORAGE_KEY, JSON.stringify(db));
}
save();

// ── 呼び出しカウンタ ──────────────────────────────────────────────────────────

const calls: Record<string, number> = {};
const args: Record<string, unknown[][]> = {};

// 記録用に引数を複製する。Wails の createFrom は map を参照のまま渡すため、引数に Solid の
// store proxy が混ざり structuredClone では複製できない。JSON 経由なら proxy も読める。
// 記録の失敗でバインディング本体を落とさないよう、複製できなければそのまま持つ。
function snapshotArgs(callArgs: unknown[]): unknown[] {
  try {
    return JSON.parse(JSON.stringify(callArgs)) as unknown[];
  } catch {
    return callArgs;
  }
}

// バインディング呼び出しを数え、引数を記録する。テストは expect.poll でこのカウンタの増加を
// 待てるので、「自動保存が終わるまで 800ms 寝る」ような固定 sleep が要らなくなる。
function counted<A extends unknown[], R>(
  name: string,
  fn: (...args: A) => R,
): (...args: A) => R {
  return (...callArgs: A) => {
    calls[name] = (calls[name] ?? 0) + 1;
    (args[name] ??= []).push(snapshotArgs(callArgs));
    // 本体にも JSON を通した複製を渡す。実際の Wails も引数を JSON で送るので、呼び出し側の
    // proxy を状態に取り込むと (UpdateRequest で保存した request など)、以降の clone が
    // DataCloneError で落ちて一覧の再取得が失敗する。記録とは別の複製にして、本体が状態を
    // 書き換えても記録済みの引数は変わらないようにする。
    return fn(...(snapshotArgs(callArgs) as A));
  };
}

/** 状態を変えるバインディング: 呼び出しを数え、終わったら sessionStorage に書き戻す。 */
function mutates<A extends unknown[], R>(
  name: string,
  fn: (...args: A) => R,
): (...args: A) => Promise<R> {
  return counted(name, async (...args: A) => {
    const result = fn(...args);
    save();
    return result;
  });
}

// ── ツリー操作 (Go 側 domain/http/types.go の TreeItem 操作に対応) ─────────────

function collection(id: string): Collection {
  const c = db.collections.find((x) => x.id === id);
  if (!c) throw new Error(`collection not found: ${id}`);
  return c;
}

// Go 側の rejectReservedCollection と同じく、予約済みの __root__ に対するコレクション単位の
// 変更（削除・リネーム）を拒否する。中のアイテムの操作は対象外。
function rejectReservedCollection(id: string): void {
  if (id === ROOT_COLLECTION_ID) {
    throw new Error(`reserved collection cannot be modified: ${id}`);
  }
}

function walk(items: TreeItem[]): TreeItem[] {
  return items.flatMap((i) => [i, ...walk(i.children)]);
}

function findNode(col: Collection, id: string): TreeItem | undefined {
  return walk(col.items).find((i) => i.id === id);
}

/** parentId が空文字ならコレクション直下、そうでなければそのフォルダの children。 */
function childrenOf(col: Collection, parentId: string): TreeItem[] | undefined {
  return parentId === "" ? col.items : findNode(col, parentId)?.children;
}

function removeNode(col: Collection, id: string): void {
  const containers = [col.items, ...walk(col.items).map((n) => n.children)];
  for (const container of containers) {
    const index = container.findIndex((i) => i.id === id);
    if (index >= 0) {
      container.splice(index, 1);
      return;
    }
  }
}

// ── HttpHandler ───────────────────────────────────────────────────────────────

// 送信中のリクエスト。CancelRequest が呼ばれたら reject し、Go 側が中断エラーを返すのを模す。
const inFlight = new Map<string, () => void>();

const DEFAULT_RESPONSE: HttpResponse = {
  statusCode: 200,
  statusText: "OK",
  headers: { "Content-Type": ["application/json"] },
  body: '{\n  "message": "ok"\n}',
  contentType: "application/json",
  size: 22,
  timingMs: 1,
  error: "",
  bodyTruncated: false,
  bodyBase64: false,
  bodyCapped: false,
};

// Go の resolveFile と同じく、ダイアログで選ばれていない token や、token の無い
// 保存済み参照ではファイルを送らない。
function fileAccessDenied(req: HttpRequest): boolean {
  const denied = (ref: FileReference | undefined) =>
    !!ref &&
    (ref.token
      ? ref.token !== seed.pickedFile?.token
      : !!ref.name || !!ref.needsReselect);
  if (req.body.type === "file") return denied(req.body.file);
  if (req.body.type === "form-data") {
    return (req.body.formData ?? []).some(
      (row) => row.enabled && row.key && row.kind === "file" && denied(row.file),
    );
  }
  return false;
}

// Go の HTTPRequestService と同じ規則で execution ID を検証する。
// キーの形式を揃えておくことで、偽バックエンドが本物より緩く振る舞わないようにする。
function validExecutionId(executionId: string): boolean {
  return (
    executionId.length > 0 &&
    executionId.length <= 64 &&
    /^[A-Za-z0-9_-]+$/.test(executionId)
  );
}

function sendRequest(
  executionId: string,
  req: HttpRequest,
): Promise<HttpResponse> {
  return new Promise<HttpResponse>((resolve, reject) => {
    if (!validExecutionId(executionId)) {
      reject(new Error(`invalid executionID: ${executionId}`));
      return;
    }
    if (fileAccessDenied(req)) {
      reject(
        new Error(
          "failed to send request: file access denied: select the file again",
        ),
      );
      return;
    }
    const timer = setTimeout(() => {
      inFlight.delete(executionId);
      if (seed.httpError) reject(new Error(seed.httpError));
      else resolve({ ...DEFAULT_RESPONSE, ...seed.httpResponse });
    }, seed.httpResponseDelayMs ?? 0);

    inFlight.set(executionId, () => {
      clearTimeout(timer);
      inFlight.delete(executionId);
      reject(new Error("request canceled"));
    });
  });
}

const HttpHandler = {
  GetCollections: counted("GetCollections", async () =>
    clone(db.collections.filter((c) => c.id !== ROOT_COLLECTION_ID)),
  ),

  GetRootItems: counted("GetRootItems", async () =>
    clone(collection(ROOT_COLLECTION_ID).items),
  ),

  GetSidebarLayout: counted("GetSidebarLayout", async () => clone(db.sidebar)),

  CreateCollection: mutates("CreateCollection", (name: string) => {
    const col: Collection = { id: newId("col"), name, items: [] };
    db.collections.push(col);
    db.sidebar.push({ kind: "collection", id: col.id });
    return clone(col);
  }),

  DeleteCollection: mutates("DeleteCollection", (id: string) => {
    rejectReservedCollection(id);
    db.collections = db.collections.filter((c) => c.id !== id);
    db.sidebar = db.sidebar.filter(
      (e) => !(e.kind === "collection" && e.id === id),
    );
  }),

  RenameCollection: mutates("RenameCollection", (id: string, name: string) => {
    rejectReservedCollection(id);
    collection(id).name = name;
  }),

  AddFolder: mutates(
    "AddFolder",
    (collectionId: string, parentId: string, name: string) => {
      const item: TreeItem = {
        type: "folder",
        id: newId("folder"),
        name,
        children: [],
      };
      childrenOf(collection(collectionId), parentId)?.push(item);
      if (collectionId === ROOT_COLLECTION_ID && parentId === "") {
        db.sidebar.push({ kind: "item", id: item.id });
      }
      return clone(item);
    },
  ),

  AddRequest: mutates(
    "AddRequest",
    (collectionId: string, parentId: string, req: HttpRequest) => {
      // Go 側と同じく、呼び出し側の id は使わず常に採番する。
      const request: HttpRequest = { ...req, id: newId("req") };
      const item: TreeItem = {
        type: "request",
        id: request.id,
        name: request.name,
        children: [],
        request,
      };
      childrenOf(collection(collectionId), parentId)?.push(item);
      if (collectionId === ROOT_COLLECTION_ID && parentId === "") {
        db.sidebar.push({ kind: "item", id: item.id });
      }
      return clone(item);
    },
  ),

  UpdateRequest: mutates(
    "UpdateRequest",
    (collectionId: string, req: HttpRequest) => {
      // seed.updateRequestError で書き込みの失敗 (ディスクの I/O エラーなど) を模す。
      if (seed.updateRequestError) throw new Error(seed.updateRequestError);
      const node = findNode(collection(collectionId), req.id);
      if (!node) return;
      // Go 側と同じく、名前はツリー側が正 (リネームは RenameItem 経由)。
      node.request = { ...req, name: node.name };
    },
  ),

  RenameItem: mutates(
    "RenameItem",
    (collectionId: string, itemId: string, name: string) => {
      const node = findNode(collection(collectionId), itemId);
      if (!node) return;
      node.name = name;
      if (node.request) node.request.name = name;
    },
  ),

  DeleteItem: mutates("DeleteItem", (collectionId: string, itemId: string) => {
    removeNode(collection(collectionId), itemId);
    if (collectionId === ROOT_COLLECTION_ID) {
      db.sidebar = db.sidebar.filter(
        (e) => !(e.kind === "item" && e.id === itemId),
      );
    }
  }),

  MoveItem: mutates(
    "MoveItem",
    (
      sourceCollectionId: string,
      itemId: string,
      targetCollectionId: string,
      targetParentId: string,
      position: number,
    ) => {
      const src = collection(sourceCollectionId);
      const dst = collection(targetCollectionId);
      const item = findNode(src, itemId);
      if (!item) return;

      // 同一コレクション内の前方移動では、削除で 1 つ詰まる分を先に補正する (Go 側と同じ)。
      let index = position;
      if (sourceCollectionId === targetCollectionId && position > 0) {
        const siblings = childrenOf(dst, targetParentId) ?? [];
        const current = siblings.findIndex((i) => i.id === itemId);
        if (current >= 0 && current < position) index--;
      }

      removeNode(src, itemId);
      const target = childrenOf(dst, targetParentId);
      if (!target) return;
      if (index < 0 || index > target.length) target.push(item);
      else target.splice(index, 0, item);
    },
  ),

  MoveSidebarEntry: mutates(
    "MoveSidebarEntry",
    (kind: string, id: string, position: number) => {
      const current = db.sidebar.findIndex(
        (e) => e.kind === kind && e.id === id,
      );
      if (current < 0) return;
      const [entry] = db.sidebar.splice(current, 1);
      const index = current < position ? position - 1 : position;
      db.sidebar.splice(
        Math.max(0, Math.min(index, db.sidebar.length)),
        0,
        entry,
      );
    },
  ),

  MoveItemToSidebar: mutates(
    "MoveItemToSidebar",
    (sourceCollectionId: string, itemId: string, sidebarPosition: number) => {
      const src = collection(sourceCollectionId);
      const item = findNode(src, itemId);
      if (!item) return;
      removeNode(src, itemId);
      collection(ROOT_COLLECTION_ID).items.push(item);
      db.sidebar.splice(
        Math.max(0, Math.min(sidebarPosition, db.sidebar.length)),
        0,
        { kind: "item", id: itemId },
      );
    },
  ),

  SendRequest: counted(
    "SendRequest",
    (executionId: string, req: HttpRequest) => sendRequest(executionId, req),
  ),

  CancelRequest: counted("CancelRequest", async (executionId: string) => {
    inFlight.get(executionId)?.();
  }),

  // ダイアログで seed.pickedFile が選ばれたものとして返す (未設定ならキャンセル)。
  // hint は Go 側と同じく初期位置にしか使わないので、戻り値に影響しない。
  OpenFilePicker: counted(
    "OpenFilePicker",
    async (_hint: string) =>
      seed.pickedFile ?? { token: "", name: "", contentType: "" },
  ),
  // Go 側と同じく execution ID だけを受け取る。seed.saveResponseError で
  // 回収済み (TTL・上限) の一時ファイルを模す。
  SaveResponseBody: counted("SaveResponseBody", async (_executionId: string) => {
    if (seed.saveResponseError) throw new Error(seed.saveResponseError);
    return true;
  }),
  DiscardResponseBody: counted(
    "DiscardResponseBody",
    async (_executionId: string) => {},
  ),
  SaveResponseBase64: counted("SaveResponseBase64", async () => {}),
};

// ── UdpHandler ────────────────────────────────────────────────────────────────

// Go の ValidationError.Error() と同じ "invalid <field>: <message>" の形で失敗させる。
function validationError(field: string, message: string): Error {
  return new Error(`invalid ${field}: ${message}`);
}

function validPort(port: number): boolean {
  return Number.isInteger(port) && port >= 1 && port <= 65535;
}

// Go の UDPTarget.Validate / UDPSendRequest.Validate (internal/domain/udp/types.go) と同じ
// host/port 規則。空白だけのホストは Go 側も通すので、ここでも trim しない。
function validateHostPort(host: string, port: number): void {
  if (host === "") throw validationError("host", "is required");
  if (!validPort(port)) throw validationError("port", "must be 1-65535");
}

function isValidJson(text: string): boolean {
  try {
    JSON.parse(text);
    return true;
  } catch {
    return false;
  }
}

/** seed の遅延 (ms)。未設定なら待たない。 */
function delay(ms: number | undefined): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms ?? 0));
}

const UdpHandler = {
  GetTargets: counted("GetTargets", async () => clone(db.udpTargets)),

  SaveTarget: mutates("SaveTarget", (target: UdpTarget) => {
    // Go の TargetService.SaveTarget と同じく、保存より先に検証する。
    validateHostPort(target.host, target.port);
    const index = db.udpTargets.findIndex((t) => t.id === target.id);
    // Go 側と同じく、未知の非空 ID は新規作成として受理しない。
    if (target.id && index < 0) throw new Error(`target not found: ${target.id}`);
    const saved: UdpTarget = { ...target, id: target.id || newId("target") };
    if (index >= 0) db.udpTargets[index] = saved;
    else db.udpTargets.push(saved);
    return clone(saved);
  }),

  DeleteTarget: mutates("DeleteTarget", (id: string) => {
    db.udpTargets = db.udpTargets.filter((t) => t.id !== id);
  }),

  GetListeners: counted("GetListeners", async () => clone(db.listeners)),

  // Go の UDPListenerService.StartListen (internal/application/udp/listener_service.go) と
  // 同じ順に、ポート範囲・エンコーディング・同じポートのセッションの有無を検証する。
  // seed.startListenDelayMs と seed.startListenError はそのあとのソケットを開く段階を模す
  // (開くまでの時間と、使用中のポートなどでの失敗)。待つ間に状態を書き戻さないよう、
  // mutates ではなく自分で save する。
  StartListen: counted(
    "StartListen",
    async (port: number, encoding: string): Promise<UdpListenSession> => {
      if (!validPort(port)) throw validationError("port", "must be 1-65535");
      if (!["text", "json", "fixed"].includes(encoding)) {
        throw validationError("encoding", `unknown: ${encoding}`);
      }
      if (db.listeners.some((l) => l.port === port)) {
        throw validationError("port", `port ${port} is already listening`);
      }
      await delay(seed.startListenDelayMs);
      if (seed.startListenError) throw new Error(seed.startListenError);
      const session = {
        id: newId("listener"),
        port,
        encoding,
      } as UdpListenSession;
      db.listeners.push(session);
      save();
      return clone(session);
    },
  ),

  StopListen: mutates("StopListen", (id: string) => {
    db.listeners = db.listeners.filter((l) => l.id !== id);
  }),

  // Go の UDPSendService.Send (internal/application/udp/send_service.go) のうち、
  // UDPSendRequest.Validate と json エンコーディングの JSON 検証 (DecodePayload) までを模す。
  // 固定長ペイロードの組み立て (DecodeFixedLengthPayload) の検証は再現しない。
  // seed.udpSendDelayMs はソケットへの送信にかかる時間を模す。
  Send: counted(
    "Send",
    async (req: UdpSendRequest): Promise<UdpSendResult> => {
      validateHostPort(req.host, req.port);
      if (req.encoding === "json" && !isValidJson(req.payload)) {
        throw validationError("payload", "invalid JSON");
      }
      await delay(seed.udpSendDelayMs);
      return { bytesSent: req.payload.length };
    },
  ),
};

// ── MqttHandler ───────────────────────────────────────────────────────────────

// Go のトピック検証 (internal/domain/mqtt/topic.go) と同じ規則。
const MAX_TOPIC_BYTES = 65535;

function validateTopicString(topic: string): void {
  if (topic === "") throw validationError("topic", "is required");
  if (new TextEncoder().encode(topic).length > MAX_TOPIC_BYTES) {
    throw validationError("topic", "must be at most 65535 bytes");
  }
  if (topic.includes("\0")) {
    throw validationError("topic", "must not contain the null character");
  }
}

/** Publish のトピック名 (ValidateTopicName)。ワイルドカードを含めない。 */
function validateTopicName(topic: string): void {
  validateTopicString(topic);
  if (/[+#]/.test(topic)) {
    throw validationError("topic", "must not contain wildcards (+ or #)");
  }
}

/** Subscribe / Unsubscribe のトピックフィルター (ValidateTopicFilter)。 */
function validateTopicFilter(filter: string): void {
  validateTopicString(filter);
  const levels = filter.split("/");
  levels.forEach((level, i) => {
    if (level.includes("#") && (level !== "#" || i !== levels.length - 1)) {
      throw validationError("topic", "# must occupy the last level entirely");
    }
    if (level.includes("+") && level !== "+") {
      throw validationError("topic", "+ must occupy an entire level");
    }
  });
}

/** UseTLS のときに TLS で接続できるスキームか (Go の ValidateBrokerScheme と同じ規則)。 */
function validateBrokerScheme(broker: string, useTLS: boolean): void {
  if (!useTLS) return;
  const index = broker.indexOf("://");
  if (index < 0) return;
  const scheme = broker.slice(0, index).toLowerCase();
  if (!["tcp", "mqtt", "ws", "ssl", "tls", "mqtts", "wss"].includes(scheme)) {
    throw validationError("broker URL", "scheme cannot be used with TLS");
  }
}

function validateQos(qos: number): void {
  if (qos > 2) throw validationError("qos", "must be 0, 1, or 2");
}

/** Go の withConn と同じく、無い接続への操作は NotFoundError の文言で失敗させる。 */
function mqttConnection(id: string): ConnectionStatus {
  const conn = db.mqttConnections.find((c) => c.id === id);
  if (!conn) throw new Error(`connection not found: ${id}`);
  return conn;
}

const MqttHandler = {
  GetProfiles: counted("GetProfiles", async () => clone(db.mqttProfiles)),

  SaveProfile: mutates("SaveProfile", (profile: BrokerProfile) => {
    const index = db.mqttProfiles.findIndex((p) => p.id === profile.id);
    // Go 側と同じく、未知の非空 ID は新規作成として受理しない。
    if (profile.id && index < 0) {
      throw new Error(`profile not found: ${profile.id}`);
    }
    const saved: BrokerProfile = {
      ...profile,
      id: profile.id || newId("profile"),
    };
    if (index >= 0) db.mqttProfiles[index] = saved;
    else db.mqttProfiles.push(saved);
    return clone(saved);
  }),

  // Go 側と同じく、未知の ID は NotFoundError の文言で失敗させる。
  DeleteProfile: mutates("DeleteProfile", (id: string) => {
    if (!db.mqttProfiles.some((p) => p.id === id)) {
      throw new Error(`profile not found: ${id}`);
    }
    db.mqttProfiles = db.mqttProfiles.filter((p) => p.id !== id);
  }),

  GetConnections: counted("GetConnections", async () =>
    clone(db.mqttConnections),
  ),

  // Go の MQTTService.Connect (internal/application/mqtt/service.go) と同じく接続 ID を先に返し、
  // 結果はあとからイベントで知らせる。UI は戻り値で接続を作ってからイベントを受けるので、
  // 戻り値が届いたあと (次のタスク) に発火する。
  // - 既定: 繋がるブローカーが無いものとして、接続を消してから mqtt:connection-failed を出す
  //   (Go は失敗した接続を一覧から外す)。
  // - seed.mqttConnect が "ok": 確立して mqtt:connected を出す。
  // - seed.mqttConnect が "reject": RPC 自体を失敗させる (Go では終了処理中の Connect に当たる)。
  Connect: counted(
    "Connect",
    async (config: {
      name: string;
      broker: string;
      profileId: string;
      useTLS: boolean;
    }): Promise<string> => {
      if (config.broker === "") {
        throw validationError("broker URL", "is required");
      }
      validateBrokerScheme(config.broker, config.useTLS);
      if (seed.mqttConnect === "reject") throw new Error("connection refused");
      const conn: ConnectionStatus = {
        id: newId("conn"),
        name: config.name,
        broker: config.broker,
        connected: false,
        profileId: config.profileId,
        subscriptions: [],
        scanning: false,
      };
      db.mqttConnections.push(conn);
      save();
      setTimeout(() => {
        // 結果が出るより先に切断された接続はイベントを出さない (Go の runConnect / onConnected と同じ)。
        const live = db.mqttConnections.find((c) => c.id === conn.id);
        if (!live) return;
        if (seed.mqttConnect !== "ok") {
          db.mqttConnections = db.mqttConnections.filter(
            (c) => c.id !== conn.id,
          );
          save();
          emitEvent(WailsEvents.mqttConnectionFailed, {
            connectionId: conn.id,
            error: "connection refused",
          });
          return;
        }
        live.connected = true;
        save();
        emitEvent(WailsEvents.mqttConnected, { connectionId: conn.id });
      });
      return conn.id;
    },
  ),

  Disconnect: mutates("Disconnect", (connectionId: string) => {
    mqttConnection(connectionId);
    db.mqttConnections = db.mqttConnections.filter(
      (c) => c.id !== connectionId,
    );
    emitEvent(WailsEvents.mqttDisconnected, { connectionId });
  }),

  // ブローカーが無いのでメッセージは届かない。受信はテストが mqtt:message を emit して模す。
  Subscribe: mutates(
    "Subscribe",
    (connectionId: string, topic: string, qos: number) => {
      validateTopicFilter(topic);
      validateQos(qos);
      const conn = mqttConnection(connectionId);
      // Go は購読を topic → qos の map で持つので、同じトピックは QoS を上書きする。
      conn.subscriptions = [
        ...conn.subscriptions.filter((s) => s.topic !== topic),
        { topic, qos },
      ];
    },
  ),

  Unsubscribe: mutates("Unsubscribe", (connectionId: string, topic: string) => {
    validateTopicFilter(topic);
    const conn = mqttConnection(connectionId);
    conn.subscriptions = conn.subscriptions.filter((s) => s.topic !== topic);
  }),

  // Go はスキャン用の接続で # を購読するが、ここでは scanning を切り替えるだけ (購読は増えない)。
  // 見つかったトピックはテストが mqtt:scan-topic を、スキャン用の接続の切断は mqtt:scan-stopped を
  // emit して模す。
  StartTopicScan: mutates("StartTopicScan", (connectionId: string) => {
    mqttConnection(connectionId).scanning = true;
  }),

  // Go と同じく、スキャンしていなければ何もしない。
  StopTopicScan: mutates("StopTopicScan", (connectionId: string) => {
    mqttConnection(connectionId).scanning = false;
  }),

  // ループバックはしない。送った内容は fake.args("Publish") で確かめる。
  Publish: counted(
    "Publish",
    async (
      connectionId: string,
      topic: string,
      _payload: string,
      qos: number,
      _retain: boolean,
    ) => {
      validateTopicName(topic);
      validateQos(qos);
      mqttConnection(connectionId);
    },
  ),
};

// ── OpenAPIHandler / LogHandler / App ─────────────────────────────────────────

// Go の FileService (internal/application/openapi/file_service.go) に合わせる。
// 許可リストは recents に載っているパスで代用する (Go は recents の保存に失敗したときだけ
// 一覧に無いパスの許可を残すが、偽バックエンドの保存は失敗しない)。パスの正規化
// (filepath.Clean) と 50 件の上限は省く。

function basename(path: string): string {
  return path.split(/[\\/]/).pop() ?? path;
}

function reindexOpenApiRecents(): void {
  db.openApiRecents.forEach((r, i) => {
    r.order = i;
  });
}

/** Go の addRecentLocked: 既存なら lastOpenedAt だけ更新し、無ければ末尾に足す。 */
function addOpenApiRecent(path: string): void {
  const now = new Date().toISOString();
  const existing = db.openApiRecents.find((r) => r.path === path);
  if (existing) {
    existing.lastOpenedAt = now;
    return;
  }
  db.openApiRecents.push({
    path,
    name: basename(path),
    order: db.openApiRecents.length,
    lastOpenedAt: now,
  });
}

function checkOpenApiGranted(path: string): void {
  if (!db.openApiRecents.some((r) => r.path === path)) {
    throw new Error("access denied: path was not granted via a file dialog");
  }
}

const OpenAPIHandler = {
  OpenFilePicker: counted("OpenAPI.OpenFilePicker", async () => ""),
  ReadFile: counted("ReadFile", async (path: string) => {
    checkOpenApiGranted(path);
    const content = db.files[path];
    if (content === undefined) throw new Error("file does not exist");
    return content;
  }),
  WriteFile: mutates("WriteFile", (path: string, content: string) => {
    checkOpenApiGranted(path);
    db.files[path] = content;
  }),
  // 保存ダイアログで seed.saveFileAsPath が選ばれたものとして書き込み、recents に載せる
  // (未設定ならキャンセルとして空文字を返す)。
  SaveFileAs: mutates("SaveFileAs", (_defaultName: string, content: string) => {
    const path = seed.saveFileAsPath;
    if (!path) return "";
    db.files[path] = content;
    addOpenApiRecent(path);
    return path;
  }),
  GetRecents: counted("GetRecents", async () => clone(db.openApiRecents)),
  RemoveRecent: mutates("RemoveRecent", (path: string) => {
    db.openApiRecents = db.openApiRecents.filter((r) => r.path !== path);
    reindexOpenApiRecents();
  }),
  // index は移動前の一覧に対する挿入先で、負または範囲外なら末尾へ移す (Go の MoveRecent)。
  MoveRecent: mutates("MoveRecent", (path: string, index: number) => {
    const from = db.openApiRecents.findIndex((r) => r.path === path);
    if (from === -1) return;
    const [item] = db.openApiRecents.splice(from, 1);
    const to = from < index ? index - 1 : index;
    if (to < 0 || to > db.openApiRecents.length) db.openApiRecents.push(item);
    else db.openApiRecents.splice(to, 0, item);
    reindexOpenApiRecents();
  }),
};

const LogHandler = { Log: counted("Log", async () => {}) };

const App = { ConfirmQuit: counted("ConfirmQuit", async () => {}) };

// ── イベント ──────────────────────────────────────────────────────────────────

// Wails ランタイム (runtime/desktop/js の events.js) と同じ意味論で購読を持つ。
// maxCallbacks が -1 なら無制限、正の数ならその回数だけ呼んだら外れる。
// テストは window.__wirexaFake.emit で、バックエンドが発火したイベントを模す。
interface EventListener {
  callback: (...data: unknown[]) => void;
  remaining: number;
}

const eventListeners = new Map<string, EventListener[]>();

function eventsOnMultiple(
  name: string,
  callback: (...data: unknown[]) => void,
  maxCallbacks: number,
): () => void {
  const listener: EventListener = { callback, remaining: maxCallbacks };
  eventListeners.set(name, [...(eventListeners.get(name) ?? []), listener]);
  return () => {
    const rest = (eventListeners.get(name) ?? []).filter((l) => l !== listener);
    if (rest.length > 0) eventListeners.set(name, rest);
    else eventListeners.delete(name);
  };
}

function emitEvent(name: string, ...data: unknown[]): void {
  const listeners = eventListeners.get(name);
  if (!listeners) return;
  // 呼び出し中の購読・解除の影響を受けないよう、先に残る購読を確定させてから呼ぶ。
  const rest = listeners.filter(
    (l) => l.remaining === -1 || --l.remaining > 0,
  );
  if (rest.length > 0) eventListeners.set(name, rest);
  else eventListeners.delete(name);
  for (const l of listeners) l.callback(...clone(data));
}

// ── window への設置 ───────────────────────────────────────────────────────────

const noop = () => {};

// biome-ignore lint/suspicious/noExplicitAny: Wails ランタイムの型は生成コード側にしかない
const w = window as any;

w.runtime = {
  EventsOnMultiple: eventsOnMultiple,
  EventsOn: (name: string, callback: (...data: unknown[]) => void) =>
    eventsOnMultiple(name, callback, -1),
  EventsOff: (...names: string[]) => {
    for (const name of names) eventListeners.delete(name);
  },
  EventsOffAll: () => eventListeners.clear(),
  // 実際の Wails もフロントエンドからの発火を同じウィンドウの購読者に届ける。
  EventsEmit: emitEvent,
  LogPrint: noop,
  LogTrace: noop,
  LogDebug: noop,
  LogInfo: noop,
  LogWarning: noop,
  LogError: noop,
  LogFatal: noop,
  ClipboardGetText: async () => "",
  ClipboardSetText: async () => true,
};

w.go = {
  // 生成される Wails バインディングのキーに合わせる (Http* → HTTP* リネーム後)。
  adapters: {
    HTTPHandler: HttpHandler,
    UDPHandler: UdpHandler,
    MQTTHandler: MqttHandler,
    OpenAPIHandler,
    LogHandler,
  },
  main: { App },
};

window.__wirexaFake = {
  calls,
  args,
  emit: emitEvent,
  snapshot: () => ({
    collections: clone(
      db.collections.filter((c) => c.id !== ROOT_COLLECTION_ID),
    ),
    rootItems: clone(collection(ROOT_COLLECTION_ID).items),
    sidebar: clone(db.sidebar),
    udpTargets: clone(db.udpTargets),
    mqttProfiles: clone(db.mqttProfiles),
    mqttConnections: clone(db.mqttConnections),
    openApiRecents: clone(db.openApiRecents),
  }),
};
