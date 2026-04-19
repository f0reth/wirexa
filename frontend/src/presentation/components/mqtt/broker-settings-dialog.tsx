import { createMemo, createSignal, onMount, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import type { BrokerProfile } from "../../../domain/mqtt/types";
import styles from "./mqtt.module.css";

function defaultPort(scheme: string): string {
  const map: Record<string, string> = {
    mqtt: "1883",
    mqtts: "8883",
    tcp: "1883",
    ws: "9001",
    wss: "8884",
  };
  return map[scheme] ?? "1883";
}

function parseBrokerUrl(url: string): {
  scheme: string;
  host: string;
  port: string;
} {
  const match = url.match(/^(mqtt|mqtts|tcp|ws|wss):\/\/([^:]+)(?::(\d+))?$/);
  if (!match) return { scheme: "mqtt", host: "localhost", port: "1883" };
  return {
    scheme: match[1],
    host: match[2],
    port: match[3] ?? defaultPort(match[1]),
  };
}

function composeBrokerUrl(scheme: string, host: string, port: string): string {
  return `${scheme}://${host}:${port}`;
}

function createEmptyProfile(): BrokerProfile {
  return {
    id: Date.now().toString(),
    name: "",
    broker: "mqtt://localhost:1883",
    clientId: "",
    username: "",
    password: "",
    useTls: false,
  };
}

export function BrokerSettingsDialog(props: {
  profile?: BrokerProfile;
  onSave: (profile: BrokerProfile) => void;
  onSaveAndConnect?: (profile: BrokerProfile) => void;
  onClose: () => void;
}) {
  let cardRef: HTMLDivElement | undefined;

  const [draft, setDraft] = createSignal<BrokerProfile>(
    props.profile ? { ...props.profile } : createEmptyProfile(),
  );

  const initialParts = parseBrokerUrl(
    props.profile?.broker ?? "mqtt://localhost:1883",
  );
  const [scheme, setScheme] = createSignal(initialParts.scheme);
  const [host, setHost] = createSignal(initialParts.host);
  const [port, setPort] = createSignal(initialParts.port);

  const update = <K extends keyof BrokerProfile>(
    key: K,
    value: BrokerProfile[K],
  ) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
  };

  const handleSchemeChange = (s: string) => {
    setScheme(s);
    setPort(defaultPort(s));
  };

  const isValid = createMemo(() => {
    const portNum = Number(port());
    return (
      draft().name.trim().length > 0 &&
      host().trim().length > 0 &&
      Number.isInteger(portNum) &&
      portNum >= 1 &&
      portNum <= 65535
    );
  });

  const handleSave = () => {
    if (!isValid()) return;
    props.onSave({
      ...draft(),
      broker: composeBrokerUrl(scheme(), host(), port()),
    });
  };

  const handleSaveAndConnect = () => {
    if (!isValid()) return;
    props.onSaveAndConnect?.({
      ...draft(),
      broker: composeBrokerUrl(scheme(), host(), port()),
    });
  };

  const getFocusable = () =>
    Array.from(
      cardRef?.querySelectorAll<HTMLElement>(
        "input:not([disabled]), select:not([disabled]), button:not([disabled])",
      ) ?? [],
    );

  const handleCardKeyDown = (e: KeyboardEvent) => {
    e.stopPropagation();
    if (e.key === "Escape") {
      props.onClose();
      return;
    }
    if (e.key === "Tab") {
      const focusable = getFocusable();
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault();
          last.focus();
        }
      } else {
        if (document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
  };

  onMount(() => {
    getFocusable()[0]?.focus();
  });

  return (
    <>
      {/* biome-ignore lint/a11y/useSemanticElements: overlay backdrop requires block-level display; button element cannot serve as full-screen backdrop */}
      <div
        class={styles.dialogOverlay}
        role="button"
        tabIndex={-1}
        aria-label="Close dialog"
        onClick={() => props.onClose()}
        onKeyDown={(e) => e.key === "Escape" && props.onClose()}
      >
        <div
          ref={cardRef}
          class={styles.dialogCard}
          role="dialog"
          aria-modal="true"
          aria-label={props.profile ? "Edit Profile" : "New Profile"}
          onClick={(e) => e.stopPropagation()}
          onKeyDown={handleCardKeyDown}
        >
          <h3 class={styles.dialogTitle}>
            {props.profile ? "Edit Profile" : "New Profile"}
          </h3>

          <div class={styles.dialogForm}>
            <label class={styles.dialogLabel} for="broker-name">
              Name
              <Input
                id="broker-name"
                value={draft().name}
                onInput={(e) => update("name", e.currentTarget.value)}
                placeholder="My Broker"
              />
            </label>

            <div class={styles.dialogLabel}>
              Broker URL
              <div class={styles.brokerInputRow}>
                <select
                  id="broker-scheme"
                  class={styles.brokerSchemeSelect}
                  value={scheme()}
                  onChange={(e) => handleSchemeChange(e.currentTarget.value)}
                >
                  <option value="mqtt">mqtt</option>
                  <option value="mqtts">mqtts</option>
                  <option value="tcp">tcp</option>
                  <option value="ws">ws</option>
                  <option value="wss">wss</option>
                </select>
                <Input
                  id="broker-host"
                  class={styles.brokerHostInput}
                  value={host()}
                  onInput={(e) => setHost(e.currentTarget.value)}
                  placeholder="localhost"
                />
                <Input
                  id="broker-port"
                  class={styles.brokerPortInput}
                  type="number"
                  min={1}
                  max={65535}
                  value={port()}
                  onInput={(e) => setPort(e.currentTarget.value)}
                  placeholder="1883"
                />
              </div>
            </div>

            <label class={styles.dialogLabel} for="broker-client-id">
              Client ID
              <Input
                id="broker-client-id"
                value={draft().clientId}
                onInput={(e) => update("clientId", e.currentTarget.value)}
                placeholder="(auto-generated if empty)"
              />
            </label>

            <label class={styles.dialogLabel} for="broker-username">
              Username
              <Input
                id="broker-username"
                value={draft().username}
                onInput={(e) => update("username", e.currentTarget.value)}
                placeholder="(optional)"
              />
            </label>

            <label class={styles.dialogLabel} for="broker-password">
              Password
              <input
                id="broker-password"
                type="password"
                value={draft().password}
                onInput={(e) => update("password", e.currentTarget.value)}
                placeholder="(optional)"
                class={styles.dialogPasswordInput}
              />
            </label>

            <label class={styles.dialogCheckboxLabel} for="broker-tls">
              <input
                id="broker-tls"
                type="checkbox"
                checked={draft().useTls}
                onChange={(e) => update("useTls", e.currentTarget.checked)}
              />
              Use TLS
            </label>
          </div>

          <div class={styles.dialogActions}>
            <Button variant="outline" onClick={() => props.onClose()}>
              Cancel
            </Button>
            <Show when={props.onSaveAndConnect !== undefined}>
              <Button
                variant="outline"
                onClick={handleSaveAndConnect}
                disabled={!isValid()}
              >
                Save & Connect
              </Button>
            </Show>
            <Button onClick={handleSave} disabled={!isValid()}>
              Save
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}
