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
import type { AuthType, BodyType, ProxyMode } from "../../../domain/http/types";
import { AUTH_TYPES, BODY_TYPES } from "../../constants/http";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";
import { KeyValueEditor } from "./key-value-editor";

const PROXY_MODES: { value: ProxyMode; label: string }[] = [
  { value: "none", label: "None" },
  { value: "system", label: "System" },
  { value: "custom", label: "Custom" },
];

const TABS = [
  { value: "params", label: "Params" },
  { value: "headers", label: "Headers" },
  { value: "body", label: "Body" },
  { value: "auth", label: "Auth" },
  { value: "settings", label: "Settings" },
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
    settings,
    setSettings,
    formatJsonBody,
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
                              setBody({
                                ...body(),
                                content: formatJsonBody(e.currentTarget.value),
                              });
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

        <Show when={requestTab() === "settings"}>
          <div
            class={styles.scrollTabPanel}
            role="tabpanel"
            id="tabpanel-settings"
            aria-labelledby="tab-settings"
          >
            <div class={styles.settingsSection}>
              <div class={styles.settingsRow}>
                <label for="setting-timeout" class={styles.settingsLabel}>
                  Timeout (s)
                </label>
                <Input
                  id="setting-timeout"
                  type="number"
                  class={styles.settingsNumberInput}
                  value={
                    settings().timeoutSec === 0 ? "" : settings().timeoutSec
                  }
                  placeholder="30"
                  min="0"
                  onInput={(e) => {
                    const v = parseInt(e.currentTarget.value, 10);
                    setSettings({
                      ...settings(),
                      timeoutSec: Number.isNaN(v) ? 0 : v,
                    });
                  }}
                />
              </div>

              <div class={styles.settingsRow}>
                <label for="setting-max-body" class={styles.settingsLabel}>
                  Max Response Body (MB)
                </label>
                <Input
                  id="setting-max-body"
                  type="number"
                  class={styles.settingsNumberInput}
                  value={
                    settings().maxResponseBodyMB === 0
                      ? ""
                      : settings().maxResponseBodyMB
                  }
                  placeholder="10"
                  min="1"
                  onInput={(e) => {
                    const v = parseInt(e.currentTarget.value, 10);
                    setSettings({
                      ...settings(),
                      maxResponseBodyMB: Number.isNaN(v) ? 0 : v,
                    });
                  }}
                />
              </div>

              <div class={styles.settingsRow}>
                <span class={styles.settingsLabel}>Proxy</span>
                <Select
                  value={settings().proxyMode}
                  onValueChange={(v) =>
                    setSettings({ ...settings(), proxyMode: v as ProxyMode })
                  }
                >
                  <SelectTrigger class={styles.settingsSelectTrigger}>
                    <SelectValue placeholder="System" />
                  </SelectTrigger>
                  <SelectContent>
                    {PROXY_MODES.map((pm) => (
                      <SelectItem value={pm.value}>{pm.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <Show when={settings().proxyMode === "custom"}>
                <div class={styles.settingsRow}>
                  <label for="setting-proxy-url" class={styles.settingsLabel}>
                    Proxy URL
                  </label>
                  <Input
                    id="setting-proxy-url"
                    class={styles.settingsInput}
                    value={settings().proxyURL}
                    placeholder="http://proxy:8080"
                    onInput={(e) =>
                      setSettings({
                        ...settings(),
                        proxyURL: e.currentTarget.value,
                      })
                    }
                  />
                </div>
              </Show>

              <div class={styles.settingsCheckRow}>
                <input
                  type="checkbox"
                  id="setting-insecure"
                  class={styles.settingsCheckbox}
                  checked={!settings().insecureSkipVerify}
                  onChange={(e) =>
                    setSettings({
                      ...settings(),
                      insecureSkipVerify: !e.currentTarget.checked,
                    })
                  }
                />
                <label for="setting-insecure" class={styles.settingsCheckLabel}>
                  Verify TLS certificate
                </label>
              </div>

              <div class={styles.settingsCheckRow}>
                <input
                  type="checkbox"
                  id="setting-redirects"
                  class={styles.settingsCheckbox}
                  checked={settings().disableRedirects}
                  onChange={(e) =>
                    setSettings({
                      ...settings(),
                      disableRedirects: e.currentTarget.checked,
                    })
                  }
                />
                <label
                  for="setting-redirects"
                  class={styles.settingsCheckLabel}
                >
                  Disable redirects
                </label>
              </div>
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
}
