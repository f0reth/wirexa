export interface MqttMessage {
  id: string; // フロントエンドで付与 (UUID)
  topic: string;
  payload: string;
  qos: 0 | 1 | 2;
  timestamp: Date;
  direction: "incoming" | "outgoing";
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
  subscriptions: Subscription[];
  messages: MqttMessage[];
  selectedMessage: MqttMessage | null;
  autoFollow: boolean;
  brokerTopics: string[];
  brokerTopicsSet: Set<string>;
  isScanning: boolean;
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
