import { clsx } from "clsx";
import { type JSX, splitProps } from "solid-js";
import styles from "./badge.module.css";

type BadgeVariant = "default" | "secondary" | "destructive" | "outline";

type BadgeProps = {
  variant?: BadgeVariant;
  class?: string;
} & Omit<JSX.IntrinsicElements["span"], "class">;

const variantStyles: Record<BadgeVariant, string> = {
  default: styles.variantDefault,
  secondary: styles.variantSecondary,
  destructive: styles.variantDestructive,
  outline: styles.variantOutline,
};

export function Badge(props: BadgeProps) {
  const [local, rest] = splitProps(props, ["variant", "class", "children"]);

  return (
    <span
      class={clsx(
        styles.badge,
        variantStyles[local.variant ?? "default"],
        local.class,
      )}
      {...rest}
    >
      {local.children}
    </span>
  );
}
