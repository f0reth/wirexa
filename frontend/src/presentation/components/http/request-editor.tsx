import { createMemo, createSignal, Show } from "solid-js";
import {
  parseFormPairs,
  serializeFormPairs,
} from "../../../application/http/form-pairs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import { TabList } from "../../../components/ui/tabs";
import { Textarea } from "../../../components/ui/textarea";
import type { BodyType } from "../../../domain/http/types";
import { BODY_TYPES } from "../../constants/http";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";
import { KeyValueEditor } from "./key-value-editor";

const TABS = [
  { value: "params", label: "Params" },
  { value: "headers", label: "Headers" },
  { value: "body", label: "Body" },
];

export function RequestEditor() {
  const { params, setParams, headers, setHeaders, body, setBody } =
    useHttpRequest();

  const [requestTab, setRequestTab] = createSignal("params");

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
              onChange={(pairs) => setParams(pairs)}
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
              onChange={(pairs) => setHeaders(pairs)}
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
