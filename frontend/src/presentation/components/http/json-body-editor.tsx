import { json, jsonParseLinter } from "@codemirror/lang-json";
import {
  defaultHighlightStyle,
  syntaxHighlighting,
} from "@codemirror/language";
import { linter } from "@codemirror/lint";
import { basicSetup } from "codemirror";
import { createCodeMirror } from "../shared/codemirror";
import { gutterTheme } from "../shared/editor-theme";
import styles from "./http.module.css";

interface Props {
  value: string;
  onChange: (value: string) => void;
}

export function JsonBodyEditor(props: Props) {
  const ref = createCodeMirror({
    value: () => props.value,
    onChange: (v) => props.onChange(v),
    extensions: [
      basicSetup,
      syntaxHighlighting(defaultHighlightStyle),
      json(),
      linter(jsonParseLinter()),
      gutterTheme,
    ],
  });

  return <div ref={ref} class={styles.jsonBodyEditor} />;
}
