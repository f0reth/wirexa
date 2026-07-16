import { Check, Copy } from "lucide-solid";
import { For, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { createCopyButton } from "../../../components/ui/copy-button";
import { useUdpReceive } from "../../providers/udp-provider";
import { formatTime } from "../../utils/format";
import styles from "./udp.module.css";

export function MessageLog() {
  const { messages, clearMessages } = useUdpReceive();
  const { copy, isCopied } = createCopyButton<number>();

  return (
    <div class={styles.messageLog}>
      <div class={styles.messageLogHeader}>
        <span class={styles.messageLogTitle}>Received ({messages.length})</span>
        <Button variant="ghost" size="sm" onClick={clearMessages}>
          Clear
        </Button>
      </div>
      <Show when={messages.length === 0}>
        <div class={styles.resultEmpty}>
          <p class={styles.resultEmptyText}>No messages received yet</p>
        </div>
      </Show>
      <div class={styles.messageList}>
        <For each={messages}>
          {(msg) => (
            <div class={styles.messageItem}>
              <div class={styles.messageMeta}>
                <span class={styles.messageTime}>
                  {formatTime(msg.timestamp)}
                </span>
                <span class={styles.messageAddr}>{msg.remoteAddr}</span>
                <span class={styles.messageEncoding}>{msg.encoding}</span>
                <button
                  type="button"
                  class={styles.copyBtn}
                  onClick={() => copy(msg.payload, msg.timestamp)}
                  title="Copy payload"
                >
                  <Show
                    when={isCopied(msg.timestamp)}
                    fallback={<Copy size={12} />}
                  >
                    <Check size={12} />
                  </Show>
                </button>
              </div>
              <pre class={styles.messagePayload}>{msg.payload}</pre>
            </div>
          )}
        </For>
      </div>
    </div>
  );
}
