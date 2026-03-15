import { createMemo, Show } from "solid-js";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select";
import { TabList } from "../ui/tabs";
import { Textarea } from "../ui/textarea";
import { useHttp } from "./context";
import styles from "./http.module.css";
import { KeyValueEditor } from "./key-value-editor";
import { BODY_TYPES, type BodyType } from "./types";

const TABS = [
  { value: "params", label: "Params" },
  { value: "headers", label: "Headers" },
  { value: "body", label: "Body" },
];

export function RequestEditor() {
  const {
    requestTab,
    setRequestTab,
    params,
    setParams,
    headers,
    setHeaders,
    body,
    setBody,
  } = useHttp();

  return (
    <div class={styles.editorPanel}>
      <TabList
        tabs={TABS}
        activeTab={requestTab()}
        onTabChange={setRequestTab}
        class={styles.editorTabBar}
      />

      <div class={styles.editorContent}>
        <Show when={requestTab() === "params"}>
          <div
            role="tabpanel"
            id="tabpanel-params"
            aria-labelledby="tab-params"
          >
            <KeyValueEditor
              pairs={params()}
              onChange={setParams}
              keyPlaceholder="Parameter"
              valuePlaceholder="Value"
            />
          </div>
        </Show>

        <Show when={requestTab() === "headers"}>
          <div
            role="tabpanel"
            id="tabpanel-headers"
            aria-labelledby="tab-headers"
          >
            <KeyValueEditor
              pairs={headers()}
              onChange={setHeaders}
              keyPlaceholder="Header"
              valuePlaceholder="Value"
            />
          </div>
        </Show>

        <Show when={requestTab() === "body"}>
          <div role="tabpanel" id="tabpanel-body" aria-labelledby="tab-body">
            <div class={styles.bodySection}>
              <div class={styles.bodyTypeRow}>
                <Select
                  value={body().type}
                  onValueChange={(v) =>
                    setBody({ ...body(), type: v as BodyType })
                  }
                >
                  <SelectTrigger class={styles.bodyTypeTrigger}>
                    <SelectValue placeholder="None" />
                  </SelectTrigger>
                  <SelectContent>
                    {BODY_TYPES.map((bt) => (
                      <SelectItem value={bt.value}>{bt.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <Show when={body().type !== "none"}>
                {(() => {
                  const isForm = () =>
                    body().type === "form-urlencoded" ||
                    body().type === "form-data";
                  const formPairs = createMemo(() =>
                    isForm() ? parseFormPairs(body().content) : [],
                  );
                  return (
                    <Show
                      when={isForm()}
                      fallback={
                        <Textarea
                          value={body().content}
                          onInput={(e) =>
                            setBody({
                              ...body(),
                              content: e.currentTarget.value,
                            })
                          }
                          placeholder={
                            body().type === "json"
                              ? '{ "key": "value" }'
                              : "Enter body content..."
                          }
                          class={styles.bodyTextarea}
                        />
                      }
                    >
                      <KeyValueEditor
                        pairs={formPairs()}
                        onChange={(pairs) =>
                          setBody({
                            ...body(),
                            content: serializeFormPairs(pairs),
                          })
                        }
                      />
                    </Show>
                  );
                })()}
              </Show>
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
}

function parseFormPairs(
  content: string,
): { key: string; value: string; enabled: boolean }[] {
  if (!content) return [];
  return content
    .split("&")
    .filter(Boolean)
    .map((pair) => {
      const [key, ...rest] = pair.split("=");
      return {
        key: decodeURIComponent(key || ""),
        value: decodeURIComponent(rest.join("=") || ""),
        enabled: true,
      };
    });
}

function serializeFormPairs(
  pairs: { key: string; value: string; enabled: boolean }[],
): string {
  return pairs
    .filter((p) => p.enabled && p.key)
    .map((p) => `${encodeURIComponent(p.key)}=${encodeURIComponent(p.value)}`)
    .join("&");
}
