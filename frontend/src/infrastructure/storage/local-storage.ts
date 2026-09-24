import type { ExpandedFoldersStorage } from "../../domain/http/ports";
import type {
  ConnectionPersistence,
  PresetStorage,
  ProfileOrderStorage,
} from "../../domain/mqtt/ports";
import type { PublishPreset } from "../../domain/mqtt/types";
import type { TargetOrderStorage } from "../../domain/udp/ports";
import type { Theme, ThemeStorage } from "../../domain/ui/ports";

const MQTT_LAST_PROFILE_KEY = "mqtt:lastActiveProfileId";
const MQTT_PRESETS_KEY = "mqtt:presets";
const MQTT_PROFILE_ORDER_KEY = "mqtt:profileOrder";
const THEME_KEY = "app:theme";
const HTTP_ACTIVE_REQUEST_KEY = "wirexa:http:activeRequest";
const HTTP_EXPANDED_FOLDERS_KEY = "wirexa:http:expandedFolders";
const UDP_TARGET_ORDER_KEY = "udp:targetOrder";

export function createLastProfileStorage(): ConnectionPersistence {
  return {
    loadLastProfileId: () =>
      loadFromStorage<string | null>(MQTT_LAST_PROFILE_KEY, null),
    saveLastProfileId: (id) => saveToStorage(MQTT_LAST_PROFILE_KEY, id),
    removeLastProfileId: () => removeFromStorage(MQTT_LAST_PROFILE_KEY),
  };
}

// retain を導入する前に保存されたプリセットには retain フィールドが無い。
type StoredPreset = Omit<PublishPreset, "retain"> & { retain?: boolean };

export function createPresetsStorage(): PresetStorage {
  return {
    load: () =>
      loadFromStorage<StoredPreset[]>(MQTT_PRESETS_KEY, []).map((p) => ({
        ...p,
        retain: p.retain ?? false,
      })),
    save: (presets) => saveToStorage(MQTT_PRESETS_KEY, presets),
  };
}

export function createThemeStorage(): ThemeStorage {
  return {
    load: () => loadFromStorage<Theme>(THEME_KEY, "light"),
    save: (t) => saveToStorage(THEME_KEY, t),
  };
}

export interface ActiveRequestEntry {
  requestId: string;
  collectionId: string;
}

export function createActiveRequestStorage() {
  return {
    load: () =>
      loadFromStorage<ActiveRequestEntry | null>(HTTP_ACTIVE_REQUEST_KEY, null),
    save: (requestId: string, collectionId: string) =>
      saveToStorage(HTTP_ACTIVE_REQUEST_KEY, { requestId, collectionId }),
    clear: () => removeFromStorage(HTTP_ACTIVE_REQUEST_KEY),
  };
}

export function createExpandedFoldersStorage(): ExpandedFoldersStorage {
  return {
    load: () =>
      loadFromStorage<Record<string, boolean>>(HTTP_EXPANDED_FOLDERS_KEY, {}),
    save: (ids) => saveToStorage(HTTP_EXPANDED_FOLDERS_KEY, ids),
  };
}

export function createProfileOrderStorage(): ProfileOrderStorage {
  return {
    load: () => loadFromStorage<string[]>(MQTT_PROFILE_ORDER_KEY, []),
    save: (ids) => saveToStorage(MQTT_PROFILE_ORDER_KEY, ids),
  };
}

export function createTargetOrderStorage(): TargetOrderStorage {
  return {
    load: () => loadFromStorage<string[]>(UDP_TARGET_ORDER_KEY, []),
    save: (ids) => saveToStorage(UDP_TARGET_ORDER_KEY, ids),
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
