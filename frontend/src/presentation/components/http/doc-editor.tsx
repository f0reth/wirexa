import { markdown } from "@codemirror/lang-markdown";
import {
  defaultHighlightStyle,
  syntaxHighlighting,
} from "@codemirror/language";
import { minimalSetup } from "codemirror";
import { createCodeMirror } from "../shared/codemirror";
import styles from "./http.module.css";

interface Props {
  value: string;
  onChange: (value: string) => void;
}

export function DocEditor(props: Props) {
  const ref = createCodeMirror({
    value: () => props.value,
    onChange: (v) => props.onChange(v),
    extensions: [
      minimalSetup,
      syntaxHighlighting(defaultHighlightStyle),
      markdown(),
    ],
  });

  return <div ref={ref} class={styles.docEditor} />;
}
