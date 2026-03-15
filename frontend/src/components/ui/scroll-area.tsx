import { clsx } from "clsx";
import { type JSX, splitProps } from "solid-js";
import styles from "./scroll-area.module.css";

type ScrollAreaProps = {
  class?: string;
} & Omit<JSX.IntrinsicElements["div"], "class">;

export function ScrollArea(props: ScrollAreaProps) {
  const [local, rest] = splitProps(props, ["class", "children"]);

  return (
    <div class={clsx(styles.scrollArea, local.class)} {...rest}>
      {local.children}
    </div>
  );
}
