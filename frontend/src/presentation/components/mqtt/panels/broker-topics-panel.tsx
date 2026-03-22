import { clsx } from "clsx";
import { Plus, ScanSearch, Square } from "lucide-solid";
import { createMemo, For, Show } from "solid-js";
import { ScrollArea } from "../../../../components/ui/scroll-area";
import { useMqttSubscribe } from "../../../providers/mqtt-provider";
import styles from "../mqtt.module.css";
import { getTopicColor } from "../utils";

function BrokerTopicItem(props: {
  topic: string;
  isSubscribed: () => boolean;
  onSubscribe: () => void;
}) {
  const color = createMemo(() => getTopicColor(props.topic));

  return (
    <div class={styles.brokerTopicItem}>
      <span class={styles.brokerTopicName} style={{ color: color() }}>
        {props.topic}
      </span>
      <button
        type="button"
        class={clsx(
          styles.brokerTopicAddButton,
          props.isSubscribed() && styles.brokerTopicAddButtonDone,
        )}
        onClick={props.onSubscribe}
        disabled={props.isSubscribed()}
        title={props.isSubscribed() ? "Already subscribed" : "Subscribe"}
      >
        <Plus size={12} />
      </button>
    </div>
  );
}

export function BrokerTopicsPanel() {
  const {
    brokerTopics,
    isScanning,
    setIsScanning,
    subscriptions,
    addSubscription,
  } = useMqttSubscribe();

  const subscribedTopicSet = createMemo(
    () => new Set(subscriptions().map((s) => s.topic)),
  );

  return (
    <div class={styles.brokerTopicsPanel}>
      <div class={styles.sectionHeader}>
        <h3 class={styles.sectionTitle}>Broker Topics</h3>
        <button
          type="button"
          class={clsx(
            styles.headerAction,
            isScanning() && styles.headerActionActive,
          )}
          onClick={() => setIsScanning((v) => !v)}
          title={isScanning() ? "Stop scanning" : "Start scanning"}
        >
          <Show when={isScanning()} fallback={<ScanSearch size={14} />}>
            <Square size={14} />
          </Show>
          {isScanning() ? "Stop" : "Scan"}
        </button>
      </div>

      <ScrollArea class={styles.subscriptionScrollArea}>
        <div class={styles.listPadding}>
          <Show
            when={brokerTopics().length > 0}
            fallback={
              <p class={styles.emptyText}>
                {isScanning() ? "Scanning..." : "No topics found"}
              </p>
            }
          >
            <div class={styles.itemList}>
              <For each={brokerTopics()}>
                {(topic) => (
                  <BrokerTopicItem
                    topic={topic}
                    isSubscribed={() => subscribedTopicSet().has(topic)}
                    onSubscribe={() => addSubscription(topic, 0)}
                  />
                )}
              </For>
            </div>
          </Show>
        </div>
      </ScrollArea>
    </div>
  );
}
