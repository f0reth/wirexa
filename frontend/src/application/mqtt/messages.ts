import { createEffect } from "solid-js";
import type { ConnectionState, MqttMessage } from "../../domain/mqtt/types";

export function createMessagesState(
  connections: () => Map<string, ConnectionState>,
  updateConnection: (
    id: string,
    updater: (state: ConnectionState) => ConnectionState,
  ) => void,
  activeConnectionId: () => string | null,
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

  // autoFollow が true のとき selectedMessage を末尾に追従させる
  // scrollTop は操作しない（Presentation 層の責務）
  createEffect(() => {
    const msgs = messages();
    const follow = autoFollow();
    if (follow && msgs.length > 0) {
      const lastMsg = msgs[msgs.length - 1];
      if (selectedMessage() !== lastMsg) {
        const connId = activeConnectionId();
        if (connId) {
          updateConnection(connId, (state) => ({
            ...state,
            selectedMessage: lastMsg,
          }));
        }
      }
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
  } as const;
}
