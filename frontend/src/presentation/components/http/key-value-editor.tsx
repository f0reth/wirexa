import { Minus, Plus } from "lucide-solid";
import { Index } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import type { KeyValuePair } from "../../../domain/http/types";
import styles from "./http.module.css";

interface KeyValueEditorProps {
  pairs: KeyValuePair[];
  onChange: (pairs: KeyValuePair[]) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
}

export function KeyValueEditor(props: KeyValueEditorProps) {
  const update = (
    index: number,
    field: keyof KeyValuePair,
    value: string | boolean,
  ) => {
    const next = props.pairs.map((p, i) =>
      i === index ? { ...p, [field]: value } : p,
    );
    props.onChange(next);
  };

  const remove = (index: number) => {
    props.onChange(props.pairs.filter((_, i) => i !== index));
  };

  const add = () => {
    props.onChange([...props.pairs, { key: "", value: "", enabled: true }]);
  };

  return (
    <div class={styles.kvEditor}>
      <Index each={props.pairs}>
        {(pair, index) => (
          <div class={styles.kvRow}>
            <input
              type="checkbox"
              checked={pair().enabled}
              onChange={(e) =>
                update(index, "enabled", e.currentTarget.checked)
              }
              class={styles.kvCheckbox}
            />
            <Input
              value={pair().key}
              onInput={(e) => update(index, "key", e.currentTarget.value)}
              placeholder={props.keyPlaceholder ?? "Key"}
              class={styles.kvInput}
            />
            <Input
              value={pair().value}
              onInput={(e) => update(index, "value", e.currentTarget.value)}
              placeholder={props.valuePlaceholder ?? "Value"}
              class={styles.kvInput}
            />
            <Button
              variant="ghost"
              size="icon"
              class={styles.kvRemove}
              aria-label="Remove row"
              onClick={() => remove(index)}
            >
              <Minus size={14} aria-hidden="true" />
            </Button>
          </div>
        )}
      </Index>
      <Button variant="ghost" size="sm" onClick={add} class={styles.kvAdd}>
        <Plus size={14} />
        Add
      </Button>
    </div>
  );
}
