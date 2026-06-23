// フロントエンド domain 層の型定義（ユニオン型・型ガードで UI に意味付けする層）。
// 配線型の正は Go ドメイン型 internal/domain/mqtt/types.go で、これが RPC 境界に直接公開され
// Wails 生成型 wailsjs/go/models.ts に反映される。Go 側を変更したら `wails generate module` を
// 実行する（再生成忘れは CI のバインディング鮮度チェックが検知する）。
// 生成型 → この型の変換は infrastructure/mqtt/client.ts で行う。

export interface ConnectionStatus {
  id: string;
  name: string;
  broker: string;
  connected: boolean;
}

export interface MqttMessage {
  topic: string;
  payload: string;
  qos: 0 | 1 | 2;
  timestamp: Date;
}

export interface BrokerProfile {
  id: string;
  name: string;
  broker: string;
  clientId: string;
  username: string;
  password: string;
  useTls: boolean;
}

export interface Subscription {
  id: string;
  topic: string;
  qos: 0 | 1 | 2;
  patternParts?: string[];
  muted: boolean;
}

export interface PublishPreset {
  id: string;
  name: string;
  topic: string;
  payload: string;
  qos: 0 | 1 | 2;
}

interface BaseConnectionState {
  profileId: string;
  profile: BrokerProfile;
}

export interface OfflineConnectionState extends BaseConnectionState {
  readonly type: "offline";
  connectionId: string;
}

export interface OnlineConnectionState extends BaseConnectionState {
  readonly type: "online";
  connectionId: string;
  connected: boolean;
}

export type ConnectionState = OfflineConnectionState | OnlineConnectionState;

/** オンライン接続かつ connected === true かどうかを返す */
export function isConnected(conn: ConnectionState): boolean {
  return conn.type === "online" && conn.connected;
}

export type Tab = "subscribe" | "publish";
