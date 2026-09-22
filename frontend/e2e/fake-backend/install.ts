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
import type { BrokerProfile } from "../../src/domain/mqtt/types";
import type {
  UdpListenSession,
  UdpSendRequest,
  UdpSendResult,
  UdpTarget,
} from "../../src/domain/udp/types";
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
  files: Record<string, string>;
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
    files: {},
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
    return fn(...callArgs);
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
    db.collections = db.collections.filter((c) => c.id !== id);
    db.sidebar = db.sidebar.filter(
      (e) => !(e.kind === "collection" && e.id === id),
    );
  }),

  RenameCollection: mutates("RenameCollection", (id: string, name: string) => {
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
      const request: HttpRequest = { ...req, id: req.id || newId("req") };
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

const UdpHandler = {
  GetTargets: counted("GetTargets", async () => clone(db.udpTargets)),

  SaveTarget: mutates("SaveTarget", (target: UdpTarget) => {
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

  StartListen: mutates("StartListen", (port: number, encoding: string) => {
    const session = {
      id: newId("listener"),
      port,
      encoding,
    } as UdpListenSession;
    db.listeners.push(session);
    return clone(session);
  }),

  StopListen: mutates("StopListen", (id: string) => {
    db.listeners = db.listeners.filter((l) => l.id !== id);
  }),

  Send: counted(
    "Send",
    async (req: UdpSendRequest): Promise<UdpSendResult> => ({
      bytesSent: req.payload.length,
    }),
  ),

  Shutdown: counted("Shutdown", async () => {}),
};

// ── MqttHandler ───────────────────────────────────────────────────────────────

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

  DeleteProfile: mutates("DeleteProfile", (id: string) => {
    db.mqttProfiles = db.mqttProfiles.filter((p) => p.id !== id);
  }),

  // 接続系は実ブローカーが要るので繋がらないままにする (UI は offline 状態を描く)。
  GetConnections: counted("GetConnections", async () => []),
  Connect: counted("Connect", async () => {
    throw new Error("connection refused");
  }),
  Disconnect: counted("Disconnect", async () => {}),
  Subscribe: counted("Subscribe", async () => {}),
  Unsubscribe: counted("Unsubscribe", async () => {}),
  Publish: counted("Publish", async () => {}),
  Shutdown: counted("Shutdown", async () => {}),
};

// ── OpenAPIHandler / LogHandler / App ─────────────────────────────────────────

const OpenAPIHandler = {
  OpenFilePicker: counted("OpenAPI.OpenFilePicker", async () => ""),
  ReadFile: counted("ReadFile", async (path: string) => db.files[path] ?? ""),
  WriteFile: mutates("WriteFile", (path: string, content: string) => {
    db.files[path] = content;
  }),
  SaveFileAs: counted("SaveFileAs", async () => ""),
  GetRecents: counted("GetRecents", async () => []),
  RemoveRecent: counted("RemoveRecent", async () => {}),
  MoveRecent: counted("MoveRecent", async () => {}),
};

const LogHandler = { Log: counted("Log", async () => {}) };

const App = { ConfirmQuit: counted("ConfirmQuit", async () => {}) };

// ── window への設置 ───────────────────────────────────────────────────────────

const noop = () => {};

// biome-ignore lint/suspicious/noExplicitAny: Wails ランタイムの型は生成コード側にしかない
const w = window as any;

w.runtime = {
  EventsOnMultiple: () => noop,
  EventsOn: () => noop,
  EventsOff: noop,
  EventsOffAll: noop,
  EventsEmit: noop,
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
  snapshot: () => ({
    collections: clone(
      db.collections.filter((c) => c.id !== ROOT_COLLECTION_ID),
    ),
    rootItems: clone(collection(ROOT_COLLECTION_ID).items),
    sidebar: clone(db.sidebar),
    udpTargets: clone(db.udpTargets),
    mqttProfiles: clone(db.mqttProfiles),
  }),
};
