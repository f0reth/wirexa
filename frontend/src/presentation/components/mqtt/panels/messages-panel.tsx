import { createVirtualizer } from "@tanstack/solid-virtual";
import { clsx } from "clsx";
import { Radio, X, Zap } from "lucide-solid";
import { createEffect, createMemo, createSignal, For, Show } from "solid-js";
import { useMqttMessages } from "../../../providers/mqtt-provider";
import styles from "../mqtt.module.css";
import { formatTime, getTopicColor } from "../utils";

export function MessagesPanel() {
  const {
    messages,
    selectedMessage,
    setSelectedMessage,
    autoFollow,
    setAutoFollow,
    clearMessages,
  } = useMqttMessages();

  const [topicFilter, setTopicFilter] = createSignal("");

  const uniqueTopics = createMemo(() => {
    const topics = new Set(messages().map((m) => m.topic));
    return Array.from(topics).sort();
  });

  // Reset filter when the selected topic disappears from messages
  createEffect(() => {
    const filter = topicFilter();
    if (filter && !uniqueTopics().includes(filter)) {
      setTopicFilter("");
    }
  });

  const filteredMessages = createMemo(() => {
    const filter = topicFilter();
    if (!filter) return messages();
    return messages().filter((m) => m.topic === filter);
  });

  let scrollRef!: HTMLDivElement;

  const virtualizer = createVirtualizer({
    get count() {
      return filteredMessages().length;
    },
    getScrollElement: () => scrollRef,
    estimateSize: () => 80,
    overscan: 5,
    paddingStart: 10,
    paddingEnd: 10,
    measureElement: (el) => el?.getBoundingClientRect().height ?? 80,
  });

  createEffect(() => {
    if (autoFollow() && filteredMessages().length > 0) {
      virtualizer.scrollToIndex(filteredMessages().length - 1, {
        align: "end",
      });
    }
  });

  // フィルター有効時は最後のフィルター済みメッセージを選択する
  createEffect(() => {
    const filter = topicFilter();
    if (!filter || !autoFollow()) return;
    const filtered = filteredMessages();
    if (filtered.length > 0) {
      setSelectedMessage(filtered[filtered.length - 1]);
    }
  });

  return (
    <div class={styles.messagesPanel}>
      <div class={styles.sectionHeader}>
        <h3 class={styles.sectionTitle}>Messages</h3>
        <div class={styles.sectionHeaderActions}>
          <select
            class={styles.topicFilterSelect}
            value={topicFilter()}
            onChange={(e) => setTopicFilter(e.target.value)}
            title="Filter by topic"
          >
            <option value="">All topics</option>
            <For each={uniqueTopics()}>
              {(topic) => <option value={topic}>{topic}</option>}
            </For>
          </select>
          <button
            type="button"
            class={clsx(
              styles.headerAction,
              autoFollow() && styles.headerActionActive,
            )}
            onClick={() => setAutoFollow((v) => !v)}
            title="Auto-follow latest message"
          >
            <Zap size={14} />
            Auto
          </button>
          <button
            type="button"
            class={styles.headerAction}
            onClick={clearMessages}
            title="Clear all messages"
          >
            <X size={14} />
            Clear
          </button>
        </div>
      </div>

      <div
        ref={(el) => {
          scrollRef = el;
        }}
        class={styles.messagesScrollArea}
      >
        <Show
          when={filteredMessages().length > 0}
          fallback={
            <div class={styles.listPadding}>
              <p class={styles.emptyText}>No messages yet</p>
            </div>
          }
        >
          <div
            class={styles.messagesVirtualContainer}
            style={{ height: `${virtualizer.getTotalSize()}px` }}
          >
            <For each={virtualizer.getVirtualItems()}>
              {(virtualItem) => {
                const msg = () => filteredMessages()[virtualItem.index];
                return (
                  <div
                    data-index={virtualItem.index}
                    ref={(el) => virtualizer.measureElement(el)}
                    class={styles.messagesVirtualItem}
                    style={{ transform: `translateY(${virtualItem.start}px)` }}
                  >
                    <button
                      type="button"
                      class={clsx(
                        styles.messageItem,
                        selectedMessage()?.id === msg().id &&
                          styles.messageItemSelected,
                      )}
                      onClick={() => setSelectedMessage(msg())}
                    >
                      <div class={styles.messageItemHeader}>
                        <div class={styles.messageItemLeft}>
                          <Radio size={14} color={getTopicColor(msg().topic)} />
                          <span class={styles.messageTopic}>{msg().topic}</span>
                        </div>
                        <span class={styles.messageTime}>
                          {formatTime(msg().timestamp)}
                        </span>
                      </div>
                      <p class={styles.messagePayload}>{msg().payload}</p>
                    </button>
                  </div>
                );
              }}
            </For>
          </div>
        </Show>
      </div>
    </div>
  );
}
