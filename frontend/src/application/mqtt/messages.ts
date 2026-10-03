import { createEffect, createMemo, createSignal, untrack } from "solid-js";
import {
  hasWildcard,
  stripSharedPrefix,
  topicMatches,
} from "../../domain/mqtt/topic";
import type { ConnectionStateExt, MqttMessageView } from "./connections";

/**
 * トピックフィルターの選択肢を作る。
 * 購読トピックそのものに加え、ワイルドカード購読に一致した実トピックも列挙する。
 * 受信したメッセージのトピックには共有購読の接頭辞が付かないので、接頭辞を外して照合する。
 */
export function collectFilterTopics(
  subscriptions: readonly { topic: string }[],
  messages: readonly { topic: string }[],
): string[] {
  const result = new Set<string>();

  for (const sub of subscriptions) {
    result.add(sub.topic);
    const match = stripSharedPrefix(sub.topic);
    if (hasWildcard(match)) {
      for (const msg of messages) {
        if (topicMatches(match, msg.topic)) {
          result.add(msg.topic);
        }
      }
    }
  }

  return Array.from(result).sort();
}

/**
 * フィルターが空なら元の配列をそのまま返す（参照の同一性を保つ）。
 * 選択肢には購読の文字列と実トピックが並ぶ。共有購読の接頭辞を外して照合するのは、
 * フィルターが購読の文字列のときだけにする（実トピックは受信したままの文字列で照合する）。
 */
export function filterMessagesByTopic(
  messages: MqttMessageView[],
  filter: string,
  subscriptions: readonly { topic: string }[],
): MqttMessageView[] {
  if (!filter) return messages;
  if (!subscriptions.some((s) => s.topic === filter)) {
    return messages.filter((m) => m.topic === filter);
  }
  const match = stripSharedPrefix(filter);
  const wildcard = hasWildcard(match);
  // 購読と同じ文字列の実トピックを受信していることもあるので、filter との完全一致も残す。
  return messages.filter(
    (m) =>
      m.topic === filter ||
      (wildcard ? topicMatches(match, m.topic) : m.topic === match),
  );
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

  // フィルターは接続ごとではなく、パネルで 1 つの値として持つ。
  const [topicFilter, setTopicFilter] = createSignal("");
  const filterTopics = createMemo(() =>
    collectFilterTopics(activeConnection()?.subscriptions ?? [], messages()),
  );
  const visibleMessages = createMemo(() =>
    filterMessagesByTopic(
      messages(),
      topicFilter(),
      activeConnection()?.subscriptions ?? [],
    ),
  );

  // 選択肢から消えたフィルターは解除する
  createEffect(() => {
    const filter = topicFilter();
    if (filter && !filterTopics().includes(filter)) setTopicFilter("");
  });

  // autoFollow が true のとき selectedMessage を表示中の一覧の末尾に追従させる。
  // 追従先を全メッセージの末尾にすると、フィルター中に一覧に無いメッセージを選んでしまう。
  // 選択を書き換える effect はここだけにする（追従先の違う effect が他にあると、
  // 互いに書き換え続けて止まらなくなる）。
  createEffect(() => {
    const visible = visibleMessages();
    const follow = autoFollow();
    if (!follow || visible.length === 0) return;
    const lastMsg = visible[visible.length - 1];
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
    visibleMessages,
    topicFilter,
    setTopicFilter,
    filterTopics,
    selectedMessage,
    autoFollow,
    setSelectedMessage,
    setAutoFollow,
    clearMessages,
  } as const;
}
