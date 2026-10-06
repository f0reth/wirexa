// 偽バックエンドが扱う型。配線型 (wailsjs/go/models.ts) は class なのでオブジェクトリテラルを
// 作れない。フロントエンド domain 層の interface が同じ JSON 形状を表しているのでそちらを使う。
import type {
  Collection,
  HttpRequest,
  HttpResponse,
  SidebarEntry,
  TreeItem,
} from "../../src/domain/http/types";
import type {
  BrokerProfile,
  ConnectionStatus,
} from "../../src/domain/mqtt/types";
import type { OpenApiFile } from "../../src/domain/openapi/types";
import type { UdpTarget } from "../../src/domain/udp/types";

/**
 * seed に置くツリーのアイテム。
 * - リクエスト: name のほかに、method・url・headers・body などリクエストの項目を任意で持つ。
 *   省いた項目は空のリクエストの値になる。
 * - フォルダ: folder が名前で、items が子。
 * id を省くと採番する。
 */
export type SeedItem =
  | ({ name: string; id?: string } & Partial<Omit<HttpRequest, "id" | "name">>)
  | { folder: string; id?: string; items?: SeedItem[] };

/** テストが addInitScript で仕込む初期状態。すべて任意。 */
export interface FakeSeed {
  /** __root__ 以外のコレクション。items を省くと空のコレクションになる。 */
  collections?: Array<{ id?: string; name: string; items?: SeedItem[] }>;
  /** __root__ 直下のアイテム。サイドバーレイアウトの末尾に並ぶ。 */
  rootItems?: SeedItem[];
  /**
   * HTTP のバインディング名 (GetSidebarLayout・UpdateRequest・SendRequest など) → 失敗させる文言。
   * 文字列なら必ず失敗し、times を付けるとその回数だけ失敗してあとは成功する (回数はリロードで
   * 数え直す)。SendRequest は httpResponseDelayMs の遅延のあとで失敗する。
   */
  httpRpcErrors?: Record<string, string | { message: string; times: number }>;
  /** SaveResponseBody が false を返す (保存ダイアログのキャンセル)。 */
  saveResponseBodyCancelled?: boolean;
  /** OpenFilePicker (ファイルダイアログ) で選ばれたことにするファイル。未設定ならキャンセル。 */
  pickedFile?: { token: string; name: string; contentType: string };
  udpTargets?: Array<{ id?: string; name: string; host: string; port: number }>;
  /** UDP の GetTargets を必ず失敗させる (起動時の読み込みの失敗を模す)。 */
  getTargetsError?: string;
  /** broker を省くと tcp://localhost:1883、clientId・username・password は空、useTls は false。 */
  mqttProfiles?: Array<{
    id?: string;
    name: string;
    broker?: string;
    clientId?: string;
    username?: string;
    password?: string;
    useTls?: boolean;
  }>;
  /**
   * 起動時からバックエンドにある MQTT 接続 (GetConnections が返す)。リロードを跨いで残った接続や、
   * プロファイルが削除済みの接続を模す。
   */
  mqttConnections?: ConnectionStatus[];
  /**
   * MQTT の Connect の結果。
   * - 未設定: 接続 ID を返してから mqtt:connection-failed を発火する (繋がるブローカーが無い状態)。
   * - "ok": 接続 ID を返してから mqtt:connected を発火する。
   * - "reject": Connect の RPC 自体を失敗させる。
   * - "pending": 接続 ID を返したあと、確立も失敗もしない (確立待ちが続いている状態)。
   */
  mqttConnect?: "ok" | "reject" | "pending";
  /** Connect の RPC が接続 ID を返すまでの遅延 (ms)。応答を待つ間の操作の検証に使う。 */
  mqttConnectDelayMs?: number;
  /** Connect が接続 ID を返してから、結果のイベントを出すまでの遅延 (ms)。 */
  mqttConnectResultDelayMs?: number;
  /**
   * Connect が接続 ID を返すより先に、結果のイベントを出す。Go は Connect が返る前に接続を
   * 始めるので、実バックエンドでも応答より先に届くことがある。
   */
  mqttConnectResultBeforeResponse?: boolean;
  /**
   * Subscribe が検証のあと、接続を見るまでの遅延 (ms)。再接続の張り直しの購読を、接続が確立した
   * あとに届かせるのに使う (確立済みの接続では subscribeError のトピックが RPC の失敗になる)。
   */
  mqttSubscribeDelayMs?: number;
  /** MQTT の GetConnections を必ず失敗させる (起動時の復元の失敗を模す)。 */
  getConnectionsError?: string;
  /** MQTT の Disconnect を必ず失敗させ、接続は残す (RPC 自体の失敗を模す)。 */
  disconnectError?: string;
  /**
   * ブローカーが topic の購読を拒否する。確立済みの接続への Subscribe は
   * "failed to subscribe: <message>" で失敗する。確立前の Subscribe は登録して成功を返し、確立した
   * ときに mqtt:connected のあとで購読を外して mqtt:subscription-dropped (error は message) を出す。
   */
  subscribeError?: { topic: string; message: string };
  /**
   * 確立済みの接続への Unsubscribe が "failed to unsubscribe: <message>" で失敗する。
   * - keepsSubscription を省くと、購読を外してから失敗する (ブローカーの応答を確認できなかった場合。
   *   Go の ErrAckTimeout)。
   * - keepsSubscription: true なら、購読を残したまま失敗する (それ以外の失敗)。
   */
  unsubscribeError?: { message: string; keepsSubscription?: boolean };
  /**
   * StartTopicScan が解決するまでの遅延 (ms)。待つ間に StopTopicScan か Disconnect が来たら、
   * "topic scan was stopped" で失敗する。
   */
  startTopicScanDelayMs?: number;
  /** MQTT の GetProfiles を必ず失敗させる (起動時の読み込みの失敗を模す)。 */
  getProfilesError?: string;
  /** MQTT の SaveProfile を必ず失敗させる (ディスクへの書き込みの失敗などを模す)。 */
  saveProfileError?: string;
  /** MQTT の DeleteProfile を必ず失敗させる。 */
  deleteProfileError?: string;
  /** SendRequest が返すレスポンス (既定値に対する上書き)。 */
  httpResponse?: Partial<HttpResponse>;
  /** SendRequest が解決するまでの遅延 (ms)。ローディング状態の検証に使う。 */
  httpResponseDelayMs?: number;
  /** StartListen を検証のあとで必ず失敗させる (使用中ポートなど、ソケットを開けない場合を模す)。 */
  startListenError?: string;
  /** StartListen が検証のあと解決するまでの遅延 (ms)。"Starting..." 表示の検証に使う。 */
  startListenDelayMs?: number;
  /** UDP の Send が検証のあと解決するまでの遅延 (ms)。"Sending..." 表示の検証に使う。 */
  udpSendDelayMs?: number;
  /**
   * OpenAPI の最近使ったファイル (この順で並ぶ) とその内容。ダイアログで開いたことがあり、
   * ReadFile / WriteFile で読み書きできる状態から始める。
   */
  openApiFiles?: Array<{ path: string; content: string }>;
  /** OpenAPI の SaveFileAs (保存ダイアログ) で選ばれたことにするパス。未設定ならキャンセル。 */
  saveFileAsPath?: string;
}

/** window に生える、偽バックエンドのテスト用操作面。 */
export interface FakeBackend {
  /** バインディング名 → 呼び出し回数。固定 sleep の代わりに expect.poll で待つ。 */
  calls: Record<string, number>;
  /** バインディング名 → 呼び出しごとの引数。RPC 境界に何が渡ったかを検証する。 */
  args: Record<string, unknown[][]>;
  /**
   * バックエンドからのイベント発火を模し、EventsOn の購読者に data を渡す。
   * 偽バックエンドの状態は変えない (install.ts のイベントの節のコメントを参照)。
   */
  emit(name: string, ...data: unknown[]): void;
  /** 現在の状態のスナップショット (構造化クローン可能な形)。 */
  snapshot(): {
    collections: Collection[];
    rootItems: TreeItem[];
    sidebar: SidebarEntry[];
    udpTargets: UdpTarget[];
    mqttProfiles: BrokerProfile[];
    mqttConnections: ConnectionStatus[];
    openApiRecents: OpenApiFile[];
  };
}

declare global {
  interface Window {
    __wirexaSeed?: FakeSeed;
    __wirexaFake: FakeBackend;
  }
}
