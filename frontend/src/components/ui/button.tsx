import { clsx } from "clsx";
import { type JSX, splitProps } from "solid-js";
import styles from "./button.module.css";

type ButtonVariant =
  | "default"
  | "destructive"
  | "outline"
  | "secondary"
  | "ghost"
  | "link";
type ButtonSize = "default" | "sm" | "lg" | "icon";

type ButtonProps = {
  variant?: ButtonVariant;
  size?: ButtonSize;
  class?: string;
} & Omit<JSX.IntrinsicElements["button"], "class">;

const variantStyles: Record<ButtonVariant, string> = {
  default: styles.variantDefault,
  destructive: styles.variantDestructive,
  outline: styles.variantOutline,
  secondary: styles.variantSecondary,
  ghost: styles.variantGhost,
  link: styles.variantLink,
};

const sizeStyles: Record<ButtonSize, string> = {
  default: styles.sizeDefault,
  sm: styles.sizeSm,
  lg: styles.sizeLg,
  icon: styles.sizeIcon,
};

export function Button(props: ButtonProps) {
  const [local, rest] = splitProps(props, [
    "variant",
    "size",
    "class",
    "children",
  ]);

  return (
    <button
      class={clsx(
        styles.button,
        variantStyles[local.variant ?? "default"],
        sizeStyles[local.size ?? "default"],
        local.class,
      )}
      {...rest}
    >
      {local.children}
    </button>
  );
}
