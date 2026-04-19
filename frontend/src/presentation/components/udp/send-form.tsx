import { createMemo, For, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Textarea } from "../../../components/ui/textarea";
import {
  ENDIANNESSES,
  FIELD_TYPE_SIZES,
  FIELD_TYPES,
  type FieldType,
  isValidNumericFieldValue,
  PAYLOAD_ENCODINGS,
  type PayloadEncoding,
} from "../../../domain/udp/types";
import { useUdpSend } from "../../providers/udp-provider";
import styles from "./udp.module.css";

function isValidAscii(value: string): boolean {
  return [...value].every((c) => (c.codePointAt(0) ?? 0) <= 0x7f);
}

function isValidHex(value: string): boolean {
  const cleaned = value.replace(/\s/g, "");
  return cleaned.length % 2 === 0 && /^[0-9a-fA-F]*$/.test(cleaned);
}

function hexByteCount(value: string): number {
  return value.replace(/\s/g, "").length / 2;
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
    endianness,
    setEndianness,
    fixedLengthFields,
    addField,
    updateField,
    removeField,
    loading,
    send,
  } = useUdpSend();

  const totalBytes = createMemo(() =>
    fixedLengthFields.reduce((sum, field) => {
      const fixedSize = FIELD_TYPE_SIZES[field.fieldType];
      return sum + (fixedSize !== undefined ? fixedSize : field.length);
    }, 0),
  );

  return (
    <div class={styles.sendForm}>
      <div class={styles.formRow}>
        <span class={styles.formLabel}>Host</span>
        <Input
          placeholder="127.0.0.1"
          value={host()}
          onInput={(e) => setHost(e.currentTarget.value)}
        />
        <span class={styles.formLabelInline}>Port</span>
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
            <div class={styles.fieldsHeaderRight}>
              <span class={styles.totalBytes}>Total: {totalBytes()} bytes</span>
              <select
                class={styles.endiannessSelect}
                value={endianness()}
                onChange={(e) =>
                  setEndianness(
                    e.currentTarget.value as (typeof ENDIANNESSES)[number],
                  )
                }
              >
                <For each={ENDIANNESSES}>
                  {(e) => <option value={e}>{e}-endian</option>}
                </For>
              </select>
            </div>
          </div>

          <For each={fixedLengthFields}>
            {(field, index) => {
              const ft = () => field.fieldType;
              const isVarLength = () => ft() === "string" || ft() === "bytes";
              const fixedSize = () => FIELD_TYPE_SIZES[ft()];

              const isValueValid = () => {
                if (ft() === "string") return isValidAscii(field.value);
                if (ft() === "bytes") return isValidHex(field.value);
                return isValidNumericFieldValue(field.value, ft());
              };

              const byteCount = () => {
                if (ft() === "bytes") return hexByteCount(field.value);
                return field.value.length;
              };

              const byteCountClass = () => {
                if (!isValueValid()) return styles.byteCountError;
                if (isVarLength() && byteCount() > field.length)
                  return styles.byteCountWarn;
                return styles.byteCountOk;
              };

              const byteCountLabel = () => {
                if (ft() === "string") {
                  if (!isValidAscii(field.value)) return "non-ASCII";
                  return `${byteCount()}/${field.length}`;
                }
                if (ft() === "bytes") {
                  if (!isValidHex(field.value)) return "invalid hex";
                  return `${byteCount()}/${field.length}`;
                }
                if (field.value !== "" && !isValueValid())
                  return "out of range";
                return `${fixedSize()} bytes`;
              };

              const valueLabel = () => {
                if (ft() === "bytes") return "Value (hex)";
                if (ft() === "string") return "Value (ASCII)";
                return "Value";
              };

              const valuePlaceholder = () => {
                if (ft() === "bytes") return "0a 1b 2c";
                if (ft() === "string") return "hello";
                if (ft() === "float32" || ft() === "float64") return "1.0";
                return "0";
              };

              const valueInputType = () => {
                if (
                  ft() === "string" ||
                  ft() === "bytes" ||
                  ft() === "int64" ||
                  ft() === "uint64"
                )
                  return "text";
                return "number";
              };

              return (
                <div class={styles.fieldItem}>
                  <div class={styles.fieldMainRow}>
                    <span class={styles.fieldNumber}>#{index() + 1}</span>
                    <div class={styles.fieldGroup}>
                      <span class={styles.fieldLabel}>Name</span>
                      <Input
                        class={styles.fieldInput}
                        type="text"
                        placeholder="field name"
                        value={field.name}
                        aria-label="Field name"
                        onInput={(e) =>
                          updateField(field.id, {
                            name: e.currentTarget.value,
                          })
                        }
                      />
                    </div>
                    <div class={styles.fieldTypeGroup}>
                      <span class={styles.fieldLabel}>Type</span>
                      <select
                        class={styles.fieldTypeSelect}
                        value={field.fieldType}
                        aria-label="Field type"
                        onChange={(e) =>
                          updateField(field.id, {
                            fieldType: e.currentTarget.value as FieldType,
                          })
                        }
                      >
                        <For each={FIELD_TYPES}>
                          {(t) => <option value={t}>{t}</option>}
                        </For>
                      </select>
                    </div>
                    <Show when={isVarLength()}>
                      <div class={styles.fieldLengthGroup}>
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
                    </Show>
                    <Button
                      class={styles.deleteFieldButton}
                      variant="ghost"
                      size="sm"
                      aria-label="Delete field"
                      onClick={() => removeField(field.id)}
                    >
                      ✕
                    </Button>
                  </div>

                  <div class={styles.fieldValueRow}>
                    <div class={styles.fieldGroup}>
                      <span class={styles.fieldLabel}>{valueLabel()}</span>
                      <Input
                        class={styles.fieldValueInput}
                        type={valueInputType()}
                        placeholder={valuePlaceholder()}
                        value={field.value}
                        aria-label="Field value"
                        onInput={(e) =>
                          updateField(field.id, {
                            value: e.currentTarget.value,
                          })
                        }
                        style={{
                          "border-color": !isValueValid()
                            ? "#ef4444"
                            : undefined,
                        }}
                      />
                    </div>
                    <span
                      class={`${styles.byteCountBadge} ${byteCountClass()}`}
                    >
                      {byteCountLabel()}
                    </span>
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
