import { clsx } from "clsx";
import { type JSX, Show } from "solid-js";
import styles from "./tabs.module.css";

export interface TabItem<T extends string = string> {
  value: T;
  label: string;
}

export function TabList<T extends string>(props: {
  tabs: TabItem<T>[];
  activeTab: T;
  onTabChange: (value: T) => void;
  class?: string;
}) {
  return (
    <div role="tablist" class={clsx(styles.tabList, props.class)}>
      {props.tabs.map((tab) => (
        <button
          type="button"
          role="tab"
          aria-selected={props.activeTab === tab.value}
          aria-controls={`tabpanel-${tab.value}`}
          id={`tab-${tab.value}`}
          class={clsx(
            styles.tab,
            props.activeTab === tab.value && styles.tabActive,
          )}
          onClick={() => props.onTabChange(tab.value)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

export function TabPanel<T extends string>(props: {
  value: T;
  active: T;
  children: JSX.Element;
  class?: string;
}) {
  return (
    <Show when={props.active === props.value}>
      <div
        role="tabpanel"
        id={`tabpanel-${props.value}`}
        aria-labelledby={`tab-${props.value}`}
        class={props.class}
      >
        {props.children}
      </div>
    </Show>
  );
}
