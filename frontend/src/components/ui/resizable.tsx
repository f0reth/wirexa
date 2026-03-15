import Resizable from "@corvu/resizable";
import { clsx } from "clsx";
import { GripVertical } from "lucide-solid";
import { type JSX, Show, splitProps } from "solid-js";
import styles from "./resizable.module.css";

type ResizablePanelGroupProps = {
  direction?: "horizontal" | "vertical";
  class?: string;
  children: JSX.Element;
};

export function ResizablePanelGroup(props: ResizablePanelGroupProps) {
  const [local, rest] = splitProps(props, ["direction", "class", "children"]);
  const orientation = () => local.direction ?? "horizontal";

  return (
    <Resizable
      orientation={orientation()}
      class={clsx(
        styles.panelGroup,
        orientation() === "vertical"
          ? styles.panelGroupVertical
          : styles.panelGroupHorizontal,
        local.class,
      )}
      {...rest}
    >
      {local.children}
    </Resizable>
  );
}

type ResizablePanelProps = {
  defaultSize?: number;
  minSize?: number;
  class?: string;
  children: JSX.Element;
};

export function ResizablePanel(props: ResizablePanelProps) {
  const [local, rest] = splitProps(props, [
    "defaultSize",
    "minSize",
    "class",
    "children",
  ]);

  return (
    <Resizable.Panel
      initialSize={local.defaultSize != null ? local.defaultSize / 100 : 0.5}
      minSize={local.minSize != null ? local.minSize / 100 : 0.05}
      class={clsx(styles.panel, local.class)}
      {...rest}
    >
      {local.children}
    </Resizable.Panel>
  );
}

type ResizableHandleProps = {
  withHandle?: boolean;
  class?: string;
};

export function ResizableHandle(props: ResizableHandleProps) {
  const [local] = splitProps(props, ["withHandle", "class"]);
  const ctx = Resizable.useContext();

  return (
    <Resizable.Handle
      class={clsx(
        styles.handle,
        ctx.orientation() === "vertical"
          ? styles.handleVertical
          : styles.handleHorizontal,
        local.class,
      )}
    >
      <Show when={local.withHandle}>
        <div
          class={clsx(
            styles.handleGrip,
            ctx.orientation() === "vertical" && styles.handleGripVertical,
          )}
        >
          <GripVertical size={10} />
        </div>
      </Show>
    </Resizable.Handle>
  );
}
