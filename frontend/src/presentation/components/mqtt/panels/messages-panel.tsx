import { createVirtualizer } from "@tanstack/solid-virtual";
import { clsx } from "clsx";
import { Radio, X, Zap } from "lucide-solid";
import { createEffect, For, Show } from "solid-js";
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

  let scrollRef!: HTMLDivElement;

  const virtualizer = createVirtualizer({
    get count() {
      return messages().length;
    },
    getScrollElement: () => scrollRef,
    estimateSize: () => 74,
    overscan: 5,
    paddingStart: 8,
    measureElement: (el) => el?.getBoundingClientRect().height ?? 74,
  });

  createEffect(() => {
    if (autoFollow() && messages().length > 0) {
      virtualizer.scrollToIndex(messages().length - 1, { align: "end" });
    }
  });

  return (
    <div class={styles.messagesPanel}>
      <div class={styles.sectionHeader}>
        <h3 class={styles.sectionTitle}>Messages</h3>
        <div class={styles.sectionHeaderActions}>
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
          when={messages().length > 0}
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
                const msg = () => messages()[virtualItem.index];
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
