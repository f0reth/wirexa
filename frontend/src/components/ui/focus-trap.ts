import { onMount } from "solid-js";

/**
 * モーダルダイアログ用のフォーカストラップ。
 *
 * Tab / Shift+Tab をダイアログ内の focusable 要素で循環させ、Escape で `onEscape`
 * を呼ぶ `onKeyDown` を返す。マウント時に先頭の focusable へ初期フォーカスする。
 *
 * `ref` はカード要素を返すゲッター（`() => cardRef`）。カード要素の `onKeyDown` に
 * 返り値の `onKeyDown` を割り当てて使う。
 */
export function createFocusTrap(
  ref: () => HTMLElement | undefined,
  onEscape: () => void,
) {
  const getFocusable = () =>
    Array.from(
      ref()?.querySelectorAll<HTMLElement>(
        "input:not([disabled]), select:not([disabled]), button:not([disabled]), textarea:not([disabled])",
      ) ?? [],
    );

  const onKeyDown = (e: KeyboardEvent) => {
    e.stopPropagation();
    if (e.key === "Escape") {
      onEscape();
      return;
    }
    if (e.key === "Tab") {
      const focusable = getFocusable();
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
  };

  onMount(() => {
    getFocusable()[0]?.focus();
  });

  return { onKeyDown };
}
