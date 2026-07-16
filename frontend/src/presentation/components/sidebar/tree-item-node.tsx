import { clsx } from "clsx";
import { ChevronRight, Folder, FolderPlus, Plus, Trash2 } from "lucide-solid";
import { For, Show } from "solid-js";
import reorderStyles from "../../../components/ui/list-reorder.module.css";
import type { HttpMethod, TreeItem } from "../../../domain/http/types";
import { METHOD_COLORS } from "../../constants/http";
import { useHttpCollections } from "../../providers/http-provider";
import { dragItem, dropTarget, setDragItem, setGhostPos } from "./drag-state";
import { RenameInput } from "./rename-input";
import styles from "./sidebar.module.css";
import { useTreeUi } from "./tree-ui-context";
import { makeLongPressDragHandlers } from "./use-long-press-drag";

export const DROP_ZONE_ATTR = "data-drop-zone";
export const DROP_COLLECTION_ID_ATTR = "data-drop-collection-id";
export const DROP_PARENT_ID_ATTR = "data-drop-parent-id";
export const DROP_POSITION_ATTR = "data-drop-position";
export const DROP_KIND_ATTR = "data-drop-kind";

export const TREE_ITEM_ID_ATTR = "data-tree-item-id";
export const TREE_ITEM_COLLECTION_ID_ATTR = "data-tree-item-collection-id";
export const TREE_ITEM_PARENT_ID_ATTR = "data-tree-item-parent-id";
export const TREE_ITEM_INDEX_ATTR = "data-tree-item-index";

/** アイテム間またはコレクション間の挿入ゾーン */
export function InsertionZone(props: {
  collectionId?: string;
  parentId?: string;
  position: number;
  kind?: "item" | "collection" | "sidebar";
}) {
  const kind = () => props.kind ?? "item";

  const isActive = () => {
    const dt = dropTarget();
    if (!dt) return false;
    if (kind() === "sidebar") {
      return dt.kind === "sidebar" && dt.position === props.position;
    }
    return (
      dt.kind === "item" &&
      dt.collectionId === props.collectionId &&
      dt.parentId === props.parentId &&
      dt.position === props.position
    );
  };

  // ドラッグ中のアイテムにとって no-op になるゾーンは
  // pointer-events: none にして elementFromPoint が透過するようにする。
  const isNoOp = () => {
    const di = dragItem();
    if (!di) return false;
    if (kind() === "sidebar") {
      // sidebar ゾーンはコレクション・アイテム両方のドラッグを受け付ける
      return (
        props.position === di.sourceIndex ||
        props.position === di.sourceIndex + 1
      );
    }
    // アイテムゾーンはアイテムドラッグ時のみ有効
    if (di.kind !== "item") return true;
    if (
      di.collectionId !== props.collectionId ||
      di.sourceParentId !== props.parentId
    )
      return false;
    return (
      props.position === di.sourceIndex || props.position === di.sourceIndex + 1
    );
  };

  return (
    <div
      class={clsx(
        reorderStyles.insertionZone,
        dragItem() && !isNoOp() && reorderStyles.insertionZoneVisible,
        isActive() && reorderStyles.insertionZoneActive,
      )}
      style={{ display: isNoOp() ? "none" : undefined }}
      {...{
        [DROP_ZONE_ATTR]: "true",
        [DROP_KIND_ATTR]: kind(),
        [DROP_COLLECTION_ID_ATTR]: props.collectionId ?? "",
        [DROP_PARENT_ID_ATTR]: props.parentId ?? "",
        [DROP_POSITION_ATTR]: String(props.position),
      }}
    />
  );
}

export function TreeItemNode(props: {
  item: TreeItem;
  collectionId: string;
  depth: number;
  sourceParentId: string;
  sourceIndex: number;
}) {
  const ui = useTreeUi();
  const collectionsCtx = useHttpCollections();
  const expanded = () => collectionsCtx.isExpanded(props.item.id, false);
  const toggleExpanded = () =>
    collectionsCtx.setExpanded(props.item.id, !expanded());

  // folder / request の両分岐で同じペイロードを積むため 1 箇所にまとめる。
  const dragHandlers = (suppressRef: { suppress: boolean }) =>
    makeLongPressDragHandlers(suppressRef, (x, y) => {
      setDragItem({
        kind: "item",
        collectionId: props.collectionId,
        itemId: props.item.id,
        name: props.item.name,
        sourceParentId: props.sourceParentId,
        sourceIndex: props.sourceIndex,
      });
      setGhostPos({ x, y });
    });

  if (props.item.type === "folder") {
    const isRenaming = () => ui.renamingItemId() === props.item.id;
    const suppressRef = { suppress: false };
    const { handleMouseDown } = dragHandlers(suppressRef);

    const handleRenameCommit = (value: string) => {
      if (!isRenaming()) return;
      const trimmed = value.trim();
      if (trimmed && trimmed !== props.item.name) {
        ui.onRenameItem(props.collectionId, props.item.id, trimmed);
      }
      ui.setRenamingItemId(null);
    };

    return (
      <div class={styles.treeNode}>
        {/* biome-ignore lint/a11y/noStaticElementInteractions: drag source for mouse-based drag */}
        <div
          class={styles.treeNodeHeader}
          style={{ "padding-left": `${props.depth * 0.75}rem` }}
          onMouseDown={handleMouseDown}
          {...{
            [TREE_ITEM_ID_ATTR]: props.item.id,
            [TREE_ITEM_COLLECTION_ID_ATTR]: props.collectionId,
            [TREE_ITEM_PARENT_ID_ATTR]: props.sourceParentId,
            [TREE_ITEM_INDEX_ATTR]: String(props.sourceIndex),
          }}
        >
          <button
            type="button"
            class={styles.treeNodeToggle}
            onClick={() => {
              if (suppressRef.suppress) {
                suppressRef.suppress = false;
                return;
              }
              toggleExpanded();
            }}
          >
            <ChevronRight
              size={12}
              class={clsx(styles.chevron, expanded() && styles.chevronExpanded)}
            />
            <Folder size={12} />
            <Show
              when={isRenaming()}
              fallback={
                <>
                  {/* biome-ignore lint/a11y/noStaticElementInteractions: double-click-to-rename */}
                  <span
                    class={styles.treeNodeName}
                    onDblClick={(e) => {
                      e.stopPropagation();
                      ui.setRenamingItemId(props.item.id);
                    }}
                  >
                    {props.item.name}
                  </span>
                </>
              }
            >
              <RenameInput
                value={props.item.name}
                onCommit={handleRenameCommit}
                onCancel={() => ui.setRenamingItemId(null)}
              />
            </Show>
          </button>
          <div class={styles.treeNodeActions}>
            <button
              type="button"
              class={styles.treeActionBtn}
              aria-label="Add folder"
              title="Add folder"
              onMouseDown={(e) => e.stopPropagation()}
              onClick={() => ui.onAddFolder(props.collectionId, props.item.id)}
            >
              <FolderPlus size={10} aria-hidden="true" />
            </button>
            <button
              type="button"
              class={styles.treeActionBtn}
              aria-label="Add request"
              title="Add request"
              onMouseDown={(e) => e.stopPropagation()}
              onClick={() => ui.onAddRequest(props.collectionId, props.item.id)}
            >
              <Plus size={10} aria-hidden="true" />
            </button>
            <button
              type="button"
              class={clsx(styles.treeActionBtn, styles.treeActionBtnDanger)}
              aria-label="Delete folder"
              title="Delete"
              onMouseDown={(e) => e.stopPropagation()}
              onClick={() =>
                ui.onDeleteItem(
                  props.collectionId,
                  props.item.id,
                  props.item.name,
                  "folder",
                )
              }
            >
              <Trash2 size={10} aria-hidden="true" />
            </button>
          </div>
        </div>
        <Show when={expanded()}>
          <div class={styles.treeChildren}>
            <For each={props.item.children}>
              {(child, index) => (
                <>
                  <InsertionZone
                    collectionId={props.collectionId}
                    parentId={props.item.id}
                    position={index()}
                  />
                  <TreeItemNode
                    item={child}
                    collectionId={props.collectionId}
                    depth={props.depth + 1}
                    sourceParentId={props.item.id}
                    sourceIndex={index()}
                  />
                </>
              )}
            </For>
            <InsertionZone
              collectionId={props.collectionId}
              parentId={props.item.id}
              position={props.item.children.length}
            />
          </div>
        </Show>
      </div>
    );
  }

  // Request item
  const method = () => (props.item.request?.method || "GET") as HttpMethod;
  const isActive = () => ui.activeRequestId() === props.item.id;
  const isRenaming = () => ui.renamingItemId() === props.item.id;
  const suppressRef = { suppress: false };
  const { handleMouseDown } = dragHandlers(suppressRef);

  const handleRenameCommit = (value: string) => {
    if (!isRenaming()) return;
    const trimmed = value.trim();
    if (trimmed && trimmed !== props.item.name) {
      ui.onRenameItem(props.collectionId, props.item.id, trimmed);
    }
    ui.setRenamingItemId(null);
  };

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: drag source for tree item move
    <div
      class={clsx(styles.requestRow, isActive() && styles.requestItemActive)}
      style={{ "padding-left": `${props.depth * 0.75 + 0.5}rem` }}
      onMouseDown={handleMouseDown}
      {...{
        [TREE_ITEM_ID_ATTR]: props.item.id,
        [TREE_ITEM_COLLECTION_ID_ATTR]: props.collectionId,
        [TREE_ITEM_PARENT_ID_ATTR]: props.sourceParentId,
        [TREE_ITEM_INDEX_ATTR]: String(props.sourceIndex),
      }}
    >
      <button
        type="button"
        class={styles.requestSelectBtn}
        aria-current={isActive() ? "true" : undefined}
        onClick={() => {
          if (suppressRef.suppress) {
            suppressRef.suppress = false;
            return;
          }
          if (!isRenaming()) ui.onSelectRequest(props.item, props.collectionId);
        }}
        onKeyDown={(e) => {
          if (!isRenaming() && (e.key === "Enter" || e.key === " "))
            ui.onSelectRequest(props.item, props.collectionId);
        }}
      >
        <span
          class={styles.methodBadge}
          style={{ color: METHOD_COLORS[method()] }}
        >
          {method()}
        </span>
        <Show
          when={isRenaming()}
          fallback={
            <>
              {/* biome-ignore lint/a11y/noStaticElementInteractions: double-click-to-rename */}
              <span
                class={styles.requestName}
                onDblClick={(e) => {
                  e.stopPropagation();
                  ui.setRenamingItemId(props.item.id);
                }}
              >
                {props.item.name}
              </span>
            </>
          }
        >
          <RenameInput
            value={props.item.name}
            onCommit={handleRenameCommit}
            onCancel={() => ui.setRenamingItemId(null)}
          />
        </Show>
      </button>
      <button
        type="button"
        class={clsx(styles.treeActionBtn, styles.treeActionBtnDanger)}
        aria-label="Delete request"
        title="Delete"
        onMouseDown={(e) => e.stopPropagation()}
        onClick={() =>
          ui.onDeleteItem(
            props.collectionId,
            props.item.id,
            props.item.name,
            "request",
          )
        }
      >
        <Trash2 size={10} aria-hidden="true" />
      </button>
    </div>
  );
}
