import { createSignal } from "solid-js";
import { compilePattern } from "../../domain/mqtt/topic";
import type { ConnectionState, Subscription } from "../../domain/mqtt/types";
import * as client from "../../infrastructure/mqtt/client";

export function createSubscriptionsState(
  connections: () => Map<string, ConnectionState>,
  updateConnection: (
    id: string,
    updater: (state: ConnectionState) => ConnectionState,
  ) => void,
  activeConnectionId: () => string | null,
) {
  const [newTopic, setNewTopic] = createSignal("");
  const [newQos, setNewQos] = createSignal<number>(0);

  const subscriptions = () => {
    const id = activeConnectionId();
    return id ? (connections().get(id)?.subscriptions ?? []) : [];
  };

  const brokerTopics = () => {
    const id = activeConnectionId();
    return id ? (connections().get(id)?.brokerTopics ?? []) : [];
  };

  const isScanning = () => {
    const id = activeConnectionId();
    return id ? (connections().get(id)?.isScanning ?? false) : false;
  };

  const addSubscription = async (topic?: string, qos?: number) => {
    const t = (topic ?? newTopic()).trim();
    if (!t) return;
    const connId = activeConnectionId();
    const conn = connId ? connections().get(connId) : null;
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
    const connId = activeConnectionId();
    if (!connId) return;
    const sub = subscriptions().find((s) => s.id === id);
    if (!sub) return;
    const isConnected = connections().get(connId)?.connected ?? false;
    if (isConnected) {
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
    const connId = activeConnectionId();
    if (!connId) return;
    const conn = connections().get(connId);
    if (!conn) return;
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
