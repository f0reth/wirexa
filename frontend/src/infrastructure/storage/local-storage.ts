import type {
  ConnectionPersistence,
  PresetStorage,
} from "../../domain/mqtt/ports";
import type { PublishPreset } from "../../domain/mqtt/types";
import type { Theme, ThemeStorage } from "../../domain/ui/ports";

const MQTT_LAST_PROFILE_KEY = "mqtt:lastActiveProfileId";
const MQTT_PRESETS_KEY = "mqtt:presets";
const THEME_KEY = "app:theme";

export function createLastProfileStorage(): ConnectionPersistence {
  return {
    loadLastProfileId: () =>
      loadFromStorage<string | null>(MQTT_LAST_PROFILE_KEY, null),
    saveLastProfileId: (id) => saveToStorage(MQTT_LAST_PROFILE_KEY, id),
    removeLastProfileId: () => removeFromStorage(MQTT_LAST_PROFILE_KEY),
  };
}

export function createPresetsStorage(): PresetStorage {
  return {
    load: () => loadFromStorage<PublishPreset[]>(MQTT_PRESETS_KEY, []),
    save: (presets) => saveToStorage(MQTT_PRESETS_KEY, presets),
  };
}

export function createThemeStorage(): ThemeStorage {
  return {
    load: () => loadFromStorage<Theme>(THEME_KEY, "light"),
    save: (t) => saveToStorage(THEME_KEY, t),
  };
}

export function loadFromStorage<T>(key: string, fallback: T): T {
  const item = localStorage.getItem(key);
  if (item === null) return fallback;
  try {
    return JSON.parse(item) as T;
  } catch (error) {
    console.warn(
      `[storage] corrupt data for key "${key}", using fallback:`,
      error,
    );
    return fallback;
  }
}

export function saveToStorage<T>(key: string, value: T): boolean {
  try {
    localStorage.setItem(key, JSON.stringify(value));
    return true;
  } catch (error) {
    console.warn(`[storage] saveToStorage failed for key "${key}":`, error);
    return false;
  }
}

export function removeFromStorage(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch (error) {
    console.warn(`[storage] removeFromStorage failed for key "${key}":`, error);
  }
}
