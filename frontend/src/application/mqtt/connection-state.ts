import type {
  BrokerProfile,
  ConnectionStatus,
  MqttMessage,
  OfflineConnectionState,
  OnlineConnectionState,
  Subscription,
} from "../../domain/mqtt/types";

/** UI 表示用に id・direction を付加したアプリケーション層のメッセージ型。 */
export type MqttMessageView = MqttMessage & {
  id: string;
  direction: "incoming" | "outgoing";
};

// Application層が管理するランタイム状態。
// Domain型 (ConnectionState) はブローカー接続の純粋なドメイン概念のみを持つ。
interface ConnectionRuntimeState {
  subscriptions: Subscription[];
  messages: MqttMessageView[];
  selectedMessage: MqttMessageView | null;
  autoFollow: boolean;
  brokerTopics: string[];
  readonly brokerTopicsSet: Set<string>;
  isScanning: boolean;
}

type OfflineStateExt = OfflineConnectionState & ConnectionRuntimeState;
type OnlineStateExt = OnlineConnectionState & ConnectionRuntimeState;
export type ConnectionStateExt = OfflineStateExt | OnlineStateExt;

// オフライン接続の ID 生成ロジックをここに集約
export function offlineId(profileId: string): string {
  return `offline-${profileId}`;
}

function emptyRuntimeState(): ConnectionRuntimeState {
  return {
    subscriptions: [],
    messages: [],
    selectedMessage: null,
    autoFollow: false,
    brokerTopics: [],
    brokerTopicsSet: new Set(),
    isScanning: false,
  };
}

export function makeOfflineState(profile: BrokerProfile): OfflineStateExt {
  return {
    type: "offline",
    connectionId: offlineId(profile.id),
    profileId: profile.id,
    profile: { ...profile },
    ...emptyRuntimeState(),
  };
}

export function makeOnlineState(
  connId: string,
  profile: BrokerProfile,
): OnlineStateExt {
  return {
    type: "online",
    connectionId: connId,
    profileId: profile.id,
    profile: { ...profile },
    connected: false,
    ...emptyRuntimeState(),
  };
}

// プロファイルが既に削除された接続を復元する際に、状態から最小限のプロファイルを合成する。
export function synthesizeProfile(status: ConnectionStatus): BrokerProfile {
  return {
    id: status.profileId || status.id,
    name: status.name,
    broker: status.broker,
    clientId: "",
    username: "",
    password: "",
    useTls: false,
  };
}
