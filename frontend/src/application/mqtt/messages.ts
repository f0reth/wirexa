import { createEffect } from "solid-js";
import type { ConnectionState, MqttMessage } from "../../domain/mqtt/types";

export function createMessagesState(
  activeConnection: () => ConnectionState | null,
  updateConnection: (
    id: string,
    updater: (state: ConnectionState) => ConnectionState,
  ) => void,
) {
  const messages = () => activeConnection()?.messages ?? [];
  const selectedMessage = () => activeConnection()?.selectedMessage ?? null;
  const autoFollow = () => activeConnection()?.autoFollow ?? false;

  // autoFollow が true のとき selectedMessage を末尾に追従させる
  // scrollTop は操作しない（Presentation 層の責務）
  createEffect(() => {
    const msgs = messages();
    const follow = autoFollow();
    if (follow && msgs.length > 0) {
      const lastMsg = msgs[msgs.length - 1];
      if (selectedMessage() !== lastMsg) {
        const connId = activeConnection()?.connectionId;
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
    const connId = activeConnection()?.connectionId;
    if (!connId) return;
    updateConnection(connId, (state) => ({ ...state, selectedMessage: msg }));
  };

  const setAutoFollow = (value: boolean | ((prev: boolean) => boolean)) => {
    const connId = activeConnection()?.connectionId;
    if (!connId) return;
    updateConnection(connId, (state) => ({
      ...state,
      autoFollow: typeof value === "function" ? value(state.autoFollow) : value,
    }));
  };

  const clearMessages = () => {
    const connId = activeConnection()?.connectionId;
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
