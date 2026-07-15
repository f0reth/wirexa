import { EventsOn } from "../../../wailsjs/runtime/runtime";
import type { MqttEventName } from "../../shared/wails-events";

export type { MqttEventName };

// クリーンアップ関数を返す
export function onMqttEvent(
  event: MqttEventName,
  handler: (data: unknown) => void,
): () => void {
  return EventsOn(event, handler);
}
