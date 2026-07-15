import { compilePattern } from "../../domain/mqtt/topic";
import type { Subscription } from "../../domain/mqtt/types";
import { generateId } from "../../infrastructure/id/generator";

/** topic/qos から UI 表示用 Subscription を生成する。ワイルドカードには patternParts を付与。 */
export function makeSubscription(topic: string, qos: number): Subscription {
  const isWildcard = topic.includes("+") || topic.includes("#");
  return {
    id: generateId(),
    topic,
    qos: qos as 0 | 1 | 2,
    patternParts: isWildcard ? compilePattern(topic) : undefined,
    muted: false,
  };
}
