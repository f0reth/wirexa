import { Show } from "solid-js";
import { tabId, tabPanelId } from "../../../components/ui/tabs";
import { useMqttConnection } from "../../providers/mqtt-provider";
import { BrokerManager } from "./broker-manager";
import styles from "./mqtt.module.css";
import { PublishTab } from "./publish-tab";
import { SubscribeTab } from "./subscribe-tab";
import { TabBar } from "./tab-bar";

export function MqttClient() {
  const { activeTab, activeConnectionId } = useMqttConnection();

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
          role="tabpanel"
          id={tabPanelId("mqtt", "subscribe")}
          aria-labelledby={tabId("mqtt", "subscribe")}
          class={styles.mainContent}
          style={{ display: activeTab() === "subscribe" ? "flex" : "none" }}
        >
          <SubscribeTab />
        </div>

        <div
          role="tabpanel"
          id={tabPanelId("mqtt", "publish")}
          aria-labelledby={tabId("mqtt", "publish")}
          class={styles.mainContent}
          style={{ display: activeTab() === "publish" ? "flex" : "none" }}
        >
          <PublishTab />
        </div>
      </Show>
    </div>
  );
}
