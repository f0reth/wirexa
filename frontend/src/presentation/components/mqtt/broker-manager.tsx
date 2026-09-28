import { clsx } from "clsx";
import { Wifi, WifiOff, Zap } from "lucide-solid";
import { createEffect, createSignal, on, Show } from "solid-js";
import {
  composeBrokerUrl,
  defaultPort,
  parseBrokerUrl,
} from "../../../application/mqtt/broker-url";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { isConnected } from "../../../domain/mqtt/types";
import { useMqttConnection } from "../../providers/mqtt-provider";
import styles from "./broker.module.css";

export function BrokerManager() {
  const {
    activeConnection,
    handleDisconnect,
    handleReconnect,
    updateConnectionBroker,
  } = useMqttConnection();

  return (
    <Show when={activeConnection()}>
      {(conn) => {
        const initial = parseBrokerUrl(conn().profile.broker);
        const [scheme, setScheme] = createSignal(initial.scheme);
        const [host, setHost] = createSignal(initial.host);
        const [port, setPort] = createSignal(initial.port);

        // Show は接続を切り替えても作り直さないので、別のブローカーへの切り替えやダイアログでの
        // 編集で broker URL が外から変わったら入力欄を読み直す。読み直さないと、前のブローカーの
        // 値のまま編集して、切り替え先のプロファイルを上書き保存してしまう。入力中は入力欄から
        // 組み立てた URL がそのまま書き戻されるので読み直さない (途中の空欄を既定値で埋めない)。
        createEffect(
          on(
            () => conn().profile.broker,
            (broker) => {
              if (broker === composeBrokerUrl(scheme(), host(), port())) return;
              const parts = parseBrokerUrl(broker);
              setScheme(parts.scheme);
              setHost(parts.host);
              setPort(parts.port);
            },
            { defer: true },
          ),
        );

        const handleSchemeChange = (s: string) => {
          const p = defaultPort(s);
          setScheme(s);
          setPort(p);
          updateConnectionBroker(
            conn().connectionId,
            composeBrokerUrl(s, host(), p),
          );
        };
        const handleHostChange = (h: string) => {
          setHost(h);
          updateConnectionBroker(
            conn().connectionId,
            composeBrokerUrl(scheme(), h, port()),
          );
        };
        const handlePortChange = (p: string) => {
          setPort(p);
          updateConnectionBroker(
            conn().connectionId,
            composeBrokerUrl(scheme(), host(), p),
          );
        };

        return (
          <div class={styles.connectionInfoBar}>
            <div class={styles.connectionInfoLeft}>
              <Show
                when={isConnected(conn())}
                fallback={
                  <WifiOff size={14} color="var(--color-muted-foreground)" />
                }
              >
                <Wifi size={14} color="var(--color-success)" />
              </Show>
              <span
                class={clsx(
                  styles.connectionInfoStatus,
                  isConnected(conn())
                    ? styles.statusConnected
                    : styles.statusDisconnected,
                )}
              >
                {isConnected(conn()) ? "Connected" : "Disconnected"}
              </span>
              <Show
                when={isConnected(conn())}
                fallback={
                  <div class={styles.connectionInfoBrokerRow}>
                    <select
                      class={styles.connectionInfoSchemeSelect}
                      value={scheme()}
                      onChange={(e) =>
                        handleSchemeChange(e.currentTarget.value)
                      }
                    >
                      <option value="mqtt">mqtt</option>
                      <option value="mqtts">mqtts</option>
                      <option value="tcp">tcp</option>
                      <option value="ws">ws</option>
                      <option value="wss">wss</option>
                    </select>
                    <Input
                      class={styles.connectionInfoHostInput}
                      value={host()}
                      onInput={(e) => handleHostChange(e.currentTarget.value)}
                      placeholder="localhost"
                    />
                    <Input
                      class={styles.connectionInfoPortInput}
                      type="number"
                      min={1}
                      max={65535}
                      value={port()}
                      onInput={(e) => handlePortChange(e.currentTarget.value)}
                      placeholder="1883"
                    />
                  </div>
                }
              >
                <span class={styles.connectionInfoBroker}>
                  {conn().profile.broker}
                </span>
              </Show>
            </div>
            <div class={styles.connectionInfoActions}>
              <Show
                when={isConnected(conn())}
                fallback={
                  <Button
                    size="sm"
                    onClick={() => handleReconnect(conn().connectionId)}
                  >
                    <Zap size={12} />
                    Connect
                  </Button>
                }
              >
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() => handleDisconnect(conn().connectionId)}
                >
                  Disconnect
                </Button>
              </Show>
            </div>
          </div>
        );
      }}
    </Show>
  );
}
