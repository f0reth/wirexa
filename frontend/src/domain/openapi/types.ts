export type OpenApiFile = {
  path: string; // 絶対パス（一意キー）
  name: string; // ファイル名（表示用）
  order: number; // 表示順（ドラッグ並び替え用）
  lastOpenedAt: string; // ISO8601
};

// アクティブ文書。ディスク上のファイル、または無題（ペースト/D&D 由来）。
export type ActiveDoc =
  | { kind: "file"; path: string; name: string }
  | { kind: "untitled"; name: string }
  | null;
