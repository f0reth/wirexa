import { X } from "lucide-solid";
import { Show } from "solid-js";
import { Badge } from "../../../components/ui/badge";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import {
  type FileReference,
  fileSelectionState,
} from "../../../domain/http/types";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";

interface FileReferenceInputProps {
  file: FileReference | undefined;
  onChange: (file: FileReference | undefined) => void;
  inputClass?: string;
}

const STATE_BADGES = {
  selected: { label: "Selected", variant: "secondary" },
  unconfirmed: { label: "Not confirmed", variant: "outline" },
  reselect: { label: "Reselect file", variant: "destructive" },
} as const;

// request file の入力欄。パスの入力・ペーストはダイアログの初期位置 (hint) にしかならず、
// ファイルが確定するのは Enter / Browse... で開いたダイアログで選んだときだけ。
// 確定後に保持・表示するのは token と basename で、実パスは持たない。
export function FileReferenceInput(props: FileReferenceInputProps) {
  const { pickFile } = useHttpRequest();

  const badge = () => {
    const state = fileSelectionState(props.file);
    return state === "none" ? undefined : STATE_BADGES[state];
  };

  const pick = async () => {
    const selected = await pickFile(props.file?.hint ?? "");
    if (selected) props.onChange(selected);
  };

  return (
    <>
      <Input
        value={props.file?.hint ?? props.file?.name ?? ""}
        placeholder="No file selected"
        onInput={(e) => {
          // 編集したら確定を取り消し、以前の token を送信に使わない。
          const hint = e.currentTarget.value;
          props.onChange(hint ? { hint } : undefined);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            void pick();
          }
        }}
        class={props.inputClass}
      />
      <Show when={badge()}>
        {(b) => (
          <Badge variant={b().variant} class={styles.fileStateBadge}>
            {b().label}
          </Badge>
        )}
      </Show>
      <Button
        variant="ghost"
        size="sm"
        class={styles.formBrowse}
        onClick={() => void pick()}
      >
        Browse...
      </Button>
      <Show when={props.file}>
        <Button
          variant="ghost"
          size="icon"
          class={styles.kvRemove}
          aria-label="Clear selection"
          onClick={() => props.onChange(undefined)}
        >
          <X size={14} aria-hidden="true" />
        </Button>
      </Show>
    </>
  );
}
