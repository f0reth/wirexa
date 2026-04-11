import { createSignal } from "solid-js";

export type DragItem = {
  collectionId: string;
  itemId: string;
  name: string;
};

// parentId が空文字はコレクションルート、それ以外はフォルダID。
// position が -1 はそのフォルダへ移動（末尾追加）、>=0 は挿入先インデックス。
export type DropTarget = {
  collectionId: string;
  parentId: string;
  position: number;
} | null;

export const [dragItem, setDragItem] = createSignal<DragItem | null>(null);
export const [dropTarget, setDropTarget] = createSignal<DropTarget>(null);
export const [ghostPos, setGhostPos] = createSignal<{
  x: number;
  y: number;
} | null>(null);
