import { clsx } from "clsx";
import { CircleAlert, CircleCheck, Info, TriangleAlert, X } from "lucide-solid";
import { type Component, For, Show } from "solid-js";
import {
  type NotificationLevel,
  notificationStore,
} from "../../application/ui/notifications";
import styles from "./toast.module.css";

const levelStyle: Record<NotificationLevel, string> = {
  error: styles.error,
  success: styles.success,
  info: styles.info,
  warning: styles.warning,
};

const levelIcon: Record<NotificationLevel, Component<{ size: number }>> = {
  error: CircleAlert,
  success: CircleCheck,
  info: Info,
  warning: TriangleAlert,
};

export function ToastViewport() {
  return (
    <section class={styles.viewport} aria-label="Notifications">
      <For each={notificationStore.notifications()}>
        {(n) => {
          const Icon = levelIcon[n.level];
          return (
            <div
              class={clsx(styles.toast, levelStyle[n.level])}
              role="alert"
              aria-live={n.level === "error" ? "assertive" : "polite"}
            >
              <span class={styles.icon}>
                <Icon size={16} />
              </span>
              <div class={styles.content}>
                <p class={styles.title}>{n.title}</p>
                <Show when={n.description}>
                  <p class={styles.description}>{n.description}</p>
                </Show>
              </div>
              <button
                type="button"
                class={styles.close}
                aria-label="Dismiss"
                onClick={() => notificationStore.dismiss(n.id)}
              >
                <X size={14} />
              </button>
            </div>
          );
        }}
      </For>
    </section>
  );
}
