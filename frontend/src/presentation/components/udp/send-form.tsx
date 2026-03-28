import { createMemo, For, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import {
  PAYLOAD_ENCODINGS,
  type PayloadEncoding,
} from "../../../domain/udp/types";
import { useUdpSend } from "../../providers/udp-provider";
import styles from "./udp.module.css";

function isValidHex(value: string): boolean {
  if (!value) return true;
  const cleaned = value.replace(/\s/g, "");
  return /^([0-9A-Fa-f]{2})*$/.test(cleaned);
}

function hexByteCount(value: string): number {
  const cleaned = value.replace(/\s/g, "");
  return Math.ceil(cleaned.length / 2);
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
    messageLength,
    setMessageLength,
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
        <Select
          value={encoding()}
          onValueChange={(v) => setEncoding(v as PayloadEncoding)}
        >
          <SelectTrigger class={styles.encodingSelect}>
            <SelectValue placeholder="encoding" />
          </SelectTrigger>
          <SelectContent>
            <For each={PAYLOAD_ENCODINGS}>
              {(enc) => <SelectItem value={enc}>{enc}</SelectItem>}
            </For>
          </SelectContent>
        </Select>
      </div>

      <Show when={encoding() !== "fixed"}>
        <div class={styles.formRow}>
          <span class={styles.formLabel}>Length</span>
          <Input
            class={styles.messageLengthInput}
            type="number"
            min={1}
            placeholder="32"
            value={messageLength() === 0 ? "" : String(messageLength())}
            onInput={(e) => setMessageLength(Number(e.currentTarget.value))}
          />
          <span class={styles.formLabelSub}>bytes</span>
        </div>
      </Show>

      <Show when={encoding() === "fixed"}>
        <div class={styles.fieldsContainer}>
          <div class={styles.fieldsHeader}>
            <span class={styles.formLabel}>Fields</span>
            <span class={styles.totalBytes}>Total: {totalBytes()} bytes</span>
          </div>

          <For each={fixedLengthFields()}>
            {(field) => {
              const isHexValid = () => isValidHex(field.value);
              const hexBytes = () => hexByteCount(field.value);

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
                      <span class={styles.fieldLabel}>Value (HEX)</span>
                      <Input
                        class={styles.fieldValueInput}
                        type="text"
                        placeholder="DEADBEEF"
                        value={field.value}
                        aria-label="Field value (HEX)"
                        onInput={(e) =>
                          updateField(field.id, {
                            value: e.currentTarget.value,
                          })
                        }
                        style={{
                          "border-color": !isHexValid() ? "#ef4444" : undefined,
                        }}
                      />
                      <span
                        class={styles.fieldHint}
                        style={{
                          color: !isHexValid()
                            ? "#ef4444"
                            : hexBytes() > field.length
                              ? "#f59e0b"
                              : undefined,
                        }}
                      >
                        {!isHexValid()
                          ? "Invalid HEX"
                          : `${hexBytes()}/${field.length} bytes`}
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
          <textarea
            class={styles.payloadTextarea}
            placeholder={
              encoding() === "hex"
                ? "DE AD BE EF ..."
                : encoding() === "base64"
                  ? "SGVsbG8gV29ybGQ="
                  : encoding() === "json"
                    ? '{"key": "value"}'
                    : "Enter payload..."
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
