import { createEffect, createSignal } from "solid-js";
import type { BrokerProfile } from "../types";

export const STORAGE_KEYS = {
  profiles: "wirexa:profiles",
  presets: "wirexa:presets",
  lastActiveProfileId: "wirexa:lastActiveProfileId",
} as const;

export function loadFromStorage<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    return raw ? JSON.parse(raw) : fallback;
  } catch {
    return fallback;
  }
}

export function saveToStorage(key: string, value: unknown): void {
  localStorage.setItem(key, JSON.stringify(value));
}

function migrateIfNeeded(profiles: BrokerProfile[]): BrokerProfile[] {
  if (profiles.length > 0) return profiles;
  const legacyUrl = localStorage.getItem("wirexa:brokerUrl");
  if (legacyUrl) {
    let broker: string;
    try {
      broker = JSON.parse(legacyUrl);
    } catch {
      broker = legacyUrl;
    }
    const migrated: BrokerProfile = {
      id: Date.now().toString(),
      name: "Default",
      broker,
      clientId: "",
      username: "",
      password: "",
      useTls: false,
    };
    localStorage.removeItem("wirexa:brokerUrl");
    localStorage.removeItem("wirexa:subscriptions");
    return [migrated];
  }
  return profiles;
}

export function createProfilesState() {
  const [profiles, setProfiles] = createSignal<BrokerProfile[]>(
    migrateIfNeeded(loadFromStorage(STORAGE_KEYS.profiles, [])),
  );

  createEffect(() => {
    saveToStorage(STORAGE_KEYS.profiles, profiles());
  });

  const saveProfile = (profile: BrokerProfile) => {
    setProfiles((prev) => {
      const idx = prev.findIndex((p) => p.id === profile.id);
      if (idx >= 0) {
        const next = [...prev];
        next[idx] = profile;
        return next;
      }
      return [...prev, profile];
    });
  };

  const deleteProfile = (id: string) => {
    setProfiles((prev) => prev.filter((p) => p.id !== id));
  };

  return { profiles, saveProfile, deleteProfile } as const;
}
