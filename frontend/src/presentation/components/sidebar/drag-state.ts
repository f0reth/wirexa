import { createSignal } from "solid-js";

export type DragItem =
  | {
      kind: "item";
      collectionId: string;
      itemId: string;
      name: string;
      sourceParentId: string;
      sourceIndex: number;
    }
  | {
      kind: "collection";
      collectionId: string;
      name: string;
      sourceIndex: number;
    };

// item ゾーン: parentId が空文字はコレクションルート、それ以外はフォルダID。
// position が -1 はそのフォルダへ移動（末尾追加）、>=0 は挿入先インデックス。
// sidebar ゾーン: position はサイドバーレイアウト内の挿入先インデックス。
export type DropTarget =
  | { kind: "item"; collectionId: string; parentId: string; position: number }
  | { kind: "sidebar"; position: number }
  | null;

export const [dragItem, setDragItem] = createSignal<DragItem | null>(null);
export const [dropTarget, setDropTarget] = createSignal<DropTarget>(null);
export const [ghostPos, setGhostPos] = createSignal<{
  x: number;
  y: number;
} | null>(null);
