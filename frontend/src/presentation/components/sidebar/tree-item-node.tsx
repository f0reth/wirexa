import { clsx } from "clsx";
import {
  ChevronRight,
  Circle,
  Folder,
  FolderPlus,
  Plus,
  Trash2,
} from "lucide-solid";
import { createSignal, For, Show } from "solid-js";
import type { HttpMethod, TreeItem } from "../../../domain/http/types";
import { METHOD_COLORS } from "../../constants/http";
import styles from "./sidebar.module.css";

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
  activeRequestId: string | null;
  dirtyRequestId: string | null;
  renamingItemId: string | null;
  setRenamingItemId: (id: string | null) => void;
}) {
  const [expanded, setExpanded] = createSignal(false);

  if (props.item.type === "folder") {
    const isRenaming = () => props.renamingItemId === props.item.id;

    const handleRenameCommit = (value: string) => {
      const trimmed = value.trim();
      if (trimmed && trimmed !== props.item.name) {
        props.onRenameItem(props.collectionId, props.item.id, trimmed);
      }
      props.setRenamingItemId(null);
    };

    return (
      <div class={styles.treeNode}>
        <div
          class={styles.treeNodeHeader}
          style={{ "padding-left": `${props.depth * 0.75}rem` }}
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
          <div class={styles.treeChildren}>
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
                  activeRequestId={props.activeRequestId}
                  dirtyRequestId={props.dirtyRequestId}
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
  const isDirty = () => props.dirtyRequestId === props.item.id;
  const isRenaming = () => props.renamingItemId === props.item.id;

  const handleRenameCommit = (value: string) => {
    const trimmed = value.trim();
    if (trimmed && trimmed !== props.item.name) {
      props.onRenameItem(props.collectionId, props.item.id, trimmed);
    }
    props.setRenamingItemId(null);
  };

  return (
    <div
      class={clsx(styles.requestRow, isActive() && styles.requestItemActive)}
      style={{ "padding-left": `${props.depth * 0.75 + 0.5}rem` }}
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
              <Show when={isDirty()}>
                <Circle size={6} class={styles.dirtyDot} fill="currentColor" />
              </Show>
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
