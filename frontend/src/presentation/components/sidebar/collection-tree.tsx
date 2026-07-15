import { Plus } from "lucide-solid";
import {
  createEffect,
  createSignal,
  For,
  onCleanup,
  onMount,
  Show,
} from "solid-js";
import { Portal } from "solid-js/web";
import { runGuarded } from "../../../application/ui/guard";
import { Button } from "../../../components/ui/button";
import { ConfirmDialog } from "../../../components/ui/confirm-dialog";
import { ScrollArea } from "../../../components/ui/scroll-area";
import {
  DEFAULT_SETTINGS,
  ROOT_COLLECTION_ID,
} from "../../../domain/http/types";
import {
  useHttpCollections,
  useHttpRequest,
} from "../../providers/http-provider";
import { CollectionNode } from "./collection-node";
import { type DragItem, dragItem, ghostPos } from "./drag-state";
import styles from "./sidebar.module.css";
import { InsertionZone, TreeItemNode } from "./tree-item-node";
import { type TreeUiContextValue, TreeUiProvider } from "./tree-ui-context";
import { useTreeDragDrop } from "./use-tree-drag-drop";

export function CollectionTree() {
  const requestCtx = useHttpRequest();
  const collectionsCtx = useHttpCollections();
  const [addMenuOpen, setAddMenuOpen] = createSignal(false);
  let addMenuWrapperRef: HTMLDivElement | undefined;

  createEffect(() => {
    if (!addMenuOpen()) return;
    const handler = (e: MouseEvent) => {
      if (addMenuWrapperRef && !addMenuWrapperRef.contains(e.target as Node)) {
        setAddMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    onCleanup(() => document.removeEventListener("mousedown", handler));
  });
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

  // コレクションロード後にアクティブリクエストを復元する
  onMount(async () => {
    await collectionsCtx.refreshCollections();
    requestCtx.restoreActiveRequest();
  });

  const handleAddRootRequest = async () => {
    setAddMenuOpen(false);
    await runGuarded("Failed to add request", async () => {
      const item = await collectionsCtx.addRequest(ROOT_COLLECTION_ID, "", {
        id: "",
        name: "New Request",
        method: "GET",
        url: "",
        headers: [],
        params: [],
        body: { type: "none", contents: {} },
        auth: { type: "none", username: "", password: "", token: "" },
        settings: { ...DEFAULT_SETTINGS },
        doc: "",
      });
      if (item?.id) setRenamingItemId(item.id);
    });
  };

  const handleCreateCollection = async () => {
    await runGuarded("Failed to create collection", async () => {
      const collection =
        await collectionsCtx.createCollection("New Collection");
      if (collection?.id) {
        setRenamingCollectionId(collection.id);
      }
    });
  };

  const handleRenameCollection = async (id: string, newName: string) => {
    const trimmed = newName.trim();
    if (!trimmed) return;
    await runGuarded("Failed to rename collection", () =>
      collectionsCtx.renameCollection(id, trimmed),
    );
  };

  const handleDeleteCollection = async (id: string) => {
    await runGuarded("Failed to delete collection", () =>
      collectionsCtx.deleteCollection(id),
    );
  };

  const handleAddFolder = async (collectionId: string, parentId: string) => {
    await runGuarded("Failed to add folder", async () => {
      const item = await collectionsCtx.addFolder(
        collectionId,
        parentId,
        "New Folder",
      );
      if (item?.id) {
        setRenamingItemId(item.id);
      }
    });
  };

  const handleAddRequest = async (collectionId: string, parentId: string) => {
    await runGuarded("Failed to add request", async () => {
      const item = await collectionsCtx.addRequest(collectionId, parentId, {
        id: "",
        name: "New Request",
        method: "GET",
        url: "",
        headers: [],
        params: [],
        body: { type: "none", contents: {} },
        auth: { type: "none", username: "", password: "", token: "" },
        settings: { ...DEFAULT_SETTINGS },
        doc: "",
      });
      if (item?.id) {
        setRenamingItemId(item.id);
      }
    });
  };

  const handleDeleteItem = async (collectionId: string, itemId: string) => {
    await runGuarded("Failed to delete item", () =>
      collectionsCtx.deleteItem(collectionId, itemId),
    );
  };

  const handleRenameItem = async (
    collectionId: string,
    itemId: string,
    newName: string,
  ) => {
    const trimmed = newName.trim();
    if (!trimmed) return;
    await runGuarded("Failed to rename item", () =>
      collectionsCtx.renameItem(collectionId, itemId, trimmed),
    );
  };

  const handleDropToSidebar = async (di: DragItem, position: number) => {
    await runGuarded("Failed to move item", async () => {
      if (di.kind === "collection") {
        await collectionsCtx.moveSidebarEntry(
          "collection",
          di.collectionId,
          position,
        );
      } else {
        // アイテムドラッグ
        if (di.collectionId === ROOT_COLLECTION_ID) {
          // __root__ 内のアイテムをサイドバーゾーン間で並び替える
          await collectionsCtx.moveSidebarEntry("item", di.itemId, position);
        } else {
          // 通常コレクションから sidebar ゾーンへ（__root__ への移動）
          await collectionsCtx.moveItemToSidebar(
            di.collectionId,
            di.itemId,
            position,
          );
        }
      }
    });
  };

  const handleMoveItem = async (
    sourceCollectionId: string,
    itemId: string,
    targetCollectionId: string,
    targetParentId: string,
    position: number,
  ) => {
    await runGuarded("Failed to move item", () =>
      collectionsCtx.moveItem(
        sourceCollectionId,
        itemId,
        targetCollectionId,
        targetParentId,
        position,
      ),
    );
  };

  // ドラッグ&ドロップの document レベルハンドラを登録する（ハンドラ定義後に呼ぶ）。
  useTreeDragDrop({
    onMoveItem: handleMoveItem,
    onDropToSidebar: handleDropToSidebar,
  });

  // ツリー描画に必要な rename 状態とハンドラを束ねて配布する。
  const treeUi: TreeUiContextValue = {
    activeRequestId: requestCtx.activeRequestId,
    renamingItemId,
    setRenamingItemId,
    renamingCollectionId,
    setRenamingCollectionId,
    onAddFolder: handleAddFolder,
    onAddRequest: handleAddRequest,
    onDeleteItem: (collectionId, itemId, name, type) =>
      setDeletingItem({ collectionId, itemId, name, type }),
    onDeleteCollection: (id, name) =>
      setDeletingItem({
        collectionId: id,
        itemId: "",
        name,
        type: "collection",
      }),
    onSelectRequest: (item, collectionId) => {
      if (item.request) requestCtx.loadRequest(item.request, collectionId);
    },
    onRenameItem: handleRenameItem,
    onRenameCollection: handleRenameCollection,
  };

  return (
    <div class={styles.collectionTree}>
      <div class={styles.collectionHeader}>
        <span class={styles.collectionTitle}>Collections</span>
        <div class={styles.addMenuWrapper} ref={addMenuWrapperRef}>
          <Button
            variant="ghost"
            size="icon"
            class={styles.collectionAction}
            onClick={() => setAddMenuOpen((v) => !v)}
            aria-label="Add"
            title="Add"
          >
            <Plus size={14} aria-hidden="true" />
          </Button>
          <Show when={addMenuOpen()}>
            <div class={styles.addMenu}>
              <button
                type="button"
                class={styles.addMenuItem}
                onClick={handleAddRootRequest}
              >
                New Request
              </button>
              <button
                type="button"
                class={styles.addMenuItem}
                onClick={() => {
                  setAddMenuOpen(false);
                  handleCreateCollection();
                }}
              >
                New Collection
              </button>
            </div>
          </Show>
        </div>
      </div>

      <ScrollArea class={styles.treeScroll}>
        <div class={styles.treeList}>
          <TreeUiProvider value={treeUi}>
            <For each={collectionsCtx.sidebarLayout}>
              {(entry, index) => {
                if (entry.kind === "collection") {
                  const collection = () =>
                    collectionsCtx.collections.find((c) => c.id === entry.id);
                  return (
                    <Show when={collection()}>
                      {(col) => (
                        <>
                          <InsertionZone kind="sidebar" position={index()} />
                          <CollectionNode
                            collection={col()}
                            sourceIndex={index()}
                          />
                        </>
                      )}
                    </Show>
                  );
                }
                // kind === "item"
                const item = () =>
                  collectionsCtx.rootItems.find((i) => i.id === entry.id);
                return (
                  <Show when={item()}>
                    {(it) => (
                      <>
                        <InsertionZone kind="sidebar" position={index()} />
                        <TreeItemNode
                          item={it()}
                          collectionId={ROOT_COLLECTION_ID}
                          depth={0}
                          sourceParentId=""
                          sourceIndex={index()}
                        />
                      </>
                    )}
                  </Show>
                );
              }}
            </For>
            <InsertionZone
              kind="sidebar"
              position={collectionsCtx.sidebarLayout.length}
            />

            <Show when={collectionsCtx.sidebarLayout.length === 0}>
              <p class={styles.emptyTree}>No collections yet</p>
            </Show>
          </TreeUiProvider>
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
