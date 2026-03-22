import { createSignal } from "solid-js";
import type { PublishPreset } from "../../domain/mqtt/types";
import {
  loadFromStorage,
  saveToStorage,
} from "../../infrastructure/storage/local-storage";

const STORAGE_KEY = "mqtt:presets";

export function createPresetsState() {
  const [presets, setPresets] = createSignal<PublishPreset[]>(
    loadFromStorage<PublishPreset[]>(STORAGE_KEY, []),
  );

  function savePreset(preset: Omit<PublishPreset, "id">) {
    const newPreset: PublishPreset = { ...preset, id: crypto.randomUUID() };
    setPresets((prev) => {
      const next = [...prev, newPreset];
      saveToStorage(STORAGE_KEY, next);
      return next;
    });
  }

  function removePreset(id: string) {
    setPresets((prev) => {
      const next = prev.filter((p) => p.id !== id);
      saveToStorage(STORAGE_KEY, next);
      return next;
    });
  }

  return { presets, savePreset, removePreset };
}
