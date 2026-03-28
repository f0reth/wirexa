import { createSignal, Show } from "solid-js";
import { useUdpSend } from "../../providers/udp-provider";
import { ListenForm } from "./listen-form";
import { MessageLog } from "./message-log";
import { SendForm } from "./send-form";
import styles from "./udp.module.css";

type Tab = "send" | "listen";

export function UdpClient() {
  const [tab, setTab] = createSignal<Tab>("send");
  const { selectedTarget } = useUdpSend();

  return (
    <div class={styles.container}>
      <Show
        when={selectedTarget()}
        fallback={
          <div class={styles.resultEmpty}>
            <span class={styles.resultEmptyText}>
              ターゲットを選択してください
            </span>
          </div>
        }
      >
        <div class={styles.tabBar}>
          <button
            type="button"
            class={`${styles.tabButton} ${tab() === "send" ? styles.tabButtonActive : ""}`}
            onClick={() => setTab("send")}
          >
            Send
          </button>
          <button
            type="button"
            class={`${styles.tabButton} ${tab() === "listen" ? styles.tabButtonActive : ""}`}
            onClick={() => setTab("listen")}
          >
            Listen
          </button>
        </div>
        {tab() === "send" ? (
          <SendForm />
        ) : (
          <>
            <ListenForm />
            <MessageLog />
          </>
        )}
      </Show>
    </div>
  );
}
