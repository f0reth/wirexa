export function parseFormPairs(
  content: string,
): { key: string; value: string; enabled: boolean }[] {
  if (!content) return [];
  return content
    .split("&")
    .filter(Boolean)
    .map((pair) => {
      const [key, ...rest] = pair.split("=");
      return {
        key: decodeURIComponent(key || ""),
        value: decodeURIComponent(rest.join("=") || ""),
        enabled: true,
      };
    });
}

export function serializeFormPairs(
  pairs: { key: string; value: string; enabled: boolean }[],
): string {
  return pairs
    .filter((p) => p.enabled && p.key)
    .map((p) => `${encodeURIComponent(p.key)}=${encodeURIComponent(p.value)}`)
    .join("&");
}
