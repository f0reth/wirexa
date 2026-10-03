import { createSignal, onCleanup } from "solid-js";
import type { Logger } from "../../application/logger";
import type { Qos } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import { errorMessage } from "../../shared/error";
import { WailsEvents } from "../../shared/wails-events";
import type { ConnectionStateExt, MqttEventListener } from "./connections";
import { makeSubscription } from "./subscription";

export interface SubscriptionApi {
  subscribe(connectionId: string, topic: string, qos: number): Promise<void>;
  unsubscribe(connectionId: string, topic: string): Promise<void>;
  startTopicScan(connectionId: string): Promise<void>;
  stopTopicScan(connectionId: string): Promise<void>;
}

/** 実行中の購読の RPC。dropped は、実行中に mqtt:subscription-dropped が届いたかを表す。 */
interface PendingSubscribe {
  connectionId: string;
  topic: string;
  dropped: boolean;
}

export function createSubscriptionsState(
  activeConnection: () => ConnectionStateExt | null,
  updateConnection: (
    id: string,
    updater: (state: ConnectionStateExt) => ConnectionStateExt,
  ) => void,
  api: SubscriptionApi,
  onEvent: MqttEventListener,
  logger: Logger,
  notifier: Notifier,
) {
  const [newTopic, setNewTopic] = createSignal("");
  const [newQos, setNewQos] = createSignal<Qos>(0);

  const subscriptions = () => activeConnection()?.subscriptions ?? [];
  const brokerTopics = () => activeConnection()?.brokerTopics ?? [];
  const isScanning = () => activeConnection()?.isScanning ?? false;

  // 確立前の Subscribe は登録だけして返り、確立時の張り直しで拒否されると mqtt:subscription-dropped が届く。
  // RPC の応答とイベントの届く順序は決まっていないので、イベントが先に届いた購読は、応答の後に行を足さない。
  const pendingSubscribes = new Set<PendingSubscribe>();

  // 張り直しに失敗してバックエンドが外した購読の行を外す。
  const cancelDropped = onEvent(WailsEvents.mqttSubscriptionDropped, (data) => {
    const { connectionId, topic, error } = data;
    let pending = false;
    for (const p of pendingSubscribes) {
      if (p.connectionId === connectionId && p.topic === topic) {
        p.dropped = true;
        pending = true;
      }
    }
    // updateConnection は updater を同期で呼ぶ。
    let removed = false;
    updateConnection(connectionId, (state) => {
      const subs = state.subscriptions.filter((s) => s.topic !== topic);
      if (subs.length === state.subscriptions.length) return state;
      removed = true;
      return { ...state, subscriptions: subs };
    });
    // 行も実行中の RPC も無い (タブを閉じた後など) なら知らせない。
    if (!removed && !pending) return;
    logger.error("MQTT subscription dropped", {
      connection_id: connectionId,
      topic,
      error,
    });
    notifier.error(`Subscription to ${topic} was dropped`, error, {
      key: `${connectionId}:${topic}`,
    });
  });
  onCleanup(cancelDropped);

  const addSubscription = async (topic?: string, qos?: Qos) => {
    const t = (topic ?? newTopic()).trim();
    if (!t) return;
    const conn = activeConnection();
    const connId = conn?.connectionId;
    // 重複トピックの場合はトピック入力をクリアしてリターン
    if (conn?.subscriptions.some((s) => s.topic === t)) {
      if (!topic) setNewTopic("");
      return;
    }
    const q = qos ?? newQos();
    if (connId) {
      const pending: PendingSubscribe = {
        connectionId: connId,
        topic: t,
        dropped: false,
      };
      pendingSubscribes.add(pending);
      try {
        await api.subscribe(connId, t, q);
        logger.info("MQTT subscribed", {
          connection_id: connId,
          topic: t,
          qos: q,
        });
      } catch (err) {
        logger.error("MQTT subscribe failed", {
          connection_id: connId,
          topic: t,
          error: String(err),
        });
        notifier.error(`Failed to subscribe to ${t}`, errorMessage(err));
        return;
      } finally {
        pendingSubscribes.delete(pending);
      }
      // 応答より先に、確立時の張り直しで外れていた。通知はリスナーが出している。
      if (pending.dropped) return;
      const newSub = makeSubscription(t, q);
      updateConnection(connId, (state) => ({
        ...state,
        subscriptions: [...state.subscriptions, newSub],
      }));
    }
    if (!topic) setNewTopic("");
  };

  const removeSubscription = async (id: string) => {
    const conn = activeConnection();
    const connId = conn?.connectionId;
    if (!connId) return;
    const sub = conn?.subscriptions.find((s) => s.id === id);
    if (!sub) return;
    // 確立待ちのオンラインタブにも送る。バックエンドは確立前の購読を保持して確立時に購読するので、
    // 送らないと外した購読が確立時に購読されてしまう。
    if (conn?.type === "online") {
      const connected = conn.connected;
      try {
        await api.unsubscribe(connId, sub.topic);
        logger.info("MQTT unsubscribed", {
          connection_id: connId,
          topic: sub.topic,
        });
      } catch (err) {
        logger.error("MQTT unsubscribe failed", {
          connection_id: connId,
          topic: sub.topic,
          error: String(err),
        });
        // 未接続での失敗 (接続が失敗して既に無い、自動再接続中) は行を外せば足りるので通知しない。
        if (connected) {
          notifier.error(
            `Failed to unsubscribe from ${sub.topic}`,
            errorMessage(err),
          );
        }
      }
    }
    updateConnection(connId, (state) => ({
      ...state,
      subscriptions: state.subscriptions.filter((s) => s.id !== id),
    }));
  };

  // isScanning は RPC の完了前に切り替えるので、開始の完了前に Stop を押せる。
  // scanSeq は接続ごとの連番で、setIsScanning を呼ぶたびに進める。開始の結果は、連番が呼び出し時の
  // ままのときだけ反映する。scanRpcs は、その接続で送った開始・停止の RPC が全て終わると解決する。
  const scanSeq = new Map<string, number>();
  const scanRpcs = new Map<string, Promise<unknown>>();

  const setIsScanning = async (
    value: boolean | ((prev: boolean) => boolean),
  ) => {
    const conn = activeConnection();
    const connId = conn?.connectionId;
    if (!connId) return;
    const newValue =
      typeof value === "function" ? value(conn.isScanning) : value;
    const seq = (scanSeq.get(connId) ?? 0) + 1;
    scanSeq.set(connId, seq);
    const superseded = () => scanSeq.get(connId) !== seq;
    const previous = scanRpcs.get(connId) ?? Promise.resolve();
    if (newValue) {
      updateConnection(connId, (state) => ({
        ...state,
        isScanning: true,
        brokerTopics: [],
        brokerTopicsSet: new Set(),
      }));
      const starting = (async () => {
        // 先に送った開始・停止が終わってから送る。待たずに送ると、バックエンドでは
        // 打ち切られる前の開始に合流して、その失敗を受け取ってしまう。
        await previous;
        if (superseded()) return;
        try {
          await api.startTopicScan(connId);
        } catch (err) {
          // 停止で打ち切られた開始のエラーは通知しない。
          if (superseded()) return;
          logger.error("MQTT topic scan failed to start", {
            connection_id: connId,
            error: String(err),
          });
          notifier.error("Failed to start topic scan", errorMessage(err));
          updateConnection(connId, (state) => ({
            ...state,
            isScanning: false,
          }));
          return;
        }
        // 開始の完了前に止められていた。停止が開始より先にバックエンドへ届いた場合に備えて、
        // もう一度止める (スキャン用の接続を残さない)。
        if (superseded()) await api.stopTopicScan(connId).catch(() => {});
      })();
      scanRpcs.set(connId, starting);
      await starting;
    } else {
      updateConnection(connId, (state) => ({ ...state, isScanning: false }));
      // 停止は待たずに送る。バックエンドは開始中のスキャンを打ち切る。
      const stopping = api.stopTopicScan(connId).catch((err) => {
        logger.error("MQTT topic scan failed to stop", {
          connection_id: connId,
          error: String(err),
        });
      });
      scanRpcs.set(connId, Promise.all([previous, stopping]));
      await stopping;
    }
  };

  const toggleMute = (id: string) => {
    const connId = activeConnection()?.connectionId;
    if (!connId) return;
    updateConnection(connId, (state) => ({
      ...state,
      subscriptions: state.subscriptions.map((s) =>
        s.id === id ? { ...s, muted: !s.muted } : s,
      ),
    }));
  };

  return {
    newTopic,
    setNewTopic,
    newQos,
    setNewQos,
    subscriptions,
    brokerTopics,
    isScanning,
    addSubscription,
    removeSubscription,
    toggleMute,
    setIsScanning,
  } as const;
}
