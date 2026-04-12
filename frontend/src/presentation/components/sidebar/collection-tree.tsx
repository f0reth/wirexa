import { Plus } from "lucide-solid";
import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { Button } from "../../../components/ui/button";
import { ConfirmDialog } from "../../../components/ui/confirm-dialog";
import { ScrollArea } from "../../../components/ui/scroll-area";
import { DEFAULT_SETTINGS } from "../../../domain/http/types";
import {
  useHttpCollections,
  useHttpRequest,
} from "../../providers/http-provider";
import { CollectionNode } from "./collection-node";
import {
  dragItem,
  ghostPos,
  setDragItem,
  setDropTarget,
  setGhostPos,
} from "./drag-state";
import styles from "./sidebar.module.css";
import {
  DROP_COLLECTION_ID_ATTR,
  DROP_PARENT_ID_ATTR,
  DROP_POSITION_ATTR,
  DROP_ZONE_ATTR,
} from "./tree-item-node";

export function CollectionTree() {
  const requestCtx = useHttpRequest();
  const collectionsCtx = useHttpCollections();
  const [renamingItemId, setRenamingItemId] = createSignal<string | null>(null);
  const [renamingCollectionId, setRenamingCollectionId] = createSignal<
    string | null
  >(null);
  const [deletingItem, setDeletingItem] = createSignal<{
    collectionId: string;
    itemId: string;
    name: string;
    type: string;
  } | null>(null);

  onMount(() => {
    collectionsCtx.refreshCollections();

    const handleMouseMove = (e: MouseEvent) => {
      const di = dragItem();
      if (!di) return;
      setGhostPos({ x: e.clientX, y: e.clientY });

      // elementFromPoint で現在マウス下にあるドロップゾーンを検出する。
      // この方式は mouseenter/mouseleave より確実で、ドラッグ開始時に
      // すでにゾーン上にある場合も正しく反応する。
      const el = document.elementFromPoint(e.clientX, e.clientY);
      const zone = el?.closest(`[${DROP_ZONE_ATTR}]`) as HTMLElement | null;
      if (zone) {
        const collectionId = zone.getAttribute(DROP_COLLECTION_ID_ATTR) ?? "";
        const parentId = zone.getAttribute(DROP_PARENT_ID_ATTR) ?? "";
        const position = parseInt(
          zone.getAttribute(DROP_POSITION_ATTR) ?? "-1",
          10,
        );
        // ソースフォルダのヘッダー（position=-1）は no-op になるため除外する。
        // これにより、上方向にドラッグした際にソースフォルダのヘッダーを
        // 通過しても元フォルダ末尾への誤ドロップが発生しなくなる。
        if (
          collectionId === di.collectionId &&
          parentId === di.sourceParentId &&
          position === -1
        ) {
          setDropTarget(null);
        } else {
          setDropTarget({ collectionId, parentId, position });
        }
      } else {
        setDropTarget(null);
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
          const collectionId = zone.getAttribute(DROP_COLLECTION_ID_ATTR) ?? "";
          const parentId = zone.getAttribute(DROP_PARENT_ID_ATTR) ?? "";
          const position = parseInt(
            zone.getAttribute(DROP_POSITION_ATTR) ?? "-1",
            10,
          );
          // ソースフォルダのヘッダーへのドロップは no-op なので除外する。
          if (
            !(
              collectionId === di.collectionId &&
              parentId === di.sourceParentId &&
              position === -1
            )
          ) {
            handleMoveItem(di.collectionId, di.itemId, parentId, position);
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

  const handleCreateCollection = async () => {
    try {
      const collection =
        await collectionsCtx.createCollection("New Collection");
      if (collection?.id) {
        setRenamingCollectionId(collection.id);
      }
    } catch (err) {
      console.error("Failed to create collection:", err);
    }
  };

  const handleRenameCollection = async (id: string, newName: string) => {
    const trimmed = newName.trim();
    if (!trimmed) return;
    try {
      await collectionsCtx.renameCollection(id, trimmed);
    } catch (err) {
      console.error("Failed to rename collection:", err);
    }
  };

  const handleDeleteCollection = async (id: string) => {
    try {
      await collectionsCtx.deleteCollection(id);
    } catch (err) {
      console.error("Failed to delete collection:", err);
    }
  };

  const handleAddFolder = async (collectionId: string, parentId: string) => {
    try {
      const item = await collectionsCtx.addFolder(
        collectionId,
        parentId,
        "New Folder",
      );
      if (item?.id) {
        setRenamingItemId(item.id);
      }
    } catch (err) {
      console.error("Failed to add folder:", err);
    }
  };

  const handleAddRequest = async (collectionId: string, parentId: string) => {
    try {
      const item = await collectionsCtx.addRequest(collectionId, parentId, {
        id: "",
        name: "New Request",
        method: "GET",
        url: "",
        headers: [],
        params: [],
        body: { type: "none", content: "" },
        auth: { type: "none", username: "", password: "", token: "" },
        settings: { ...DEFAULT_SETTINGS },
      });
      if (item?.id) {
        setRenamingItemId(item.id);
      }
    } catch (err) {
      console.error("Failed to add request:", err);
    }
  };

  const handleDeleteItem = async (collectionId: string, itemId: string) => {
    try {
      await collectionsCtx.deleteItem(collectionId, itemId);
    } catch (err) {
      console.error("Failed to delete item:", err);
    }
  };

  const handleRenameItem = async (
    collectionId: string,
    itemId: string,
    newName: string,
  ) => {
    const trimmed = newName.trim();
    if (!trimmed) return;
    try {
      await collectionsCtx.renameItem(collectionId, itemId, trimmed);
    } catch (err) {
      console.error("Failed to rename item:", err);
    }
  };

  const handleMoveItem = async (
    collectionId: string,
    itemId: string,
    targetParentId: string,
    position: number,
  ) => {
    try {
      await collectionsCtx.moveItem(
        collectionId,
        itemId,
        targetParentId,
        position,
      );
    } catch (err) {
      console.error("Failed to move item:", err);
    }
  };

  return (
    <div class={styles.collectionTree}>
      <div class={styles.collectionHeader}>
        <span class={styles.collectionTitle}>Collections</span>
        <Button
          variant="ghost"
          size="icon"
          class={styles.collectionAction}
          onClick={() => handleCreateCollection()}
          title="New Collection"
        >
          <Plus size={14} />
        </Button>
      </div>

      <ScrollArea class={styles.treeScroll}>
        <div class={styles.treeList}>
          <For each={collectionsCtx.collections}>
            {(collection) => (
              <CollectionNode
                collection={collection}
                onDeleteCollection={(id, name) =>
                  setDeletingItem({
                    collectionId: id,
                    itemId: "",
                    name,
                    type: "collection",
                  })
                }
                onAddFolder={handleAddFolder}
                onAddRequest={handleAddRequest}
                onDeleteItem={(collectionId, itemId, name, type) =>
                  setDeletingItem({ collectionId, itemId, name, type })
                }
                onSelectRequest={(item) => {
                  if (item.request) {
                    requestCtx.loadRequest(item.request, collection.id);
                  }
                }}
                onRenameItem={handleRenameItem}
                onRenameCollection={handleRenameCollection}
                onMoveItem={handleMoveItem}
                activeRequestId={requestCtx.activeRequestId()}
                renamingItemId={renamingItemId()}
                setRenamingItemId={setRenamingItemId}
                renamingCollectionId={renamingCollectionId()}
                setRenamingCollectionId={setRenamingCollectionId}
              />
            )}
          </For>

          <Show when={collectionsCtx.collections.length === 0}>
            <p class={styles.emptyTree}>No collections yet</p>
          </Show>
        </div>
      </ScrollArea>

      <Show when={deletingItem()}>
        {(item) => (
          <Portal>
            <ConfirmDialog
              title={`Delete ${item().type}`}
              message={`Are you sure you want to delete "${item().name}"? This action cannot be undone.`}
              onConfirm={async () => {
                const d = item();
                if (d.type === "collection") {
                  await handleDeleteCollection(d.collectionId);
                } else {
                  await handleDeleteItem(d.collectionId, d.itemId);
                }
                setDeletingItem(null);
              }}
              onCancel={() => setDeletingItem(null)}
            />
          </Portal>
        )}
      </Show>

      {/* ドラッグ中のゴースト要素 */}
      <Portal>
        <Show when={dragItem() && ghostPos()}>
          <div
            class={styles.dragGhost}
            style={{
              left: `${(ghostPos()?.x ?? 0) + 14}px`,
              top: `${(ghostPos()?.y ?? 0) - 10}px`,
            }}
          >
            {dragItem()?.name}
          </div>
        </Show>
      </Portal>
    </div>
  );
}
