import { clsx } from "clsx";
import { type JSX, Show } from "solid-js";
import styles from "./tabs.module.css";

export interface TabItem<T extends string = string> {
  value: T;
  label: string;
}

/**
 * タブの id。同じ画面に同名のタブ (HTTP のリクエスト側とレスポンス側の Body / Headers など)
 * が並ぶので、タブ列ごとの idPrefix で区別する。
 */
export function tabId(idPrefix: string, value: string): string {
  return `${idPrefix}-tab-${value}`;
}

/** タブに対応するパネルの id。 */
export function tabPanelId(idPrefix: string, value: string): string {
  return `${idPrefix}-tabpanel-${value}`;
}

export function TabList<T extends string>(props: {
  tabs: TabItem<T>[];
  activeTab: T;
  onTabChange: (value: T) => void;
  /** 画面内で一意なタブ列の名前。対応する TabPanel にも同じ値を渡す。 */
  idPrefix: string;
  class?: string;
}) {
  return (
    <div role="tablist" class={clsx(styles.tabList, props.class)}>
      {props.tabs.map((tab) => (
        <button
          type="button"
          role="tab"
          aria-selected={props.activeTab === tab.value}
          aria-controls={tabPanelId(props.idPrefix, tab.value)}
          id={tabId(props.idPrefix, tab.value)}
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
  /** 対応する TabList の idPrefix。 */
  idPrefix: string;
  children: JSX.Element;
  class?: string;
}) {
  return (
    <Show when={props.active === props.value}>
      <div
        role="tabpanel"
        id={tabPanelId(props.idPrefix, props.value)}
        aria-labelledby={tabId(props.idPrefix, props.value)}
        class={props.class}
      >
        {props.children}
      </div>
    </Show>
  );
}
