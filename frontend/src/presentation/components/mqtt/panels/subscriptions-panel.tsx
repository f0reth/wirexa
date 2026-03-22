import { Plus, Trash2 } from "lucide-solid";
import { For, Show } from "solid-js";
import { Badge } from "../../../../components/ui/badge";
import { Button } from "../../../../components/ui/button";
import { Input } from "../../../../components/ui/input";
import { ScrollArea } from "../../../../components/ui/scroll-area";
import {
  useMqttConnection,
  useMqttSubscribe,
} from "../../../providers/mqtt-provider";
import styles from "../mqtt.module.css";
import { QosSelect } from "../qos-select";

export function SubscriptionsPanel() {
  const {
    subscriptions,
    newTopic,
    setNewTopic,
    newQos,
    setNewQos,
    addSubscription,
    removeSubscription,
  } = useMqttSubscribe();
  const { activeConnection } = useMqttConnection();
  const isConnected = () => {
    const conn = activeConnection();
    return conn?.type === "online" && conn.connected;
  };

  return (
    <div class={styles.subscriptionsPanel}>
      <div class={styles.sectionHeader}>
        <h3 class={styles.sectionTitle}>Subscriptions</h3>
      </div>

      <div class={styles.addSubscriptionSection}>
        <div class={styles.addSubscriptionInputs}>
          <Input
            value={newTopic()}
            onInput={(e) => setNewTopic(e.currentTarget.value)}
            onKeyDown={(e) => e.key === "Enter" && addSubscription()}
            placeholder="Topic (e.g., sensors/#)"
            class={styles.monoInput}
          />
          <div class={styles.addSubscriptionRow}>
            <QosSelect value={newQos()} onChange={setNewQos} />
            <Button
              size="sm"
              onClick={() => addSubscription()}
              class={styles.subscribeButton}
              disabled={!isConnected()}
            >
              <Plus size={16} />
              Subscribe
            </Button>
          </div>
        </div>
      </div>

      <ScrollArea class={styles.subscriptionScrollArea}>
        <div class={styles.listPadding}>
          <Show
            when={subscriptions().length > 0}
            fallback={<p class={styles.emptyText}>No subscriptions</p>}
          >
            <div class={styles.itemList}>
              <For each={subscriptions()}>
                {(sub) => (
                  <div class={styles.subscriptionItem}>
                    <div class={styles.subscriptionInfo}>
                      <span class={styles.subscriptionTopic}>{sub.topic}</span>
                      <Badge variant="secondary">QoS {sub.qos}</Badge>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => removeSubscription(sub.id)}
                      class={styles.deleteButton}
                    >
                      <Trash2 size={16} />
                    </Button>
                  </div>
                )}
              </For>
            </div>
          </Show>
        </div>
      </ScrollArea>
    </div>
  );
}
