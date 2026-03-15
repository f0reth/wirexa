import { Show } from "solid-js";
import { BrokerManager } from "./broker-manager";
import { useMqtt } from "./context";
import styles from "./mqtt.module.css";
import { PublishTab } from "./publish-tab";
import { SubscribeTab } from "./subscribe-tab";
import { TabBar } from "./tab-bar";

export function MqttClient() {
  const { activeTab, activeConnectionId } = useMqtt();

  return (
    <div class={styles.container}>
      <BrokerManager />

      <Show
        when={activeConnectionId()}
        fallback={
          <div class={styles.emptyState}>
            <p class={styles.emptyStateText}>
              No active connection. Select a broker from the sidebar to connect.
            </p>
          </div>
        }
      >
        <TabBar />

        <div
          class={styles.mainContent}
          style={{ display: activeTab() === "subscribe" ? "flex" : "none" }}
        >
          <SubscribeTab />
        </div>

        <div
          class={styles.mainContent}
          style={{ display: activeTab() === "publish" ? "flex" : "none" }}
        >
          <PublishTab />
        </div>
      </Show>
    </div>
  );
}
