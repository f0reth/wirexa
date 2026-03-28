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
import {
  PAYLOAD_ENCODINGS,
  type PayloadEncoding,
} from "../../../domain/udp/types";
import { useUdpReceive } from "../../providers/udp-provider";
import styles from "./udp.module.css";

export function ListenForm() {
  const {
    listenPort,
    setListenPort,
    listenEncoding,
    setListenEncoding,
    loading,
    error,
    sessions,
    startListen,
    stopListen,
  } = useUdpReceive();

  return (
    <div class={styles.listenForm}>
      <div class={styles.formRow}>
        <span class={styles.formLabel}>Port</span>
        <Input
          class={styles.portInput}
          type="number"
          min={1}
          max={65535}
          placeholder="12345"
          value={listenPort() === 0 ? "" : String(listenPort())}
          onInput={(e) => setListenPort(Number(e.currentTarget.value))}
        />
        <span class={styles.formLabel} style={{ "min-width": "5.5rem" }}>
          Encoding
        </span>
        <Select
          value={listenEncoding()}
          onValueChange={(v) => setListenEncoding(v as PayloadEncoding)}
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
        <Button
          disabled={loading()}
          onClick={() => {
            startListen().catch((err: unknown) => {
              console.error("UDP listen failed:", err);
            });
          }}
        >
          {loading() ? "Starting..." : "Start"}
        </Button>
      </div>
      <Show when={error()}>
        {(msg) => <p class={styles.listenError}>{msg()}</p>}
      </Show>
      <div class={styles.sessionList}>
        <For each={sessions}>
          {(session) => (
            <div class={styles.sessionItem}>
              <span class={styles.sessionBadge}>
                :{session.port} ({session.encoding})
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  stopListen(session.id).catch((err: unknown) => {
                    console.error("UDP stop listen failed:", err);
                  });
                }}
              >
                Stop
              </Button>
            </div>
          )}
        </For>
      </div>
    </div>
  );
}
