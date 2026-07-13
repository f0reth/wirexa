// 偽バックエンドが扱う型。配線型 (wailsjs/go/models.ts) は class なのでオブジェクトリテラルを
// 作れない。フロントエンド domain 層の interface が同じ JSON 形状を表しているのでそちらを使う。
import type {
  Collection,
  HttpResponse,
  SidebarEntry,
  TreeItem,
} from "../../src/domain/http/types";
import type { BrokerProfile } from "../../src/domain/mqtt/types";
import type { UdpTarget } from "../../src/domain/udp/types";

/** テストが addInitScript で仕込む初期状態。すべて任意。 */
export interface FakeSeed {
  /** __root__ 以外のコレクション。requests を省くと空のコレクションになる。 */
  collections?: Array<{
    id?: string;
    name: string;
    requests?: Array<{ id?: string; name: string; url?: string }>;
  }>;
  udpTargets?: Array<{ id?: string; name: string; host: string; port: number }>;
  mqttProfiles?: Array<{ id?: string; name: string; broker?: string }>;
  /** SendRequest が返すレスポンス (既定値に対する上書き)。 */
  httpResponse?: Partial<HttpResponse>;
  /** SendRequest が解決するまでの遅延 (ms)。ローディング状態の検証に使う。 */
  httpResponseDelayMs?: number;
  /** SendRequest を必ず失敗させる (接続エラーの検証に使う)。 */
  httpError?: string;
}

/** window に生える、偽バックエンドのテスト用操作面。 */
export interface FakeBackend {
  /** バインディング名 → 呼び出し回数。固定 sleep の代わりに expect.poll で待つ。 */
  calls: Record<string, number>;
  /** 現在の状態のスナップショット (構造化クローン可能な形)。 */
  snapshot(): {
    collections: Collection[];
    rootItems: TreeItem[];
    sidebar: SidebarEntry[];
    udpTargets: UdpTarget[];
    mqttProfiles: BrokerProfile[];
  };
}

declare global {
  interface Window {
    __wirexaSeed?: FakeSeed;
    __wirexaFake: FakeBackend;
  }
}
