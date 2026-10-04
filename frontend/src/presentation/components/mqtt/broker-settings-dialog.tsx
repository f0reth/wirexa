import { clsx } from "clsx";
import { createMemo, createSignal, For, Show } from "solid-js";
import {
  BROKER_SCHEMES,
  composeBrokerUrl,
  defaultPort,
  parseBrokerUrl,
} from "../../../application/mqtt/broker-url";
import { isValidProfileDraft } from "../../../application/mqtt/profile-validation";
import { createEmptyProfile } from "../../../application/mqtt/profiles";
import { Button } from "../../../components/ui/button";
import dialog from "../../../components/ui/dialog.module.css";
import { createFocusTrap } from "../../../components/ui/focus-trap";
import { Input } from "../../../components/ui/input";
import type { BrokerProfile } from "../../../domain/mqtt/types";
import styles from "./broker.module.css";

export function BrokerSettingsDialog(props: {
  profile?: BrokerProfile;
  /** 編集中のプロファイルの接続が生きているか。真の間は Save を押せない（Save & Connect で張り直す）。 */
  connectionLive?: boolean;
  onSave: (profile: BrokerProfile) => void;
  onSaveAndConnect?: (profile: BrokerProfile) => void;
  onClose: () => void;
}) {
  let cardRef: HTMLDivElement | undefined;

  const initial = props.profile ? { ...props.profile } : createEmptyProfile();
  const [draft, setDraft] = createSignal<BrokerProfile>(initial);

  const initialParts = parseBrokerUrl(initial.broker);
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

  const isValid = createMemo(() =>
    isValidProfileDraft({
      name: draft().name,
      host: host(),
      port: port(),
    }),
  );

  const buildProfile = (): BrokerProfile => ({
    ...draft(),
    broker: composeBrokerUrl(scheme(), host(), port()),
  });

  const handleSave = () => {
    if (!isValid() || props.connectionLive) return;
    props.onSave(buildProfile());
  };

  const handleSaveAndConnect = () => {
    if (!isValid()) return;
    props.onSaveAndConnect?.(buildProfile());
  };

  const { onKeyDown } = createFocusTrap(
    () => cardRef,
    () => props.onClose(),
  );

  return (
    <>
      {/* biome-ignore lint/a11y/useSemanticElements: overlay backdrop requires block-level display; button element cannot serve as full-screen backdrop */}
      <div
        class={dialog.overlay}
        role="button"
        tabIndex={-1}
        aria-label="Close dialog"
        onClick={() => props.onClose()}
        onKeyDown={(e) => e.key === "Escape" && props.onClose()}
      >
        <div
          ref={cardRef}
          class={dialog.card}
          role="dialog"
          aria-modal="true"
          aria-label={props.profile ? "Edit Profile" : "New Profile"}
          onClick={(e) => e.stopPropagation()}
          onKeyDown={onKeyDown}
        >
          <h3 class={dialog.title}>
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
                  <For each={BROKER_SCHEMES}>
                    {(s) => <option value={s}>{s}</option>}
                  </For>
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

          <Show when={props.connectionLive}>
            <p class={styles.dialogNotice}>
              This broker has an active connection. Use Save & Connect to apply
              changes.
            </p>
          </Show>

          <div class={clsx(dialog.actions, styles.dialogActions)}>
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
            <Button
              onClick={handleSave}
              disabled={!isValid() || props.connectionLive}
            >
              Save
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}
