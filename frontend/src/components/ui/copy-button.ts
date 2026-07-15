import { createSignal, onCleanup } from "solid-js";

/** 単一ボタン用の内部センチネルキー。 */
const SINGLE = Symbol("copy-single");

/**
 * コピー成功時に一定時間チェックマークを表示するための状態を管理するフック。
 *
 * 単一ボタン（キー省略）と、リスト内の複数ボタン（各アイテムのキーを渡す）の
 * 両方に対応する。`onCleanup` でタイマーを確実に破棄するため、コンポーネント破棄時の
 * タイマーリークが起きない。
 *
 * @example 単一ボタン
 *   const { copy, isCopied } = createCopyButton();
 *   copy(text);           isCopied();
 * @example リスト（timestamp をキーに）
 *   const { copy, isCopied } = createCopyButton<number>();
 *   copy(payload, ts);    isCopied(ts);
 */
export function createCopyButton<K = symbol>(resetMs = 1500) {
  const [copiedKey, setCopiedKey] = createSignal<K | symbol | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => clearTimeout(timer));

  const copy = (text: string, key: K | symbol = SINGLE) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(() => key);
    clearTimeout(timer);
    timer = setTimeout(() => setCopiedKey(null), resetMs);
  };

  const isCopied = (key: K | symbol = SINGLE) => copiedKey() === key;

  return { copy, isCopied };
}
