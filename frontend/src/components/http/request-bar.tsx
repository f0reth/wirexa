import { Send } from "lucide-solid";
import { createMemo } from "solid-js";
import { Button } from "../ui/button";
import { Input } from "../ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "../ui/select";
import { useHttp } from "./context";
import styles from "./http.module.css";
import { HTTP_METHODS, type HttpMethod, METHOD_COLORS } from "./types";

function isValidHttpUrl(s: string): boolean {
  try {
    new URL(s);
    return true;
  } catch {
    return false;
  }
}

export function RequestBar() {
  const { method, setMethod, url, setUrl, sendRequest, loading } = useHttp();

  const urlValid = createMemo(() => isValidHttpUrl(url()));
  const methodColor = createMemo(() => METHOD_COLORS[method()]);

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
          if (e.key === "Enter") sendRequest();
        }}
      />

      <Button
        onClick={() => sendRequest()}
        disabled={loading() || !urlValid()}
        class={styles.sendButton}
      >
        <Send size={14} />
        {loading() ? "Sending..." : "Send"}
      </Button>
    </div>
  );
}
