/** サイドバーのフォルダ・コレクションの展開状態（id → 展開しているか）の保存先。 */
export interface ExpandedFoldersStorage {
  load(): Record<string, boolean>;
  save(ids: Record<string, boolean>): void;
}
