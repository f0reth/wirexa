export function formatTime(date: Date): string {
  return date.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function formatPayload(payload: string): string {
  try {
    return JSON.stringify(JSON.parse(payload), null, 2);
  } catch {
    return payload;
  }
}

const topicColorCache = new Map<string, string>();

export function getTopicColor(topic: string): string {
  const cached = topicColorCache.get(topic);
  if (cached) return cached;

  let hash = 0;
  for (let i = 0; i < topic.length; i++) {
    hash = topic.charCodeAt(i) + ((hash << 5) - hash);
  }
  const hue = Math.abs(hash) % 360;
  const color = `hsl(${hue}, 65%, 55%)`;

  if (topicColorCache.size >= 1000) {
    topicColorCache.clear();
  }
  topicColorCache.set(topic, color);
  return color;
}
