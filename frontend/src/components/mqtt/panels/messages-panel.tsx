import { createVirtualizer } from "@tanstack/solid-virtual";
import { clsx } from "clsx";
import { Radio, X, Zap } from "lucide-solid";
import { createEffect, createMemo, createSignal, Show } from "solid-js";
import { useMqtt } from "../context";
import styles from "../mqtt.module.css";
import { formatTime, getTopicColor } from "../utils";

const ROW_HEIGHT = 56;

export function MessagesPanel() {
  const {
    messages,
    selectedMessage,
    setSelectedMessage,
    autoFollow,
    setAutoFollow,
    clearMessages,
  } = useMqtt();

  const [scrollRef, setScrollRef] = createSignal<HTMLDivElement>();

  const virtualizer = createVirtualizer({
    get count() {
      return messages().length;
    },
    getScrollElement: () => scrollRef() ?? null,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  });

  // Auto-follow: scroll to bottom when new messages arrive
  createEffect(() => {
    const len = messages().length;
    if (autoFollow() && len > 0) {
      virtualizer.scrollToIndex(len - 1, { align: "end" });
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

      <Show
        when={messages().length > 0}
        fallback={
          <div class={styles.listPadding}>
            <p class={styles.emptyText}>No messages yet</p>
          </div>
        }
      >
        {(() => {
          const formattedTimes = createMemo(() => {
            const msgs = messages();
            const times = new Array<string>(msgs.length);
            for (let i = 0; i < msgs.length; i++) {
              times[i] = formatTime(msgs[i].timestamp);
            }
            return times;
          });

          return (
            <div
              ref={setScrollRef}
              class={styles.messagesScrollArea}
              style={{ overflow: "auto" }}
            >
              <div
                style={{
                  height: `${virtualizer.getTotalSize()}px`,
                  width: "100%",
                  position: "relative",
                }}
              >
                {virtualizer.getVirtualItems().map((virtualRow) => {
                  const msg = messages()[virtualRow.index];
                  if (!msg) return null;
                  return (
                    <button
                      type="button"
                      class={clsx(
                        styles.messageItem,
                        selectedMessage()?.id === msg.id &&
                          styles.messageItemSelected,
                      )}
                      style={{
                        position: "absolute",
                        top: 0,
                        left: 0,
                        width: "100%",
                        height: `${virtualRow.size}px`,
                        transform: `translateY(${virtualRow.start}px)`,
                      }}
                      onClick={() => setSelectedMessage(msg)}
                    >
                      <div class={styles.messageItemHeader}>
                        <div class={styles.messageItemLeft}>
                          <Radio size={14} color={getTopicColor(msg.topic)} />
                          <span class={styles.messageTopic}>{msg.topic}</span>
                        </div>
                        <span class={styles.messageTime}>
                          {formattedTimes()[virtualRow.index]}
                        </span>
                      </div>
                      <p class={styles.messagePayload}>{msg.payload}</p>
                    </button>
                  );
                })}
              </div>
            </div>
          );
        })()}
      </Show>
    </div>
  );
}
