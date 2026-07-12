import { type Accessor, createSignal } from "solid-js";
import type { Notification, NotificationLevel } from "../../domain/ui/ports";
import { generateId } from "../../infrastructure/id/generator";

export type { Notification, NotificationLevel };

export interface NotifyOptions {
  /** 同一 key のアクティブ通知が既にあれば新規追加を抑制する（連続発火のスパム防止）。 */
  key?: string;
}

interface PushInput {
  level: NotificationLevel;
  title: string;
  description?: string;
  key?: string;
}

export interface NotificationStoreOptions {
  /** error 以外の自動消滅までの時間 (ms)。0 以下で自動消滅しない。 */
  autoDismissMs?: number;
  /** error の自動消滅までの時間 (ms)。0 以下で自動消滅しない。 */
  errorDismissMs?: number;
  /** 同時に保持する最大件数。超過時は最古を落とす。 */
  max?: number;
}

export type NotifyFn = (
  title: string,
  description?: string,
  opts?: NotifyOptions,
) => string;

export interface NotificationStore {
  notifications: Accessor<Notification[]>;
  notify: Record<NotificationLevel, NotifyFn>;
  dismiss(id: string): void;
  clear(): void;
}

export function createNotificationStore(
  opts: NotificationStoreOptions = {},
): NotificationStore {
  const autoDismissMs = opts.autoDismissMs ?? 4000;
  const errorDismissMs = opts.errorDismissMs ?? 8000;
  const max = opts.max ?? 5;

  const [notifications, setNotifications] = createSignal<Notification[]>([]);
  const timers = new Map<string, ReturnType<typeof setTimeout>>();
  // key → notification id。アクティブな通知だけを追跡する。
  const activeKeys = new Map<string, string>();

  function dismiss(id: string): void {
    const timer = timers.get(id);
    if (timer !== undefined) {
      clearTimeout(timer);
      timers.delete(id);
    }
    for (const [key, mappedId] of activeKeys) {
      if (mappedId === id) activeKeys.delete(key);
    }
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  }

  function clear(): void {
    for (const timer of timers.values()) clearTimeout(timer);
    timers.clear();
    activeKeys.clear();
    setNotifications([]);
  }

  function push(input: PushInput): string {
    // 重複抑制: 同一 key のアクティブ通知があれば既存を維持し、追加しない。
    if (input.key !== undefined) {
      const existing = activeKeys.get(input.key);
      if (existing !== undefined) return existing;
    }

    const id = generateId();
    const notification: Notification = {
      id,
      level: input.level,
      title: input.title,
      description: input.description,
      createdAt: Date.now(),
    };

    if (input.key !== undefined) activeKeys.set(input.key, id);

    setNotifications((prev) => {
      const next = [...prev, notification];
      // 上限超過分は最古から落とす（タイマー/keyも解放）。
      while (next.length > max) {
        const dropped = next.shift();
        if (dropped) {
          const timer = timers.get(dropped.id);
          if (timer !== undefined) {
            clearTimeout(timer);
            timers.delete(dropped.id);
          }
          for (const [key, mappedId] of activeKeys) {
            if (mappedId === dropped.id) activeKeys.delete(key);
          }
        }
      }
      return next;
    });

    const dismissMs = input.level === "error" ? errorDismissMs : autoDismissMs;
    if (dismissMs > 0) {
      timers.set(
        id,
        setTimeout(() => dismiss(id), dismissMs),
      );
    }

    return id;
  }

  const makeNotifier =
    (level: NotificationLevel): NotifyFn =>
    (title, description, o) =>
      push({ level, title, description, key: o?.key });

  const notify: Record<NotificationLevel, NotifyFn> = {
    error: makeNotifier("error"),
    success: makeNotifier("success"),
    info: makeNotifier("info"),
    warning: makeNotifier("warning"),
  };

  return { notifications, notify, dismiss, clear };
}

/** アプリ全体で共有するシングルトン通知ストア。 */
export const notificationStore = createNotificationStore();
export const notify = notificationStore.notify;
