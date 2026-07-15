import { Check, Copy } from "lucide-solid";
import { createMemo, createSignal, For, Match, Show, Switch } from "solid-js";
import { Badge } from "../../../components/ui/badge";
import { createCopyButton } from "../../../components/ui/copy-button";
import { ScrollArea } from "../../../components/ui/scroll-area";
import { TabList } from "../../../components/ui/tabs";
import {
  saveResponseBinary,
  saveResponseBody,
} from "../../../infrastructure/http/client";
import { useHttpRequest } from "../../providers/http-provider";
import { highlightJson } from "../../utils/json-highlight";
import { HexView } from "../shared/hex-view";
import styles from "./http.module.css";

const TABS = [
  { value: "body", label: "Body" },
  { value: "headers", label: "Headers" },
  { value: "timing", label: "Timing" },
];

const HIGHLIGHT_SIZE_LIMIT = 1024 * 1024; // 1 MB

// バックエンド (net_client.go の defaultMaxTempBytes) が受信を打ち切る絶対上限の表示用ラベル。
const HARD_LIMIT_LABEL = "1 GB";

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
  const { copy, isCopied } = createCopyButton();
  const [showTruncatedBody, setShowTruncatedBody] = createSignal(false);

  function handleCopy() {
    const resp = response();
    if (!resp) return;
    copy(bodyDisplay().text);
  }

  async function handleSaveToFile() {
    const resp = response();
    if (!resp) return;
    // 切り詰め時は temp ファイル（全文）から、非切り詰めバイナリはメモリ上の base64 から保存する。
    if (resp.bodyTruncated) {
      await saveResponseBody(resp.tempFilePath, resp.contentType);
    } else {
      await saveResponseBinary(resp.body, resp.contentType);
    }
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
    return { text: formatted, html: highlightJson(formatted) };
  });

  // body の表示種別。切り捨て/空などの直交状態は別途 Show で扱う。
  const bodyKind = createMemo<"image" | "binary" | "json" | "text">(() => {
    const resp = response();
    if (!resp) return "text";
    if (isImageContentType(resp.contentType)) return "image";
    if (resp.bodyBase64) return "binary";
    if (bodyDisplay().html !== null) return "json";
    return "text";
  });

  return (
    <div class={styles.responsePanel}>
      <div class={styles.responsePanelHeader}>
        <span class={styles.responsePanelTitle}>Response</span>
        <Show
          when={
            response() &&
            !response()?.error &&
            !response()?.bodyTruncated &&
            !response()?.bodyBase64
          }
        >
          <button
            type="button"
            class={styles.responsePanelCloseBtn}
            onClick={handleCopy}
            aria-label="Copy body"
            title="Copy body"
          >
            <Show
              when={isCopied()}
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
                            <Show when={resp().bodyCapped}>
                              <p>
                                The response also exceeded the{" "}
                                {HARD_LIMIT_LABEL} hard limit, so the download
                                was cut short — even the saved file will be
                                incomplete.
                              </p>
                            </Show>
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
                              <Show
                                when={resp().bodyCapped}
                                fallback={
                                  <>
                                    ⚠ Showing truncated body. The full body was
                                    not loaded.
                                  </>
                                }
                              >
                                ⚠ Showing truncated body. The response exceeded
                                the {HARD_LIMIT_LABEL} hard limit and was cut
                                short, so the full body is unavailable.
                              </Show>
                            </div>
                          </Show>

                          <Switch>
                            <Match when={bodyKind() === "image"}>
                              <div class={styles.responseImageContainer}>
                                <img
                                  src={`data:${resp().contentType.split(";")[0]};base64,${resp().body}`}
                                  alt="Response"
                                  class={styles.responseImage}
                                />
                              </div>
                            </Match>
                            <Match when={bodyKind() === "binary"}>
                              {/* 非 UTF-8 バイナリ: hex ダンプ + 保存ボタン */}
                              <div class={styles.responseBodyLimitActions}>
                                <button
                                  type="button"
                                  class={styles.responseBodyLimitBtn}
                                  onClick={handleSaveToFile}
                                >
                                  Save body to file
                                </button>
                              </div>
                              <HexView base64={resp().body} />
                            </Match>
                            <Match when={bodyKind() === "json"}>
                              <pre
                                class={styles.responseBody}
                                innerHTML={bodyDisplay().html ?? ""}
                              />
                            </Match>
                            <Match when={bodyKind() === "text"}>
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
                              <pre class={styles.responseBody}>
                                {bodyDisplay().text}
                              </pre>
                            </Match>
                          </Switch>
                        </Show>
                      </Show>
                    </ScrollArea>
                  </Show>

                  <Show when={responseTab() === "headers"}>
                    <ScrollArea class={styles.responseScrollArea}>
                      <div class={styles.responseHeaders}>
                        <For each={Object.entries(resp().headers)}>
                          {([key, values]) => (
                            <For each={values}>
                              {(value) => (
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
