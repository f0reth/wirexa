import { createSignal } from "solid-js";
import type { PresetStorage } from "../../domain/mqtt/ports";
import type { PublishPreset } from "../../domain/mqtt/types";

export type { PresetStorage };

export function createPresetsState(storage: PresetStorage) {
  const [presets, setPresets] = createSignal<PublishPreset[]>(storage.load());
  const [selectedPresetId, setSelectedPresetId] = createSignal<string | null>(
    null,
  );

  function savePreset(preset: Omit<PublishPreset, "id">) {
    const id = crypto.randomUUID();
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

  return {
    presets,
    savePreset,
    updatePreset,
    removePreset,
    selectedPresetId,
    setSelectedPresetId,
  };
}
