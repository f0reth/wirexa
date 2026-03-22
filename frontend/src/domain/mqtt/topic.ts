export const MAX_MESSAGES = 5000;
export const MAX_TOPICS = 500;

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
