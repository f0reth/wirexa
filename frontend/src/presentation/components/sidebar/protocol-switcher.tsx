import { clsx } from "clsx";
import { Globe, Moon, Radio, Sun } from "lucide-solid";
import type { Component } from "solid-js";
import styles from "./sidebar.module.css";

export type Protocol = "mqtt" | "http";

const PROTOCOLS: {
  value: Protocol;
  label: string;
  icon: Component<{ size: number }>;
}[] = [
  { value: "mqtt", label: "MQTT", icon: Radio },
  { value: "http", label: "HTTP", icon: Globe },
];

interface ProtocolSwitcherProps {
  protocol: Protocol;
  onProtocolChange: (p: Protocol) => void;
  theme: "light" | "dark";
  onThemeToggle: () => void;
}

export function ProtocolSwitcher(props: ProtocolSwitcherProps) {
  return (
    <div class={styles.activityBar}>
      {PROTOCOLS.map((p) => (
        <button
          type="button"
          class={clsx(
            styles.activityIcon,
            props.protocol === p.value && styles.activityIconActive,
          )}
          onClick={() => props.onProtocolChange(p.value)}
          title={p.label}
        >
          <p.icon size={22} />
        </button>
      ))}
      <div style={{ flex: "1" }} />
      <button
        type="button"
        class={styles.activityIcon}
        onClick={props.onThemeToggle}
        title={
          props.theme === "light"
            ? "Switch to dark mode"
            : "Switch to light mode"
        }
      >
        {props.theme === "light" ? <Moon size={22} /> : <Sun size={22} />}
      </button>
    </div>
  );
}
