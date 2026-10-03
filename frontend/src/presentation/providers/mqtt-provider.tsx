import {
  type Accessor,
  createContext,
  type JSX,
  onMount,
  type Setter,
  useContext,
} from "solid-js";
import {
  createConnectionsState,
  type MqttMessageView,
} from "../../application/mqtt/connections";
import { createMessagesState } from "../../application/mqtt/messages";
import {
  createPresetsState,
  type PublishDraft,
} from "../../application/mqtt/presets";
import { createProfilesState } from "../../application/mqtt/profiles";
import { createPublishState } from "../../application/mqtt/publish";
import { createSubscriptionsState } from "../../application/mqtt/subscriptions";
import { notify } from "../../application/ui/notifications";
import { MQTT_MAX_MESSAGES, MQTT_MAX_TOPICS } from "../../config/limits";
import type {
  BrokerProfile,
  ConnectionState,
  PublishPreset,
  Qos,
  Subscription,
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

// --- MqttSubscribeContext ---
export interface SubscribeContextValue {
  subscriptions: Accessor<Subscription[]>;
  newTopic: Accessor<string>;
  setNewTopic: Setter<string>;
  newQos: Accessor<Qos>;
  setNewQos: Setter<Qos>;
  addSubscription: (topic?: string, qos?: Qos) => Promise<void>;
  removeSubscription: (id: string) => Promise<void>;
  toggleMute: (id: string) => void;
  brokerTopics: Accessor<string[]>;
  isScanning: Accessor<boolean>;
  setIsScanning: (
    value: boolean | ((prev: boolean) => boolean),
  ) => Promise<void>;
}

// --- MqttMessagesContext ---
export interface MessagesContextValue {
  /** トピックフィルターを通した、一覧に表示するメッセージ。 */
  visibleMessages: Accessor<MqttMessageView[]>;
  topicFilter: Accessor<string>;
  setTopicFilter: Setter<string>;
  filterTopics: Accessor<string[]>;
  selectedMessage: Accessor<MqttMessageView | null>;
  autoFollow: Accessor<boolean>;
  setSelectedMessage: (msg: MqttMessageView | null) => void;
  setAutoFollow: (value: boolean | ((prev: boolean) => boolean)) => void;
  clearMessages: () => void;
}

// --- MqttPublishContext ---
export interface PublishContextValue {
  presets: Accessor<PublishPreset[]>;
  addPreset: () => void;
  removePreset: (id: string) => void;
  updatePreset: (
    id: string,
    updates: Partial<Omit<PublishPreset, "id">>,
  ) => void;
  reorderPresets: (fromIndex: number, toIndex: number) => void;
  selectedPresetId: Accessor<string | null>;
  selectPreset: (id: string) => void;
  draft: Accessor<PublishDraft>;
  updateDraft: (patch: Partial<PublishDraft>) => void;
  /** フォームの内容をアクティブな接続へ送信する。 */
  publishDraft: () => Promise<void>;
}

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
  // 一覧に出すのは visibleMessages なので、フィルター前の messages はコンテキストに出さない。
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
export function useMqttConnection(): ConnectionContextValue {
  const ctx = useContext(MqttContext);
  if (!ctx)
    throw new Error("useMqttConnection must be used within MqttProvider");
  return ctx;
}

export function useMqttSubscribe(): SubscribeContextValue {
  const ctx = useContext(MqttContext);
  if (!ctx)
    throw new Error("useMqttSubscribe must be used within MqttProvider");
  return ctx;
}

export function useMqttMessages(): MessagesContextValue {
  const ctx = useContext(MqttContext);
  if (!ctx) throw new Error("useMqttMessages must be used within MqttProvider");
  return ctx;
}

export function useMqttPublish(): PublishContextValue {
  const ctx = useContext(MqttContext);
  if (!ctx) throw new Error("useMqttPublish must be used within MqttProvider");
  return ctx;
}
