import { clsx } from "clsx";
import { FileCode, Trash2 } from "lucide-solid";
import reorderStyles from "../../../components/ui/list-reorder.module.css";
import type { OpenApiFile } from "../../../domain/openapi/types";
import styles from "./sidebar.module.css";
import { makeLongPressDragHandlers } from "./use-long-press-drag";

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
        reorderStyles.insertionZone,
        props.isDragging && reorderStyles.insertionZoneVisible,
        props.isActive && reorderStyles.insertionZoneActive,
      )}
      {...{
        [OPENAPI_DROP_ZONE_ATTR]: "true",
        [OPENAPI_DROP_INDEX_ATTR]: String(props.index),
      }}
    />
  );
}

export function OpenApiFileNode(props: {
  file: OpenApiFile;
  isActive: boolean;
  onSelect: (path: string) => void;
  onRemove: (path: string) => void;
  onDragStart: (path: string, x: number, y: number) => void;
}) {
  const suppressRef = { suppress: false };
  const { handleMouseDown } = makeLongPressDragHandlers(suppressRef, (x, y) =>
    props.onDragStart(props.file.path, x, y),
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
          props.onSelect(props.file.path);
        }}
      >
        <FileCode size={12} style={{ "flex-shrink": "0" }} />
        <span class={styles.requestName}>{props.file.name}</span>
      </button>
      <button
        type="button"
        class={clsx(styles.treeActionBtn, styles.treeActionBtnDanger)}
        onMouseDown={(e) => e.stopPropagation()}
        onClick={() => props.onRemove(props.file.path)}
        title="Remove from history"
      >
        <Trash2 size={10} />
      </button>
    </div>
  );
}
