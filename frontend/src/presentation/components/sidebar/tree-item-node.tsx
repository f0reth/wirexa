import { clsx } from "clsx";
import { ChevronRight, Folder, FolderPlus, Plus, Trash2 } from "lucide-solid";
import { createSignal, For, Show } from "solid-js";
import type { HttpMethod, TreeItem } from "../../../domain/http/types";
import { METHOD_COLORS } from "../../constants/http";
import styles from "./sidebar.module.css";

function encodeDragData(collectionId: string, itemId: string): string {
  return JSON.stringify({ collectionId, itemId });
}

function decodeDragData(
  data: string,
): { collectionId: string; itemId: string } | null {
  try {
    return JSON.parse(data);
  } catch {
    return null;
  }
}

export function TreeItemNode(props: {
  item: TreeItem;
  collectionId: string;
  depth: number;
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
  ) => void;
  activeRequestId: string | null;
  renamingItemId: string | null;
  setRenamingItemId: (id: string | null) => void;
}) {
  const [expanded, setExpanded] = createSignal(false);
  const [isDragging, setIsDragging] = createSignal(false);
  const [isDragOver, setIsDragOver] = createSignal(false);
  const [isDragOverChildren, setIsDragOverChildren] = createSignal(false);

  if (props.item.type === "folder") {
    const isRenaming = () => props.renamingItemId === props.item.id;

    const handleRenameCommit = (value: string) => {
      const trimmed = value.trim();
      if (trimmed && trimmed !== props.item.name) {
        props.onRenameItem(props.collectionId, props.item.id, trimmed);
      }
      props.setRenamingItemId(null);
    };

    const handleDragStart = (e: DragEvent) => {
      e.stopPropagation();
      e.dataTransfer?.setData(
        "application/wirexa-item",
        encodeDragData(props.collectionId, props.item.id),
      );
      setIsDragging(true);
    };

    const handleDragEnd = () => setIsDragging(false);

    const handleDragOver = (e: DragEvent) => {
      if (!e.dataTransfer?.types.includes("application/wirexa-item")) return;
      e.preventDefault();
      e.stopPropagation();
      setIsDragOver(true);
    };

    const handleDragLeave = (e: DragEvent) => {
      if (
        e.currentTarget instanceof Element &&
        e.currentTarget.contains(e.relatedTarget as Node)
      )
        return;
      setIsDragOver(false);
    };

    const handleDrop = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsDragOver(false);
      const raw = e.dataTransfer?.getData("application/wirexa-item");
      if (!raw) return;
      const data = decodeDragData(raw);
      if (!data || data.collectionId !== props.collectionId) return;
      if (data.itemId === props.item.id) return;
      props.onMoveItem(data.collectionId, data.itemId, props.item.id);
      setExpanded(true);
    };

    const handleDragOverChildren = (e: DragEvent) => {
      if (!e.dataTransfer?.types.includes("application/wirexa-item")) return;
      e.preventDefault();
      e.stopPropagation();
      setIsDragOverChildren(true);
    };

    const handleDragLeaveChildren = (e: DragEvent) => {
      if (
        e.currentTarget instanceof Element &&
        e.currentTarget.contains(e.relatedTarget as Node)
      )
        return;
      setIsDragOverChildren(false);
    };

    const handleDropChildren = (e: DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsDragOverChildren(false);
      const raw = e.dataTransfer?.getData("application/wirexa-item");
      if (!raw) return;
      const data = decodeDragData(raw);
      if (!data || data.collectionId !== props.collectionId) return;
      if (data.itemId === props.item.id) return;
      props.onMoveItem(data.collectionId, data.itemId, props.item.id);
      setExpanded(true);
    };

    return (
      // biome-ignore lint/a11y/noStaticElementInteractions: drag source for tree item move
      <div
        class={clsx(styles.treeNode, isDragging() && styles.dragging)}
        draggable
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
      >
        {/* biome-ignore lint/a11y/noStaticElementInteractions: drop target for tree item move */}
        <div
          class={clsx(styles.treeNodeHeader, isDragOver() && styles.dropTarget)}
          style={{ "padding-left": `${props.depth * 0.75}rem` }}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
        >
          <button
            type="button"
            class={styles.treeNodeToggle}
            onClick={() => setExpanded(!expanded())}
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
                  {/* biome-ignore lint/a11y/noStaticElementInteractions: double-click-to-rename is a progressive enhancement; primary rename path is accessible via keyboard */}
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
          {/* biome-ignore lint/a11y/noStaticElementInteractions: drop target for moving items into this folder */}
          <div
            class={clsx(
              styles.treeChildren,
              isDragOverChildren() && styles.dropTarget,
            )}
            onDragOver={handleDragOverChildren}
            onDragLeave={handleDragLeaveChildren}
            onDrop={handleDropChildren}
          >
            <For each={props.item.children}>
              {(child) => (
                <TreeItemNode
                  item={child}
                  collectionId={props.collectionId}
                  depth={props.depth + 1}
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
              )}
            </For>
          </div>
        </Show>
      </div>
    );
  }

  // Request item
  const method = () => (props.item.request?.method || "GET") as HttpMethod;
  const isActive = () => props.activeRequestId === props.item.id;
  const isRenaming = () => props.renamingItemId === props.item.id;

  const handleRenameCommit = (value: string) => {
    const trimmed = value.trim();
    if (trimmed && trimmed !== props.item.name) {
      props.onRenameItem(props.collectionId, props.item.id, trimmed);
    }
    props.setRenamingItemId(null);
  };

  const handleDragStart = (e: DragEvent) => {
    e.stopPropagation();
    e.dataTransfer?.setData(
      "application/wirexa-item",
      encodeDragData(props.collectionId, props.item.id),
    );
    setIsDragging(true);
  };

  const handleDragEnd = () => setIsDragging(false);

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: drag source for tree item move
    <div
      class={clsx(
        styles.requestRow,
        isActive() && styles.requestItemActive,
        isDragging() && styles.dragging,
      )}
      style={{ "padding-left": `${props.depth * 0.75 + 0.5}rem` }}
      draggable
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
    >
      <button
        type="button"
        class={styles.requestSelectBtn}
        onClick={() => {
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
              {/* biome-ignore lint/a11y/noStaticElementInteractions: double-click-to-rename is a progressive enhancement; primary rename is accessible via keyboard within the select button */}
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
