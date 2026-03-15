export interface MqttMessage {
  id: string;
  topic: string;
  payload: string;
  qos: number;
  timestamp: Date;
  direction: "incoming" | "outgoing";
}

export interface Subscription {
  id: string;
  topic: string;
  qos: number;
  /** Pre-split pattern parts for wildcard subscriptions (contains + or #) */
  patternParts?: string[];
}

export interface PublishPreset {
  id: string;
  name: string;
  topic: string;
  payload: string;
  qos: number;
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
