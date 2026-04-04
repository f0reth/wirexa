import { createEffect, createSignal } from "solid-js";

type Theme = "light" | "dark";

export type { Theme };

export interface ThemeStorage {
  load(): Theme;
  save(theme: Theme): void;
}

export function createThemeStore(storage: ThemeStorage) {
  const [theme, setTheme] = createSignal<Theme>(storage.load());

  createEffect(() => {
    storage.save(theme());
  });

  return {
    theme,
    toggleTheme: () => setTheme((t) => (t === "light" ? "dark" : "light")),
  };
}
