import { createMemo, For, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Textarea } from "../../../components/ui/textarea";
import {
  PAYLOAD_ENCODINGS,
  type PayloadEncoding,
} from "../../../domain/udp/types";
import { useUdpSend } from "../../providers/udp-provider";
import styles from "./udp.module.css";

function isValidAscii(value: string): boolean {
  return [...value].every((c) => (c.codePointAt(0) ?? 0) <= 0x7f);
}

export function SendForm() {
  const {
    host,
    setHost,
    port,
    setPort,
    payload,
    setPayload,
    encoding,
    setEncoding,
    fixedLengthFields,
    addField,
    updateField,
    removeField,
    loading,
    send,
  } = useUdpSend();

  const totalBytes = createMemo(() => {
    return fixedLengthFields().reduce((sum, field) => sum + field.length, 0);
  });

  return (
    <div class={styles.sendForm}>
      <div class={styles.formRow}>
        <span class={styles.formLabel}>Host</span>
        <Input
          class={styles.hostInput}
          type="text"
          placeholder="127.0.0.1"
          value={host()}
          onInput={(e) => setHost(e.currentTarget.value)}
        />
        <span class={styles.formLabel} style={{ "min-width": "2.5rem" }}>
          Port
        </span>
        <Input
          class={styles.portInput}
          type="number"
          min={1}
          max={65535}
          placeholder="12345"
          value={port() === 0 ? "" : String(port())}
          onInput={(e) => setPort(Number(e.currentTarget.value))}
        />
      </div>
      <div class={styles.formRow}>
        <span class={styles.formLabel}>Encoding</span>
        <div class={styles.encodingRadioGroup}>
          <For each={PAYLOAD_ENCODINGS}>
            {(enc) => (
              <label class={styles.encodingRadioLabel}>
                <input
                  type="radio"
                  name="encoding"
                  value={enc}
                  checked={encoding() === enc}
                  onChange={() => setEncoding(enc as PayloadEncoding)}
                  class={styles.encodingRadioInput}
                />
                {enc}
              </label>
            )}
          </For>
        </div>
      </div>

      <Show when={encoding() === "fixed"}>
        <div class={styles.fieldsContainer}>
          <div class={styles.fieldsHeader}>
            <span class={styles.formLabel}>Fields</span>
            <span class={styles.totalBytes}>Total: {totalBytes()} bytes</span>
          </div>

          <For each={fixedLengthFields()}>
            {(field) => {
              const isAsciiValid = () => isValidAscii(field.value);
              const byteCount = () => field.value.length;

              return (
                <div class={styles.fieldItem}>
                  <div class={styles.fieldRow}>
                    <div class={styles.fieldGroup}>
                      <span class={styles.fieldLabel}>Name</span>
                      <Input
                        class={styles.fieldInput}
                        type="text"
                        placeholder="Field name"
                        value={field.name}
                        aria-label="Field name"
                        onInput={(e) =>
                          updateField(field.id, { name: e.currentTarget.value })
                        }
                      />
                    </div>
                    <div class={styles.fieldGroup}>
                      <span class={styles.fieldLabel}>Length</span>
                      <Input
                        class={styles.fieldLengthInput}
                        type="number"
                        min={1}
                        placeholder="1"
                        value={field.length}
                        aria-label="Field length"
                        onInput={(e) =>
                          updateField(field.id, {
                            length: Number(e.currentTarget.value),
                          })
                        }
                      />
                    </div>
                  </div>

                  <div class={styles.fieldRow}>
                    <div class={styles.fieldGroup} style={{ flex: 1 }}>
                      <span class={styles.fieldLabel}>Value (UTF-8)</span>
                      <Input
                        class={styles.fieldValueInput}
                        type="text"
                        placeholder="hello"
                        value={field.value}
                        aria-label="Field value (UTF-8)"
                        onInput={(e) =>
                          updateField(field.id, {
                            value: e.currentTarget.value,
                          })
                        }
                        style={{
                          "border-color": !isAsciiValid()
                            ? "#ef4444"
                            : undefined,
                        }}
                      />
                      <span
                        class={styles.fieldHint}
                        style={{
                          color: !isAsciiValid()
                            ? "#ef4444"
                            : byteCount() > field.length
                              ? "#f59e0b"
                              : undefined,
                        }}
                      >
                        {!isAsciiValid()
                          ? "ASCII only (1 char = 1 byte)"
                          : `${byteCount()}/${field.length} bytes`}
                      </span>
                    </div>
                    <Button
                      class={styles.deleteFieldButton}
                      variant="ghost"
                      size="sm"
                      onClick={() => removeField(field.id)}
                    >
                      ✕
                    </Button>
                  </div>
                </div>
              );
            }}
          </For>

          <Button
            class={styles.addFieldButton}
            variant="outline"
            size="sm"
            onClick={() => addField()}
          >
            + Add Field
          </Button>
        </div>
      </Show>

      <Show when={encoding() !== "fixed"}>
        <div class={`${styles.formRow} ${styles.payloadRow}`}>
          <span
            class={styles.formLabel}
            style={{ "align-self": "flex-start", "padding-top": "0.375rem" }}
          >
            Payload
          </span>
          <Textarea
            class={styles.payloadTextarea}
            placeholder={
              encoding() === "json" ? '{"key": "value"}' : "Enter payload..."
            }
            value={payload()}
            onInput={(e) => setPayload(e.currentTarget.value)}
          />
        </div>
      </Show>

      <Button
        class={styles.sendButton}
        disabled={loading()}
        onClick={() => {
          send().catch((err: unknown) => {
            console.error("UDP send failed:", err);
          });
        }}
      >
        {loading() ? "Sending..." : "Send"}
      </Button>
    </div>
  );
}
