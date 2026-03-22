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
import { createSubscriptionsState } from "../../application/mqtt/subscriptions";
import type {
  BrokerProfile,
  ConnectionState,
  MqttMessage,
  PublishPreset,
  Subscription,
  Tab,
} from "../../domain/mqtt/types";
import * as mqttClient from "../../infrastructure/mqtt/client";
import {
  loadFromStorage,
  removeFromStorage,
  saveToStorage,
} from "../../infrastructure/storage/local-storage";

const LAST_ACTIVE_PROFILE_KEY = "mqtt:lastActiveProfileId";

// --- MqttConnectionContext ---
export interface ConnectionContextValue {
  profiles: Accessor<BrokerProfile[]>;
  saveProfile: (p: BrokerProfile) => Promise<void>;
  deleteProfile: (id: string) => Promise<void>;
  connections: Accessor<Map<string, ConnectionState>>;
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
  brokerTopics: Accessor<string[]>;
  isScanning: Accessor<boolean>;
  setIsScanning: (
    value: boolean | ((prev: boolean) => boolean),
  ) => Promise<void>;
}

// --- MqttMessagesContext ---
export interface MessagesContextValue {
  messages: Accessor<MqttMessage[]>;
  selectedMessage: Accessor<MqttMessage | null>;
  autoFollow: Accessor<boolean>;
  setSelectedMessage: (msg: MqttMessage | null) => void;
  setAutoFollow: (value: boolean | ((prev: boolean) => boolean)) => void;
  clearMessages: () => void;
}

// --- MqttPublishContext ---
export interface PublishContextValue {
  presets: Accessor<PublishPreset[]>;
  savePreset: (preset: Omit<PublishPreset, "id">) => void;
  removePreset: (id: string) => void;
}

const MqttConnectionContext = createContext<ConnectionContextValue>();
const MqttSubscribeContext = createContext<SubscribeContextValue>();
const MqttMessagesContext = createContext<MessagesContextValue>();
const MqttPublishContext = createContext<PublishContextValue>();

export function MqttProvider(props: { children: JSX.Element }) {
  const { profiles, loadProfiles, saveProfile, deleteProfile } =
    createProfilesState();
  const connState = createConnectionsState(
    mqttClient,
    {
      loadLastProfileId: () =>
        loadFromStorage<string | null>(LAST_ACTIVE_PROFILE_KEY, null),
      saveLastProfileId: (id) => saveToStorage(LAST_ACTIVE_PROFILE_KEY, id),
      removeLastProfileId: () => removeFromStorage(LAST_ACTIVE_PROFILE_KEY),
    },
    profiles,
    saveProfile,
  );

  onMount(() => {
    loadProfiles();
  });
  const subsState = createSubscriptionsState(
    connState.connections,
    connState.updateConnection,
    connState.activeConnectionId,
  );
  const msgState = createMessagesState(
    connState.connections,
    connState.updateConnection,
    connState.activeConnectionId,
  );
  const presetState = createPresetsState();

  return (
    <MqttConnectionContext.Provider
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
      }}
    >
      <MqttSubscribeContext.Provider value={subsState}>
        <MqttMessagesContext.Provider value={msgState}>
          <MqttPublishContext.Provider value={presetState}>
            {props.children}
          </MqttPublishContext.Provider>
        </MqttMessagesContext.Provider>
      </MqttSubscribeContext.Provider>
    </MqttConnectionContext.Provider>
  );
}

// Hooks
export function useMqttConnection(): ConnectionContextValue {
  const ctx = useContext(MqttConnectionContext);
  if (!ctx)
    throw new Error("useMqttConnection must be used within MqttProvider");
  return ctx;
}

export function useMqttSubscribe(): SubscribeContextValue {
  const ctx = useContext(MqttSubscribeContext);
  if (!ctx)
    throw new Error("useMqttSubscribe must be used within MqttProvider");
  return ctx;
}

export function useMqttMessages(): MessagesContextValue {
  const ctx = useContext(MqttMessagesContext);
  if (!ctx) throw new Error("useMqttMessages must be used within MqttProvider");
  return ctx;
}

export function useMqttPublish(): PublishContextValue {
  const ctx = useContext(MqttPublishContext);
  if (!ctx) throw new Error("useMqttPublish must be used within MqttProvider");
  return ctx;
}
