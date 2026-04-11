export type Theme = "light" | "dark";

export interface ThemeStorage {
  load(): Theme;
  save(theme: Theme): void;
}
