import { clsx } from "clsx";
import { createMemo, For, Show } from "solid-js";
import { Badge } from "../ui/badge";
import { ScrollArea } from "../ui/scroll-area";
import { useHttp } from "./context";
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
  const { response, loading, responseTab, setResponseTab } = useHttp();

  return (
    <div class={styles.responsePanel}>
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
                        <pre class={styles.responseBody}>{formattedBody()}</pre>
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
