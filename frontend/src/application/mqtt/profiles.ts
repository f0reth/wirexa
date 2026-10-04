import { createSignal, untrack } from "solid-js";
import type { ProfileOrderStorage } from "../../domain/mqtt/ports";
import type { BrokerProfile } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import { moveItem } from "../../shared/array";
import { errorMessage } from "../../shared/error";
import { applyOrder } from "../shared/order";
import { notifyOnError } from "../ui/guard";
import { DEFAULT_BROKER_URL } from "./broker-url";

/** 新規作成ダイアログの初期プロファイル。ID はサーバが採番するので空にする。 */
export function createEmptyProfile(): BrokerProfile {
  return {
    id: "",
    name: "",
    broker: DEFAULT_BROKER_URL,
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
  // 読み込みに成功するまで、一覧は保存済みのプロファイルを含まない。その一覧で並び順を保存すると、
  // 読み込めなかったプロファイルの並びを消してしまう。
  let loaded = false;

  // 一覧を更新して並び順を保存する。update が null を返したら何もしない。
  // 保存してから signal に入れる（保存より前に effect を走らせない）。
  function commit(
    update: (prev: BrokerProfile[]) => BrokerProfile[] | null,
  ): void {
    const next = update(untrack(profiles));
    if (!next) return;
    if (loaded) orderStorage.save(next.map((p) => p.id));
    setProfiles(next);
  }

  async function loadProfiles(): Promise<void> {
    const fetched = await api.getProfiles();
    loaded = true;
    const order = orderStorage.load();
    setProfiles(applyOrder(fetched, order));
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
    commit((prev) =>
      prev.some((p) => p.id === saved.id)
        ? prev.map((p) => (p.id === saved.id ? saved : p))
        : [...prev, saved],
    );
    return saved;
  }

  // 失敗時は通知したうえで例外を伝える（呼び出し側は成功したときだけタブを閉じる）。
  async function deleteProfile(id: string): Promise<void> {
    await notifyOnError(notifier, "Failed to delete broker", () =>
      api.deleteProfile(id),
    );
    commit((prev) => prev.filter((p) => p.id !== id));
  }

  function reorderProfiles(fromIndex: number, toIndex: number): void {
    commit((prev) => moveItem(prev, fromIndex, toIndex));
  }

  return {
    profiles,
    loadProfiles,
    saveProfile,
    deleteProfile,
    reorderProfiles,
  };
}
