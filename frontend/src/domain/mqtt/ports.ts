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

/** プロファイルの表示順（プロファイル id の並び）の保存先。 */
export interface ProfileOrderStorage {
  load(): string[];
  save(ids: string[]): void;
}
