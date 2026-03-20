import { createSignal } from "solid-js";
import {
  Subscribe,
  Unsubscribe,
} from "../../../../wailsjs/go/mqtt/MqttService";
import type { ConnectionState, Subscription } from "../types";
import { compilePattern } from "./shared";

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
    if (conn?.subscriptions.some((s) => s.topic === t)) {
      if (!topic) setNewTopic("");
      return;
    }

    const q = qos ?? newQos();

    if (connId) {
      try {
        await Subscribe(connId, t, q);
      } catch (err) {
        console.error(`[MQTT] Subscribe failed for ${t}:`, err);
        return;
      }
      const isWildcard = t.includes("+") || t.includes("#");
      const newSub: Subscription = {
        id: Date.now().toString(),
        topic: t,
        qos: q,
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
    const conn = connections().get(connId);
    if (!conn) return;
    const sub = conn.subscriptions.find((s) => s.id === id);
    if (!sub) return;

    try {
      await Unsubscribe(connId, sub.topic);
    } catch (err) {
      console.error(`[MQTT] Unsubscribe failed for ${sub.topic}:`, err);
    }

    updateConnection(connId, (state) => ({
      ...state,
      messages: state.messages.filter((m) => m.topic !== sub.topic),
      selectedMessage:
        state.selectedMessage?.topic === sub.topic
          ? null
          : state.selectedMessage,
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
        await Subscribe(connId, "#", 0);
      } catch (err) {
        console.error("[MQTT] Failed to subscribe to #:", err);
        updateConnection(connId, (state) => ({ ...state, isScanning: false }));
      }
    } else {
      try {
        await Unsubscribe(connId, "#");
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
