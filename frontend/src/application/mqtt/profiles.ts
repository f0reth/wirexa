import { createSignal } from "solid-js";
import type { ProfileOrderStorage } from "../../domain/mqtt/ports";
import type { BrokerProfile } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import { moveItem } from "../../shared/array";
import { errorMessage } from "../../shared/error";
import { applyOrder } from "../shared/order";
import { notifyOnError } from "../ui/guard";

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
  notifier: Notifier,
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
  // 失敗時は通知したうえで例外を伝える（呼び出し側がダイアログを閉じない）。
  async function saveProfile(profile: BrokerProfile): Promise<BrokerProfile> {
    let saved: BrokerProfile;
    try {
      saved = await api.saveProfile(profile);
    } catch (err) {
      // 接続バーの入力のたびに保存が走るので、同じプロファイルの失敗が続いても
      // 通知が 1 つにとどまるよう key を付ける（notifyOnError は key を渡せない）。
      notifier.error("Failed to save broker", errorMessage(err), {
        key: `mqtt:save-profile:${profile.id}`,
      });
      throw err;
    }
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

  // 失敗時は通知したうえで例外を伝える（呼び出し側は成功したときだけタブを閉じる）。
  async function deleteProfile(id: string): Promise<void> {
    await notifyOnError(notifier, "Failed to delete broker", () =>
      api.deleteProfile(id),
    );
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
