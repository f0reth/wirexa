import { createVirtualizer } from "@tanstack/solid-virtual";
import { clsx } from "clsx";
import { Radio, X, Zap } from "lucide-solid";
import { createEffect, For, Show } from "solid-js";
import { useMqttMessages } from "../../../providers/mqtt-provider";
import { formatTime } from "../../../utils/format";
import { base64ByteLength } from "../../shared/hex-view";
import styles from "../messages.module.css";
import base from "../mqtt.module.css";
import { getTopicColor } from "../utils";

export function MessagesPanel() {
  const {
    visibleMessages,
    topicFilter,
    setTopicFilter,
    filterTopics,
    selectedMessage,
    setSelectedMessage,
    autoFollow,
    setAutoFollow,
    clearMessages,
  } = useMqttMessages();

  let scrollRef!: HTMLDivElement;

  const virtualizer = createVirtualizer({
    get count() {
      return visibleMessages().length;
    },
    getScrollElement: () => scrollRef,
    estimateSize: () => 80,
    overscan: 5,
    paddingStart: 10,
    paddingEnd: 10,
    measureElement: (el) => el?.getBoundingClientRect().height ?? 80,
  });

  // 選択の追従は application 層 (createMessagesState) が行う。ここはスクロールだけを追従させる。
  createEffect(() => {
    if (autoFollow() && visibleMessages().length > 0) {
      virtualizer.scrollToIndex(visibleMessages().length - 1, {
        align: "end",
      });
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
            <For each={filterTopics()}>
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
          when={visibleMessages().length > 0}
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
                return (
                  <div
                    data-index={virtualItem.index}
                    ref={(el) => virtualizer.measureElement(el)}
                    class={styles.messagesVirtualItem}
                    style={{ transform: `translateY(${virtualItem.start}px)` }}
                  >
                    {/* 一覧が縮むと (フィルターの切り替えなど)、範囲外になった行は For が
                        取り除く前に再描画されることがある。メッセージが無い行は中身を描かない。 */}
                    <Show when={visibleMessages()[virtualItem.index]}>
                      {(msg) => (
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
                              <Radio
                                size={14}
                                color={getTopicColor(msg().topic)}
                              />
                              <span class={styles.messageTopic}>
                                {msg().topic}
                              </span>
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
                      )}
                    </Show>
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
