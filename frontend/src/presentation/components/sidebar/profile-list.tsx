import { clsx } from "clsx";
import { GripVertical, Plus, Settings, Trash2 } from "lucide-solid";
import { createSignal, For, type JSX, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { Button } from "../../../components/ui/button";
import { ConfirmDialog } from "../../../components/ui/confirm-dialog";
import {
  createListReorder,
  InsertionZone,
} from "../../../components/ui/list-reorder";
import reorderStyles from "../../../components/ui/list-reorder.module.css";
import { ScrollArea } from "../../../components/ui/scroll-area";
import styles from "./sidebar.module.css";

interface ProfileListProps<T extends { id: string; name: string }> {
  /** サイドバー見出し（例: "Brokers" / "Targets"）。 */
  title: string;
  /** 新規追加ボタンの aria-label / title。 */
  addLabel: string;
  /** 一覧が空のときの表示文言。 */
  emptyMessage: string;
  items: T[];
  onAdd: () => void;
  onItemClick: (item: T) => void;
  onEdit: (item: T) => void;
  onDelete: (item: T) => void;
  onReorder: (from: number, to: number) => void;
  /** 未指定ならアクティブ表示なし。 */
  isActive?: (item: T) => boolean;
  /** グリップと操作ボタンの間に描画する本体（接続ドット + 情報など）。 */
  renderContent: (item: T) => JSX.Element;
  editAriaLabel: string;
  deleteAriaLabel: string;
  deleteTitle: string;
}

/**
 * ブローカー / ターゲットなど「並べ替え可能な単純リスト + 編集/削除 + 確認ダイアログ」の
 * 共通スケルトン。相違点（本体描画・クリック挙動・アクティブ判定）は props スロットで受ける。
 * 編集ダイアログは実装差が大きいため含めず、親が onEdit/onAdd を受けて自前で描画する。
 */
export function ProfileList<T extends { id: string; name: string }>(
  props: ProfileListProps<T>,
) {
  const [deletingItem, setDeletingItem] = createSignal<T | null>(null);
  const { draggingIndex, dropIndicatorIndex, itemHandlers } = createListReorder(
    props.onReorder,
  );

  return (
    <div class={styles.collectionTree}>
      <div class={styles.collectionHeader}>
        <span class={styles.collectionTitle}>{props.title}</span>
        <Button
          variant="ghost"
          size="icon"
          class={styles.collectionAction}
          onClick={() => props.onAdd()}
          aria-label={props.addLabel}
          title={props.addLabel}
        >
          <Plus size={14} aria-hidden="true" />
        </Button>
      </div>

      <ScrollArea class={styles.treeScroll}>
        <div class={styles.treeList}>
          <For each={props.items}>
            {(item, index) => (
              <>
                <InsertionZone
                  visible={draggingIndex() !== null}
                  active={dropIndicatorIndex() === index()}
                />
                {/* biome-ignore lint/a11y/useSemanticElements: contains nested action buttons; cannot use <button> with nested interactive elements */}
                <div
                  role="button"
                  tabIndex={0}
                  class={clsx(
                    styles.listItem,
                    props.isActive?.(item) && styles.listItemActive,
                    draggingIndex() === index() && styles.listItemDragging,
                  )}
                  {...itemHandlers(index)}
                  onClick={() => {
                    if (draggingIndex() !== null) return;
                    props.onItemClick(item);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ")
                      props.onItemClick(item);
                  }}
                >
                  <GripVertical
                    size={12}
                    class={reorderStyles.dragHandle}
                    aria-hidden="true"
                  />
                  {props.renderContent(item)}
                  <div class={styles.treeNodeActions}>
                    <button
                      type="button"
                      class={styles.treeActionBtn}
                      aria-label={props.editAriaLabel}
                      title="Edit"
                      onClick={(e) => {
                        e.stopPropagation();
                        props.onEdit(item);
                      }}
                    >
                      <Settings size={12} aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      class={clsx(
                        styles.treeActionBtn,
                        styles.treeActionBtnDanger,
                      )}
                      aria-label={props.deleteAriaLabel}
                      title="Delete"
                      onClick={(e) => {
                        e.stopPropagation();
                        setDeletingItem(() => item);
                      }}
                    >
                      <Trash2 size={12} aria-hidden="true" />
                    </button>
                  </div>
                </div>
              </>
            )}
          </For>
          <InsertionZone
            visible={draggingIndex() !== null}
            active={dropIndicatorIndex() === props.items.length}
          />

          <Show when={props.items.length === 0}>
            <p class={styles.emptyTree}>{props.emptyMessage}</p>
          </Show>
        </div>
      </ScrollArea>

      <Show when={deletingItem()}>
        {(item) => (
          <Portal>
            <ConfirmDialog
              title={props.deleteTitle}
              message={`Are you sure you want to delete "${item().name}"? This action cannot be undone.`}
              onConfirm={() => {
                props.onDelete(item());
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
