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
import { createPresetsState } from "../../application/mqtt/presets";
import { createProfilesState } from "../../application/mqtt/profiles";
import { createSubscriptionsState } from "../../application/mqtt/subscriptions";
import type {
  BrokerProfile,
  ConnectionState,
  PublishPreset,
  Subscription,
  Tab,
} from "../../domain/mqtt/types";
import { createLogger } from "../../infrastructure/logger/client";
import * as mqttClient from "../../infrastructure/mqtt/client";
import { onMqttEvent } from "../../infrastructure/mqtt/events";
import {
  createLastProfileStorage,
  createPresetsStorage,
} from "../../infrastructure/storage/local-storage";

// --- MqttConnectionContext ---
export interface ConnectionContextValue {
  profiles: Accessor<BrokerProfile[]>;
  saveProfile: (p: BrokerProfile) => Promise<void>;
  deleteProfile: (id: string) => Promise<void>;
  connections: Record<string, ConnectionState>;
  activeConnectionId: Accessor<string | null>;
  activeConnection: Accessor<ConnectionState | null>;
  activeTab: Accessor<Tab>;
  setActiveTab: Setter<Tab>;
  createOfflineConnection: (profile: BrokerProfile) => void;
  handleConnect: (profileId: string) => Promise<void>;
  handleDisconnect: (connectionId?: string) => Promise<void>;
  handleReconnect: (connectionId: string) => Promise<void>;
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
  newQos: Accessor<number>;
  setNewQos: Setter<number>;
  addSubscription: (topic?: string, qos?: number) => Promise<void>;
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
  messages: Accessor<MqttMessageView[]>;
  selectedMessage: Accessor<MqttMessageView | null>;
  autoFollow: Accessor<boolean>;
  setSelectedMessage: (msg: MqttMessageView | null) => void;
  setAutoFollow: (value: boolean | ((prev: boolean) => boolean)) => void;
  clearMessages: () => void;
}

// --- MqttPublishContext ---
export interface PublishContextValue {
  presets: Accessor<PublishPreset[]>;
  savePreset: (preset: Omit<PublishPreset, "id">) => void;
  removePreset: (id: string) => void;
  publish: (topic: string, payload: string, qos: number) => Promise<void>;
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
  } = createProfilesState(mqttClient);
  const mqttLogger = createLogger("frontend:mqtt");
  const connState = createConnectionsState(
    mqttClient,
    onMqttEvent,
    createLastProfileStorage(),
    profiles,
    saveProfile,
    mqttLogger,
  );

  onMount(() => {
    loadProfiles();
  });
  const subsState = createSubscriptionsState(
    connState.activeConnection,
    connState.updateConnection,
    mqttClient,
    mqttLogger,
  );
  const msgState = createMessagesState(
    connState.activeConnection,
    connState.updateConnection,
  );
  const presetState = createPresetsState(createPresetsStorage());

  const publish = async (
    topic: string,
    payload: string,
    qos: number,
  ): Promise<void> => {
    const connId = connState.activeConnectionId();
    if (!connId) return;
    await mqttClient.publish(connId, topic, payload, qos, false);
  };

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
        ...msgState,
        ...presetState,
        publish,
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
