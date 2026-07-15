import { Show } from "solid-js";
import { Button } from "./button";
import styles from "./confirm-dialog.module.css";
import { createFocusTrap } from "./focus-trap";

export function ConfirmDialog(props: {
  title: string;
  message: string;
  confirmLabel?: string;
  variant?: "destructive" | "default";
  onConfirm: () => void;
  onCancel: () => void;
  // 任意の第 3 ボタン（例: 「破棄して切替」）。指定時のみ描画される。
  secondaryLabel?: string;
  secondaryVariant?: "destructive" | "default" | "outline";
  onSecondary?: () => void;
}) {
  let cardRef: HTMLDivElement | undefined;

  const { onKeyDown } = createFocusTrap(
    () => cardRef,
    () => props.onCancel(),
  );

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: keyboard handling (Escape, Tab) managed in dialog card
    // biome-ignore lint/a11y/noStaticElementInteractions: dialog overlay backdrop
    <div class={styles.overlay} onClick={() => props.onCancel()}>
      <div
        ref={cardRef}
        class={styles.card}
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={onKeyDown}
      >
        <h3 class={styles.title}>{props.title}</h3>
        <p class={styles.message}>{props.message}</p>
        <div class={styles.actions}>
          <Button variant="outline" size="sm" onClick={() => props.onCancel()}>
            Cancel
          </Button>
          <Show when={props.secondaryLabel}>
            <Button
              variant={props.secondaryVariant ?? "destructive"}
              size="sm"
              onClick={() => props.onSecondary?.()}
            >
              {props.secondaryLabel}
            </Button>
          </Show>
          <Button
            variant={props.variant ?? "destructive"}
            size="sm"
            onClick={() => props.onConfirm()}
          >
            {props.confirmLabel ?? "Delete"}
          </Button>
        </div>
      </div>
    </div>
  );
}
