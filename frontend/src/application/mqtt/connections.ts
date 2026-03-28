import {
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  untrack,
} from "solid-js";
import {
  MAX_MESSAGES,
  MAX_TOPICS,
  topicMatchesParts,
} from "../../domain/mqtt/topic";
import type {
  BrokerProfile,
  MqttMessage,
  OfflineConnectionState,
  OnlineConnectionState,
  Tab,
} from "../../domain/mqtt/types";
import { onMqttEvent } from "../../infrastructure/mqtt/events";

// Application層内部でのみ使うSet最適化を保持する拡張型。
// Domain型 (ConnectionState) には実装詳細のSetは含まれない。
export type OfflineStateExt = OfflineConnectionState & {
  readonly brokerTopicsSet: Set<string>;
};
export type OnlineStateExt = OnlineConnectionState & {
  readonly brokerTopicsSet: Set<string>;
};
export type ConnectionStateExt = OfflineStateExt | OnlineStateExt;

export interface MqttConnectionApi {
  connect(profile: BrokerProfile): Promise<string>;
  disconnect(connectionId: string): Promise<void>;
  subscribe(connectionId: string, topic: string, qos: number): Promise<void>;
  unsubscribe(connectionId: string, topic: string): Promise<void>;
}

export interface ConnectionPersistence {
  loadLastProfileId(): string | null;
  saveLastProfileId(id: string): void;
  removeLastProfileId(): void;
}

interface RawMessage {
  connectionId: string;
  topic: string;
  payload: string;
  qos: number;
  timestamp: number;
}

// オフライン接続の ID 生成ロジックをここに集約
function offlineId(profileId: string): string {
  return `offline-${profileId}`;
}

function makeOfflineState(profile: BrokerProfile): OfflineStateExt {
  return {
    type: "offline",
    connectionId: offlineId(profile.id),
    profileId: profile.id,
    profile: { ...profile },
    subscriptions: [],
    messages: [],
    selectedMessage: null,
    autoFollow: false,
    brokerTopics: [],
    brokerTopicsSet: new Set(),
    isScanning: false,
  };
}

function makeOnlineState(
  connId: string,
  profile: BrokerProfile,
): OnlineStateExt {
  return {
    type: "online",
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
}

export function createConnectionsState(
  api: MqttConnectionApi,
  persistence: ConnectionPersistence,
  profiles: () => BrokerProfile[],
  saveProfile: (p: BrokerProfile) => Promise<void>,
) {
  const [connections, setConnections] = createSignal<
    Map<string, ConnectionStateExt>
  >(new Map());
  const [activeConnectionId, setActiveConnectionId] = createSignal<
    string | null
  >(null);
  const [activeTab, setActiveTab] = createSignal<Tab>("subscribe");

  const activeConnection = createMemo((): ConnectionStateExt | null => {
    const id = activeConnectionId();
    return id ? (connections().get(id) ?? null) : null;
  });

  function updateConnection(
    connId: string,
    updater: (state: ConnectionStateExt) => ConnectionStateExt,
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
  const messageBuffer: RawMessage[] = [];
  let flushScheduled = false;

  function flushMessages() {
    flushScheduled = false;
    if (messageBuffer.length === 0) return;

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

    for (const [connId, batch] of grouped) {
      updateConnection(connId, (state) => {
        let newBrokerTopics = state.brokerTopics;
        let newBrokerTopicsSet = state.brokerTopicsSet;
        const pendingMessages: MqttMessage[] = [];

        for (const data of batch) {
          if (state.isScanning && !newBrokerTopicsSet.has(data.topic)) {
            if (newBrokerTopicsSet === state.brokerTopicsSet) {
              newBrokerTopicsSet = new Set(state.brokerTopicsSet);
            }
            newBrokerTopicsSet.add(data.topic);
            newBrokerTopics = [...newBrokerTopics, data.topic];
          }

          const msgParts = data.topic.split("/");
          const matchingSub = state.subscriptions.find(
            (s) =>
              s.topic === data.topic ||
              (s.patternParts && topicMatchesParts(s.patternParts, msgParts)),
          );
          if (matchingSub && !matchingSub.muted) {
            pendingMessages.push({
              id: `${data.timestamp}-${Math.random().toString(36).slice(2, 8)}`,
              topic: data.topic,
              payload: data.payload,
              qos: data.qos as 0 | 1 | 2,
              timestamp: new Date(data.timestamp),
              direction: "incoming",
            });
          }
        }

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

  // Wails イベントリスナー登録 → onCleanup で解除
  const cancelMessage = onMqttEvent("mqtt:message", (data) => {
    messageBuffer.push(data as RawMessage);
    if (!flushScheduled) {
      flushScheduled = true;
      requestAnimationFrame(flushMessages);
    }
  });

  const cancelConnected = onMqttEvent("mqtt:connected", (data) => {
    const { connectionId } = data as { connectionId: string };
    updateConnection(connectionId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: true };
    });
  });

  const cancelDisconnected = onMqttEvent("mqtt:disconnected", (data) => {
    const { connectionId } = data as { connectionId: string };
    updateConnection(connectionId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: false, isScanning: false };
    });
  });

  const cancelConnectionLost = onMqttEvent("mqtt:connection-lost", (data) => {
    const { connectionId, error } = data as {
      connectionId: string;
      error: string;
    };
    console.error("[MQTT] Connection lost:", error);
    updateConnection(connectionId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: false };
    });
  });

  const cancelConnectionFailed = onMqttEvent(
    "mqtt:connection-failed",
    (data) => {
      const { connectionId, error } = data as {
        connectionId: string;
        error: string;
      };
      console.error("[MQTT] Connection failed:", error);
      updateConnection(connectionId, (state) => {
        if (state.type !== "online") return state;
        return { ...state, connected: false, isScanning: false };
      });
    },
  );

  onCleanup(() => {
    cancelMessage();
    cancelConnected();
    cancelDisconnected();
    cancelConnectionLost();
    cancelConnectionFailed();
  });

  // 起動時に全プロファイルをオフラインタブとして復元し、最後に使ったプロファイルをアクティブにする
  // profiles はバックエンドから非同期でロードされるため、createEffect で反応する
  let profilesRestored = false;
  createEffect(() => {
    const ps = profiles();
    if (ps.length === 0 || profilesRestored) return;
    profilesRestored = true;

    const savedProfileId = persistence.loadLastProfileId();

    untrack(() => {
      setConnections((prev) => {
        const next = new Map(prev);
        for (const profile of ps) {
          const entry = makeOfflineState(profile);
          next.set(entry.connectionId, entry);
        }
        return next;
      });

      if (savedProfileId) {
        const savedProfile = ps.find((p) => p.id === savedProfileId);
        if (savedProfile) {
          setActiveConnectionId(offlineId(savedProfile.id));
        }
      }
    });
  });

  // アクティブプロファイルを永続化
  createEffect(() => {
    const conn = activeConnection();
    if (conn) {
      persistence.saveLastProfileId(conn.profileId);
    } else {
      persistence.removeLastProfileId();
    }
  });

  const updateConnectionBroker = (connectionId: string, broker: string) => {
    const conn = connections().get(connectionId);
    if (!conn || (conn.type === "online" && conn.connected)) return;
    const updatedProfile = { ...conn.profile, broker };
    updateConnection(connectionId, (state) => ({
      ...state,
      profile: updatedProfile,
    }));
    saveProfile(updatedProfile);
  };

  const createOfflineConnection = (profile: BrokerProfile) => {
    const entry = makeOfflineState(profile);
    setConnections((prev) => {
      const next = new Map(prev);
      for (const [key, conn] of next) {
        if (conn.profileId === profile.id) {
          next.delete(key);
        }
      }
      next.set(entry.connectionId, entry);
      return next;
    });
    setActiveConnectionId(entry.connectionId);
  };

  const handleConnect = async (profileId: string) => {
    const profile = profiles().find((p) => p.id === profileId);
    if (!profile) return;
    try {
      const connId = await api.connect(profile);
      const newState = makeOnlineState(connId, profile);
      setConnections((prev) => {
        const next = new Map(prev);
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
      await api.disconnect(connId);
    } catch (err) {
      console.error("[MQTT] Disconnect failed:", err);
    }
    updateConnection(connId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: false, isScanning: false };
    });
  };

  const handleReconnect = async (connectionId: string) => {
    const conn = connections().get(connectionId);
    if (!conn) return;
    const profile = conn.profile;
    if (conn.type === "online") {
      try {
        await api.disconnect(connectionId);
      } catch {
        // 既に切断済みの可能性
      }
    }
    try {
      const newConnId = await api.connect(profile);
      setConnections((prev) => {
        const next = new Map(prev);
        next.delete(connectionId);
        next.set(newConnId, {
          ...conn,
          type: "online" as const,
          connectionId: newConnId,
          connected: false,
        });
        return next;
      });
      if (activeConnectionId() === connectionId) {
        setActiveConnectionId(newConnId);
      }
      for (const sub of conn.subscriptions) {
        await api
          .subscribe(newConnId, sub.topic, sub.qos)
          .catch((err) =>
            console.error(
              `[MQTT] Failed to re-subscribe to ${sub.topic}:`,
              err,
            ),
          );
      }
    } catch (err) {
      console.error("[MQTT] Reconnect failed:", err);
    }
  };

  const closeConnection = (connectionId: string) => {
    const conn = connections().get(connectionId);
    if (conn?.type === "online" && conn.connected) {
      api
        .disconnect(connectionId)
        .catch((err) => console.error("[MQTT] Disconnect failed:", err));
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
    createOfflineConnection,
    handleConnect,
    handleDisconnect,
    handleReconnect,
    closeConnection,
    updateConnectionBroker,
  } as const;
}
