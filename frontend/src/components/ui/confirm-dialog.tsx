import { onMount } from "solid-js";
import { Button } from "./button";
import styles from "./confirm-dialog.module.css";

export function ConfirmDialog(props: {
  title: string;
  message: string;
  confirmLabel?: string;
  variant?: "destructive" | "default";
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Escape") props.onCancel();
  };

  onMount(() => {
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  });

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: Escape key handled via document listener
    // biome-ignore lint/a11y/noStaticElementInteractions: dialog overlay backdrop
    <div class={styles.overlay} onClick={() => props.onCancel()}>
      <div
        class={styles.card}
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => e.stopPropagation()}
      >
        <h3 class={styles.title}>{props.title}</h3>
        <p class={styles.message}>{props.message}</p>
        <div class={styles.actions}>
          <Button variant="outline" size="sm" onClick={() => props.onCancel()}>
            Cancel
          </Button>
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
