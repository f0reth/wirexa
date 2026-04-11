import { clsx } from "clsx";
import { ChevronRight, Folder, FolderPlus, Plus, Trash2 } from "lucide-solid";
import { createSignal, For, Show } from "solid-js";
import type { HttpMethod, TreeItem } from "../../../domain/http/types";
import { METHOD_COLORS } from "../../constants/http";
import { dragItem, dropTarget, setDragItem, setGhostPos } from "./drag-state";
import styles from "./sidebar.module.css";

const LONG_PRESS_MS = 250;

export const DROP_ZONE_ATTR = "data-drop-zone";
export const DROP_COLLECTION_ID_ATTR = "data-drop-collection-id";
export const DROP_PARENT_ID_ATTR = "data-drop-parent-id";
export const DROP_POSITION_ATTR = "data-drop-position";

/** アイテム間の挿入ゾーン */
export function InsertionZone(props: {
  collectionId: string;
  parentId: string;
  position: number;
}) {
  const isActive = () => {
    const dt = dropTarget();
    return (
      dragItem() !== null &&
      dt?.collectionId === props.collectionId &&
      dt?.parentId === props.parentId &&
      dt?.position === props.position
    );
  };

  // ドラッグ中のアイテムにとって no-op になるゾーン（元の位置の前後）は
  // pointer-events: none にして elementFromPoint が透過するようにする。
  // これにより、元フォルダ末尾の InsertionZone にカーソルが乗っても
  // その下にある別フォルダのヘッダーが正しく検出される。
  const isNoOp = () => {
    const di = dragItem();
    if (!di) return false;
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
        styles.insertionZone,
        dragItem() && !isNoOp() && styles.insertionZoneVisible,
        isActive() && styles.insertionZoneActive,
      )}
      style={{ display: isNoOp() ? "none" : undefined }}
      {...{
        [DROP_ZONE_ATTR]: "true",
        [DROP_COLLECTION_ID_ATTR]: props.collectionId,
        [DROP_PARENT_ID_ATTR]: props.parentId,
        [DROP_POSITION_ATTR]: String(props.position),
      }}
    />
  );
}

/** 長押し（LONG_PRESS_MS）またはマウス移動5px超でドラッグ開始する */
function makeDragHandlers(
  collectionId: string,
  itemId: string,
  name: string,
  sourceParentId: string,
  sourceIndex: number,
  suppressRef: { suppress: boolean },
) {
  const handleMouseDown = (e: MouseEvent) => {
    if (e.button !== 0) return;
    const startX = e.clientX;
    const startY = e.clientY;

    const activate = (x: number, y: number) => {
      suppressRef.suppress = true;
      setDragItem({ collectionId, itemId, name, sourceParentId, sourceIndex });
      setGhostPos({ x, y });
      document.removeEventListener("mousemove", handleMove);
      document.removeEventListener("mouseup", handleUp);
    };

    const handleMove = (me: MouseEvent) => {
      const dx = me.clientX - startX;
      const dy = me.clientY - startY;
      if (Math.sqrt(dx * dx + dy * dy) > 5) {
        clearTimeout(timer);
        activate(me.clientX, me.clientY);
      }
    };

    const handleUp = () => {
      clearTimeout(timer);
      document.removeEventListener("mousemove", handleMove);
      document.removeEventListener("mouseup", handleUp);
    };

    const timer = setTimeout(() => {
      document.removeEventListener("mousemove", handleMove);
      document.removeEventListener("mouseup", handleUp);
      activate(startX, startY);
    }, LONG_PRESS_MS);

    document.addEventListener("mousemove", handleMove);
    document.addEventListener("mouseup", handleUp);
  };

  return { handleMouseDown };
}

export function TreeItemNode(props: {
  item: TreeItem;
  collectionId: string;
  depth: number;
  sourceParentId: string;
  sourceIndex: number;
  onAddFolder: (collectionId: string, parentId: string) => void;
  onAddRequest: (collectionId: string, parentId: string) => void;
  onDeleteItem: (
    collectionId: string,
    itemId: string,
    name: string,
    type: string,
  ) => void;
  onSelectRequest: (item: TreeItem) => void;
  onRenameItem: (collectionId: string, itemId: string, name: string) => void;
  onMoveItem: (
    collectionId: string,
    itemId: string,
    targetParentId: string,
    position: number,
  ) => void;
  activeRequestId: string | null;
  renamingItemId: string | null;
  setRenamingItemId: (id: string | null) => void;
}) {
  const [expanded, setExpanded] = createSignal(false);

  if (props.item.type === "folder") {
    const isRenaming = () => props.renamingItemId === props.item.id;
    const suppressRef = { suppress: false };
    const { handleMouseDown } = makeDragHandlers(
      props.collectionId,
      props.item.id,
      props.item.name,
      props.sourceParentId,
      props.sourceIndex,
      suppressRef,
    );

    // dropTarget がこのフォルダを指しているとき（position=-1）にハイライト
    const isFolderDropTarget = () => {
      const dt = dropTarget();
      return (
        dragItem() !== null &&
        dt?.collectionId === props.collectionId &&
        dt?.parentId === props.item.id &&
        dt?.position === -1
      );
    };

    const handleRenameCommit = (value: string) => {
      const trimmed = value.trim();
      if (trimmed && trimmed !== props.item.name) {
        props.onRenameItem(props.collectionId, props.item.id, trimmed);
      }
      props.setRenamingItemId(null);
    };

    return (
      <div class={styles.treeNode}>
        {/* biome-ignore lint/a11y/noStaticElementInteractions: drop target and drag source for mouse-based drag */}
        <div
          class={clsx(
            styles.treeNodeHeader,
            isFolderDropTarget() && styles.dropTarget,
          )}
          style={{ "padding-left": `${props.depth * 0.75}rem` }}
          onMouseDown={handleMouseDown}
          {...{
            [DROP_ZONE_ATTR]: "true",
            [DROP_COLLECTION_ID_ATTR]: props.collectionId,
            [DROP_PARENT_ID_ATTR]: props.item.id,
            [DROP_POSITION_ATTR]: "-1",
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
              setExpanded(!expanded());
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
                      props.setRenamingItemId(props.item.id);
                    }}
                  >
                    {props.item.name}
                  </span>
                </>
              }
            >
              <input
                class={styles.renameInput}
                value={props.item.name}
                ref={(el) => {
                  requestAnimationFrame(() => {
                    el.focus();
                    el.select();
                  });
                }}
                onClick={(e) => e.stopPropagation()}
                onKeyDown={(e) => {
                  if (e.key === "Enter")
                    handleRenameCommit(e.currentTarget.value);
                  if (e.key === "Escape") props.setRenamingItemId(null);
                }}
                onBlur={(e) => handleRenameCommit(e.currentTarget.value)}
              />
            </Show>
          </button>
          <div class={styles.treeNodeActions}>
            <button
              type="button"
              class={styles.treeActionBtn}
              title="Add folder"
              onMouseDown={(e) => e.stopPropagation()}
              onClick={() =>
                props.onAddFolder(props.collectionId, props.item.id)
              }
            >
              <FolderPlus size={10} />
            </button>
            <button
              type="button"
              class={styles.treeActionBtn}
              title="Add request"
              onMouseDown={(e) => e.stopPropagation()}
              onClick={() =>
                props.onAddRequest(props.collectionId, props.item.id)
              }
            >
              <Plus size={10} />
            </button>
            <button
              type="button"
              class={clsx(styles.treeActionBtn, styles.treeActionBtnDanger)}
              title="Delete"
              onMouseDown={(e) => e.stopPropagation()}
              onClick={() =>
                props.onDeleteItem(
                  props.collectionId,
                  props.item.id,
                  props.item.name,
                  "folder",
                )
              }
            >
              <Trash2 size={10} />
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
                    onAddFolder={props.onAddFolder}
                    onAddRequest={props.onAddRequest}
                    onDeleteItem={props.onDeleteItem}
                    onSelectRequest={props.onSelectRequest}
                    onRenameItem={props.onRenameItem}
                    onMoveItem={props.onMoveItem}
                    activeRequestId={props.activeRequestId}
                    renamingItemId={props.renamingItemId}
                    setRenamingItemId={props.setRenamingItemId}
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
  const isActive = () => props.activeRequestId === props.item.id;
  const isRenaming = () => props.renamingItemId === props.item.id;
  const suppressRef = { suppress: false };
  const { handleMouseDown } = makeDragHandlers(
    props.collectionId,
    props.item.id,
    props.item.name,
    props.sourceParentId,
    props.sourceIndex,
    suppressRef,
  );

  const handleRenameCommit = (value: string) => {
    const trimmed = value.trim();
    if (trimmed && trimmed !== props.item.name) {
      props.onRenameItem(props.collectionId, props.item.id, trimmed);
    }
    props.setRenamingItemId(null);
  };

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: drag source for tree item move
    <div
      class={clsx(styles.requestRow, isActive() && styles.requestItemActive)}
      style={{ "padding-left": `${props.depth * 0.75 + 0.5}rem` }}
      onMouseDown={handleMouseDown}
    >
      <button
        type="button"
        class={styles.requestSelectBtn}
        onClick={() => {
          if (suppressRef.suppress) {
            suppressRef.suppress = false;
            return;
          }
          if (!isRenaming()) props.onSelectRequest(props.item);
        }}
        onKeyDown={(e) => {
          if (!isRenaming() && (e.key === "Enter" || e.key === " "))
            props.onSelectRequest(props.item);
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
                  props.setRenamingItemId(props.item.id);
                }}
              >
                {props.item.name}
              </span>
            </>
          }
        >
          <input
            class={styles.renameInput}
            value={props.item.name}
            ref={(el) => {
              requestAnimationFrame(() => {
                el.focus();
                el.select();
              });
            }}
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleRenameCommit(e.currentTarget.value);
              if (e.key === "Escape") props.setRenamingItemId(null);
            }}
            onBlur={(e) => handleRenameCommit(e.currentTarget.value)}
          />
        </Show>
      </button>
      <button
        type="button"
        class={clsx(styles.treeActionBtn, styles.treeActionBtnDanger)}
        onMouseDown={(e) => e.stopPropagation()}
        onClick={() =>
          props.onDeleteItem(
            props.collectionId,
            props.item.id,
            props.item.name,
            "request",
          )
        }
      >
        <Trash2 size={10} />
      </button>
    </div>
  );
}
