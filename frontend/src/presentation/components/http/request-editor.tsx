import { createMemo, createSignal, Show } from "solid-js";
import {
  parseFormPairs,
  serializeFormPairs,
} from "../../../application/http/form-pairs";
import { Input } from "../../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import { TabList } from "../../../components/ui/tabs";
import { Textarea } from "../../../components/ui/textarea";
import type { AuthType, BodyType } from "../../../domain/http/types";
import { AUTH_TYPES, BODY_TYPES } from "../../constants/http";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";
import { KeyValueEditor } from "./key-value-editor";

const TABS = [
  { value: "params", label: "Params" },
  { value: "headers", label: "Headers" },
  { value: "body", label: "Body" },
  { value: "auth", label: "Auth" },
];

export function RequestEditor() {
  const {
    params,
    setParams,
    headers,
    setHeaders,
    body,
    setBody,
    auth,
    setAuth,
  } = useHttpRequest();

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
            class={styles.scrollTabPanel}
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
            class={styles.scrollTabPanel}
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
          <div
            class={styles.bodyTabPanel}
            role="tabpanel"
            id="tabpanel-body"
            aria-labelledby="tab-body"
          >
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
                          onBlur={(e) => {
                            if (body().type === "json") {
                              try {
                                const formatted = JSON.stringify(
                                  JSON.parse(e.currentTarget.value),
                                  null,
                                  2,
                                );
                                setBody({ ...body(), content: formatted });
                              } catch {
                                // invalid JSON, keep as-is
                              }
                            }
                          }}
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

        <Show when={requestTab() === "auth"}>
          <div
            class={styles.scrollTabPanel}
            role="tabpanel"
            id="tabpanel-auth"
            aria-labelledby="tab-auth"
          >
            <div class={styles.authSection}>
              <div class={styles.authTypeRow}>
                <Select
                  value={auth().type}
                  onValueChange={(v) =>
                    setAuth({ ...auth(), type: v as AuthType })
                  }
                >
                  <SelectTrigger class={styles.authTypeTrigger}>
                    <SelectValue placeholder="None" />
                  </SelectTrigger>
                  <SelectContent>
                    {AUTH_TYPES.map((at) => (
                      <SelectItem value={at.value}>{at.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <Show when={auth().type === "basic"}>
                <div class={styles.authFields}>
                  <Input
                    value={auth().username}
                    onInput={(e) =>
                      setAuth({ ...auth(), username: e.currentTarget.value })
                    }
                    placeholder="Username"
                  />
                  <Input
                    type="password"
                    value={auth().password}
                    onInput={(e) =>
                      setAuth({ ...auth(), password: e.currentTarget.value })
                    }
                    placeholder="Password"
                  />
                </div>
              </Show>

              <Show when={auth().type === "bearer"}>
                <div class={styles.authFields}>
                  <Input
                    value={auth().token}
                    onInput={(e) =>
                      setAuth({ ...auth(), token: e.currentTarget.value })
                    }
                    placeholder="Token"
                  />
                </div>
              </Show>
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
}
