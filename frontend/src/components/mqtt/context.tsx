import {
  createContext,
  createEffect,
  createSignal,
  useContext,
} from "solid-js";
import { Publish } from "../../../wailsjs/go/mqtt/MqttService";
import { createConnectionsState } from "./state/connections";
import { createMessagesState } from "./state/messages";
import {
  createProfilesState,
  loadFromStorage,
  STORAGE_KEYS,
  saveToStorage,
} from "./state/profiles";
import { createSubscriptionsState } from "./state/subscriptions";
import type { PublishPreset } from "./types";

function createMqttState() {
  const { profiles, saveProfile, deleteProfile } = createProfilesState();

  const {
    connections,
    activeConnectionId,
    activeConnection,
    activeTab,
    setActiveTab,
    updateConnection,
    switchConnection,
    handleConnect,
    handleDisconnect,
    handleReconnect,
    closeConnection,
    updateConnectionBroker,
  } = createConnectionsState(profiles, saveProfile);

  const {
    newTopic,
    setNewTopic,
    newQos,
    setNewQos,
    subscriptions,
    brokerTopics,
    isScanning,
    addSubscription,
    removeSubscription,
    setIsScanning,
  } = createSubscriptionsState(
    connections,
    updateConnection,
    activeConnectionId,
  );

  const {
    messages,
    selectedMessage,
    autoFollow,
    setSelectedMessage,
    setAutoFollow,
    clearMessages,
    setMessagesScrollRef,
  } = createMessagesState(
    connections,
    updateConnection,
    activeConnectionId,
    activeTab,
  );

  const isConnected = () => activeConnection()?.connected ?? false;

  // Publish state (global)
  const [publishTopic, setPublishTopic] = createSignal("");
  const [publishPayload, setPublishPayload] = createSignal("");
  const [publishQos, setPublishQos] = createSignal<number>(0);
  const [presetName, setPresetName] = createSignal("");
  const [savedPresets, setSavedPresets] = createSignal<PublishPreset[]>(
    loadFromStorage(STORAGE_KEYS.presets, []),
  );

  createEffect(() => {
    saveToStorage(STORAGE_KEYS.presets, savedPresets());
  });

  const publishMessage = async () => {
    const connId = activeConnectionId();
    if (!connId || !publishTopic().trim() || !publishPayload().trim()) return;
    try {
      await Publish(
        connId,
        publishTopic(),
        publishPayload(),
        publishQos(),
        false,
      );
    } catch (err) {
      console.error("[MQTT] Publish failed:", err);
    }
  };

  const savePreset = () => {
    if (!presetName().trim() || !publishTopic().trim()) return;
    setSavedPresets((prev) => [
      ...prev,
      {
        id: Date.now().toString(),
        name: presetName().trim(),
        topic: publishTopic(),
        payload: publishPayload(),
        qos: publishQos(),
      },
    ]);
    setPresetName("");
  };

  const loadPreset = (preset: PublishPreset) => {
    setPublishTopic(preset.topic);
    setPublishPayload(preset.payload);
    setPublishQos(preset.qos);
  };

  const removePreset = (id: string) => {
    setSavedPresets((prev) => prev.filter((p) => p.id !== id));
  };

  return {
    // Profiles
    profiles,
    saveProfile,
    deleteProfile,
    // Connections
    connections,
    activeConnectionId,
    activeConnection,
    switchConnection,
    handleConnect,
    handleDisconnect,
    handleReconnect,
    closeConnection,
    updateConnectionBroker,
    // Connection state (derived from active)
    isConnected,
    isConnecting: () => false,
    activeTab,
    setActiveTab,
    // Subscribe
    subscriptions,
    newTopic,
    setNewTopic,
    newQos,
    setNewQos,
    addSubscription,
    removeSubscription,
    // Messages
    messages,
    selectedMessage,
    setSelectedMessage,
    autoFollow,
    setAutoFollow,
    clearMessages,
    setMessagesScrollRef,
    // Broker topics
    brokerTopics,
    isScanning,
    setIsScanning,
    // Publish
    publishTopic,
    setPublishTopic,
    publishPayload,
    setPublishPayload,
    publishQos,
    setPublishQos,
    presetName,
    setPresetName,
    savedPresets,
    publishMessage,
    savePreset,
    loadPreset,
    removePreset,
  } as const;
}

type MqttContextValue = ReturnType<typeof createMqttState>;

const MqttContext = createContext<MqttContextValue>();

export function MqttProvider(props: {
  children: import("solid-js").JSX.Element;
}) {
  const state = createMqttState();
  return (
    <MqttContext.Provider value={state}>{props.children}</MqttContext.Provider>
  );
}

export function useMqtt() {
  const ctx = useContext(MqttContext);
  if (!ctx) throw new Error("useMqtt must be used within MqttProvider");
  return ctx;
}
