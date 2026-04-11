import { json } from "@codemirror/lang-json";
import { yaml } from "@codemirror/lang-yaml";
import {
  defaultHighlightStyle,
  syntaxHighlighting,
} from "@codemirror/language";
import { type Diagnostic, linter } from "@codemirror/lint";
import { basicSetup, EditorView } from "codemirror";
import { createEffect, onCleanup, onMount } from "solid-js";
import {
  useOpenApiEditor,
  useOpenApiFiles,
} from "../../providers/openapi-provider";
import styles from "./openapi.module.css";

function getLangExtension(filename: string) {
  if (filename.endsWith(".json")) return json();
  return yaml();
}

export function EditorPanel() {
  let containerRef: HTMLDivElement | undefined;
  let view: EditorView | undefined;

  const editorCtx = useOpenApiEditor();
  const filesCtx = useOpenApiFiles();

  // Build a linter extension that reads current parseErrors signal
  const linterExtension = linter(() => editorCtx.parseErrors() as Diagnostic[]);

  onMount(() => {
    const activeFile = filesCtx.getActiveFile();
    const lang = getLangExtension(activeFile?.name ?? "spec.yaml");

    view = new EditorView({
      doc: editorCtx.editorContent(),
      extensions: [
        basicSetup,
        syntaxHighlighting(defaultHighlightStyle),
        lang,
        linterExtension,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            editorCtx.onContentChange(update.state.doc.toString());
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

  // When a new file is loaded from outside (e.g. sidebar click), sync editor content
  createEffect(() => {
    const content = editorCtx.editorContent();
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== content) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: content },
      });
    }
  });

  return <div ref={containerRef} class={styles.codeMirrorWrap} />;
}
