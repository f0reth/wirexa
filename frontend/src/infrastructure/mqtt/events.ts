import { EventsOn } from "../../../wailsjs/runtime/runtime";
import type { MqttEventPayloads } from "../../domain/mqtt/types";
import type { MqttEventName } from "../../shared/wails-events";

// クリーンアップ関数を返す。
// ペイロードの型はここで付ける (EventsOn は any で渡すので、形は検証していない)。
export function onMqttEvent<E extends MqttEventName>(
  event: E,
  handler: (data: MqttEventPayloads[E]) => void,
): () => void {
  return EventsOn(event, handler);
}
