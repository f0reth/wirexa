import styles from "./sidebar.module.css";

/**
 * ツリーアイテムのインラインリネーム用 input。
 * マウント時に focus + select し、Enter で確定 / Escape で取消 / blur で確定する。
 */
export function RenameInput(props: {
  value: string;
  onCommit: (value: string) => void;
  onCancel: () => void;
}) {
  return (
    <input
      data-testid="rename-input"
      class={styles.renameInput}
      ref={(el) => {
        el.value = props.value;
        requestAnimationFrame(() => {
          el.focus();
          el.select();
        });
      }}
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => {
        if (e.key === "Enter") props.onCommit(e.currentTarget.value);
        if (e.key === "Escape") props.onCancel();
      }}
      onBlur={(e) => props.onCommit(e.currentTarget.value)}
    />
  );
}
