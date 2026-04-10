import { Check, Copy } from "lucide-solid";
import { createMemo, createSignal, Show } from "solid-js";
import { Badge } from "../../../../components/ui/badge";
import { useMqttMessages } from "../../../providers/mqtt-provider";
import styles from "../mqtt.module.css";
import { formatPayload, formatTime } from "../utils";

export function MessageDetail() {
  const { selectedMessage } = useMqttMessages();

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
          const [copied, setCopied] = createSignal(false);

          function handleCopy() {
            navigator.clipboard.writeText(formattedPayload());
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          }

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
                  <button
                    type="button"
                    class={styles.copyBtn}
                    onClick={handleCopy}
                    title="Copy payload"
                  >
                    <Show when={copied()} fallback={<Copy size={13} />}>
                      <Check size={13} />
                    </Show>
                  </button>
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
