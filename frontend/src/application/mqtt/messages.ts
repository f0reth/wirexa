import { createEffect, createSignal, untrack } from "solid-js";
import type { ConnectionStateExt, MqttMessageView } from "./connections";

export function createMessagesState(
  activeConnection: () => ConnectionStateExt | null,
  updateConnection: (
    id: string,
    updater: (state: ConnectionStateExt) => ConnectionStateExt,
  ) => void,
) {
  const messages = () => activeConnection()?.messages ?? [];
  const [selectedMessage, setSelectedMessageSignal] =
    createSignal<MqttMessageView | null>(null);
  const [autoFollow, setAutoFollowSignal] = createSignal(false);

  // autoFollow が true のとき selectedMessage を末尾に追従させる
  // scrollTop は操作しない（Presentation 層の責務）
  createEffect(() => {
    const msgs = messages();
    const follow = autoFollow();
    if (follow && msgs.length > 0) {
      const lastMsg = msgs[msgs.length - 1];
      if (untrack(selectedMessage) !== lastMsg) {
        setSelectedMessageSignal(lastMsg);
      }
    }
  });

  const setSelectedMessage = (msg: MqttMessageView | null) => {
    setSelectedMessageSignal(msg);
  };

  const setAutoFollow = (value: boolean | ((prev: boolean) => boolean)) => {
    setAutoFollowSignal(value);
  };

  const clearMessages = () => {
    const connId = activeConnection()?.connectionId;
    if (!connId) return;
    updateConnection(connId, (state) => ({
      ...state,
      messages: [],
    }));
    setSelectedMessageSignal(null);
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
