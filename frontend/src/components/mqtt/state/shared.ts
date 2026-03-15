export const MAX_MESSAGES = 5000;
export const MAX_TOPICS = 500;

/** Pre-split pattern parts for efficient repeated matching */
export function compilePattern(pattern: string): string[] {
  return pattern.split("/");
}

/** Check if a subscription topic pattern matches a concrete topic */
export function topicMatches(pattern: string, topic: string): boolean {
  if (pattern === "#") return true;
  return topicMatchesParts(pattern.split("/"), topic.split("/"));
}

/** Match using pre-split pattern parts against a pre-split topic */
export function topicMatchesParts(
  patternParts: string[],
  topicParts: string[],
): boolean {
  for (let i = 0; i < patternParts.length; i++) {
    if (patternParts[i] === "#") return true;
    if (patternParts[i] === "+") continue;
    if (i >= topicParts.length || patternParts[i] !== topicParts[i])
      return false;
  }
  return patternParts.length === topicParts.length;
}
