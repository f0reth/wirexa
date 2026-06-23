import { createSignal } from "solid-js";
import type { PresetStorage } from "../../domain/mqtt/ports";
import type { PublishPreset } from "../../domain/mqtt/types";
import { generateId } from "../../infrastructure/id/generator";

export type { PresetStorage };

export function createPresetsState(storage: PresetStorage) {
  const [presets, setPresets] = createSignal<PublishPreset[]>(storage.load());
  const [selectedPresetId, setSelectedPresetId] = createSignal<string | null>(
    null,
  );

  function savePreset(preset: Omit<PublishPreset, "id">) {
    const id = generateId();
    const newPreset: PublishPreset = { ...preset, id };
    setPresets((prev) => {
      const next = [...prev, newPreset];
      storage.save(next);
      return next;
    });
    setSelectedPresetId(id);
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
    };
    setPresets((prev) => {
      const next = [...prev, newPreset];
      storage.save(next);
      return next;
    });
    setSelectedPresetId(id);
  }

  function reorderPresets(fromIndex: number, toIndex: number): void {
    if (fromIndex < 0 || toIndex < 0) return;
    setPresets((prev) => {
      if (fromIndex >= prev.length || toIndex >= prev.length) return prev;
      const next = [...prev];
      const [moved] = next.splice(fromIndex, 1);
      next.splice(toIndex, 0, moved);
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
    setSelectedPresetId,
  };
}
