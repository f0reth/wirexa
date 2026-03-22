import clsx from "clsx";
import { Wifi, WifiOff, Zap } from "lucide-solid";
import { Show } from "solid-js";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { isConnected } from "../../../domain/mqtt/types";
import { useMqttConnection } from "../../providers/mqtt-provider";
import styles from "./mqtt.module.css";

export function BrokerManager() {
  const {
    activeConnection,
    handleDisconnect,
    handleReconnect,
    updateConnectionBroker,
  } = useMqttConnection();

  return (
    <Show when={activeConnection()}>
      {(conn) => (
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
                <Input
                  value={conn().profile.broker}
                  onInput={(e) =>
                    updateConnectionBroker(
                      conn().connectionId,
                      e.currentTarget.value,
                    )
                  }
                  placeholder="tcp://localhost:1883"
                  class={styles.connectionInfoBrokerInput}
                />
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
      )}
    </Show>
  );
}
