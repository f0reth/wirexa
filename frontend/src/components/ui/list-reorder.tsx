import { clsx } from "clsx";
import { createSignal } from "solid-js";
import styles from "./list-reorder.module.css";

/**
 * ネイティブ HTML5 D&D による並べ替えの状態とハンドラを提供するフック。
 *
 * `draggingIndex` / `dropIndicatorIndex` の 2 状態を内包し、`itemHandlers(index)` が
 * 各アイテムに spread する drag ハンドラ一式を返す。クリック挙動はアイテムごとに
 * 異なるため含めない（呼び出し側で `draggingIndex() !== null` をガードすること）。
 *
 * ツリー用の `sidebar/use-tree-drag-drop.ts`（マウス長押し方式）とは別ロジック。
 */
export function createListReorder(
  onReorder: (from: number, to: number) => void,
) {
  const [draggingIndex, setDraggingIndex] = createSignal<number | null>(null);
  const [dropIndicatorIndex, setDropIndicatorIndex] = createSignal<
    number | null
  >(null);

  const itemHandlers = (index: () => number) => ({
    draggable: true,
    onDragStart: (e: DragEvent) => {
      setDraggingIndex(index());
      e.dataTransfer?.setData("text/plain", String(index()));
    },
    onDragEnd: () => {
      setTimeout(() => setDraggingIndex(null), 0);
      setDropIndicatorIndex(null);
    },
    onDragOver: (e: DragEvent) => {
      e.preventDefault();
      const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
      const mid = rect.top + rect.height / 2;
      setDropIndicatorIndex(e.clientY < mid ? index() : index() + 1);
    },
    onDragLeave: (e: DragEvent) => {
      if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
        setDropIndicatorIndex(null);
      }
    },
    onDrop: (e: DragEvent) => {
      e.preventDefault();
      const from = parseInt(e.dataTransfer?.getData("text/plain") ?? "-1", 10);
      const p = dropIndicatorIndex();
      if (p !== null && from >= 0) {
        const to = p > from ? p - 1 : p;
        onReorder(from, to);
      }
      setDropIndicatorIndex(null);
    },
  });

  return { draggingIndex, dropIndicatorIndex, itemHandlers };
}

/** アイテム間の挿入位置インジケータ。 */
export function InsertionZone(props: { visible: boolean; active: boolean }) {
  return (
    <div
      class={clsx(
        styles.insertionZone,
        props.visible && styles.insertionZoneVisible,
        props.active && styles.insertionZoneActive,
      )}
    />
  );
}
