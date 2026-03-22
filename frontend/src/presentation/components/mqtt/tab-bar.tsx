import type { TabItem } from "../../../components/ui/tabs";
import { TabList } from "../../../components/ui/tabs";
import type { Tab } from "../../../domain/mqtt/types";
import { useMqttConnection } from "../../providers/mqtt-provider";
import styles from "./mqtt.module.css";

const TABS: TabItem<Tab>[] = [
  { value: "subscribe", label: "Subscribe" },
  { value: "publish", label: "Publish" },
];

export function TabBar() {
  const { activeTab, setActiveTab } = useMqttConnection();

  return (
    <TabList
      tabs={TABS}
      activeTab={activeTab()}
      onTabChange={setActiveTab}
      class={styles.tabBar}
    />
  );
}
