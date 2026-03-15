import type { TabItem } from "../ui/tabs";
import { TabList } from "../ui/tabs";
import { useMqtt } from "./context";
import styles from "./mqtt.module.css";
import type { Tab } from "./types";

const TABS: TabItem<Tab>[] = [
  { value: "subscribe", label: "Subscribe" },
  { value: "publish", label: "Publish" },
];

export function TabBar() {
  const { activeTab, setActiveTab } = useMqtt();

  return (
    <TabList
      tabs={TABS}
      activeTab={activeTab()}
      onTabChange={setActiveTab}
      class={styles.tabBar}
    />
  );
}
