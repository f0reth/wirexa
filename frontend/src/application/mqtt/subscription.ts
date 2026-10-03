import {
  compilePattern,
  stripSharedPrefix,
  topicMatchesParts,
} from "../../domain/mqtt/topic";
import type { Qos, Subscription } from "../../domain/mqtt/types";
import { generateId } from "../../shared/id";

/**
 * topic/qos から UI 表示用 Subscription を生成する。ワイルドカードと共有購読には patternParts を付与。
 * 共有購読の patternParts は接頭辞を外したフィルターから作る (受信したトピックには接頭辞が付かない)。
 * topic は表示と RPC に使うので、入力のまま保つ。
 */
export function makeSubscription(topic: string, qos: Qos): Subscription {
  const matchFilter = stripSharedPrefix(topic);
  const isPattern =
    matchFilter !== topic ||
    matchFilter.includes("+") ||
    matchFilter.includes("#");
  return {
    id: generateId(),
    topic,
    qos,
    patternParts: isPattern ? compilePattern(matchFilter) : undefined,
    muted: false,
  };
}

/**
 * 受信したトピックが購読に一致するかを返す。topicParts は topic を "/" で分けたもの
 * (同じトピックを複数の購読と照合するので、呼び出し側で 1 回だけ分ける)。
 */
export function subscriptionMatches(
  sub: Subscription,
  topic: string,
  topicParts: string[],
): boolean {
  return (
    sub.topic === topic ||
    (sub.patternParts !== undefined &&
      topicMatchesParts(sub.patternParts, topicParts))
  );
}
