import { clsx } from "clsx";
import { type JSX, splitProps } from "solid-js";
import styles from "./textarea.module.css";

type TextareaProps = {
  class?: string;
} & Omit<JSX.IntrinsicElements["textarea"], "class">;

export function Textarea(props: TextareaProps) {
  const [local, rest] = splitProps(props, ["class"]);

  return <textarea class={clsx(styles.textarea, local.class)} {...rest} />;
}
