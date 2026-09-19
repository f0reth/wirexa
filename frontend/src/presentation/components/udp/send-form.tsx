import { clsx } from "clsx";
import { createMemo, For, Show } from "solid-js";
import {
  type ByteCountStatus,
  fieldByteCountLabel,
  fieldByteCountStatus,
  fieldValueInputType,
  fieldValueLabel,
  fieldValuePlaceholder,
  isValidFieldValue,
  isVarLengthFieldType,
  totalFieldBytes,
} from "../../../application/udp/field-validation";
import { notify } from "../../../application/ui/notifications";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Textarea } from "../../../components/ui/textarea";
import {
  ENDIANNESSES,
  FIELD_TYPES,
  type FieldType,
  PAYLOAD_ENCODINGS,
  type PayloadEncoding,
} from "../../../domain/udp/types";
import { errorMessage } from "../../../shared/error";
import { useUdpSend } from "../../providers/udp-provider";
import styles from "./udp.module.css";

const BYTE_COUNT_CLASSES: Record<ByteCountStatus, string> = {
  ok: styles.byteCountOk,
  warn: styles.byteCountWarn,
  error: styles.byteCountError,
};

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

  const totalBytes = createMemo(() => totalFieldBytes(fixedLengthFields));

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
              const isVarLength = () => isVarLengthFieldType(field.fieldType);
              const isValueValid = () => isValidFieldValue(field);
              const byteCountClass = () =>
                BYTE_COUNT_CLASSES[fieldByteCountStatus(field)];
              const byteCountLabel = () => fieldByteCountLabel(field);
              const valueLabel = () => fieldValueLabel(field.fieldType);
              const valuePlaceholder = () =>
                fieldValuePlaceholder(field.fieldType);
              const valueInputType = () => fieldValueInputType(field.fieldType);

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
                            ? "var(--color-destructive)"
                            : undefined,
                        }}
                      />
                    </div>
                    <span class={clsx(styles.byteCountBadge, byteCountClass())}>
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
        <div class={clsx(styles.formRow, styles.payloadRow)}>
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
            notify.error("Failed to send packet", errorMessage(err));
          });
        }}
      >
        {loading() ? "Sending..." : "Send"}
      </Button>
    </div>
  );
}
