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

export interface ConnectionState {
  connectionId: string;
  profileId: string;
  profile: BrokerProfile;
  connected: boolean;
  subscriptions: Subscription[];
  messages: MqttMessage[];
  selectedMessage: MqttMessage | null;
  autoFollow: boolean;
  brokerTopics: string[];
  brokerTopicsSet: Set<string>;
  isScanning: boolean;
}

export type Tab = "subscribe" | "publish";
