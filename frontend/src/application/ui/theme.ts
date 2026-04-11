import { createEffect, createSignal } from "solid-js";
import type { Theme, ThemeStorage } from "../../domain/ui/ports";

export type { Theme, ThemeStorage };

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
