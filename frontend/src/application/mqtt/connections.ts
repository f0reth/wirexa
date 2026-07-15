import { createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import { createStore, produce } from "solid-js/store";
import type { Logger } from "../../application/logger";
import { notify } from "../../application/ui/notifications";
import type { ConnectionPersistence } from "../../domain/mqtt/ports";
import { topicMatchesParts } from "../../domain/mqtt/topic";
import type {
  BrokerProfile,
  ConnectionStatus,
  MqttMessage,
  OfflineConnectionState,
  OnlineConnectionState,
  Subscription,
  Tab,
} from "../../domain/mqtt/types";
import { generateId } from "../../infrastructure/id/generator";
import { errorMessage } from "../../shared/error";
import { type MqttEventName, WailsEvents } from "../../shared/wails-events";
import { makeSubscription } from "./subscription";

export type { ConnectionPersistence };

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

export type OfflineStateExt = OfflineConnectionState & ConnectionRuntimeState;
export type OnlineStateExt = OnlineConnectionState & ConnectionRuntimeState;
export type ConnectionStateExt = OfflineStateExt | OnlineStateExt;

export type { MqttEventName };

export type MqttEventListener = (
  event: MqttEventName,
  handler: (data: unknown) => void,
) => () => void;

export interface MqttConnectionApi {
  connect(profile: BrokerProfile): Promise<string>;
  disconnect(connectionId: string): Promise<void>;
  subscribe(connectionId: string, topic: string, qos: number): Promise<void>;
  unsubscribe(connectionId: string, topic: string): Promise<void>;
  getConnections(): Promise<ConnectionStatus[]>;
}

interface RawMessage {
  connectionId: string;
  topic: string;
  payload: string;
  payloadBase64?: boolean;
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

// プロファイルが既に削除された接続を復元する際に、状態から最小限のプロファイルを合成する。
function synthesizeProfile(status: ConnectionStatus): BrokerProfile {
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
  onEvent: MqttEventListener,
  persistence: ConnectionPersistence,
  profiles: () => BrokerProfile[],
  saveProfile: (p: BrokerProfile) => Promise<void>,
  logger: Logger,
  maxMessages: number,
  maxTopics: number,
) {
  const [connections, setConnections] = createStore<
    Record<string, ConnectionStateExt>
  >({});
  const [activeConnectionId, setActiveConnectionId] = createSignal<
    string | null
  >(null);
  const [activeTab, setActiveTab] = createSignal<Tab>("subscribe");

  const activeConnection = createMemo((): ConnectionStateExt | null => {
    const id = activeConnectionId();
    return id ? (connections[id] ?? null) : null;
  });

  function updateConnection(
    connId: string,
    updater: (state: ConnectionStateExt) => ConnectionStateExt,
  ) {
    const existing = connections[connId];
    if (!existing) return;
    setConnections(connId, updater(existing));
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
        const pendingMessages: MqttMessageView[] = [];

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
              id: generateId(),
              topic: data.topic,
              payload: data.payload,
              payloadBase64: data.payloadBase64 ?? false,
              qos: data.qos as 0 | 1 | 2,
              timestamp: new Date(data.timestamp),
              direction: "incoming",
            });
          }
        }

        if (newBrokerTopics.length > maxTopics) {
          const removed = newBrokerTopics.slice(
            0,
            newBrokerTopics.length - maxTopics,
          );
          for (const t of removed) newBrokerTopicsSet.delete(t);
          newBrokerTopics = newBrokerTopics.slice(
            newBrokerTopics.length - maxTopics,
          );
        }

        let newMessages = state.messages;
        if (pendingMessages.length > 0) {
          const combined = [...state.messages, ...pendingMessages];
          if (combined.length > maxMessages) {
            combined.splice(0, combined.length - maxMessages);
          }
          newMessages = combined;
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
  const cancelMessage = onEvent(WailsEvents.mqttMessage, (data) => {
    if (messageBuffer.length < 5000) {
      messageBuffer.push(data as RawMessage);
    }
    if (!flushScheduled) {
      flushScheduled = true;
      requestAnimationFrame(flushMessages);
    }
  });

  const cancelConnected = onEvent(WailsEvents.mqttConnected, (data) => {
    const { connectionId } = data as { connectionId: string };
    updateConnection(connectionId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: true };
    });
  });

  const cancelDisconnected = onEvent(WailsEvents.mqttDisconnected, (data) => {
    const { connectionId } = data as { connectionId: string };
    updateConnection(connectionId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: false, isScanning: false };
    });
  });

  const cancelConnectionLost = onEvent(
    WailsEvents.mqttConnectionLost,
    (data) => {
      const { connectionId, error } = data as {
        connectionId: string;
        error: string;
      };
      console.error("[MQTT] Connection lost:", error);
      notify.error("MQTT connection lost", error, { key: connectionId });
      updateConnection(connectionId, (state) => {
        if (state.type !== "online") return state;
        return { ...state, connected: false };
      });
    },
  );

  const cancelConnectionFailed = onEvent(
    WailsEvents.mqttConnectionFailed,
    (data) => {
      const { connectionId, error } = data as {
        connectionId: string;
        error: string;
      };
      console.error("[MQTT] Connection failed:", error);
      notify.error("MQTT connection failed", error, { key: connectionId });
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

  // 起動時にバックエンドの実接続状態から UI を復元する。
  // webview リロード後もバックエンドは接続・購読を維持しているため、GetConnections で
  // 生きている接続をオンラインタブ (購読付き) として復元し、残りのプロファイルを
  // オフラインタブとして並べる。最後に使ったプロファイルをアクティブにする。
  // 呼び出し側 (provider) は loadProfiles() の完了後に一度だけ呼ぶ。
  let restored = false;
  async function restore(): Promise<void> {
    if (restored) return;
    restored = true;

    let live: ConnectionStatus[] = [];
    try {
      live = await api.getConnections();
    } catch (err) {
      logger.error("MQTT restore failed", { error: String(err) });
    }

    const ps = profiles();
    const onlineProfileIds = new Set<string>();

    setConnections(
      produce((s) => {
        for (const status of live) {
          const profile =
            ps.find((p) => p.id === status.profileId) ??
            synthesizeProfile(status);
          const st = makeOnlineState(status.id, profile);
          st.connected = status.connected;
          st.subscriptions = status.subscriptions.map((s) =>
            makeSubscription(s.topic, s.qos),
          );
          s[status.id] = st;
          onlineProfileIds.add(profile.id);
        }
        for (const profile of ps) {
          if (onlineProfileIds.has(profile.id)) continue;
          const entry = makeOfflineState(profile);
          s[entry.connectionId] = entry;
        }
      }),
    );

    const savedProfileId = persistence.loadLastProfileId();
    if (savedProfileId) {
      const onlineMatch = live.find((c) => c.profileId === savedProfileId);
      if (onlineMatch) {
        setActiveConnectionId(onlineMatch.id);
      } else if (ps.some((p) => p.id === savedProfileId)) {
        setActiveConnectionId(offlineId(savedProfileId));
      }
    }
  }

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
    const conn = connections[connectionId];
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
    setConnections(
      produce((s) => {
        for (const key of Object.keys(s)) {
          if (s[key].profileId === profile.id) delete s[key];
        }
        s[entry.connectionId] = entry;
      }),
    );
    setActiveConnectionId(entry.connectionId);
  };

  const handleConnect = async (profileId: string) => {
    const profile = profiles().find((p) => p.id === profileId);
    if (!profile) return;
    logger.info("MQTT connecting", {
      broker: profile.broker,
      profile: profile.name,
    });
    try {
      const connId = await api.connect(profile);
      const newState = makeOnlineState(connId, profile);
      setConnections(
        produce((s) => {
          for (const key of Object.keys(s)) {
            if (s[key].profileId === profile.id) delete s[key];
          }
          s[connId] = newState;
        }),
      );
      setActiveConnectionId(connId);
      logger.info("MQTT connect initiated", {
        connection_id: connId,
        broker: profile.broker,
      });
    } catch (err) {
      logger.error("MQTT connect failed", {
        broker: profile.broker,
        error: String(err),
      });
      notify.error("Failed to connect", errorMessage(err));
    }
  };

  const handleDisconnect = async (connectionId?: string) => {
    const connId = connectionId ?? activeConnectionId();
    if (!connId) return;
    try {
      await api.disconnect(connId);
      logger.info("MQTT disconnected", { connection_id: connId });
    } catch (err) {
      logger.error("MQTT disconnect failed", {
        connection_id: connId,
        error: String(err),
      });
      notify.error("Failed to disconnect", errorMessage(err));
    }
    updateConnection(connId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: false, isScanning: false };
    });
  };

  const handleReconnect = async (connectionId: string) => {
    const conn = connections[connectionId];
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
      setConnections(
        produce((s) => {
          delete s[connectionId];
          s[newConnId] = {
            ...conn,
            type: "online" as const,
            connectionId: newConnId,
            connected: false,
          };
        }),
      );
      if (activeConnectionId() === connectionId) {
        setActiveConnectionId(newConnId);
      }
      for (const sub of conn.subscriptions) {
        await api
          .subscribe(newConnId, sub.topic, sub.qos)
          .catch((err) =>
            notify.error(
              `Failed to re-subscribe to ${sub.topic}`,
              errorMessage(err),
            ),
          );
      }
    } catch (err) {
      notify.error("Failed to reconnect", errorMessage(err));
    }
  };

  const closeConnection = (connectionId: string) => {
    const conn = connections[connectionId];
    if (conn?.type === "online" && conn.connected) {
      api
        .disconnect(connectionId)
        .catch((err) =>
          notify.error("Failed to disconnect", errorMessage(err)),
        );
    }
    setConnections(
      produce((s) => {
        delete s[connectionId];
      }),
    );
    if (activeConnectionId() === connectionId) {
      const first = Object.keys(connections)[0] ?? null;
      setActiveConnectionId(first);
    }
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
    restore,
  } as const;
}
