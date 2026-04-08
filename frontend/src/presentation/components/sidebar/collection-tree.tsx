import { Plus } from "lucide-solid";
import { createSignal, For, onMount, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { Button } from "../../../components/ui/button";
import { ConfirmDialog } from "../../../components/ui/confirm-dialog";
import { ScrollArea } from "../../../components/ui/scroll-area";
import {
  useHttpCollections,
  useHttpRequest,
} from "../../providers/http-provider";
import { CollectionNode } from "./collection-node";
import styles from "./sidebar.module.css";

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
        settings: {
          timeoutSec: 0,
          proxyMode: "system" as const,
          proxyURL: "",
          insecureSkipVerify: false,
          disableRedirects: false,
          maxResponseBodyMB: 0,
        },
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
                activeRequestId={requestCtx.activeRequestId()}
                dirtyRequestId={
                  requestCtx.dirty() ? requestCtx.activeRequestId() : null
                }
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
    </div>
  );
}
