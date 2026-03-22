import { EventsOn } from "../../../wailsjs/runtime/runtime";

export type MqttEventName =
  | "mqtt:connected"
  | "mqtt:disconnected"
  | "mqtt:connection-lost"
  | "mqtt:connection-failed"
  | "mqtt:message";

// クリーンアップ関数を返す
export function onMqttEvent(
  event: MqttEventName,
  handler: (data: unknown) => void,
): () => void {
  return EventsOn(event, handler);
}
