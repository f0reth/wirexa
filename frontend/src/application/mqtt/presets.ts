import { createSignal } from "solid-js";
import type { PublishPreset } from "../../domain/mqtt/types";

export interface PresetStorage {
  load(): PublishPreset[];
  save(presets: PublishPreset[]): void;
}

export function createPresetsState(storage: PresetStorage) {
  const [presets, setPresets] = createSignal<PublishPreset[]>(storage.load());

  function savePreset(preset: Omit<PublishPreset, "id">) {
    const newPreset: PublishPreset = { ...preset, id: crypto.randomUUID() };
    setPresets((prev) => {
      const next = [...prev, newPreset];
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
  }

  return { presets, savePreset, removePreset };
}
