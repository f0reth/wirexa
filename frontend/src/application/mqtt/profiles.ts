import { createSignal } from "solid-js";
import type { ProfileOrderStorage } from "../../domain/mqtt/ports";
import type { BrokerProfile } from "../../domain/mqtt/types";
import { moveItem } from "../../shared/array";
import { applyOrder } from "../shared/order";

/** 新規作成ダイアログの初期プロファイル。ID はサーバが採番するので空にする。 */
export function createEmptyProfile(): BrokerProfile {
  return {
    id: "",
    name: "",
    broker: "mqtt://localhost:1883",
    clientId: "",
    username: "",
    password: "",
    useTls: false,
  };
}

export interface ProfileApi {
  getProfiles(): Promise<BrokerProfile[]>;
  /** 保存済みプロファイルを返す。新規作成では ID がサーバ採番されている。 */
  saveProfile(profile: BrokerProfile): Promise<BrokerProfile>;
  deleteProfile(id: string): Promise<void>;
}

export function createProfilesState(
  api: ProfileApi,
  orderStorage: ProfileOrderStorage,
) {
  const [profiles, setProfiles] = createSignal<BrokerProfile[]>([]);

  async function loadProfiles(): Promise<void> {
    const loaded = await api.getProfiles();
    const order = orderStorage.load();
    setProfiles(applyOrder(loaded, order));
  }

  // 新規作成では ID がサーバ採番されるため、state と並び順には
  // 引数ではなく保存結果の ID を使う。
  async function saveProfile(profile: BrokerProfile): Promise<BrokerProfile> {
    const saved = await api.saveProfile(profile);
    setProfiles((prev) => {
      const idx = prev.findIndex((p) => p.id === saved.id);
      const next =
        idx >= 0
          ? prev.map((p) => (p.id === saved.id ? saved : p))
          : [...prev, saved];
      orderStorage.save(next.map((p) => p.id));
      return next;
    });
    return saved;
  }

  async function deleteProfile(id: string): Promise<void> {
    await api.deleteProfile(id);
    setProfiles((prev) => {
      const next = prev.filter((p) => p.id !== id);
      orderStorage.save(next.map((p) => p.id));
      return next;
    });
  }

  function reorderProfiles(fromIndex: number, toIndex: number): void {
    setProfiles((prev) => {
      const next = moveItem(prev, fromIndex, toIndex);
      if (!next) return prev;
      orderStorage.save(next.map((p) => p.id));
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
