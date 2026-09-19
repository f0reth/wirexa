import { createVirtualizer } from "@tanstack/solid-virtual";
import { clsx } from "clsx";
import { Radio, X, Zap } from "lucide-solid";
import { createEffect, createMemo, createSignal, For, Show } from "solid-js";
import {
  collectFilterTopics,
  filterMessagesByTopic,
} from "../../../../application/mqtt/messages";
import {
  useMqttMessages,
  useMqttSubscribe,
} from "../../../providers/mqtt-provider";
import { formatTime } from "../../../utils/format";
import { base64ByteLength } from "../../shared/hex-view";
import styles from "../messages.module.css";
import base from "../mqtt.module.css";
import { getTopicColor } from "../utils";

export function MessagesPanel() {
  const {
    messages,
    selectedMessage,
    setSelectedMessage,
    autoFollow,
    setAutoFollow,
    clearMessages,
  } = useMqttMessages();
  const { subscriptions } = useMqttSubscribe();

  const [topicFilter, setTopicFilter] = createSignal("");

  const uniqueTopics = createMemo(() =>
    collectFilterTopics(subscriptions(), messages()),
  );

  // Reset filter when the selected topic disappears from subscriptions
  createEffect(() => {
    const filter = topicFilter();
    if (filter && !uniqueTopics().includes(filter)) {
      setTopicFilter("");
    }
  });

  const filteredMessages = createMemo(() =>
    filterMessagesByTopic(messages(), topicFilter()),
  );

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
      <div class={base.sectionHeader}>
        <h3 class={base.sectionTitle}>Messages</h3>
        <div class={base.sectionHeaderActions}>
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
              base.headerAction,
              autoFollow() && base.headerActionActive,
            )}
            onClick={() => setAutoFollow((v) => !v)}
            title="Auto-follow latest message"
          >
            <Zap size={14} />
            Auto
          </button>
          <button
            type="button"
            class={base.headerAction}
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
            <div class={base.listPadding}>
              <p class={base.emptyText}>No messages yet</p>
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
                      <p class={styles.messagePayload}>
                        {msg().payloadBase64
                          ? `[binary ${base64ByteLength(msg().payload)} bytes]`
                          : msg().payload}
                      </p>
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
