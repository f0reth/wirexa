import { compilePattern, stripSharedPrefix } from "../../domain/mqtt/topic";
import type { Subscription } from "../../domain/mqtt/types";
import { generateId } from "../../shared/id";

/**
 * topic/qos から UI 表示用 Subscription を生成する。ワイルドカードと共有購読には patternParts を付与。
 * 共有購読の patternParts は接頭辞を外したフィルターから作る (受信したトピックには接頭辞が付かない)。
 * topic は表示と RPC に使うので、入力のまま保つ。
 */
export function makeSubscription(topic: string, qos: number): Subscription {
  const matchFilter = stripSharedPrefix(topic);
  const isPattern =
    matchFilter !== topic ||
    matchFilter.includes("+") ||
    matchFilter.includes("#");
  return {
    id: generateId(),
    topic,
    qos: qos as 0 | 1 | 2,
    patternParts: isPattern ? compilePattern(matchFilter) : undefined,
    muted: false,
  };
}
