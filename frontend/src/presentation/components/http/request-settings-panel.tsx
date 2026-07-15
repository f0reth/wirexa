import { Show } from "solid-js";
import { Input } from "../../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import type { ProxyMode } from "../../../domain/http/types";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";

const PROXY_MODES: { value: ProxyMode; label: string }[] = [
  { value: "none", label: "None" },
  { value: "system", label: "System" },
  { value: "custom", label: "Custom" },
];

export function RequestSettingsPanel() {
  const { settings, setSettings } = useHttpRequest();

  return (
    <div class={styles.settingsSection}>
      <div class={styles.settingsRow}>
        <label for="setting-timeout" class={styles.settingsLabel}>
          Timeout (s)
        </label>
        <Input
          id="setting-timeout"
          type="number"
          class={styles.settingsNumberInput}
          value={settings().timeoutSec === 0 ? "" : settings().timeoutSec}
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
        <label for="setting-redirects" class={styles.settingsCheckLabel}>
          Disable redirects
        </label>
      </div>
    </div>
  );
}
