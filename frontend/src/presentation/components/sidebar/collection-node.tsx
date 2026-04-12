import { clsx } from "clsx";
import { ChevronRight, Folder, FolderPlus, Plus, Trash2 } from "lucide-solid";
import { createSignal, For, Show } from "solid-js";
import type { Collection, TreeItem } from "../../../domain/http/types";
import styles from "./sidebar.module.css";
import { InsertionZone, TreeItemNode } from "./tree-item-node";

export function CollectionNode(props: {
  collection: Collection;
  onDeleteCollection: (id: string, name: string) => void;
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
  onRenameCollection: (id: string, name: string) => void;
  onMoveItem: (
    collectionId: string,
    itemId: string,
    targetParentId: string,
    position: number,
  ) => void;
  activeRequestId: string | null;
  renamingItemId: string | null;
  setRenamingItemId: (id: string | null) => void;
  renamingCollectionId: string | null;
  setRenamingCollectionId: (id: string | null) => void;
}) {
  const [expanded, setExpanded] = createSignal(true);

  const isRenaming = () => props.renamingCollectionId === props.collection.id;

  const handleRenameCommit = (value: string) => {
    const trimmed = value.trim();
    if (trimmed && trimmed !== props.collection.name) {
      props.onRenameCollection(props.collection.id, trimmed);
    }
    props.setRenamingCollectionId(null);
  };

  return (
    <div class={styles.treeNode}>
      <div class={styles.treeNodeHeader}>
        <button
          type="button"
          class={styles.treeNodeToggle}
          onClick={() => setExpanded(!expanded())}
        >
          <ChevronRight
            size={12}
            class={clsx(styles.chevron, expanded() && styles.chevronExpanded)}
          />
          <Folder size={14} />
          <Show
            when={isRenaming()}
            fallback={
              <>
                {/* biome-ignore lint/a11y/noStaticElementInteractions: double-click-to-rename */}
                <span
                  class={styles.treeNodeName}
                  onDblClick={(e) => {
                    e.stopPropagation();
                    props.setRenamingCollectionId(props.collection.id);
                  }}
                >
                  {props.collection.name}
                </span>
              </>
            }
          >
            <input
              class={styles.renameInput}
              value={props.collection.name}
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
                if (e.key === "Escape") props.setRenamingCollectionId(null);
              }}
              onBlur={(e) => handleRenameCommit(e.currentTarget.value)}
            />
          </Show>
        </button>
        <div class={styles.treeNodeActions}>
          <button
            type="button"
            class={styles.treeActionBtn}
            aria-label="Add folder"
            title="Add folder"
            onClick={() => props.onAddFolder(props.collection.id, "")}
          >
            <FolderPlus size={12} aria-hidden="true" />
          </button>
          <button
            type="button"
            class={styles.treeActionBtn}
            aria-label="Add request"
            title="Add request"
            onClick={() => props.onAddRequest(props.collection.id, "")}
          >
            <Plus size={12} aria-hidden="true" />
          </button>
          <button
            type="button"
            class={clsx(styles.treeActionBtn, styles.treeActionBtnDanger)}
            aria-label="Delete collection"
            title="Delete collection"
            onClick={() =>
              props.onDeleteCollection(
                props.collection.id,
                props.collection.name,
              )
            }
          >
            <Trash2 size={12} aria-hidden="true" />
          </button>
        </div>
      </div>
      <Show when={expanded()}>
        <div class={styles.treeChildren}>
          <For each={props.collection.items}>
            {(item, index) => (
              <>
                <InsertionZone
                  collectionId={props.collection.id}
                  parentId=""
                  position={index()}
                />
                <TreeItemNode
                  item={item}
                  collectionId={props.collection.id}
                  depth={1}
                  sourceParentId=""
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
            collectionId={props.collection.id}
            parentId=""
            position={props.collection.items.length}
          />
        </div>
      </Show>
    </div>
  );
}
