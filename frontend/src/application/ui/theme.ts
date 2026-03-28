import { createEffect, createSignal } from "solid-js";
import {
  loadFromStorage,
  saveToStorage,
} from "../../infrastructure/storage/local-storage";

const THEME_KEY = "app:theme";
type Theme = "light" | "dark";

export function createThemeStore() {
  const initial = loadFromStorage<Theme>(THEME_KEY, "light");
  const [theme, setTheme] = createSignal<Theme>(initial);

  createEffect(() => {
    const current = theme();
    saveToStorage(THEME_KEY, current);
    if (current === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  });

  return {
    theme,
    toggleTheme: () => setTheme((t) => (t === "light" ? "dark" : "light")),
  };
}
