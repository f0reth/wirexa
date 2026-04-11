export type OpenApiFile = {
  id: string; // uuid
  name: string; // ファイル名（表示用）
  path: string; // 絶対パス
  order: number; // 表示順（ドラッグ並び替え用）
  lastOpenedAt: string; // ISO8601
};
