import { createSignal } from "solid-js";
import type { PresetStorage } from "../../domain/mqtt/ports";
import type { PublishPreset } from "../../domain/mqtt/types";
import { moveItem } from "../../shared/array";
import { generateId } from "../../shared/id";

export type { PresetStorage };

/** Publish フォームの編集中の値。プリセット未選択でも publish できるよう独立させる。 */
export interface PublishDraft {
  topic: string;
  payload: string;
  qos: 0 | 1 | 2;
  retain: boolean;
}

function emptyDraft(): PublishDraft {
  return { topic: "", payload: "", qos: 0, retain: false };
}

export function createPresetsState(storage: PresetStorage) {
  const [presets, setPresets] = createSignal<PublishPreset[]>(storage.load());
  const [selectedPresetId, setSelectedPresetId] = createSignal<string | null>(
    null,
  );
  const [draft, setDraft] = createSignal<PublishDraft>(emptyDraft());

  function loadDraftFromPreset(id: string): void {
    const preset = presets().find((p) => p.id === id);
    if (!preset) return;
    setDraft({
      topic: preset.topic,
      payload: preset.payload,
      qos: preset.qos,
      retain: preset.retain,
    });
  }

  /** プリセットを選択し、その内容をフォームへ読み込む。 */
  function selectPreset(id: string): void {
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

  function savePreset(preset: Omit<PublishPreset, "id">) {
    const id = generateId();
    const newPreset: PublishPreset = { ...preset, id };
    setPresets((prev) => {
      const next = [...prev, newPreset];
      storage.save(next);
      return next;
    });
    setSelectedPresetId(id);
    loadDraftFromPreset(id);
  }

  function updatePreset(
    id: string,
    updates: Partial<Omit<PublishPreset, "id">>,
  ) {
    setPresets((prev) => {
      const next = prev.map((p) => (p.id === id ? { ...p, ...updates } : p));
      storage.save(next);
      return next;
    });
  }

  function removePreset(id: string) {
    setPresets((prev) => {
      const next = prev.filter((p) => p.id !== id);
      storage.save(next);
      return next;
    });
    setSelectedPresetId((prev) => (prev === id ? null : prev));
  }

  function addPreset(name?: string) {
    const id = generateId();
    const newPreset: PublishPreset = {
      id,
      name: name ?? "no name",
      topic: "",
      payload: "",
      qos: 0,
      retain: false,
    };
    setPresets((prev) => {
      const next = [...prev, newPreset];
      storage.save(next);
      return next;
    });
    setSelectedPresetId(id);
    // 追加直後のプリセットは空なので、フォームも空に戻す。
    setDraft(emptyDraft());
  }

  function reorderPresets(fromIndex: number, toIndex: number): void {
    setPresets((prev) => {
      const next = moveItem(prev, fromIndex, toIndex);
      if (!next) return prev;
      storage.save(next);
      return next;
    });
  }

  return {
    presets,
    savePreset,
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
