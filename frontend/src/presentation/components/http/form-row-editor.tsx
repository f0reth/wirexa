import { ChevronDown, ChevronRight, Minus, Plus } from "lucide-solid";
import { createResource, createSignal, Index, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "../../../components/ui/select";
import type {
  FormBodyType,
  FormRow,
  FormRowKind,
} from "../../../domain/http/types";
import { FORM_ROW_KINDS_BY_BODY_TYPE } from "../../../domain/http/types";
import {
  guessFormPartContentType,
  openFilePicker,
} from "../../../infrastructure/http/client";
import { FORM_ROW_KIND_LABELS } from "../../constants/http";
import styles from "./http.module.css";
import { JsonBodyEditor } from "./json-body-editor";

interface FormRowEditorProps {
  rows: FormRow[];
  onChange: (rows: FormRow[]) => void;
  bodyType: FormBodyType;
}

// form 系ボディの行エディタ。Params/Headers の KeyValueEditor と違い、
// 値の種別（text/json/file）とパートごとの Content-Type を扱う。
export function FormRowEditor(props: FormRowEditorProps) {
  // 同時に展開するのは 1 行だけ。Index は位置キーなので行の増減で
  // 展開中の index が別の行を指してしまうのを避ける意味もある。
  const [expanded, setExpanded] = createSignal<number | null>(null);

  const update = <K extends keyof FormRow>(
    index: number,
    field: K,
    value: FormRow[K],
  ) => {
    props.onChange(
      props.rows.map((r, i) => (i === index ? { ...r, [field]: value } : r)),
    );
  };

  const remove = (index: number) => {
    setExpanded(null);
    props.onChange(props.rows.filter((_, i) => i !== index));
  };

  const add = () => {
    setExpanded(null);
    props.onChange([
      ...props.rows,
      { key: "", value: "", kind: "text", enabled: true },
    ]);
  };

  const kindOf = (row: FormRow): FormRowKind => row.kind ?? "text";

  // urlencoded の text 行は展開しても出すものが無いのでトグルごと隠す。
  const canExpand = (row: FormRow) =>
    props.bodyType === "form-data" || kindOf(row) === "json";

  const browse = async (index: number) => {
    const path = await openFilePicker();
    if (path) update(index, "filePath", path);
  };

  return (
    <div class={styles.kvEditor}>
      <Index each={props.rows}>
        {(row, index) => {
          // file 行の自動 Content-Type は Go に問い合わせる（拡張子判定は OS 依存で、
          // ここで再実装すると実際に送られる値とヒントがずれる）。
          const [autoFileType] = createResource(
            () =>
              kindOf(row()) === "file" && row().filePath
                ? row().filePath
                : undefined,
            (path: string) => guessFormPartContentType(path),
          );
          const autoContentType = () => {
            switch (kindOf(row())) {
              case "json":
                return "application/json";
              case "file":
                return autoFileType() ?? "";
              default:
                return "none";
            }
          };
          const isExpanded = () => expanded() === index;

          return (
            <div class={styles.formRowGroup}>
              <div class={styles.kvRow}>
                <input
                  type="checkbox"
                  checked={row().enabled}
                  onChange={(e) =>
                    update(index, "enabled", e.currentTarget.checked)
                  }
                  class={styles.kvCheckbox}
                />
                <Input
                  value={row().key}
                  onInput={(e) => update(index, "key", e.currentTarget.value)}
                  placeholder="Field"
                  class={styles.kvInput}
                />
                <div data-testid="form-kind-select">
                  <Select
                    value={kindOf(row())}
                    onValueChange={(v) =>
                      update(index, "kind", v as FormRow["kind"])
                    }
                  >
                    <SelectTrigger class={styles.formKindTrigger}>
                      <span>{FORM_ROW_KIND_LABELS[kindOf(row())]}</span>
                    </SelectTrigger>
                    <SelectContent>
                      <Index each={FORM_ROW_KINDS_BY_BODY_TYPE[props.bodyType]}>
                        {(kind) => (
                          <SelectItem value={kind()}>
                            {FORM_ROW_KIND_LABELS[kind()]}
                          </SelectItem>
                        )}
                      </Index>
                    </SelectContent>
                  </Select>
                </div>
                <Show
                  when={kindOf(row()) === "file"}
                  fallback={
                    <Input
                      value={row().value}
                      onInput={(e) =>
                        update(index, "value", e.currentTarget.value)
                      }
                      placeholder="Value"
                      class={styles.kvInput}
                    />
                  }
                >
                  <Input
                    value={row().filePath ?? ""}
                    onInput={(e) =>
                      update(index, "filePath", e.currentTarget.value)
                    }
                    placeholder="No file selected"
                    class={styles.kvInput}
                  />
                  <Button
                    variant="ghost"
                    size="sm"
                    class={styles.formBrowse}
                    onClick={() => void browse(index)}
                  >
                    Browse...
                  </Button>
                </Show>
                <Show when={canExpand(row())}>
                  <Button
                    variant="ghost"
                    size="icon"
                    class={styles.formExpand}
                    aria-label={isExpanded() ? "Collapse row" : "Expand row"}
                    aria-expanded={isExpanded()}
                    onClick={() => setExpanded(isExpanded() ? null : index)}
                  >
                    <Show
                      when={isExpanded()}
                      fallback={<ChevronRight size={14} aria-hidden="true" />}
                    >
                      <ChevronDown size={14} aria-hidden="true" />
                    </Show>
                  </Button>
                </Show>
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

              <Show when={canExpand(row()) && isExpanded()}>
                <div class={styles.formRowDetail}>
                  {/* Content-Type はパートごとにしか載せられないので form-data だけ。 */}
                  <Show when={props.bodyType === "form-data"}>
                    <div class={styles.formContentTypeRow}>
                      <span class={styles.formDetailLabel}>Content-Type</span>
                      <Input
                        value={row().contentType ?? ""}
                        onInput={(e) =>
                          update(index, "contentType", e.currentTarget.value)
                        }
                        placeholder="auto"
                        class={styles.kvInput}
                      />
                      <Show when={!row().contentType && autoContentType()}>
                        <span class={styles.formAutoHint}>
                          auto: {autoContentType()}
                        </span>
                      </Show>
                    </div>
                  </Show>
                  <Show when={kindOf(row()) === "json"}>
                    <JsonBodyEditor
                      value={row().value}
                      onChange={(v) => update(index, "value", v)}
                    />
                  </Show>
                </div>
              </Show>
            </div>
          );
        }}
      </Index>
      <Button variant="ghost" size="sm" onClick={add} class={styles.kvAdd}>
        <Plus size={14} />
        Add
      </Button>
    </div>
  );
}
