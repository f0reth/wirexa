import { Show } from "solid-js";
import { BrokerTree } from "./broker-tree";
import { CollectionTree } from "./collection-tree";
import type { Protocol } from "./protocol-switcher";
import styles from "./sidebar.module.css";
import { TargetTree } from "./target-tree";

interface SidebarProps {
  protocol: Protocol;
}

export function Sidebar(props: SidebarProps) {
  return (
    <div class={styles.sidebar}>
      <Show when={props.protocol === "http"}>
        <CollectionTree />
      </Show>
      <Show when={props.protocol === "mqtt"}>
        <BrokerTree />
      </Show>
      <Show when={props.protocol === "udp"}>
        <TargetTree />
      </Show>
    </div>
  );
}
