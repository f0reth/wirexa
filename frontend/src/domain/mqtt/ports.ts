import type { PublishPreset } from "./types";

export interface ConnectionPersistence {
  loadLastProfileId(): string | null;
  saveLastProfileId(id: string): void;
  removeLastProfileId(): void;
}

export interface PresetStorage {
  load(): PublishPreset[];
  save(presets: PublishPreset[]): void;
}
