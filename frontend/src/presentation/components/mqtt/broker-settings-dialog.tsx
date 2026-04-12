import { createMemo, createSignal, onMount, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import type { BrokerProfile } from "../../../domain/mqtt/types";
import styles from "./mqtt.module.css";

const BROKER_PATTERN = /^(mqtt|mqtts|tcp|ws|wss):\/\/.+/;

function createEmptyProfile(): BrokerProfile {
  return {
    id: Date.now().toString(),
    name: "",
    broker: "tcp://localhost:1883",
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

  const update = <K extends keyof BrokerProfile>(
    key: K,
    value: BrokerProfile[K],
  ) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
  };

  const isValid = createMemo(() => {
    const d = draft();
    return d.name.trim().length > 0 && BROKER_PATTERN.test(d.broker.trim());
  });

  const handleSave = () => {
    if (!isValid()) return;
    props.onSave(draft());
  };

  const handleSaveAndConnect = () => {
    if (!isValid()) return;
    props.onSaveAndConnect?.(draft());
  };

  const getFocusable = () =>
    Array.from(
      cardRef?.querySelectorAll<HTMLElement>(
        "input:not([disabled]), button:not([disabled])",
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

            <label class={styles.dialogLabel} for="broker-url">
              Broker URL
              <Input
                id="broker-url"
                value={draft().broker}
                onInput={(e) => update("broker", e.currentTarget.value)}
                placeholder="tcp://localhost:1883"
              />
            </label>

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
