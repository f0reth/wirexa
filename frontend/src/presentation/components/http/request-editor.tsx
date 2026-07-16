import { createSignal, Match, Show, Switch } from "solid-js";
import { Input } from "../../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import { TabList, TabPanel } from "../../../components/ui/tabs";
import { Textarea } from "../../../components/ui/textarea";
import type {
  AuthType,
  BodyType,
  KeyValuePair,
} from "../../../domain/http/types";
import { FORM_PAIR_FIELDS, isFormBodyType } from "../../../domain/http/types";
import { openFilePicker } from "../../../infrastructure/http/client";
import { AUTH_TYPES, BODY_TYPES } from "../../constants/http";
import { useHttpRequest } from "../../providers/http-provider";
import { DocEditor } from "./doc-editor";
import styles from "./http.module.css";
import { JsonBodyEditor } from "./json-body-editor";
import { KeyValueEditor } from "./key-value-editor";
import { RequestSettingsPanel } from "./request-settings-panel";

const JSON_BODY_DEFAULT = '{\n  "": ""\n}';

// 行が無いときに毎回新しい配列を作らないよう、空配列の同一性を固定する。
const EMPTY_PAIRS: KeyValuePair[] = [];

const TABS = [
  { value: "params", label: "Params" },
  { value: "headers", label: "Headers" },
  { value: "body", label: "Body" },
  { value: "auth", label: "Auth" },
  { value: "settings", label: "Settings" },
  { value: "doc", label: "Doc" },
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
    doc,
    setDoc,
  } = useHttpRequest();

  const bodyContent = () => {
    const content = body().contents[body().type];
    if (content === undefined && body().type === "json") {
      return JSON_BODY_DEFAULT;
    }
    return content ?? "";
  };
  const setBodyContent = (content: string) =>
    setBody({
      ...body(),
      contents: { ...body().contents, [body().type]: content },
    });

  // form 系の行は body の専用フィールドそのものを読み書きする。
  // 文字列へ畳んで導出し直すと空キー行が直列化で落ちて Add が効かなくなるため、
  // Params/Headers と同じく実体の state を KeyValueEditor に直結させる。
  const formField = () => {
    const type = body().type;
    return isFormBodyType(type) ? FORM_PAIR_FIELDS[type] : null;
  };
  const isFormBody = () => formField() !== null;
  const formPairs = () => {
    const field = formField();
    return field ? (body()[field] ?? EMPTY_PAIRS) : EMPTY_PAIRS;
  };
  const setFormPairs = (pairs: KeyValuePair[]) => {
    const field = formField();
    if (!field) return;
    setBody({ ...body(), [field]: pairs });
  };

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
        <TabPanel
          value="params"
          active={requestTab()}
          class={styles.scrollTabPanel}
        >
          <KeyValueEditor
            pairs={params()}
            onChange={(pairs) => setParams(pairs)}
            keyPlaceholder="Parameter"
            valuePlaceholder="Value"
          />
        </TabPanel>

        <TabPanel
          value="headers"
          active={requestTab()}
          class={styles.scrollTabPanel}
        >
          <KeyValueEditor
            pairs={headers()}
            onChange={(pairs) => setHeaders(pairs)}
            keyPlaceholder="Header"
            valuePlaceholder="Value"
          />
        </TabPanel>

        <TabPanel
          value="body"
          active={requestTab()}
          class={styles.bodyTabPanel}
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
              <Switch>
                <Match when={body().type === "file"}>
                  <div class={styles.filePickerRow}>
                    <Input
                      value={bodyContent()}
                      placeholder="No file selected"
                      onInput={(e) => setBodyContent(e.currentTarget.value)}
                      class={styles.filePathInput}
                    />
                    <button
                      type="button"
                      class={styles.fileBrowseButton}
                      onClick={async () => {
                        const path = await openFilePicker();
                        if (path) {
                          setBodyContent(path);
                        }
                      }}
                    >
                      Browse...
                    </button>
                  </div>
                </Match>
                <Match when={isFormBody()}>
                  <KeyValueEditor
                    pairs={formPairs()}
                    onChange={setFormPairs}
                    keyPlaceholder="Field"
                    valuePlaceholder="Value"
                  />
                </Match>
                <Match when={body().type === "json"}>
                  <JsonBodyEditor
                    value={bodyContent()}
                    onChange={(content) => setBodyContent(content)}
                  />
                </Match>
                <Match when={true}>
                  <Textarea
                    value={bodyContent()}
                    onInput={(e) => setBodyContent(e.currentTarget.value)}
                    placeholder="Enter body content..."
                    class={styles.bodyTextarea}
                  />
                </Match>
              </Switch>
            </Show>
          </div>
        </TabPanel>

        <TabPanel
          value="auth"
          active={requestTab()}
          class={styles.scrollTabPanel}
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
        </TabPanel>

        <TabPanel
          value="settings"
          active={requestTab()}
          class={styles.scrollTabPanel}
        >
          <RequestSettingsPanel />
        </TabPanel>

        <TabPanel value="doc" active={requestTab()} class={styles.bodyTabPanel}>
          <DocEditor value={doc()} onChange={(v) => setDoc(v)} />
        </TabPanel>
      </div>
    </div>
  );
}
