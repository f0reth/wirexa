import { clsx } from "clsx";
import { Check, ChevronDown } from "lucide-solid";
import {
  createContext,
  createSignal,
  type JSX,
  Show,
  splitProps,
  useContext,
} from "solid-js";
import styles from "./select.module.css";

interface SelectContextValue {
  value: () => string;
  onValueChange: (v: string) => void;
  open: () => boolean;
  setOpen: (v: boolean) => void;
}

const SelectContext = createContext<SelectContextValue>({
  value: () => "",
  onValueChange: () => {},
  open: () => false,
  setOpen: () => {},
});

function useSelect() {
  return useContext(SelectContext);
}

export interface SelectProps {
  value: string;
  onValueChange: (value: string) => void;
  children: JSX.Element;
}

export function Select(props: SelectProps) {
  const [open, setOpen] = createSignal(false);

  const handleFocusOut = (e: FocusEvent) => {
    const container = e.currentTarget as HTMLElement;
    if (!e.relatedTarget || !container.contains(e.relatedTarget as Node)) {
      setOpen(false);
    }
  };

  return (
    <SelectContext.Provider
      value={{
        value: () => props.value,
        onValueChange: props.onValueChange,
        open,
        setOpen,
      }}
    >
      <div onFocusOut={handleFocusOut} class={styles.root}>
        {props.children}
      </div>
    </SelectContext.Provider>
  );
}

type SelectTriggerProps = {
  class?: string;
  children: JSX.Element;
};

export function SelectTrigger(props: SelectTriggerProps) {
  const ctx = useSelect();
  const [local] = splitProps(props, ["class", "children"]);

  return (
    <button
      type="button"
      class={clsx(styles.trigger, local.class)}
      onClick={() => ctx.setOpen(!ctx.open())}
    >
      {local.children}
      <ChevronDown class={styles.chevron} />
    </button>
  );
}

export interface SelectValueProps {
  placeholder?: string;
}

export function SelectValue(props: SelectValueProps) {
  const ctx = useSelect();
  return <span>{ctx.value() || props.placeholder}</span>;
}

type SelectContentProps = {
  class?: string;
  children: JSX.Element;
};

export function SelectContent(props: SelectContentProps) {
  const ctx = useSelect();
  const [local] = splitProps(props, ["class", "children"]);

  function positionDropdown(el: HTMLDivElement) {
    requestAnimationFrame(() => {
      const rect = el.getBoundingClientRect();
      const vw = window.innerWidth;
      const vh = window.innerHeight;

      if (rect.right > vw) {
        el.style.left = "auto";
        el.style.right = "0";
      }

      if (rect.bottom > vh) {
        el.style.top = "auto";
        el.style.bottom = "100%";
        el.style.marginTop = "0";
        el.style.marginBottom = "0.25rem";
      }
    });
  }

  return (
    <Show when={ctx.open()}>
      <div ref={positionDropdown} class={clsx(styles.content, local.class)}>
        {local.children}
      </div>
    </Show>
  );
}

export interface SelectItemProps {
  value: string;
  children: JSX.Element;
  class?: string;
}

export function SelectItem(props: SelectItemProps) {
  const ctx = useSelect();
  const isSelected = () => ctx.value() === props.value;

  return (
    <button
      type="button"
      class={clsx(styles.item, props.class)}
      onClick={() => {
        ctx.onValueChange(props.value);
        ctx.setOpen(false);
      }}
    >
      <Show when={isSelected()}>
        <span class={styles.checkIcon}>
          <Check size={16} />
        </span>
      </Show>
      {props.children}
    </button>
  );
}
