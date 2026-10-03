import {
  type Accessor,
  createContext,
  type JSX,
  onMount,
  type Setter,
  useContext,
} from "solid-js";
import { createConnectionsState } from "../../application/mqtt/connections";
import { createMessagesState } from "../../application/mqtt/messages";
import { createPresetsState } from "../../application/mqtt/presets";
import { createProfilesState } from "../../application/mqtt/profiles";
import { createPublishState } from "../../application/mqtt/publish";
import { createSubscriptionsState } from "../../application/mqtt/subscriptions";
import { notify } from "../../application/ui/notifications";
import { MQTT_MAX_MESSAGES, MQTT_MAX_TOPICS } from "../../config/limits";
import type {
  BrokerProfile,
  ConnectionState,
  Tab,
} from "../../domain/mqtt/types";
import { createLogger } from "../../infrastructure/logger/client";
import * as mqttClient from "../../infrastructure/mqtt/client";
import { onMqttEvent } from "../../infrastructure/mqtt/events";
import {
  createLastProfileStorage,
  createPresetsStorage,
  createProfileOrderStorage,
} from "../../infrastructure/storage/local-storage";

// --- MqttConnectionContext ---
export interface ConnectionContextValue {
  profiles: Accessor<BrokerProfile[]>;
  saveProfile: (p: BrokerProfile) => Promise<BrokerProfile>;
  deleteProfile: (id: string) => Promise<void>;
  connections: Record<string, ConnectionState>;
  activeConnectionId: Accessor<string | null>;
  activeConnection: Accessor<ConnectionState | null>;
  activeTab: Accessor<Tab>;
  setActiveTab: Setter<Tab>;
  createOfflineConnection: (profile: BrokerProfile) => void;
  handleConnect: (profileId: string) => Promise<void>;
  handleDisconnect: (connectionId: string) => Promise<void>;
  handleReconnect: (
    connectionId: string,
    profile?: BrokerProfile,
  ) => Promise<void>;
  closeConnection: (connectionId: string) => void;
  switchConnection: (id: string) => void;
  updateConnectionBroker: (connectionId: string, broker: string) => void;
  reorderProfiles: (fromIndex: number, toIndex: number) => void;
}

// 以下の 3 つは application の state をそのまま公開するので、型も戻り値から作る
// (application 側に項目を足せば、コンポーネントからも見える)。
export type SubscribeContextValue = ReturnType<typeof createSubscriptionsState>;

// 一覧に出すのは visibleMessages なので、フィルター前の messages は公開しない。
export type MessagesContextValue = Omit<
  ReturnType<typeof createMessagesState>,
  "messages"
>;

export type PublishContextValue = ReturnType<typeof createPresetsState> &
  ReturnType<typeof createPublishState>;

type MqttContextValue = ConnectionContextValue &
  SubscribeContextValue &
  MessagesContextValue &
  PublishContextValue;

const MqttContext = createContext<MqttContextValue>();

export function MqttProvider(props: { children: JSX.Element }) {
  const {
    profiles,
    loadProfiles,
    saveProfile,
    deleteProfile,
    reorderProfiles,
  } = createProfilesState(mqttClient, notify, createProfileOrderStorage());
  const mqttLogger = createLogger("frontend:mqtt");
  const connState = createConnectionsState(
    mqttClient,
    onMqttEvent,
    createLastProfileStorage(),
    profiles,
    saveProfile,
    mqttLogger,
    notify,
    MQTT_MAX_MESSAGES,
    MQTT_MAX_TOPICS,
  );

  const subsState = createSubscriptionsState(
    connState.activeConnection,
    connState.updateConnection,
    mqttClient,
    onMqttEvent,
    mqttLogger,
    notify,
  );
  const msgState = createMessagesState(
    connState.activeConnection,
    connState.updateConnection,
  );
  const { messages: _messages, ...messagesContext } = msgState;
  const presetState = createPresetsState(createPresetsStorage());
  const publishState = createPublishState(
    mqttClient,
    connState.activeConnection,
    presetState.draft,
    notify,
  );

  onMount(async () => {
    // プロファイルをロードしてからバックエンドの実接続状態を復元する。
    await loadProfiles();
    await connState.restore();
    if (presetState.presets().length === 0) {
      presetState.addPreset();
    }
  });

  return (
    <MqttContext.Provider
      value={{
        profiles,
        saveProfile,
        deleteProfile,
        connections: connState.connections,
        activeConnectionId: connState.activeConnectionId,
        activeConnection: connState.activeConnection,
        activeTab: connState.activeTab,
        setActiveTab: connState.setActiveTab,
        createOfflineConnection: connState.createOfflineConnection,
        handleConnect: connState.handleConnect,
        handleDisconnect: connState.handleDisconnect,
        handleReconnect: connState.handleReconnect,
        closeConnection: connState.closeConnection,
        switchConnection: connState.switchConnection,
        updateConnectionBroker: connState.updateConnectionBroker,
        reorderProfiles,
        ...subsState,
        ...messagesContext,
        ...presetState,
        ...publishState,
      }}
    >
      {props.children}
    </MqttContext.Provider>
  );
}

// Hooks
function useMqttContext(hookName: string): MqttContextValue {
  const ctx = useContext(MqttContext);
  if (!ctx) throw new Error(`${hookName} must be used within MqttProvider`);
  return ctx;
}

export function useMqttConnection(): ConnectionContextValue {
  return useMqttContext("useMqttConnection");
}

export function useMqttSubscribe(): SubscribeContextValue {
  return useMqttContext("useMqttSubscribe");
}

export function useMqttMessages(): MessagesContextValue {
  return useMqttContext("useMqttMessages");
}

export function useMqttPublish(): PublishContextValue {
  return useMqttContext("useMqttPublish");
}
