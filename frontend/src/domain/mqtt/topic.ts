const SHARE_PREFIX = "$share/";
const QUEUE_PREFIX = "$queue/";

/**
 * 共有購読の接頭辞 ($share/<group>/ と $queue/) を外したフィルターを返す。共有購読でなければそのまま返す。
 * 受信したメッセージのトピックには接頭辞が付かないので、照合はこのフィルターで行う。
 * バックエンドの振り分け (internal/infrastructure/mqtt/paho_client.go の filterMatches) と同じ規則にする。
 * 片方を変えるときはもう片方も直す。
 */
export function stripSharedPrefix(filter: string): string {
  if (filter.startsWith(SHARE_PREFIX)) {
    // $share/<group> のようにグループの後ろが無ければ、そのまま返す。
    const groupEnd = filter.indexOf("/", SHARE_PREFIX.length);
    return groupEnd === -1 ? filter : filter.slice(groupEnd + 1);
  }
  if (filter.startsWith(QUEUE_PREFIX)) {
    return filter.slice(QUEUE_PREFIX.length);
  }
  return filter;
}

export function compilePattern(pattern: string): string[] {
  return pattern.split("/");
}

export function topicMatchesParts(
  patternParts: string[],
  topicParts: string[],
): boolean {
  for (let i = 0; i < patternParts.length; i++) {
    if (patternParts[i] === "#") return true;
    if (i >= topicParts.length) return false;
    if (patternParts[i] !== "+" && patternParts[i] !== topicParts[i])
      return false;
  }
  return patternParts.length === topicParts.length;
}

export function topicMatches(pattern: string, topic: string): boolean {
  return topicMatchesParts(compilePattern(pattern), topic.split("/"));
}
