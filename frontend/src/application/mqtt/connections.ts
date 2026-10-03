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
import {
  type BrokerProfile,
  type ConnectionStatus,
  isConnected,
  type MqttEventPayloads,
  type Qos,
  type Subscription,
  type Tab,
} from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import { errorMessage } from "../../shared/error";
import { type MqttEventName, WailsEvents } from "../../shared/wails-events";
import {
  type ConnectionStateExt,
  makeOfflineState,
  makeOnlineState,
  offlineId,
  synthesizeProfile,
} from "./connection-state";
import { createEventBuffer } from "./message-buffer";
import { makeSubscription } from "./subscription";

export type {
  ConnectionStateExt,
  MqttMessageView,
  OfflineStateExt,
  OnlineStateExt,
} from "./connection-state";

export type { ConnectionPersistence };

/** イベントの購読を登録し、解除する関数を返す。ペイロードの型はイベント名で決まる。 */
export type MqttEventListener = <E extends MqttEventName>(
  event: E,
  handler: (data: MqttEventPayloads[E]) => void,
) => () => void;

export interface MqttConnectionApi {
  connect(profile: BrokerProfile): Promise<string>;
  disconnect(connectionId: string): Promise<void>;
  subscribe(connectionId: string, topic: string, qos: number): Promise<void>;
  stopTopicScan(connectionId: string): Promise<void>;
  getConnections(): Promise<ConnectionStatus[]>;
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

  const buffer = createEventBuffer(updateConnection, maxMessages, maxTopics);

  // 同じプロファイルのタブを消して、新しいタブに置き換える。
  function replaceProfileTab(profileId: string, entry: ConnectionStateExt) {
    setConnections(
      produce((s) => {
        for (const key of Object.keys(s)) {
          if (s[key].profileId === profileId) delete s[key];
        }
        s[entry.connectionId] = entry;
      }),
    );
  }

  // オンラインのタブを未接続にする。stopScan が true なら、スキャン中の表示も止める。
  function markOffline(connId: string, opts: { stopScan: boolean }) {
    updateConnection(connId, (state) => {
      if (state.type !== "online") return state;
      return opts.stopScan
        ? { ...state, connected: false, isScanning: false }
        : { ...state, connected: false };
    });
  }

  // Wails イベントリスナー登録 → onCleanup で解除
  const cancelMessage = onEvent(WailsEvents.mqttMessage, buffer.pushMessage);

  // Broker Topics の一覧はスキャン用の接続が見つけたトピックから作る (mqtt:message からは作らない)。
  const cancelScanTopic = onEvent(WailsEvents.mqttScanTopic, buffer.pushTopic);

  // スキャン用の接続が切れてスキャンが止まった (自動では再開しない)。
  const cancelScanStopped = onEvent(WailsEvents.mqttScanStopped, (data) => {
    const { connectionId, error } = data;
    notifier.error("MQTT topic scan stopped", error, { key: connectionId });
    updateConnection(connectionId, (state) => ({
      ...state,
      isScanning: false,
    }));
  });

  const cancelConnected = onEvent(WailsEvents.mqttConnected, (data) => {
    const { connectionId } = data;
    updateConnection(connectionId, (state) => {
      if (state.type !== "online") return state;
      return { ...state, connected: true };
    });
  });

  const cancelDisconnected = onEvent(WailsEvents.mqttDisconnected, (data) => {
    markOffline(data.connectionId, { stopScan: true });
  });

  const cancelConnectionLost = onEvent(
    WailsEvents.mqttConnectionLost,
    (data) => {
      const { connectionId, error } = data;
      console.error("[MQTT] Connection lost:", error);
      notifier.error("MQTT connection lost", error, { key: connectionId });
      // スキャンの状態は変えない。
      markOffline(connectionId, { stopScan: false });
    },
  );

  const cancelConnectionFailed = onEvent(
    WailsEvents.mqttConnectionFailed,
    (data) => {
      const { connectionId, error } = data;
      console.error("[MQTT] Connection failed:", error);
      notifier.error("MQTT connection failed", error, { key: connectionId });
      markOffline(connectionId, { stopScan: true });
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
          // RPC の qos は number で届く。
          st.subscriptions = status.subscriptions.map((s) =>
            makeSubscription(s.topic, s.qos as Qos),
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
    if (!conn || isConnected(conn)) return;
    const updatedProfile = { ...conn.profile, broker };
    updateConnection(connectionId, (state) => ({
      ...state,
      profile: updatedProfile,
    }));
    // 失敗の通知は注入された saveProfile が出す。失敗してもタブの URL は入力した値のままにする
    // （入力欄を巻き戻すと打てなくなる）。
    saveProfile(updatedProfile).catch(() => {});
  };

  const createOfflineConnection = (profile: BrokerProfile) => {
    const entry = makeOfflineState(profile);
    replaceProfileTab(profile.id, entry);
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
      replaceProfileTab(profile.id, makeOnlineState(connId, profile));
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
    markOffline(connId, { stopScan: true });
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
