import { createMemo, Show } from "solid-js";
import { Badge } from "../../ui/badge";
import { useMqtt } from "../context";
import styles from "../mqtt.module.css";
import { formatPayload, formatTime } from "../utils";

export function MessageDetail() {
  const { selectedMessage } = useMqtt();

  return (
    <div class={styles.messageDetail}>
      <Show
        when={selectedMessage()}
        fallback={
          <p class={styles.emptyText}>Select a message to view details</p>
        }
      >
        {(msg) => {
          const formattedPayload = createMemo(() =>
            formatPayload(msg().payload),
          );
          const formattedTime = createMemo(() => formatTime(msg().timestamp));

          return (
            <>
              <div class={styles.messageDetailHeader}>
                <span class={styles.messageDetailTopic}>{msg().topic}</span>
                <div class={styles.messageDetailBadges}>
                  <span class={styles.messageDetailTime}>
                    {formattedTime()}
                  </span>
                  <Badge variant="outline">QoS {msg().qos}</Badge>
                  <Badge
                    variant={
                      msg().direction === "incoming" ? "default" : "secondary"
                    }
                  >
                    {msg().direction}
                  </Badge>
                </div>
              </div>
              <pre class={styles.messageDetailPayload}>
                {formattedPayload()}
              </pre>
            </>
          );
        }}
      </Show>
    </div>
  );
}
