// フロントエンド domain 層の型定義（ユニオン型・型ガードで UI に意味付けする層）。
// 配線型の正は Go ドメイン型 internal/domain/mqtt/types.go で、これが RPC 境界に直接公開され
// Wails 生成型 wailsjs/go/models.ts に反映される。Go 側を変更したら `wails generate module` を
// 実行する（再生成忘れは CI のバインディング鮮度チェックが検知する）。
// 生成型 → この型の変換は infrastructure/mqtt/client.ts で行う。

export interface SubscriptionInfo {
  topic: string;
  qos: number;
}

export interface ConnectionStatus {
  id: string;
  name: string;
  broker: string;
  connected: boolean;
  profileId: string;
  subscriptions: SubscriptionInfo[];
  /** Broker Topics のスキャンが稼働中か。スキャンは専用の接続で行うので subscriptions には現れない。 */
  scanning: boolean;
}

/** mqtt:message のペイロード (Go の MQTTMessage)。 */
export interface MqttRawMessage {
  connectionId: string;
  topic: string;
  payload: string;
  payloadBase64?: boolean;
  qos: number;
  timestamp: number;
}

/** mqtt:scan-topic のペイロード。 */
export interface ScannedTopic {
  connectionId: string;
  topic: string;
}

/** mqtt:subscription-dropped のペイロード。 */
export interface SubscriptionDropped {
  connectionId: string;
  topic: string;
  error: string;
}

/** mqtt:connected・mqtt:disconnected のペイロード。 */
export interface ConnectionEvent {
  connectionId: string;
}

/** mqtt:connection-lost・mqtt:connection-failed・mqtt:scan-stopped のペイロード。 */
export interface ConnectionErrorEvent {
  connectionId: string;
  error: string;
}

/**
 * イベント名 (shared/wails-events の MqttEventName) からペイロード型への対応表。
 * Go 側は internal/application/mqtt の Emit の呼び出し。MQTT のイベントが増えてここに無いと、
 * MqttEventPayloads[E] を使う箇所で tsc がエラーにする。
 */
export interface MqttEventPayloads {
  "mqtt:connected": ConnectionEvent;
  "mqtt:disconnected": ConnectionEvent;
  "mqtt:connection-lost": ConnectionErrorEvent;
  "mqtt:connection-failed": ConnectionErrorEvent;
  "mqtt:message": MqttRawMessage;
  "mqtt:scan-topic": ScannedTopic;
  "mqtt:scan-stopped": ConnectionErrorEvent;
  "mqtt:subscription-dropped": SubscriptionDropped;
}

export interface MqttMessage {
  topic: string;
  payload: string;
  payloadBase64: boolean;
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
  retain: boolean;
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
