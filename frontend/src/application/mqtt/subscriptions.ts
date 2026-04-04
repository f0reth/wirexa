import { createSignal } from "solid-js";
import { compilePattern } from "../../domain/mqtt/topic";
import type { Subscription } from "../../domain/mqtt/types";
import { log } from "../../infrastructure/logger/client";
import type { ConnectionStateExt } from "./connections";

export interface SubscriptionApi {
  subscribe(connectionId: string, topic: string, qos: number): Promise<void>;
  unsubscribe(connectionId: string, topic: string): Promise<void>;
}

export function createSubscriptionsState(
  activeConnection: () => ConnectionStateExt | null,
  updateConnection: (
    id: string,
    updater: (state: ConnectionStateExt) => ConnectionStateExt,
  ) => void,
  api: SubscriptionApi,
) {
  const [newTopic, setNewTopic] = createSignal("");
  const [newQos, setNewQos] = createSignal<number>(0);

  const subscriptions = () => activeConnection()?.subscriptions ?? [];
  const brokerTopics = () => activeConnection()?.brokerTopics ?? [];
  const isScanning = () => activeConnection()?.isScanning ?? false;

  const addSubscription = async (topic?: string, qos?: number) => {
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
      try {
        await api.subscribe(connId, t, q);
        log({
          level: "INFO",
          source: "frontend:mqtt",
          message: "MQTT subscribed",
          attrs: { connection_id: connId, topic: t, qos: q },
        });
      } catch (err) {
        log({
          level: "ERROR",
          source: "frontend:mqtt",
          message: "MQTT subscribe failed",
          attrs: { connection_id: connId, topic: t, error: String(err) },
        });
        console.error(`[MQTT] Subscribe failed for ${t}:`, err);
        return;
      }
      const isWildcard = t.includes("+") || t.includes("#");
      const newSub: Subscription = {
        id: Date.now().toString(),
        topic: t,
        qos: q as 0 | 1 | 2,
        patternParts: isWildcard ? compilePattern(t) : undefined,
        muted: false,
      };
      updateConnection(connId, (state) => ({
        ...state,
        subscriptions: [...state.subscriptions, newSub],
      }));
    }
    if (!topic) setNewTopic("");
  };

  const removeSubscription = async (id: string) => {
    const connId = activeConnection()?.connectionId;
    if (!connId) return;
    const sub = subscriptions().find((s) => s.id === id);
    if (!sub) return;
    const conn = activeConnection();
    if (conn?.type === "online" && conn.connected) {
      try {
        await api.unsubscribe(connId, sub.topic);
        log({
          level: "INFO",
          source: "frontend:mqtt",
          message: "MQTT unsubscribed",
          attrs: { connection_id: connId, topic: sub.topic },
        });
      } catch (err) {
        log({
          level: "ERROR",
          source: "frontend:mqtt",
          message: "MQTT unsubscribe failed",
          attrs: {
            connection_id: connId,
            topic: sub.topic,
            error: String(err),
          },
        });
        console.error(`[MQTT] Unsubscribe failed for ${sub.topic}:`, err);
      }
    }
    updateConnection(connId, (state) => ({
      ...state,
      subscriptions: state.subscriptions.filter((s) => s.id !== id),
    }));
  };

  const setIsScanning = async (
    value: boolean | ((prev: boolean) => boolean),
  ) => {
    const conn = activeConnection();
    const connId = conn?.connectionId;
    if (!connId) return;
    const newValue =
      typeof value === "function" ? value(conn.isScanning) : value;
    if (newValue) {
      updateConnection(connId, (state) => ({
        ...state,
        isScanning: true,
        brokerTopics: [],
        brokerTopicsSet: new Set(),
      }));
      try {
        await api.subscribe(connId, "#", 0);
      } catch (err) {
        console.error("[MQTT] Failed to subscribe to #:", err);
        updateConnection(connId, (state) => ({ ...state, isScanning: false }));
      }
    } else {
      try {
        await api.unsubscribe(connId, "#");
      } catch {}
      updateConnection(connId, (state) => ({ ...state, isScanning: false }));
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
