import { markdown } from "@codemirror/lang-markdown";
import {
  defaultHighlightStyle,
  syntaxHighlighting,
} from "@codemirror/language";
import { EditorView, minimalSetup } from "codemirror";
import { createEffect, onCleanup, onMount } from "solid-js";
import styles from "./http.module.css";

interface Props {
  value: string;
  onChange: (value: string) => void;
}

export function DocEditor(props: Props) {
  let containerRef: HTMLDivElement | undefined;
  let view: EditorView | undefined;

  onMount(() => {
    view = new EditorView({
      doc: props.value,
      extensions: [
        minimalSetup,
        syntaxHighlighting(defaultHighlightStyle),
        markdown(),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            props.onChange(update.state.doc.toString());
          }
        }),
        EditorView.theme({
          "&": { height: "100%" },
          ".cm-scroller": { overflow: "auto" },
        }),
      ],
      // biome-ignore lint/style/noNonNullAssertion: ref is always set before onMount fires
      parent: containerRef!,
    });

    onCleanup(() => {
      view?.destroy();
      view = undefined;
    });
  });

  createEffect(() => {
    const content = props.value;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== content) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: content },
      });
    }
  });

  return <div ref={containerRef} class={styles.docEditor} />;
}
