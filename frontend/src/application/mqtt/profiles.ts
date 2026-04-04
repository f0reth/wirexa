import { createSignal } from "solid-js";
import type { BrokerProfile } from "../../domain/mqtt/types";

export interface ProfileApi {
  getProfiles(): Promise<BrokerProfile[]>;
  saveProfile(profile: BrokerProfile): Promise<void>;
  deleteProfile(id: string): Promise<void>;
}

export function createProfilesState(api: ProfileApi) {
  const [profiles, setProfiles] = createSignal<BrokerProfile[]>([]);

  async function loadProfiles(): Promise<void> {
    const loaded = await api.getProfiles();
    setProfiles(loaded);
  }

  async function saveProfile(profile: BrokerProfile): Promise<void> {
    await api.saveProfile(profile);
    setProfiles((prev) => {
      const idx = prev.findIndex((p) => p.id === profile.id);
      return idx >= 0
        ? prev.map((p) => (p.id === profile.id ? profile : p))
        : [...prev, profile];
    });
  }

  async function deleteProfile(id: string): Promise<void> {
    await api.deleteProfile(id);
    setProfiles((prev) => prev.filter((p) => p.id !== id));
  }

  return { profiles, loadProfiles, saveProfile, deleteProfile };
}
