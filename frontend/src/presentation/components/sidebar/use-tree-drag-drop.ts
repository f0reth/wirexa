import { onCleanup, onMount } from "solid-js";
import {
  type DragItem,
  dragItem,
  setDragItem,
  setDropTarget,
  setGhostPos,
} from "./drag-state";
import {
  DROP_COLLECTION_ID_ATTR,
  DROP_KIND_ATTR,
  DROP_PARENT_ID_ATTR,
  DROP_POSITION_ATTR,
  DROP_ZONE_ATTR,
  TREE_ITEM_COLLECTION_ID_ATTR,
  TREE_ITEM_ID_ATTR,
  TREE_ITEM_INDEX_ATTR,
  TREE_ITEM_PARENT_ID_ATTR,
} from "./tree-item-node";

// TreeDragDropDeps はドロップ確定時に呼ぶコレクション操作コールバック。
export interface TreeDragDropDeps {
  onMoveItem: (
    sourceCollectionId: string,
    itemId: string,
    targetCollectionId: string,
    targetParentId: string,
    position: number,
  ) => void;
  onDropToSidebar: (di: DragItem, position: number) => void;
}

// useTreeDragDrop はツリーのドラッグ&ドロップを担う document レベルの
// mousemove/mouseup ハンドラを登録する。ドラッグ状態は drag-state の signal で共有し、
// ドロップ確定時のコレクション更新だけを呼び出し側コールバックに委譲する。
// CollectionTree から分離し、描画とドラッグロジックの関心を分ける。
export function useTreeDragDrop(deps: TreeDragDropDeps): void {
  onMount(() => {
    const handleMouseMove = (e: MouseEvent) => {
      const di = dragItem();
      if (!di) return;
      setGhostPos({ x: e.clientX, y: e.clientY });

      // elementFromPoint で現在マウス下にあるドロップゾーンを検出する。
      const el = document.elementFromPoint(e.clientX, e.clientY);
      const zone = el?.closest(`[${DROP_ZONE_ATTR}]`) as HTMLElement | null;
      if (zone) {
        const zoneKind = zone.getAttribute(DROP_KIND_ATTR) ?? "item";
        if (zoneKind === "collection-header" && di.kind === "item") {
          const collectionId = zone.getAttribute(DROP_COLLECTION_ID_ATTR) ?? "";
          setDropTarget({
            kind: "item",
            collectionId,
            parentId: "",
            position: -1,
          });
          return;
        }
        const position = parseInt(
          zone.getAttribute(DROP_POSITION_ATTR) ?? "-1",
          10,
        );
        if (position === -1) {
          setDropTarget(null);
          return;
        }
        if (zoneKind === "sidebar") {
          setDropTarget({ kind: "sidebar", position });
        } else if (zoneKind === "item" && di.kind === "item") {
          const collectionId = zone.getAttribute(DROP_COLLECTION_ID_ATTR) ?? "";
          const parentId = zone.getAttribute(DROP_PARENT_ID_ATTR) ?? "";
          setDropTarget({ kind: "item", collectionId, parentId, position });
        } else {
          setDropTarget(null);
        }
      } else {
        // InsertionZone が見つからない場合、アイテム行上か確認する
        const itemEl = el?.closest(
          `[${TREE_ITEM_ID_ATTR}]`,
        ) as HTMLElement | null;
        if (itemEl && di.kind === "item") {
          const rect = itemEl.getBoundingClientRect();
          const isUpperHalf = e.clientY < rect.top + rect.height / 2;
          const itemIndex = parseInt(
            itemEl.getAttribute(TREE_ITEM_INDEX_ATTR) ?? "-1",
            10,
          );
          const collectionId =
            itemEl.getAttribute(TREE_ITEM_COLLECTION_ID_ATTR) ?? "";
          const parentId = itemEl.getAttribute(TREE_ITEM_PARENT_ID_ATTR) ?? "";
          const position = isUpperHalf ? itemIndex : itemIndex + 1;
          setDropTarget({ kind: "item", collectionId, parentId, position });
        } else {
          setDropTarget(null);
        }
      }
    };

    const handleMouseUp = (e: MouseEvent) => {
      const di = dragItem();
      if (di) {
        // mousemove のスロットリングで dropTarget が古くなる可能性があるため、
        // mouseup 時の正確な座標からドロップゾーンを直接検出する。
        const el = document.elementFromPoint(e.clientX, e.clientY);
        const zone = el?.closest(`[${DROP_ZONE_ATTR}]`) as HTMLElement | null;
        if (zone) {
          const zoneKind = zone.getAttribute(DROP_KIND_ATTR) ?? "item";
          if (zoneKind === "collection-header" && di.kind === "item") {
            const collectionId =
              zone.getAttribute(DROP_COLLECTION_ID_ATTR) ?? "";
            deps.onMoveItem(di.collectionId, di.itemId, collectionId, "", -1);
          } else {
            const position = parseInt(
              zone.getAttribute(DROP_POSITION_ATTR) ?? "-1",
              10,
            );
            if (position !== -1) {
              if (zoneKind === "sidebar") {
                deps.onDropToSidebar(di, position);
              } else if (zoneKind === "item" && di.kind === "item") {
                const collectionId =
                  zone.getAttribute(DROP_COLLECTION_ID_ATTR) ?? "";
                const parentId = zone.getAttribute(DROP_PARENT_ID_ATTR) ?? "";
                deps.onMoveItem(
                  di.collectionId,
                  di.itemId,
                  collectionId,
                  parentId,
                  position,
                );
              }
            }
          }
        } else {
          // InsertionZone が見つからない場合、アイテム行上か確認する
          const itemEl = el?.closest(
            `[${TREE_ITEM_ID_ATTR}]`,
          ) as HTMLElement | null;
          if (itemEl && di.kind === "item") {
            const rect = itemEl.getBoundingClientRect();
            const isUpperHalf = e.clientY < rect.top + rect.height / 2;
            const itemIndex = parseInt(
              itemEl.getAttribute(TREE_ITEM_INDEX_ATTR) ?? "-1",
              10,
            );
            const collectionId =
              itemEl.getAttribute(TREE_ITEM_COLLECTION_ID_ATTR) ?? "";
            const parentId =
              itemEl.getAttribute(TREE_ITEM_PARENT_ID_ATTR) ?? "";
            const position = isUpperHalf ? itemIndex : itemIndex + 1;
            deps.onMoveItem(
              di.collectionId,
              di.itemId,
              collectionId,
              parentId,
              position,
            );
          }
        }
      }
      setDragItem(null);
      setDropTarget(null);
      setGhostPos(null);
    };

    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
    onCleanup(() => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    });
  });
}
