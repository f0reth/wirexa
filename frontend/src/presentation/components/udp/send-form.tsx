import { For, Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import { Textarea } from "../../../components/ui/textarea";
import {
  PAYLOAD_ENCODINGS,
  type PayloadEncoding,
} from "../../../domain/udp/types";
import { useUdpSend } from "../../providers/udp-provider";
import styles from "./udp.module.css";

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
    loading,
    send,
  } = useUdpSend();

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
      <Show when={encoding() === "fixed"}>
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
      <div class={styles.formRow} style={{ "align-items": "flex-start" }}>
        <span class={styles.formLabel} style={{ "padding-top": "0.375rem" }}>
          Payload
        </span>
        <Textarea
          class={styles.payloadTextarea}
          placeholder={
            encoding() === "fixed"
              ? "DE AD BE EF ..."
              : encoding() === "hex"
                ? "DE AD BE EF ..."
                : encoding() === "json"
                  ? '{"key": "value"}'
                  : "Enter payload..."
          }
          value={payload()}
          onInput={(e) => setPayload(e.currentTarget.value)}
        />
      </div>
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
