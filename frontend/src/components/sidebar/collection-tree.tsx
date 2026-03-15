import { Plus } from "lucide-solid";
import { createSignal, For, onMount, Show } from "solid-js";
import { Portal } from "solid-js/web";
import {
  AddFolder,
  AddRequest,
  CreateCollection,
  DeleteCollection,
  DeleteItem,
  RenameCollection,
  RenameItem,
} from "../../../wailsjs/go/http/HttpService";
import { useHttp } from "../http/context";
import { Button } from "../ui/button";
import { ConfirmDialog } from "../ui/confirm-dialog";
import { ScrollArea } from "../ui/scroll-area";
import { CollectionNode } from "./collection-node";
import styles from "./sidebar.module.css";

export function CollectionTree() {
  const ctx = useHttp();
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
    ctx.refreshCollections();
  });

  // Shared refresh helper — single point of API call after mutations
  const refresh = () => ctx.refreshCollections();

  const handleCreateCollection = async () => {
    try {
      const collection = await CreateCollection("New Collection");
      await refresh();
      if (collection?.id) {
        setRenamingCollectionId(collection.id);
      }
    } catch {
      // ignore
    }
  };

  const handleRenameCollection = async (id: string, newName: string) => {
    const trimmed = newName.trim();
    if (!trimmed) return;
    try {
      await RenameCollection(id, trimmed);
      refresh();
    } catch {
      // ignore
    }
  };

  const handleDeleteCollection = async (id: string) => {
    try {
      await DeleteCollection(id);
      refresh();
    } catch {
      // ignore
    }
  };

  const handleAddFolder = async (collectionId: string, parentId: string) => {
    try {
      const item = await AddFolder(collectionId, parentId, "New Folder");
      await refresh();
      if (item?.id) {
        setRenamingItemId(item.id);
      }
    } catch {
      // ignore
    }
  };

  const handleAddRequest = async (collectionId: string, parentId: string) => {
    try {
      const item = await AddRequest(collectionId, parentId, {
        id: "",
        name: "New Request",
        method: "GET",
        url: "",
        headers: [],
        params: [],
        body: { type: "none", content: "" },
        // biome-ignore lint/suspicious/noExplicitAny: Wails generated types differ from local interfaces
      } as any);
      await refresh();
      if (item?.id) {
        setRenamingItemId(item.id);
      }
    } catch {
      // ignore
    }
  };

  const handleDeleteItem = async (collectionId: string, itemId: string) => {
    try {
      await DeleteItem(collectionId, itemId);
      refresh();
    } catch {
      // ignore
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
      await RenameItem(collectionId, itemId, trimmed);
      refresh();
    } catch {
      // ignore
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
          <For each={ctx.collections()}>
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
                onSelectRequest={(item) => ctx.loadRequest(collection.id, item)}
                onRenameItem={handleRenameItem}
                onRenameCollection={handleRenameCollection}
                activeRequestId={ctx.activeRequestId()}
                dirtyRequestId={ctx.dirty() ? ctx.activeRequestId() : null}
                renamingItemId={renamingItemId()}
                setRenamingItemId={setRenamingItemId}
                renamingCollectionId={renamingCollectionId()}
                setRenamingCollectionId={setRenamingCollectionId}
              />
            )}
          </For>

          <Show when={ctx.collections().length === 0}>
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
    </div>
  );
}
