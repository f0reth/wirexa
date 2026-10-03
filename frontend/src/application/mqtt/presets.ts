import { createSignal, untrack } from "solid-js";
import type { PresetStorage } from "../../domain/mqtt/ports";
import type { PublishPreset } from "../../domain/mqtt/types";
import { moveItem } from "../../shared/array";
import { generateId } from "../../shared/id";

/** Publish フォームの編集中の値。プリセット未選択でも publish できるよう独立させる。 */
export type PublishDraft = Omit<PublishPreset, "id" | "name">;

function emptyDraft(): PublishDraft {
  return { topic: "", payload: "", qos: 0, retain: false };
}

export function createPresetsState(storage: PresetStorage) {
  const [presets, setPresets] = createSignal<PublishPreset[]>(storage.load());
  const [selectedPresetId, setSelectedPresetId] = createSignal<string | null>(
    null,
  );
  const [draft, setDraft] = createSignal<PublishDraft>(emptyDraft());

  // 一覧を更新して保存する。update が null を返したら何もしない。
  // 保存してから signal に入れる（保存より前に effect を走らせない）。
  function commit(
    update: (prev: PublishPreset[]) => PublishPreset[] | null,
  ): void {
    const next = update(untrack(presets));
    if (!next) return;
    storage.save(next);
    setPresets(next);
  }

  function loadDraftFromPreset(id: string): void {
    const preset = presets().find((p) => p.id === id);
    if (!preset) return;
    const { id: _id, name: _name, ...rest } = preset;
    setDraft(rest);
  }

  /** プリセットを選択し、その内容をフォームへ読み込む。 */
  function selectPreset(id: string): void {
    // 存在しない id を選ぶと、以後の編集がどのプリセットにも書き戻されなくなる。
    if (!presets().some((p) => p.id === id)) return;
    setSelectedPresetId(id);
    loadDraftFromPreset(id);
  }

  /**
   * フォーム入力を draft へ反映する。プリセット選択中は同じ値をプリセットへ書き戻す
   * （選択中プリセットは常にフォームの内容と一致する）。
   */
  function updateDraft(patch: Partial<PublishDraft>): void {
    setDraft((prev) => ({ ...prev, ...patch }));
    const id = selectedPresetId();
    if (id) updatePreset(id, patch);
  }

  function updatePreset(
    id: string,
    updates: Partial<Omit<PublishPreset, "id">>,
  ) {
    commit((prev) => prev.map((p) => (p.id === id ? { ...p, ...updates } : p)));
  }

  function removePreset(id: string) {
    commit((prev) => prev.filter((p) => p.id !== id));
    setSelectedPresetId((prev) => (prev === id ? null : prev));
  }

  function addPreset() {
    const id = generateId();
    const newPreset: PublishPreset = { id, name: "no name", ...emptyDraft() };
    commit((prev) => [...prev, newPreset]);
    setSelectedPresetId(id);
    // 追加直後のプリセットは空なので、フォームも空に戻す。
    setDraft(emptyDraft());
  }

  function reorderPresets(fromIndex: number, toIndex: number): void {
    commit((prev) => moveItem(prev, fromIndex, toIndex));
  }

  return {
    presets,
    addPreset,
    updatePreset,
    removePreset,
    reorderPresets,
    selectedPresetId,
    selectPreset,
    draft,
    updateDraft,
  };
}
