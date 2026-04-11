import { FolderOpen } from "lucide-solid";
import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { Button } from "../../../components/ui/button";
import { ScrollArea } from "../../../components/ui/scroll-area";
import { useOpenApiFiles } from "../../providers/openapi-provider";
import {
  OPENAPI_DROP_INDEX_ATTR,
  OPENAPI_DROP_ZONE_ATTR,
  OpenApiFileNode,
  OpenApiInsertionZone,
} from "./openapi-file-node";
import styles from "./sidebar.module.css";

export function OpenApiFileTree() {
  const filesCtx = useOpenApiFiles();

  const [dragFileId, setDragFileId] = createSignal<string | null>(null);
  const [dropIndex, setDropIndex] = createSignal<number | null>(null);
  const [ghostPos, setGhostPos] = createSignal<{ x: number; y: number } | null>(
    null,
  );

  onMount(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!dragFileId()) return;
      setGhostPos({ x: e.clientX, y: e.clientY });

      const el = document.elementFromPoint(e.clientX, e.clientY);
      const zone = el?.closest(
        `[${OPENAPI_DROP_ZONE_ATTR}]`,
      ) as HTMLElement | null;
      if (zone) {
        const idx = parseInt(
          zone.getAttribute(OPENAPI_DROP_INDEX_ATTR) ?? "-1",
          10,
        );
        setDropIndex(idx >= 0 ? idx : null);
      } else {
        setDropIndex(null);
      }
    };

    const handleMouseUp = () => {
      const id = dragFileId();
      const idx = dropIndex();
      if (id !== null && idx !== null) {
        filesCtx.moveFile(id, idx);
      }
      setDragFileId(null);
      setDropIndex(null);
      setGhostPos(null);
    };

    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
    onCleanup(() => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    });
  });

  const sortedFiles = () =>
    [...filesCtx.files()].sort((a, b) => a.order - b.order);

  return (
    <div class={styles.collectionTree}>
      <div class={styles.collectionHeader}>
        <span class={styles.collectionTitle}>OpenAPI Files</span>
        <Button
          variant="ghost"
          size="icon"
          class={styles.collectionAction}
          onClick={() => filesCtx.openFile()}
          title="Open File"
        >
          <FolderOpen size={14} />
        </Button>
      </div>

      <ScrollArea class={styles.treeScroll}>
        <div class={styles.treeList}>
          <For each={sortedFiles()}>
            {(file, index) => (
              <>
                <OpenApiInsertionZone
                  index={index()}
                  isActive={dropIndex() === index()}
                  isDragging={dragFileId() !== null}
                />
                <OpenApiFileNode
                  file={file}
                  isActive={filesCtx.activeFileId() === file.id}
                  onSelect={(id) => filesCtx.selectFile(id)}
                  onRemove={(id) => filesCtx.removeFile(id)}
                  onDragStart={(id, x, y) => {
                    setDragFileId(id);
                    setGhostPos({ x, y });
                  }}
                />
              </>
            )}
          </For>
          <Show when={sortedFiles().length > 0}>
            <OpenApiInsertionZone
              index={sortedFiles().length}
              isActive={dropIndex() === sortedFiles().length}
              isDragging={dragFileId() !== null}
            />
          </Show>

          <Show when={sortedFiles().length === 0}>
            <p class={styles.emptyTree}>No files opened yet</p>
          </Show>
        </div>
      </ScrollArea>

      <Portal>
        <Show when={ghostPos()}>
          {(pos) => {
            const file = filesCtx.files().find((f) => f.id === dragFileId());
            return (
              <div
                class={styles.dragGhost}
                style={{
                  left: `${pos().x + 14}px`,
                  top: `${pos().y - 10}px`,
                }}
              >
                {file?.name}
              </div>
            );
          }}
        </Show>
      </Portal>
    </div>
  );
}
