import {
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
} from "solid-js";
import { mqtt } from "../../../../wailsjs/go/models";
import {
  Connect,
  Disconnect,
  Subscribe,
} from "../../../../wailsjs/go/mqtt/MqttService";
import { EventsOn } from "../../../../wailsjs/runtime/runtime";
import type {
  BrokerProfile,
  ConnectionState,
  MqttMessage,
  Subscription,
  Tab,
} from "../types";
import { loadFromStorage, STORAGE_KEYS, saveToStorage } from "./profiles";
import { MAX_MESSAGES, MAX_TOPICS, topicMatchesParts } from "./shared";

export function createConnectionsState(
  profiles: () => BrokerProfile[],
  saveProfile: (p: BrokerProfile) => void,
) {
  const [connections, setConnections] = createSignal<
    Map<string, ConnectionState>
  >(new Map());
  const [activeConnectionId, setActiveConnectionId] = createSignal<
    string | null
  >(null);
  const [activeTab, setActiveTab] = createSignal<Tab>("subscribe");

  const activeConnection = createMemo(() => {
    const id = activeConnectionId();
    return id ? (connections().get(id) ?? null) : null;
  });

  function updateConnection(
    connId: string,
    updater: (state: ConnectionState) => ConnectionState,
  ) {
    setConnections((prev) => {
      const existing = prev.get(connId);
      if (!existing) return prev;
      const next = new Map(prev);
      next.set(connId, updater(existing));
      return next;
    });
  }

  // Micro-batch: buffer incoming messages and flush once per animation frame.
  // At ~16ms intervals this feels instant to users while reducing Map/array
  // recreation from N updates per frame to 1.
  interface RawMessage {
    connectionId: string;
    topic: string;
    payload: string;
    qos: number;
    timestamp: number;
  }

  const messageBuffer: RawMessage[] = [];
  let flushScheduled = false;

  function flushMessages() {
    flushScheduled = false;
    if (messageBuffer.length === 0) return;

    // Group buffered messages by connectionId
    const grouped = new Map<string, RawMessage[]>();
    for (const msg of messageBuffer) {
      let arr = grouped.get(msg.connectionId);
      if (!arr) {
        arr = [];
        grouped.set(msg.connectionId, arr);
      }
      arr.push(msg);
    }
    messageBuffer.length = 0;

    // Apply one updateConnection per connection with all its messages
    for (const [connId, batch] of grouped) {
      updateConnection(connId, (state) => {
        let newBrokerTopics = state.brokerTopics;
        let newBrokerTopicsSet = state.brokerTopicsSet;
        const pendingMessages: MqttMessage[] = [];

        for (const data of batch) {
          // Track discovered topics (deduplicate via Set + cap)
          if (state.isScanning && !newBrokerTopicsSet.has(data.topic)) {
            // Lazily copy the Set on first new topic in this batch
            if (newBrokerTopicsSet === state.brokerTopicsSet) {
              newBrokerTopicsSet = new Set(state.brokerTopicsSet);
            }
            newBrokerTopicsSet.add(data.topic);
            newBrokerTopics = [...newBrokerTopics, data.topic];
          }

          // Check subscription match (pre-split patterns avoid repeated .split())
          const msgParts = data.topic.split("/");
          const isSubscribed = state.subscriptions.some(
            (s) =>
              s.topic === data.topic ||
              (s.patternParts && topicMatchesParts(s.patternParts, msgParts)),
          );
          if (isSubscribed) {
            pendingMessages.push({
              id: `${data.timestamp}-${Math.random().toString(36).slice(2, 8)}`,
              topic: data.topic,
              payload: data.payload,
              qos: data.qos,
              timestamp: new Date(data.timestamp),
              direction: "incoming",
            });
          }
        }

        // Trim broker topics if over capacity
        if (newBrokerTopics.length > MAX_TOPICS) {
          const removed = newBrokerTopics.slice(
            0,
            newBrokerTopics.length - MAX_TOPICS,
          );
          for (const t of removed) newBrokerTopicsSet.delete(t);
          newBrokerTopics = newBrokerTopics.slice(
            newBrokerTopics.length - MAX_TOPICS,
          );
        }

        // Merge pending messages, trim to MAX_MESSAGES
        let newMessages = state.messages;
        if (pendingMessages.length > 0) {
          const combined = [...state.messages, ...pendingMessages];
          newMessages =
            combined.length > MAX_MESSAGES
              ? combined.slice(combined.length - MAX_MESSAGES)
              : combined;
        }

        return {
          ...state,
          brokerTopics: newBrokerTopics,
          brokerTopicsSet: newBrokerTopicsSet,
          messages: newMessages,
        };
      });
    }
  }

  // Wails event listeners
  onMount(() => {
    const cancelMessage = EventsOn(
      "mqtt:message",
      (data: {
        connectionId: string;
        topic: string;
        payload: string;
        qos: number;
        retained: boolean;
        timestamp: number;
      }) => {
        messageBuffer.push(data);
        if (!flushScheduled) {
          flushScheduled = true;
          requestAnimationFrame(flushMessages);
        }
      },
    );

    const cancelConnected = EventsOn(
      "mqtt:connected",
      (data: { connectionId: string }) => {
        updateConnection(data.connectionId, (state) => ({
          ...state,
          connected: true,
        }));
      },
    );

    const cancelDisconnected = EventsOn(
      "mqtt:disconnected",
      (data: { connectionId: string }) => {
        updateConnection(data.connectionId, (state) => ({
          ...state,
          connected: false,
          isScanning: false,
        }));
      },
    );

    const cancelConnectionLost = EventsOn(
      "mqtt:connection-lost",
      (data: { connectionId: string; error: string }) => {
        console.error("[MQTT] Connection lost:", data.error);
        updateConnection(data.connectionId, (state) => ({
          ...state,
          connected: false,
        }));
      },
    );

    const cancelConnectionFailed = EventsOn(
      "mqtt:connection-failed",
      (data: { connectionId: string; error: string }) => {
        console.error("[MQTT] Connection failed:", data.error);
        // Keep the entry as disconnected so the user's broker selection is
        // preserved.  The entry already has connected: false, so we only need
        // to ensure scanning state is cleared.
        updateConnection(data.connectionId, (state) => ({
          ...state,
          connected: false,
          isScanning: false,
        }));
      },
    );

    onCleanup(() => {
      cancelMessage();
      cancelConnected();
      cancelDisconnected();
      cancelConnectionLost();
      cancelConnectionFailed();
    });

    // Restore last active tab in disconnected state
    const savedProfileId = loadFromStorage<string | null>(
      STORAGE_KEYS.lastActiveProfileId,
      null,
    );
    const savedProfile = savedProfileId
      ? profiles().find((p) => p.id === savedProfileId)
      : undefined;
    if (savedProfile) {
      const offlineId = `offline-${savedProfile.id}`;
      setConnections((prev) => {
        const next = new Map(prev);
        next.set(offlineId, {
          connectionId: offlineId,
          profileId: savedProfile.id,
          profile: { ...savedProfile },
          connected: false,
          subscriptions: [],
          messages: [],
          selectedMessage: null,
          autoFollow: false,
          brokerTopics: [],
          brokerTopicsSet: new Set(),
          isScanning: false,
        });
        return next;
      });
      setActiveConnectionId(offlineId);
    }
  });

  // Persist last active profile
  createEffect(() => {
    const conn = activeConnection();
    if (conn) {
      saveToStorage(STORAGE_KEYS.lastActiveProfileId, conn.profileId);
    } else {
      localStorage.removeItem(STORAGE_KEYS.lastActiveProfileId);
    }
  });

  const updateConnectionBroker = (connectionId: string, broker: string) => {
    const conn = connections().get(connectionId);
    if (!conn || conn.connected) return;
    const updatedProfile = { ...conn.profile, broker };
    updateConnection(connectionId, (state) => ({
      ...state,
      profile: updatedProfile,
    }));
    saveProfile(updatedProfile);
  };

  const handleConnect = async (profileId: string) => {
    const profile = profiles().find((p) => p.id === profileId);
    if (!profile) return;

    try {
      const config = new mqtt.ConnectionConfig({
        name: profile.name,
        broker: profile.broker,
        clientId: profile.clientId,
        username: profile.username,
        password: profile.password,
        useTls: profile.useTls,
      });
      const connId = await Connect(config);

      const newState: ConnectionState = {
        connectionId: connId,
        profileId: profile.id,
        profile: { ...profile },
        connected: false,
        subscriptions: [],
        messages: [],
        selectedMessage: null,
        autoFollow: false,
        brokerTopics: [],
        brokerTopicsSet: new Set(),
        isScanning: false,
      };

      setConnections((prev) => {
        const next = new Map(prev);
        // Remove any stale entry for this profile (offline or previously failed)
        for (const [key, conn] of next) {
          if (conn.profileId === profile.id) {
            next.delete(key);
          }
        }
        next.set(connId, newState);
        return next;
      });
      setActiveConnectionId(connId);
    } catch (err) {
      console.error("[MQTT] Connect failed:", err);
    }
  };

  const handleDisconnect = async (connectionId?: string) => {
    const connId = connectionId ?? activeConnectionId();
    if (!connId) return;
    try {
      await Disconnect(connId);
    } catch (err) {
      console.error("[MQTT] Disconnect failed:", err);
    }
    updateConnection(connId, (state) => ({
      ...state,
      connected: false,
      isScanning: false,
    }));
  };

  const handleReconnect = async (connectionId: string) => {
    const conn = connections().get(connectionId);
    if (!conn) return;
    const profile = conn.profile;

    try {
      await Disconnect(connectionId);
    } catch {
      // Expected for offline tabs or already-removed connections
    }

    try {
      const config = new mqtt.ConnectionConfig({
        name: profile.name,
        broker: profile.broker,
        clientId: profile.clientId,
        username: profile.username,
        password: profile.password,
        useTls: profile.useTls,
      });
      const newConnId = await Connect(config);

      setConnections((prev) => {
        const next = new Map(prev);
        next.delete(connectionId);
        next.set(newConnId, {
          ...conn,
          connectionId: newConnId,
          connected: false,
        });
        return next;
      });

      if (activeConnectionId() === connectionId) {
        setActiveConnectionId(newConnId);
      }

      for (const sub of conn.subscriptions) {
        await Subscribe(newConnId, sub.topic, sub.qos).catch((err) =>
          console.error(`[MQTT] Failed to re-subscribe to ${sub.topic}:`, err),
        );
      }
    } catch (err) {
      console.error("[MQTT] Reconnect failed:", err);
    }
  };

  const closeConnection = (connectionId: string) => {
    const conn = connections().get(connectionId);
    if (conn?.connected) {
      Disconnect(connectionId).catch((err) =>
        console.error("[MQTT] Disconnect failed:", err),
      );
    }
    setConnections((prev) => {
      const next = new Map(prev);
      next.delete(connectionId);

      if (activeConnectionId() === connectionId) {
        const first = next.keys().next().value;
        setActiveConnectionId(first ?? null);
      }

      return next;
    });
  };

  const switchConnection = (connectionId: string) => {
    setActiveConnectionId(connectionId);
  };

  return {
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
  } as const;
}

export type { Subscription };
