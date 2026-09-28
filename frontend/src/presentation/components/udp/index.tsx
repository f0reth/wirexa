import { clsx } from "clsx";
import { createSignal, For, Show } from "solid-js";
import { tabId, tabPanelId } from "../../../components/ui/tabs";
import { useUdpSend } from "../../providers/udp-provider";
import { ListenForm } from "./listen-form";
import { MessageLog } from "./message-log";
import { SendForm } from "./send-form";
import styles from "./udp.module.css";

type Tab = "send" | "listen";

const TABS: { value: Tab; label: string }[] = [
  { value: "send", label: "Send" },
  { value: "listen", label: "Listen" },
];

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
        <div role="tablist" class={styles.tabBar}>
          <For each={TABS}>
            {(t) => (
              <button
                type="button"
                role="tab"
                aria-selected={tab() === t.value}
                aria-controls={tabPanelId("udp", t.value)}
                id={tabId("udp", t.value)}
                class={clsx(
                  styles.tabButton,
                  tab() === t.value && styles.tabButtonActive,
                )}
                onClick={() => setTab(t.value)}
              >
                {t.label}
              </button>
            )}
          </For>
        </div>
        <div
          role="tabpanel"
          id={tabPanelId("udp", tab())}
          aria-labelledby={tabId("udp", tab())}
          class={styles.tabPanel}
        >
          {tab() === "send" ? (
            <SendForm />
          ) : (
            <>
              <ListenForm />
              <MessageLog />
            </>
          )}
        </div>
      </Show>
    </div>
  );
}
