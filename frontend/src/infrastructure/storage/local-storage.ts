import type { ExpandedFoldersStorage } from "../../domain/http/ports";
import type {
  ConnectionPersistence,
  PresetStorage,
  ProfileOrderStorage,
} from "../../domain/mqtt/ports";
import { isQos, type PublishPreset } from "../../domain/mqtt/types";
import type { TargetOrderStorage } from "../../domain/udp/ports";
import type { Theme, ThemeStorage } from "../../domain/ui/ports";

const MQTT_LAST_PROFILE_KEY = "mqtt:lastActiveProfileId";
const MQTT_PRESETS_KEY = "mqtt:presets";
const MQTT_PROFILE_ORDER_KEY = "mqtt:profileOrder";
const THEME_KEY = "app:theme";
const HTTP_ACTIVE_REQUEST_KEY = "wirexa:http:activeRequest";
const HTTP_EXPANDED_FOLDERS_KEY = "wirexa:http:expandedFolders";
const UDP_TARGET_ORDER_KEY = "udp:targetOrder";

// 保存値の形の検査。JSON としては正しいが形が違う値（手で書き換えた値など）を読んでも
// 呼び出し側（.map や applyOrder）で落ちないよう、読み込み時に既定値へ戻す。
type Guard<T> = (v: unknown) => v is T;

const isString = (v: unknown): v is string => typeof v === "string";

const isStringOrNull = (v: unknown): v is string | null =>
  v === null || isString(v);

const isStringArray = (v: unknown): v is string[] =>
  Array.isArray(v) && v.every(isString);

const isRecord = (v: unknown): v is Record<string, unknown> =>
  typeof v === "object" && v !== null && !Array.isArray(v);

const isTheme = (v: unknown): v is Theme => v === "light" || v === "dark";

const isBooleanRecord = (v: unknown): v is Record<string, boolean> =>
  isRecord(v) && Object.values(v).every((x) => typeof x === "boolean");

export function createLastProfileStorage(): ConnectionPersistence {
  return {
    loadLastProfileId: () =>
      loadFromStorage(MQTT_LAST_PROFILE_KEY, null, isStringOrNull),
    saveLastProfileId: (id) => saveToStorage(MQTT_LAST_PROFILE_KEY, id),
    removeLastProfileId: () => removeFromStorage(MQTT_LAST_PROFILE_KEY),
  };
}

// retain を導入する前に保存されたプリセットには retain フィールドが無い。
type StoredPreset = Omit<PublishPreset, "retain"> & { retain?: boolean };

function isStoredPreset(v: unknown): v is StoredPreset {
  return (
    isRecord(v) &&
    isString(v.id) &&
    isString(v.name) &&
    isString(v.topic) &&
    isString(v.payload) &&
    isQos(v.qos) &&
    (v.retain === undefined || typeof v.retain === "boolean")
  );
}

export function createPresetsStorage(): PresetStorage {
  return {
    // 形の違う要素だけを捨て、読めるプリセットは残す。
    load: () =>
      loadFromStorage(MQTT_PRESETS_KEY, [], Array.isArray)
        .filter(isStoredPreset)
        .map((p) => ({
          ...p,
          retain: p.retain ?? false,
        })),
    save: (presets) => saveToStorage(MQTT_PRESETS_KEY, presets),
  };
}

export function createThemeStorage(): ThemeStorage {
  return {
    load: () => loadFromStorage(THEME_KEY, "light", isTheme),
    save: (t) => saveToStorage(THEME_KEY, t),
  };
}

export interface ActiveRequestEntry {
  requestId: string;
  collectionId: string;
}

const isActiveRequestEntry = (v: unknown): v is ActiveRequestEntry | null =>
  v === null ||
  (isRecord(v) && isString(v.requestId) && isString(v.collectionId));

export function createActiveRequestStorage() {
  return {
    load: () =>
      loadFromStorage(HTTP_ACTIVE_REQUEST_KEY, null, isActiveRequestEntry),
    save: (requestId: string, collectionId: string) =>
      saveToStorage(HTTP_ACTIVE_REQUEST_KEY, { requestId, collectionId }),
    clear: () => removeFromStorage(HTTP_ACTIVE_REQUEST_KEY),
  };
}

export function createExpandedFoldersStorage(): ExpandedFoldersStorage {
  return {
    load: () => loadFromStorage(HTTP_EXPANDED_FOLDERS_KEY, {}, isBooleanRecord),
    save: (ids) => saveToStorage(HTTP_EXPANDED_FOLDERS_KEY, ids),
  };
}

export function createProfileOrderStorage(): ProfileOrderStorage {
  return {
    load: () => loadFromStorage(MQTT_PROFILE_ORDER_KEY, [], isStringArray),
    save: (ids) => saveToStorage(MQTT_PROFILE_ORDER_KEY, ids),
  };
}

export function createTargetOrderStorage(): TargetOrderStorage {
  return {
    load: () => loadFromStorage(UDP_TARGET_ORDER_KEY, [], isStringArray),
    save: (ids) => saveToStorage(UDP_TARGET_ORDER_KEY, ids),
  };
}

/**
 * key の値を JSON として読む。無い・読めない・壊れている・isValid に合わない場合は fallback。
 * isValid を省くと形は検査しない（呼び出し側が任意の値を受け付けられる場合だけ）。
 */
export function loadFromStorage<T>(
  key: string,
  fallback: T,
  isValid?: Guard<T>,
): T {
  let item: string | null;
  try {
    // ストレージへのアクセスが拒否された環境では getItem 自体が投げる。
    item = localStorage.getItem(key);
  } catch (error) {
    console.warn(`[storage] loadFromStorage failed for key "${key}":`, error);
    return fallback;
  }
  if (item === null) return fallback;
  let value: unknown;
  try {
    value = JSON.parse(item);
  } catch (error) {
    console.warn(
      `[storage] corrupt data for key "${key}", using fallback:`,
      error,
    );
    return fallback;
  }
  if (isValid && !isValid(value)) {
    console.warn(`[storage] unexpected shape for key "${key}", using fallback`);
    return fallback;
  }
  return value as T;
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
