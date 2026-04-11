import { clsx } from "clsx";
import { FileCode, Trash2 } from "lucide-solid";
import type { OpenApiFile } from "../../../domain/openapi/types";
import styles from "./sidebar.module.css";

const LONG_PRESS_MS = 250;

export const OPENAPI_DROP_ZONE_ATTR = "data-openapi-drop-zone";
export const OPENAPI_DROP_INDEX_ATTR = "data-openapi-drop-index";

export function OpenApiInsertionZone(props: {
  index: number;
  isActive: boolean;
  isDragging: boolean;
}) {
  return (
    <div
      class={clsx(
        styles.insertionZone,
        props.isDragging && styles.insertionZoneVisible,
        props.isActive && styles.insertionZoneActive,
      )}
      {...{
        [OPENAPI_DROP_ZONE_ATTR]: "true",
        [OPENAPI_DROP_INDEX_ATTR]: String(props.index),
      }}
    />
  );
}

export function makeDragHandlers(
  fileId: string,
  suppressRef: { suppress: boolean },
  onDragStart: (id: string, x: number, y: number) => void,
) {
  const handleMouseDown = (e: MouseEvent) => {
    if (e.button !== 0) return;
    const startX = e.clientX;
    const startY = e.clientY;

    const activate = (x: number, y: number) => {
      suppressRef.suppress = true;
      onDragStart(fileId, x, y);
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

export function OpenApiFileNode(props: {
  file: OpenApiFile;
  isActive: boolean;
  onSelect: (id: string) => void;
  onRemove: (id: string) => void;
  onDragStart: (id: string, x: number, y: number) => void;
}) {
  const suppressRef = { suppress: false };
  const { handleMouseDown } = makeDragHandlers(
    props.file.id,
    suppressRef,
    props.onDragStart,
  );

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: drag source
    <div
      class={clsx(
        styles.requestRow,
        props.isActive && styles.requestItemActive,
      )}
      onMouseDown={handleMouseDown}
    >
      <button
        type="button"
        class={styles.requestSelectBtn}
        onClick={() => {
          if (suppressRef.suppress) {
            suppressRef.suppress = false;
            return;
          }
          props.onSelect(props.file.id);
        }}
      >
        <FileCode size={12} style={{ "flex-shrink": "0" }} />
        <span class={styles.requestName}>{props.file.name}</span>
      </button>
      <button
        type="button"
        class={clsx(styles.treeActionBtn, styles.treeActionBtnDanger)}
        onMouseDown={(e) => e.stopPropagation()}
        onClick={() => props.onRemove(props.file.id)}
        title="Remove from history"
      >
        <Trash2 size={10} />
      </button>
    </div>
  );
}
