import { Check, Copy } from "lucide-solid";
import { createMemo, createSignal, For, Show } from "solid-js";
import { Badge } from "../../../components/ui/badge";
import { ScrollArea } from "../../../components/ui/scroll-area";
import { TabList } from "../../../components/ui/tabs";
import { saveResponseBody } from "../../../infrastructure/http/client";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";

const TABS = [
  { value: "body", label: "Body" },
  { value: "headers", label: "Headers" },
  { value: "timing", label: "Timing" },
];

const HIGHLIGHT_SIZE_LIMIT = 1024 * 1024; // 1 MB

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
  const [showTruncatedBody, setShowTruncatedBody] = createSignal(false);

  function handleCopy() {
    const resp = response();
    if (!resp) return;
    navigator.clipboard.writeText(bodyDisplay().text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  async function handleSaveToFile() {
    const resp = response();
    if (!resp) return;
    await saveResponseBody(resp.tempFilePath, resp.contentType);
  }

  // パースを1回だけ行い、コピー用テキストと表示用HTMLを同時に生成する
  const bodyDisplay = createMemo(() => {
    const resp = response();
    if (!resp) return { text: "", html: null as string | null };
    const body = resp.body;
    const ct = resp.contentType;

    if (!isJsonContentType(ct) || body.length > HIGHLIGHT_SIZE_LIMIT) {
      return { text: body, html: null as string | null };
    }
    let formatted: string;
    try {
      formatted = JSON.stringify(JSON.parse(body), null, 2);
    } catch {
      return { text: body, html: null as string | null };
    }
    return { text: formatted, html: highlightFormatted(formatted) };
  });

  return (
    <div class={styles.responsePanel}>
      <div class={styles.responsePanelHeader}>
        <span class={styles.responsePanelTitle}>Response</span>
        <Show
          when={response() && !response()?.error && !response()?.bodyTruncated}
        >
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
          {(resp) => (
            <>
              <Show when={resp().error}>
                <div class={styles.responseError}>
                  <span
                    class={styles.responseErrorText}
                    data-testid="response-error"
                  >
                    {resp().error}
                  </span>
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
                  <span class={styles.responseMeta}>{resp().timingMs} ms</span>
                </div>

                <TabList
                  tabs={TABS}
                  activeTab={responseTab()}
                  onTabChange={setResponseTab}
                  class={styles.editorTabBar}
                />

                <div class={styles.responseContent}>
                  <Show when={responseTab() === "body"}>
                    <ScrollArea class={styles.responseScrollArea}>
                      <Show
                        when={resp().body !== "" || resp().bodyTruncated}
                        fallback={
                          <div class={styles.responseEmpty}>
                            <p class={styles.responseEmptyText}>
                              No response body
                            </p>
                          </div>
                        }
                      >
                        {/* 上限超過: 選択ダイアログを表示 */}
                        <Show
                          when={resp().bodyTruncated && !showTruncatedBody()}
                        >
                          <div class={styles.responseBodyLimitBanner}>
                            <p>
                              ⚠ Response body exceeds the size limit. The body
                              was not fully loaded to prevent memory issues.
                            </p>
                            <div class={styles.responseBodyLimitActions}>
                              <button
                                type="button"
                                class={styles.responseBodyLimitBtn}
                                onClick={() => setShowTruncatedBody(true)}
                              >
                                Show truncated body
                              </button>
                              <button
                                type="button"
                                class={styles.responseBodyLimitBtn}
                                onClick={handleSaveToFile}
                              >
                                Save body to file
                              </button>
                            </div>
                          </div>
                        </Show>

                        {/* 通常表示 (上限未超過 or 切り捨て表示を選択した場合) */}
                        <Show
                          when={!resp().bodyTruncated || showTruncatedBody()}
                        >
                          <Show when={resp().bodyTruncated}>
                            <div class={styles.responseBodyTruncatedBanner}>
                              ⚠ Showing truncated body. The full body was not
                              loaded.
                            </div>
                          </Show>

                          <Show
                            when={isImageContentType(resp().contentType)}
                            fallback={
                              <>
                                <Show
                                  when={
                                    isJsonContentType(resp().contentType) &&
                                    resp().body.length > HIGHLIGHT_SIZE_LIMIT
                                  }
                                >
                                  <div class={styles.responseHighlightBanner}>
                                    ℹ Syntax highlighting is disabled for large
                                    responses (&gt; 1 MB).
                                  </div>
                                </Show>
                                <Show
                                  when={bodyDisplay().html !== null}
                                  fallback={
                                    <pre class={styles.responseBody}>
                                      {bodyDisplay().text}
                                    </pre>
                                  }
                                >
                                  <pre
                                    class={styles.responseBody}
                                    innerHTML={bodyDisplay().html ?? ""}
                                  />
                                </Show>
                              </>
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
                        </Show>
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
          )}
        </Show>
      </Show>
    </div>
  );
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

// フォーマット済み文字列に対してregexを走らせるだけ（JSON.parseは呼ばない）
function highlightFormatted(formatted: string): string {
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
