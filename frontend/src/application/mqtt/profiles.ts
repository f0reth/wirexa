import { createSignal } from "solid-js";
import type { BrokerProfile } from "../../domain/mqtt/types";
import {
  deleteProfile as deleteProfileRPC,
  getProfiles,
  saveProfile as saveProfileRPC,
} from "../../infrastructure/mqtt/client";

export function createProfilesState() {
  const [profiles, setProfiles] = createSignal<BrokerProfile[]>([]);

  async function loadProfiles(): Promise<void> {
    const loaded = await getProfiles();
    setProfiles(loaded);
  }

  async function saveProfile(profile: BrokerProfile): Promise<void> {
    await saveProfileRPC(profile);
    setProfiles((prev) => {
      const idx = prev.findIndex((p) => p.id === profile.id);
      return idx >= 0
        ? prev.map((p) => (p.id === profile.id ? profile : p))
        : [...prev, profile];
    });
  }

  async function deleteProfile(id: string): Promise<void> {
    await deleteProfileRPC(id);
    setProfiles((prev) => prev.filter((p) => p.id !== id));
  }

  return { profiles, loadProfiles, saveProfile, deleteProfile };
}
