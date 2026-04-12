import { clsx } from "clsx";
import { Check, Copy } from "lucide-solid";
import { createMemo, createSignal, For, Show } from "solid-js";
import { Badge } from "../../../components/ui/badge";
import { ScrollArea } from "../../../components/ui/scroll-area";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";

const TABS = [
  { value: "body", label: "Body" },
  { value: "headers", label: "Headers" },
  { value: "timing", label: "Timing" },
];

function statusVariant(code: number): "default" | "secondary" | "destructive" {
  if (code >= 200 && code < 300) return "default";
  if (code >= 400) return "destructive";
  return "secondary";
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function ResponseViewer() {
  const { response, loading } = useHttpRequest();

  const [responseTab, setResponseTab] = createSignal("body");
  const [copied, setCopied] = createSignal(false);

  function handleCopy() {
    const resp = response();
    if (!resp) return;
    navigator.clipboard.writeText(formatBody(resp.body, resp.contentType));
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  return (
    <div class={styles.responsePanel}>
      <div class={styles.responsePanelHeader}>
        <span class={styles.responsePanelTitle}>Response</span>
        <Show when={response() && !response()?.error}>
          <button
            type="button"
            class={styles.responsePanelCloseBtn}
            onClick={handleCopy}
            aria-label="Copy body"
            title="Copy body"
          >
            <Show
              when={copied()}
              fallback={<Copy size={14} aria-hidden="true" />}
            >
              <Check size={14} aria-hidden="true" />
            </Show>
          </button>
        </Show>
      </div>
      <Show
        when={response() || loading()}
        fallback={
          <div class={styles.responseEmpty}>
            <p class={styles.responseEmptyText}>
              Send a request to see the response
            </p>
          </div>
        }
      >
        <Show when={loading()}>
          <div class={styles.responseEmpty}>
            <p class={styles.responseEmptyText}>Sending request...</p>
          </div>
        </Show>

        <Show when={response()}>
          {(resp) => {
            const formattedBody = createMemo(() =>
              formatBody(resp().body, resp().contentType),
            );

            return (
              <>
                <Show when={resp().error}>
                  <div class={styles.responseError}>
                    <span class={styles.responseErrorText}>{resp().error}</span>
                    <Show when={resp().timingMs > 0}>
                      <span class={styles.responseTiming}>
                        {resp().timingMs} ms
                      </span>
                    </Show>
                  </div>
                </Show>

                <Show when={!resp().error}>
                  <div class={styles.responseStatusBar}>
                    <Badge variant={statusVariant(resp().statusCode)}>
                      {resp().statusCode}
                    </Badge>
                    <span class={styles.responseStatusText}>
                      {resp().statusText}
                    </span>
                    <span class={styles.responseMeta}>
                      {formatSize(resp().size)}
                    </span>
                    <span class={styles.responseMeta}>
                      {resp().timingMs} ms
                    </span>
                  </div>

                  <div class={styles.editorTabBar}>
                    {TABS.map((tab) => (
                      <button
                        type="button"
                        class={clsx(
                          styles.editorTab,
                          responseTab() === tab.value && styles.editorTabActive,
                        )}
                        onClick={() => setResponseTab(tab.value)}
                      >
                        {tab.label}
                      </button>
                    ))}
                  </div>

                  <div class={styles.responseContent}>
                    <Show when={responseTab() === "body"}>
                      <ScrollArea class={styles.responseScrollArea}>
                        <Show
                          when={isImageContentType(resp().contentType)}
                          fallback={
                            <Show
                              when={isJsonContentType(resp().contentType)}
                              fallback={
                                <pre class={styles.responseBody}>
                                  {formattedBody()}
                                </pre>
                              }
                            >
                              <pre
                                class={styles.responseBody}
                                innerHTML={highlightJson(resp().body)}
                              />
                            </Show>
                          }
                        >
                          <div class={styles.responseImageContainer}>
                            <img
                              src={`data:${resp().contentType.split(";")[0]};base64,${resp().body}`}
                              alt="Response"
                              class={styles.responseImage}
                            />
                          </div>
                        </Show>
                      </ScrollArea>
                    </Show>

                    <Show when={responseTab() === "headers"}>
                      <ScrollArea class={styles.responseScrollArea}>
                        <div class={styles.responseHeaders}>
                          <For each={Object.entries(resp().headers)}>
                            {([key, value]) => (
                              <div class={styles.responseHeaderRow}>
                                <span class={styles.responseHeaderKey}>
                                  {key}
                                </span>
                                <span class={styles.responseHeaderValue}>
                                  {value}
                                </span>
                              </div>
                            )}
                          </For>
                        </div>
                      </ScrollArea>
                    </Show>

                    <Show when={responseTab() === "timing"}>
                      <div class={styles.timingInfo}>
                        <div class={styles.timingRow}>
                          <span class={styles.timingLabel}>Total time</span>
                          <span class={styles.timingValue}>
                            {resp().timingMs} ms
                          </span>
                        </div>
                        <div class={styles.timingRow}>
                          <span class={styles.timingLabel}>Response size</span>
                          <span class={styles.timingValue}>
                            {formatSize(resp().size)}
                          </span>
                        </div>
                      </div>
                    </Show>
                  </div>
                </Show>
              </>
            );
          }}
        </Show>
      </Show>
    </div>
  );
}

function formatBody(body: string, contentType: string): string {
  if (contentType?.includes("json")) {
    try {
      return JSON.stringify(JSON.parse(body), null, 2);
    } catch {
      return body;
    }
  }
  return body;
}

function isImageContentType(contentType: string): boolean {
  return contentType?.toLowerCase().startsWith("image/") ?? false;
}

function isJsonContentType(contentType: string): boolean {
  return contentType?.includes("json") ?? false;
}

function escapeHtml(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

const JSON_TOKEN_RE =
  /("(?:\\u[a-fA-F0-9]{4}|\\[^u]|[^\\"])*"(?:\s*:)?|true|false|null|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g;

function getTokenClass(token: string): string {
  if (token.startsWith('"')) {
    return token.trimEnd().endsWith(":") ? "json-key" : "json-string";
  }
  if (token === "true" || token === "false") return "json-boolean";
  if (token === "null") return "json-null";
  return "json-number";
}

function highlightJson(body: string): string {
  let formatted: string;
  try {
    formatted = JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return escapeHtml(body);
  }

  let result = "";
  let lastIndex = 0;

  for (const match of formatted.matchAll(JSON_TOKEN_RE)) {
    const start = match.index;
    result += escapeHtml(formatted.slice(lastIndex, start));
    const token = match[0];
    result += `<span class="${getTokenClass(token)}">${escapeHtml(token)}</span>`;
    lastIndex = start + token.length;
  }

  result += escapeHtml(formatted.slice(lastIndex));
  return result;
}
