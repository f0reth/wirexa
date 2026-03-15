import { createEffect } from "solid-js";
import type { ConnectionState, MqttMessage, Tab } from "../types";

export function createMessagesState(
  connections: () => Map<string, ConnectionState>,
  updateConnection: (
    id: string,
    updater: (state: ConnectionState) => ConnectionState,
  ) => void,
  activeConnectionId: () => string | null,
  activeTab: () => Tab,
) {
  const messages = () => {
    const id = activeConnectionId();
    return id ? (connections().get(id)?.messages ?? []) : [];
  };

  const selectedMessage = () => {
    const id = activeConnectionId();
    return id ? (connections().get(id)?.selectedMessage ?? null) : null;
  };

  const autoFollow = () => {
    const id = activeConnectionId();
    return id ? (connections().get(id)?.autoFollow ?? false) : false;
  };

  let messagesScrollRef: HTMLDivElement | undefined;

  const scrollToBottom = () => {
    requestAnimationFrame(() => {
      if (messagesScrollRef) {
        messagesScrollRef.scrollTop = messagesScrollRef.scrollHeight;
      }
    });
  };

  const setMessagesScrollRef = (el: HTMLDivElement) => {
    messagesScrollRef = el;
  };

  createEffect(() => {
    const msgs = messages();
    const follow = autoFollow();
    const tab = activeTab();

    if (follow && msgs.length > 0) {
      const connId = activeConnectionId();
      if (connId) {
        updateConnection(connId, (state) => ({
          ...state,
          selectedMessage: msgs[msgs.length - 1],
        }));
      }
    }

    if (follow && tab === "subscribe") {
      scrollToBottom();
    }
  });

  const setSelectedMessage = (msg: MqttMessage | null) => {
    const connId = activeConnectionId();
    if (!connId) return;
    updateConnection(connId, (state) => ({ ...state, selectedMessage: msg }));
  };

  const setAutoFollow = (value: boolean | ((prev: boolean) => boolean)) => {
    const connId = activeConnectionId();
    if (!connId) return;
    updateConnection(connId, (state) => ({
      ...state,
      autoFollow: typeof value === "function" ? value(state.autoFollow) : value,
    }));
  };

  const clearMessages = () => {
    const connId = activeConnectionId();
    if (!connId) return;
    updateConnection(connId, (state) => ({
      ...state,
      messages: [],
      selectedMessage: null,
    }));
  };

  return {
    messages,
    selectedMessage,
    autoFollow,
    setSelectedMessage,
    setAutoFollow,
    clearMessages,
    setMessagesScrollRef,
  } as const;
}
