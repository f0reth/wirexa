import { For, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { useUdpReceive } from "../../providers/udp-provider";
import styles from "./udp.module.css";

function formatTime(timestamp: number): string {
  const d = new Date(timestamp);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  const ms = String(d.getMilliseconds()).padStart(3, "0");
  return `${hh}:${mm}:${ss}.${ms}`;
}

export function MessageLog() {
  const { messages, clearMessages } = useUdpReceive();

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
              </div>
              <pre class={styles.messagePayload}>{msg.payload}</pre>
            </div>
          )}
        </For>
      </div>
    </div>
  );
}
