/** 送信先ターゲットの表示順（ターゲット id の並び）の保存先。 */
export interface TargetOrderStorage {
  load(): string[];
  save(ids: string[]): void;
}
