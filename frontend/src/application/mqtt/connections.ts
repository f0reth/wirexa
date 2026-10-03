import {
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  untrack,
} from "solid-js";
import { createStore, produce } from "solid-js/store";
import type { Logger } from "../../application/logger";
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
import type { Notifier } from "../../domain/ui/ports";
import { errorMessage } from "../../shared/error";
import { generateId } from "../../shared/id";
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
  stopTopicScan(connectionId: string): Promise<void>;
  getConnections(): Promise<ConnectionStatus[]>;
}

interface ScannedTopic {
  connectionId: string;
  topic: string;
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
  saveProfile: (p: BrokerProfile) => Promise<BrokerProfile>,
  logger: Logger,
  notifier: Notifier,
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
    // updateConnection は書き込みなので、呼び出し元の effect に依存を足さない。
    // updater は store プロキシをスプレッドすることが多く、追跡したままだと接続全体への
    // 依存が付いて、無関係なプロパティの変更でも effect が再実行されてしまう。
    untrack(() => {
      const existing = connections[connId];
      if (!existing) return;
      setConnections(connId, updater(existing));
    });
  }

  // Micro-batch: buffer incoming messages and flush once per animation frame.
  const messageBuffer: RawMessage[] = [];
  // スキャンで見つかったトピック (mqtt:scan-topic)。メッセージと同じフレームでまとめて反映する。
  const topicBuffer: ScannedTopic[] = [];
  let flushScheduled = false;

  function scheduleFlush() {
    if (flushScheduled) return;
    flushScheduled = true;
    requestAnimationFrame(() => {
      flushScheduled = false;
      flushTopics();
      flushMessages();
    });
  }

  function flushTopics() {
    if (topicBuffer.length === 0) return;

    const grouped = new Map<string, string[]>();
    for (const { connectionId, topic } of topicBuffer) {
      let arr = grouped.get(connectionId);
      if (!arr) {
        arr = [];
        grouped.set(connectionId, arr);
      }
      arr.push(topic);
    }
    topicBuffer.length = 0;

    for (const [connId, topics] of grouped) {
      updateConnection(connId, (state) => {
        // 停止と行き違いで届いたトピックは捨てる。
        if (!state.isScanning) return state;
        const added = [
          ...new Set(topics.filter((t) => !state.brokerTopicsSet.has(t))),
        ];
        if (added.length === 0) return state;

        let brokerTopics = [...state.brokerTopics, ...added];
        const brokerTopicsSet = new Set(state.brokerTopicsSet);
        for (const t of added) brokerTopicsSet.add(t);
        if (brokerTopics.length > maxTopics) {
          const excess = brokerTopics.length - maxTopics;
          for (const t of brokerTopics.slice(0, excess)) {
            brokerTopicsSet.delete(t);
          }
          brokerTopics = brokerTopics.slice(excess);
        }
        return { ...state, brokerTopics, brokerTopicsSet };
      });
    }
  }

  function flushMessages() {
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
        const pendingMessages: MqttMessageView[] = [];

        for (const data of batch) {
          const msgParts = data.topic.split("/");
          // 重なる購読（例: muted の sensors/# と sensors/temp）があれば、
          // muted でない購読が 1 つでも一致すれば表示する。
          const delivered = state.subscriptions.some(
            (s) =>
              !s.muted &&
              (s.topic === data.topic ||
                (s.patternParts &&
                  topicMatchesParts(s.patternParts, msgParts))),
          );
          if (delivered) {
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

        if (pendingMessages.length === 0) return state;
        const combined = [...state.messages, ...pendingMessages];
        if (combined.length > maxMessages) {
          combined.splice(0, combined.length - maxMessages);
        }
        return { ...state, messages: combined };
      });
    }
  }

  // Wails イベントリスナー登録 → onCleanup で解除
  const cancelMessage = onEvent(WailsEvents.mqttMessage, (data) => {
    if (messageBuffer.length < 5000) {
      messageBuffer.push(data as RawMessage);
    }
    scheduleFlush();
  });

  // Broker Topics の一覧はスキャン用の接続が見つけたトピックから作る (mqtt:message からは作らない)。
  const cancelScanTopic = onEvent(WailsEvents.mqttScanTopic, (data) => {
    if (topicBuffer.length < 5000) {
      topicBuffer.push(data as ScannedTopic);
    }
    scheduleFlush();
  });

  // スキャン用の接続が切れてスキャンが止まった (自動では再開しない)。
  const cancelScanStopped = onEvent(WailsEvents.mqttScanStopped, (data) => {
    const { connectionId, error } = data as {
      connectionId: string;
      error: string;
    };
    notifier.error("MQTT topic scan stopped", error, { key: connectionId });
    updateConnection(connectionId, (state) => ({
      ...state,
      isScanning: false,
    }));
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
      notifier.error("MQTT connection lost", error, { key: connectionId });
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
      notifier.error("MQTT connection failed", error, { key: connectionId });
      updateConnection(connectionId, (state) => {
        if (state.type !== "online") return state;
        return { ...state, connected: false, isScanning: false };
      });
    },
  );

  onCleanup(() => {
    cancelMessage();
    cancelScanTopic();
    cancelScanStopped();
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
  // restore() が保存済みのプロファイル ID を読み終えるまで、アクティブプロファイルを保存しない。
  // 起動直後はアクティブな接続が無いので、先に保存すると読む前に消してしまう。
  const [persistActive, setPersistActive] = createSignal(false);
  async function restore(): Promise<void> {
    if (restored) return;
    restored = true;

    let live: ConnectionStatus[] = [];
    try {
      live = await api.getConnections();
    } catch (err) {
      logger.error("MQTT restore failed", { error: String(err) });
    }

    // スキャン中だった接続のスキャンは止め、スキャン中としては復元しない。Broker Topics の一覧は
    // フロントエンドだけが持つのでリロードで消え、スキャン用の接続が続いていても retained メッセージは
    // 再送されない。復元すると、retained のトピックが欠けた一覧をスキャン中と表示してしまう。
    for (const status of live) {
      if (!status.scanning) continue;
      api.stopTopicScan(status.id).catch((err) =>
        logger.error("MQTT topic scan failed to stop", {
          connection_id: status.id,
          error: String(err),
        }),
      );
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
    setPersistActive(true);
  }

  // アクティブプロファイルを永続化
  createEffect(() => {
    if (!persistActive()) return;
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
      notifier.error("Failed to connect", errorMessage(err));
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
      notifier.error("Failed to disconnect", errorMessage(err));
    }
    updateConnection(connId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: false, isScanning: false };
    });
  };

  // profile を渡すと、タブが持つプロファイルの代わりにそれで接続し、タブのプロファイルも置き換える
  // (編集して保存したプロファイルで張り直す「Save & Connect」)。
  const handleReconnect = async (
    connectionId: string,
    newProfile?: BrokerProfile,
  ) => {
    const conn = connections[connectionId];
    if (!conn) return;
    const profile = newProfile ? { ...newProfile } : conn.profile;
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
            profile,
            connected: false,
            // スキャンは前の接続と一緒に止まっている。
            isScanning: false,
          };
        }),
      );
      if (activeConnectionId() === connectionId) {
        setActiveConnectionId(newConnId);
      }
      // バックエンドは確立前の購読を受け付けて確立時に購読するので、確立を待たずに送る。
      for (const sub of conn.subscriptions) {
        // 前の購読を送っている間に、タブが閉じられたか購読が外されていたら送らない。
        // 送ると、UI に無い購読がバックエンドに残って確立時に購読される。
        const current = connections[newConnId];
        if (!current) break;
        if (!current.subscriptions.some((s) => s.id === sub.id)) continue;
        try {
          await api.subscribe(newConnId, sub.topic, sub.qos);
        } catch (err) {
          notifier.error(
            `Failed to re-subscribe to ${sub.topic}`,
            errorMessage(err),
          );
          if (!(await settleFailedResubscribe(newConnId, sub))) break;
        }
      }
    } catch (err) {
      notifier.error("Failed to reconnect", errorMessage(err));
    }
  };

  // handleReconnect で購読の RPC が失敗した行を、バックエンドの購読と突き合わせて片付ける。
  // 確立後の購読の失敗は張り直しを通らず mqtt:subscription-dropped も出ないので、ここで行を外す。
  // 残りの購読を続けて送るなら true を返す。
  async function settleFailedResubscribe(
    connId: string,
    sub: Subscription,
  ): Promise<boolean> {
    let live: ConnectionStatus[];
    try {
      live = await api.getConnections();
    } catch (err) {
      // バックエンドの購読が分からないので、行は残す。
      logger.error("MQTT re-subscribe check failed", {
        connection_id: connId,
        topic: sub.topic,
        error: String(err),
      });
      return true;
    }
    const status = live.find((c) => c.id === connId);
    // 新しい接続が無い (接続に失敗して閉じた)。行は次の Reconnect で引き継ぐので残し、残りは送らない。
    if (!status) return false;
    if (!status.subscriptions.some((s) => s.topic === sub.topic)) {
      updateConnection(connId, (state) => ({
        ...state,
        subscriptions: state.subscriptions.filter((s) => s.id !== sub.id),
      }));
    }
    return true;
  }

  const closeConnection = (connectionId: string) => {
    const conn = connections[connectionId];
    // 確立待ち・自動再接続中のオンラインタブも切断する。切断しないと、バックエンドに接続と
    // スキャン用の接続 (確立前でも始められる) が画面から見えないまま残る。
    if (conn?.type === "online") {
      const connected = conn.connected;
      api.disconnect(connectionId).catch((err) => {
        // 未接続での失敗 (接続が失敗して既に無い) は通知しない。
        if (connected) {
          notifier.error("Failed to disconnect", errorMessage(err));
        }
      });
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
