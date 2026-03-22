import { clsx } from "clsx";
import { Radio, X, Zap } from "lucide-solid";
import { createEffect, createMemo, For, Show } from "solid-js";
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

  const formattedTimes = createMemo(() => {
    const msgs = messages();
    const times = new Array<string>(msgs.length);
    for (let i = 0; i < msgs.length; i++) {
      times[i] = formatTime(msgs[i].timestamp);
    }
    return times;
  });

  createEffect(() => {
    if (autoFollow() && messages().length > 0) {
      requestAnimationFrame(() => {
        scrollRef.scrollTop = scrollRef.scrollHeight;
      });
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
        style={{ overflow: "auto" }}
      >
        <Show
          when={messages().length > 0}
          fallback={
            <div class={styles.listPadding}>
              <p class={styles.emptyText}>No messages yet</p>
            </div>
          }
        >
          <For each={messages()}>
            {(msg, i) => (
              <button
                type="button"
                class={clsx(
                  styles.messageItem,
                  selectedMessage()?.id === msg.id &&
                    styles.messageItemSelected,
                )}
                onClick={() => setSelectedMessage(msg)}
              >
                <div class={styles.messageItemHeader}>
                  <div class={styles.messageItemLeft}>
                    <Radio size={14} color={getTopicColor(msg.topic)} />
                    <span class={styles.messageTopic}>{msg.topic}</span>
                  </div>
                  <span class={styles.messageTime}>
                    {formattedTimes()[i()]}
                  </span>
                </div>
                <p class={styles.messagePayload}>{msg.payload}</p>
              </button>
            )}
          </For>
        </Show>
      </div>
    </div>
  );
}
