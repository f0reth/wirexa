import type { Extension } from "@codemirror/state";
import { EditorView } from "codemirror";
import { createEffect, onCleanup, onMount } from "solid-js";

interface CodeMirrorConfig {
  /** エディタに表示する値。外部から変わると同期される。 */
  value: () => string;
  /** ドキュメント変更時のコールバック。 */
  onChange: (value: string) => void;
  /** 言語・ハイライト・linter などコンポーネント固有の拡張。 */
  extensions: Extension[];
}

/**
 * CodeMirror エディタのライフサイクルを管理するフック。
 *
 * `onMount` で `EditorView` を生成し、`onCleanup` で破棄する。ドキュメント変更を
 * `onChange` に伝える updateListener と共通テーマ、そして外部値を取り込む同期
 * `createEffect` を内包する。差分となる拡張は `extensions` で渡す。
 *
 * 返り値をコンテナ要素の `ref` に割り当てて使う:
 *   `<div ref={createCodeMirror({ value, onChange, extensions })} class={...} />`
 */
export function createCodeMirror(config: CodeMirrorConfig) {
  let view: EditorView | undefined;
  let container: HTMLDivElement | undefined;

  const ref = (el: HTMLDivElement) => {
    container = el;
  };

  onMount(() => {
    view = new EditorView({
      doc: config.value(),
      extensions: [
        ...config.extensions,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            config.onChange(update.state.doc.toString());
          }
        }),
        EditorView.theme({
          "&": { height: "100%" },
          ".cm-scroller": { overflow: "auto" },
        }),
      ],
      // biome-ignore lint/style/noNonNullAssertion: ref is always set before onMount fires
      parent: container!,
    });

    onCleanup(() => {
      view?.destroy();
      view = undefined;
    });
  });

  // 外部から値が変わったとき（保存済みデータの読み込み等）をエディタへ同期する
  createEffect(() => {
    const content = config.value();
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== content) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: content },
      });
    }
  });

  return ref;
}
