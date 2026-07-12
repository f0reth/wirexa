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
