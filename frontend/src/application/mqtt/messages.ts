import { createEffect, untrack } from "solid-js";
import { topicMatches } from "../../domain/mqtt/topic";
import type { ConnectionStateExt, MqttMessageView } from "./connections";

/** トピックにワイルドカードが含まれるか。含む場合はパターン照合が必要になる。 */
function hasWildcard(topic: string): boolean {
  return topic.includes("#") || topic.includes("+");
}

/**
 * トピックフィルターの選択肢を作る。
 * 購読トピックそのものに加え、ワイルドカード購読に一致した実トピックも列挙する。
 */
export function collectFilterTopics(
  subscriptions: readonly { topic: string }[],
  messages: readonly { topic: string }[],
): string[] {
  const result = new Set<string>();

  for (const sub of subscriptions) {
    result.add(sub.topic);
    if (hasWildcard(sub.topic)) {
      for (const msg of messages) {
        if (topicMatches(sub.topic, msg.topic)) {
          result.add(msg.topic);
        }
      }
    }
  }

  return Array.from(result).sort();
}

/** フィルターが空なら元の配列をそのまま返す（参照の同一性を保つ）。 */
export function filterMessagesByTopic(
  messages: MqttMessageView[],
  filter: string,
): MqttMessageView[] {
  if (!filter) return messages;
  if (hasWildcard(filter)) {
    return messages.filter((m) => topicMatches(filter, m.topic));
  }
  return messages.filter((m) => m.topic === filter);
}

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
