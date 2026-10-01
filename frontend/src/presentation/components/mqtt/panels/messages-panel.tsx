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

/**
 * 一覧の 1 行の高さ (px)。ペイロードのプレビューが 2 行の行の高さで、行間の余白を含む。
 * 行の高さは計測せず、全行をこの高さに揃える。@tanstack/solid-virtual は件数が変わるたびに
 * 計測結果を捨てるので、受信が続く一覧では計測した高さで並べ続けられない。
 * 行の中の寸法 (messages.module.css、rem) はルートのフォントサイズ 16px でこの値になる。
 */
const MESSAGE_ROW_HEIGHT = 81;

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
    estimateSize: () => MESSAGE_ROW_HEIGHT,
    overscan: 5,
    paddingStart: 10,
    paddingEnd: 10,
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
                    class={styles.messagesVirtualItem}
                    style={{
                      height: `${virtualItem.size}px`,
                      transform: `translateY(${virtualItem.start}px)`,
                    }}
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
