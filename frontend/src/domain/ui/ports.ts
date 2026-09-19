export type Theme = "light" | "dark";

export interface ThemeStorage {
  load(): Theme;
  save(theme: Theme): void;
}

export type NotificationLevel = "error" | "success" | "info" | "warning";

export interface Notification {
  id: string;
  level: NotificationLevel;
  title: string;
  description?: string;
  createdAt: number;
}

export interface NotifyOptions {
  /** 同一 key のアクティブ通知が既にあれば新規追加を抑制する（連続発火のスパム防止）。 */
  key?: string;
}

export type NotifyFn = (
  title: string,
  description?: string,
  opts?: NotifyOptions,
) => void;

/**
 * 通知の出力ポート。application 層はこのポート越しに通知し、
 * 実体（通知ストア）は composition root である Provider が注入する。
 */
export type Notifier = Record<NotificationLevel, NotifyFn>;
