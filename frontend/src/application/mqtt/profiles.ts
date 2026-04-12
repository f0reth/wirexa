import { createSignal } from "solid-js";
import type { BrokerProfile } from "../../domain/mqtt/types";
import {
  loadFromStorage,
  saveToStorage,
} from "../../infrastructure/storage/local-storage";

const PROFILE_ORDER_KEY = "mqtt:profileOrder";

export interface ProfileApi {
  getProfiles(): Promise<BrokerProfile[]>;
  saveProfile(profile: BrokerProfile): Promise<void>;
  deleteProfile(id: string): Promise<void>;
}

function applyOrder(
  profiles: BrokerProfile[],
  order: string[],
): BrokerProfile[] {
  const orderMap = new Map(order.map((id, i) => [id, i]));
  return [...profiles].sort((a, b) => {
    const ai = orderMap.get(a.id) ?? Number.POSITIVE_INFINITY;
    const bi = orderMap.get(b.id) ?? Number.POSITIVE_INFINITY;
    return ai - bi;
  });
}

export function createProfilesState(api: ProfileApi) {
  const [profiles, setProfiles] = createSignal<BrokerProfile[]>([]);

  async function loadProfiles(): Promise<void> {
    const loaded = await api.getProfiles();
    const order = loadFromStorage<string[]>(PROFILE_ORDER_KEY, []);
    setProfiles(applyOrder(loaded, order));
  }

  async function saveProfile(profile: BrokerProfile): Promise<void> {
    await api.saveProfile(profile);
    setProfiles((prev) => {
      const idx = prev.findIndex((p) => p.id === profile.id);
      const next =
        idx >= 0
          ? prev.map((p) => (p.id === profile.id ? profile : p))
          : [...prev, profile];
      saveToStorage(
        PROFILE_ORDER_KEY,
        next.map((p) => p.id),
      );
      return next;
    });
  }

  async function deleteProfile(id: string): Promise<void> {
    await api.deleteProfile(id);
    setProfiles((prev) => {
      const next = prev.filter((p) => p.id !== id);
      saveToStorage(
        PROFILE_ORDER_KEY,
        next.map((p) => p.id),
      );
      return next;
    });
  }

  function reorderProfiles(fromIndex: number, toIndex: number): void {
    setProfiles((prev) => {
      if (
        fromIndex < 0 ||
        fromIndex >= prev.length ||
        toIndex < 0 ||
        toIndex >= prev.length
      )
        return prev;
      const next = [...prev];
      const [item] = next.splice(fromIndex, 1);
      next.splice(toIndex, 0, item);
      saveToStorage(
        PROFILE_ORDER_KEY,
        next.map((p) => p.id),
      );
      return next;
    });
  }

  return {
    profiles,
    loadProfiles,
    saveProfile,
    deleteProfile,
    reorderProfiles,
  };
}
