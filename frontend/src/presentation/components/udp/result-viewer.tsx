import { Show } from "solid-js";
import { useUdpSend } from "../../providers/udp-provider";
import styles from "./udp.module.css";

export function ResultViewer() {
  const { result } = useUdpSend();

  return (
    <div class={styles.resultViewer}>
      <Show when={result() === null}>
        <div class={styles.resultEmpty}>
          <p class={styles.resultEmptyText}>No result yet — press Send</p>
        </div>
      </Show>
      <Show when={result() !== null}>
        <div class={styles.resultSuccess}>
          <span class={styles.resultSuccessText}>
            Sent {result()?.bytesSent} byte
            {result()?.bytesSent !== 1 ? "s" : ""}
          </span>
        </div>
      </Show>
    </div>
  );
}
