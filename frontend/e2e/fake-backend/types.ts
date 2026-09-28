// 偽バックエンドが扱う型。配線型 (wailsjs/go/models.ts) は class なのでオブジェクトリテラルを
// 作れない。フロントエンド domain 層の interface が同じ JSON 形状を表しているのでそちらを使う。
import type {
  Collection,
  HttpResponse,
  RequestBody,
  SidebarEntry,
  TreeItem,
} from "../../src/domain/http/types";
import type {
  BrokerProfile,
  ConnectionStatus,
} from "../../src/domain/mqtt/types";
import type { OpenApiFile } from "../../src/domain/openapi/types";
import type { UdpTarget } from "../../src/domain/udp/types";

/** テストが addInitScript で仕込む初期状態。すべて任意。 */
export interface FakeSeed {
  /** __root__ 以外のコレクション。requests を省くと空のコレクションになる。 */
  collections?: Array<{
    id?: string;
    name: string;
    requests?: Array<{
      id?: string;
      name: string;
      url?: string;
      /** 保存済みのボディ。file 参照の再選択表示などの検証に使う。 */
      body?: RequestBody;
    }>;
  }>;
  /** OpenFilePicker (ファイルダイアログ) で選ばれたことにするファイル。未設定ならキャンセル。 */
  pickedFile?: { token: string; name: string; contentType: string };
  udpTargets?: Array<{ id?: string; name: string; host: string; port: number }>;
  mqttProfiles?: Array<{ id?: string; name: string; broker?: string }>;
  /**
   * "ok" なら MQTT の Connect を成功させ、続けて mqtt:connected を発火する。
   * 未設定なら Connect は必ず失敗する (繋がるブローカーが無い状態)。
   */
  mqttConnect?: "ok";
  /** SendRequest が返すレスポンス (既定値に対する上書き)。 */
  httpResponse?: Partial<HttpResponse>;
  /** SendRequest が解決するまでの遅延 (ms)。ローディング状態の検証に使う。 */
  httpResponseDelayMs?: number;
  /** SendRequest を必ず失敗させる (接続エラーの検証に使う)。 */
  httpError?: string;
  /** UpdateRequest を必ず失敗させる (自動保存の失敗バナーの検証に使う)。 */
  updateRequestError?: string;
  /** SaveResponseBody を必ず失敗させる (回収済み一時ファイルの検証に使う)。 */
  saveResponseError?: string;
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
  /** バックエンドからのイベント発火を模し、EventsOn の購読者に data を渡す。 */
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
