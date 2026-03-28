import { createSignal } from "solid-js";
import { ListenForm } from "./listen-form";
import { MessageLog } from "./message-log";
import { ResultViewer } from "./result-viewer";
import { SendForm } from "./send-form";
import styles from "./udp.module.css";

type Tab = "send" | "listen";

export function UdpClient() {
  const [tab, setTab] = createSignal<Tab>("send");

  return (
    <div class={styles.container}>
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
        <>
          <SendForm />
          <ResultViewer />
        </>
      ) : (
        <>
          <ListenForm />
          <MessageLog />
        </>
      )}
    </div>
  );
}
