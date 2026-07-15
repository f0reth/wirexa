import { Check, Copy } from "lucide-solid";
import { createMemo, Show } from "solid-js";
import { Badge } from "../../../../components/ui/badge";
import { createCopyButton } from "../../../../components/ui/copy-button";
import { useMqttMessages } from "../../../providers/mqtt-provider";
import { HexView } from "../../shared/hex-view";
import styles from "../messages.module.css";
import base from "../mqtt.module.css";
import { formatPayload, formatTime } from "../utils";

export function MessageDetail() {
  const { selectedMessage } = useMqttMessages();

  return (
    <div class={styles.messageDetail}>
      <Show
        when={selectedMessage()}
        fallback={
          <p class={base.emptyText}>Select a message to view details</p>
        }
      >
        {(msg) => {
          const formattedPayload = createMemo(() =>
            formatPayload(msg().payload),
          );
          const formattedTime = createMemo(() => formatTime(msg().timestamp));
          const { copy, isCopied } = createCopyButton();

          function handleCopy() {
            copy(formattedPayload());
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
                    <Show when={isCopied()} fallback={<Copy size={13} />}>
                      <Check size={13} />
                    </Show>
                  </button>
                </div>
              </div>
              <Show
                when={msg().payloadBase64}
                fallback={
                  <pre class={styles.messageDetailPayload}>
                    {formattedPayload()}
                  </pre>
                }
              >
                <HexView base64={msg().payload} />
              </Show>
            </>
          );
        }}
      </Show>
    </div>
  );
}
