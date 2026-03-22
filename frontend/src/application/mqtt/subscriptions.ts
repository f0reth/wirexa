import { createSignal } from "solid-js";
import { compilePattern } from "../../domain/mqtt/topic";
import type { ConnectionState, Subscription } from "../../domain/mqtt/types";
import * as client from "../../infrastructure/mqtt/client";

export function createSubscriptionsState(
  activeConnection: () => ConnectionState | null,
  updateConnection: (
    id: string,
    updater: (state: ConnectionState) => ConnectionState,
  ) => void,
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
        await client.subscribe(connId, t, q);
      } catch (err) {
        console.error(`[MQTT] Subscribe failed for ${t}:`, err);
        return;
      }
      const isWildcard = t.includes("+") || t.includes("#");
      const newSub: Subscription = {
        id: Date.now().toString(),
        topic: t,
        qos: q as 0 | 1 | 2,
        patternParts: isWildcard ? compilePattern(t) : undefined,
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
        await client.unsubscribe(connId, sub.topic);
      } catch (err) {
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
        await client.subscribe(connId, "#", 0);
      } catch (err) {
        console.error("[MQTT] Failed to subscribe to #:", err);
        updateConnection(connId, (state) => ({ ...state, isScanning: false }));
      }
    } else {
      try {
        await client.unsubscribe(connId, "#");
      } catch {}
      updateConnection(connId, (state) => ({ ...state, isScanning: false }));
    }
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
    setIsScanning,
  } as const;
}
