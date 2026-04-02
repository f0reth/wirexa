import { clsx } from "clsx";
import { PanelRight, Send, X } from "lucide-solid";
import { createMemo } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "../../../components/ui/select";
import { HTTP_METHODS, type HttpMethod } from "../../../domain/http/types";
import { METHOD_COLORS } from "../../constants/http";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";

function isValidHttpUrl(s: string): boolean {
  try {
    new URL(s);
    return true;
  } catch {
    return false;
  }
}

type RequestBarProps = {
  showResponse: boolean;
  onToggleResponse: () => void;
};

export function RequestBar(props: RequestBarProps) {
  const {
    method,
    setMethod,
    url,
    setUrl,
    sendRequest,
    cancelRequest,
    loading,
  } = useHttpRequest();

  const urlValid = createMemo(() => isValidHttpUrl(url()));
  const methodColor = createMemo(() => METHOD_COLORS[method()]);

  async function handleSend() {
    try {
      await sendRequest();
    } catch (err) {
      console.error("Request failed:", err);
    }
  }

  return (
    <div class={styles.requestBar}>
      <Select
        value={method()}
        onValueChange={(v) => setMethod(v as HttpMethod)}
      >
        <SelectTrigger class={styles.methodTrigger}>
          <span style={{ color: methodColor(), "font-weight": "600" }}>
            {method()}
          </span>
        </SelectTrigger>
        <SelectContent>
          {HTTP_METHODS.map((m) => (
            <SelectItem value={m}>
              <span style={{ color: METHOD_COLORS[m], "font-weight": "600" }}>
                {m}
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Input
        value={url()}
        onInput={(e) => setUrl(e.currentTarget.value)}
        placeholder="https://api.example.com/endpoint"
        class={styles.urlInput}
        onKeyDown={(e) => {
          if (e.key === "Enter") handleSend();
        }}
      />

      {loading() ? (
        <Button
          onClick={cancelRequest}
          variant="destructive"
          class={styles.sendButton}
        >
          <X size={14} />
          Cancel
        </Button>
      ) : (
        <Button
          onClick={handleSend}
          disabled={!urlValid()}
          class={styles.sendButton}
        >
          <Send size={14} />
          Send
        </Button>
      )}

      <button
        type="button"
        class={clsx(
          styles.responseToggleBtn,
          props.showResponse && styles.responseToggleBtnActive,
        )}
        onClick={props.onToggleResponse}
        title={
          props.showResponse ? "Hide response panel" : "Show response panel"
        }
      >
        <PanelRight size={15} />
      </button>
    </div>
  );
}
