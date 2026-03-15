import { clsx } from "clsx";
import { type JSX, splitProps } from "solid-js";
import styles from "./input.module.css";

type InputProps = {
  class?: string;
} & Omit<JSX.IntrinsicElements["input"], "class">;

export function Input(props: InputProps) {
  const [local, rest] = splitProps(props, ["class"]);

  return <input class={clsx(styles.input, local.class)} {...rest} />;
}
