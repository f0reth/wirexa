import { clsx } from "clsx";
import { ChevronRight, Folder, FolderPlus, Plus, Trash2 } from "lucide-solid";
import { For, Show } from "solid-js";
import type { Collection } from "../../../domain/http/types";
import { useHttpCollections } from "../../providers/http-provider";
import { dragItem, dropTarget, setDragItem, setGhostPos } from "./drag-state";
import { RenameInput } from "./rename-input";
import styles from "./sidebar.module.css";
import {
  DROP_COLLECTION_ID_ATTR,
  DROP_KIND_ATTR,
  DROP_ZONE_ATTR,
  InsertionZone,
  TreeItemNode,
} from "./tree-item-node";
import { useTreeUi } from "./tree-ui-context";
import { makeLongPressDragHandlers } from "./use-long-press-drag";

export function CollectionNode(props: {
  collection: Collection;
  sourceIndex: number;
}) {
  const ui = useTreeUi();
  const collectionsCtx = useHttpCollections();
  const expanded = () => collectionsCtx.isExpanded(props.collection.id, true);
  const toggleExpanded = () =>
    collectionsCtx.setExpanded(props.collection.id, !expanded());

  const isRenaming = () => ui.renamingCollectionId() === props.collection.id;

  const suppressRef = { suppress: false };
  const { handleMouseDown: handleCollectionMouseDown } =
    makeLongPressDragHandlers(suppressRef, (x, y) => {
      setDragItem({
        kind: "collection",
        collectionId: props.collection.id,
        name: props.collection.name,
        sourceIndex: props.sourceIndex,
      });
      setGhostPos({ x, y });
    });

  const handleRenameCommit = (value: string) => {
    if (!isRenaming()) return;
    const trimmed = value.trim();
    if (trimmed && trimmed !== props.collection.name) {
      ui.onRenameCollection(props.collection.id, trimmed);
    }
    ui.setRenamingCollectionId(null);
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
                    ui.setRenamingCollectionId(props.collection.id);
                  }}
                >
                  {props.collection.name}
                </span>
              </>
            }
          >
            <RenameInput
              value={props.collection.name}
              onCommit={handleRenameCommit}
              onCancel={() => ui.setRenamingCollectionId(null)}
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
            onClick={() => ui.onAddFolder(props.collection.id, "")}
          >
            <FolderPlus size={12} aria-hidden="true" />
          </button>
          <button
            type="button"
            class={styles.treeActionBtn}
            aria-label="Add request"
            title="Add request"
            onMouseDown={(e) => e.stopPropagation()}
            onClick={() => ui.onAddRequest(props.collection.id, "")}
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
              ui.onDeleteCollection(props.collection.id, props.collection.name)
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
