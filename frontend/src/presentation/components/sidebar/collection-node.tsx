import { clsx } from "clsx";
import { ChevronRight, Folder, FolderPlus, Plus, Trash2 } from "lucide-solid";
import { For, Show } from "solid-js";
import type { Collection, TreeItem } from "../../../domain/http/types";
import { useHttpCollections } from "../../providers/http-provider";
import { dragItem, dropTarget, setDragItem, setGhostPos } from "./drag-state";
import styles from "./sidebar.module.css";
import {
  DROP_COLLECTION_ID_ATTR,
  DROP_KIND_ATTR,
  DROP_ZONE_ATTR,
  InsertionZone,
  TreeItemNode,
} from "./tree-item-node";

const LONG_PRESS_MS = 250;

function makeCollectionDragHandlers(
  collectionId: string,
  name: string,
  getSourceIndex: () => number,
  suppressRef: { suppress: boolean },
) {
  const handleMouseDown = (e: MouseEvent) => {
    if (e.button !== 0) return;
    const startX = e.clientX;
    const startY = e.clientY;

    const activate = (x: number, y: number) => {
      suppressRef.suppress = true;
      setDragItem({
        kind: "collection",
        collectionId,
        name,
        sourceIndex: getSourceIndex(),
      });
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

export function CollectionNode(props: {
  collection: Collection;
  sourceIndex: number;
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
    sourceCollectionId: string,
    itemId: string,
    targetCollectionId: string,
    targetParentId: string,
    position: number,
  ) => void;
  activeRequestId: string | null;
  renamingItemId: string | null;
  setRenamingItemId: (id: string | null) => void;
  renamingCollectionId: string | null;
  setRenamingCollectionId: (id: string | null) => void;
}) {
  const collectionsCtx = useHttpCollections();
  const expanded = () => collectionsCtx.isExpanded(props.collection.id, true);
  const toggleExpanded = () =>
    collectionsCtx.setExpanded(props.collection.id, !expanded());

  const isRenaming = () => props.renamingCollectionId === props.collection.id;

  const suppressRef = { suppress: false };
  const { handleMouseDown: handleCollectionMouseDown } =
    makeCollectionDragHandlers(
      props.collection.id,
      props.collection.name,
      () => props.sourceIndex,
      suppressRef,
    );

  const handleRenameCommit = (value: string) => {
    const trimmed = value.trim();
    if (trimmed && trimmed !== props.collection.name) {
      props.onRenameCollection(props.collection.id, trimmed);
    }
    props.setRenamingCollectionId(null);
  };

  return (
    <div class={styles.treeNode}>
      {/* biome-ignore lint/a11y/noStaticElementInteractions: drag source for collection reorder */}
      <div
        class={clsx(
          styles.treeNodeHeader,
          (() => {
            const dt = dropTarget();
            return (
              dt?.kind === "item" &&
              dt.collectionId === props.collection.id &&
              dt.position === -1
            );
          })() && styles.collectionHeaderDropTarget,
        )}
        style={{
          "pointer-events":
            dragItem()?.kind === "collection" ? "none" : undefined,
        }}
        onMouseDown={handleCollectionMouseDown}
        {...{
          [DROP_ZONE_ATTR]: "true",
          [DROP_KIND_ATTR]: "collection-header",
          [DROP_COLLECTION_ID_ATTR]: props.collection.id,
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
            onMouseDown={(e) => e.stopPropagation()}
            onClick={() => props.onAddFolder(props.collection.id, "")}
          >
            <FolderPlus size={12} aria-hidden="true" />
          </button>
          <button
            type="button"
            class={styles.treeActionBtn}
            aria-label="Add request"
            title="Add request"
            onMouseDown={(e) => e.stopPropagation()}
            onClick={() => props.onAddRequest(props.collection.id, "")}
          >
            <Plus size={12} aria-hidden="true" />
          </button>
          <button
            type="button"
            class={clsx(styles.treeActionBtn, styles.treeActionBtnDanger)}
            aria-label="Delete collection"
            title="Delete collection"
            onMouseDown={(e) => e.stopPropagation()}
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
