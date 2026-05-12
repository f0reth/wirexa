import { createEffect, untrack } from "solid-js";
import type { ConnectionStateExt, MqttMessageView } from "./connections";

export function createMessagesState(
  activeConnection: () => ConnectionStateExt | null,
  updateConnection: (
    id: string,
    updater: (state: ConnectionStateExt) => ConnectionStateExt,
  ) => void,
) {
  const messages = () => activeConnection()?.messages ?? [];
  const selectedMessage = () => activeConnection()?.selectedMessage ?? null;
  const autoFollow = () => activeConnection()?.autoFollow ?? false;

  // autoFollow が true のとき selectedMessage を末尾に追従させる
  createEffect(() => {
    const msgs = messages();
    const follow = autoFollow();
    if (!follow || msgs.length === 0) return;
    const lastMsg = msgs[msgs.length - 1];
    const connId = activeConnection()?.connectionId;
    if (!connId) return;
    // untrack で selectedMessage への依存を切り、追従後の再実行を防ぐ
    if (untrack(selectedMessage) !== lastMsg) {
      updateConnection(connId, (state) => ({
        ...state,
        selectedMessage: lastMsg,
      }));
    }
  });

  const setSelectedMessage = (msg: MqttMessageView | null) => {
    const connId = activeConnection()?.connectionId;
    if (!connId) return;
    updateConnection(connId, (state) => ({ ...state, selectedMessage: msg }));
  };

  const setAutoFollow = (value: boolean | ((prev: boolean) => boolean)) => {
    const connId = activeConnection()?.connectionId;
    if (!connId) return;
    const next = typeof value === "function" ? value(autoFollow()) : value;
    updateConnection(connId, (state) => ({ ...state, autoFollow: next }));
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
