import { json } from "@codemirror/lang-json";
import { yaml } from "@codemirror/lang-yaml";
import {
  defaultHighlightStyle,
  syntaxHighlighting,
} from "@codemirror/language";
import { type Diagnostic, linter } from "@codemirror/lint";
import { basicSetup } from "codemirror";
import {
  useOpenApiEditor,
  useOpenApiFiles,
} from "../../providers/openapi-provider";
import { createCodeMirror } from "../shared/codemirror";
import { gutterTheme } from "../shared/editor-theme";
import styles from "./openapi.module.css";

function getLangExtension(filename: string) {
  if (filename.endsWith(".json")) return json();
  return yaml();
}

export function EditorPanel() {
  const editorCtx = useOpenApiEditor();
  const filesCtx = useOpenApiFiles();

  const lang = getLangExtension(filesCtx.activeDoc()?.name ?? "spec.yaml");
  // Build a linter extension that reads current parseErrors signal
  const linterExtension = linter(() => editorCtx.parseErrors() as Diagnostic[]);

  const ref = createCodeMirror({
    value: () => editorCtx.editorContent(),
    onChange: (v) => editorCtx.onContentChange(v),
    extensions: [
      basicSetup,
      syntaxHighlighting(defaultHighlightStyle),
      lang,
      linterExtension,
      gutterTheme,
    ],
  });

  return <div ref={ref} class={styles.codeMirrorWrap} />;
}
