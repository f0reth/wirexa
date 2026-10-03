import { describe, expect, it } from "vitest";
import { makeSubscription, subscriptionMatches } from "./subscription";

describe("makeSubscription", () => {
  it("compiles a shared wildcard subscription without its prefix", () => {
    const sub = makeSubscription("$share/g/sensors/#", 1);
    expect(sub.patternParts).toEqual(["sensors", "#"]);
    expect(sub.topic).toBe("$share/g/sensors/#");
  });

  it("compiles a shared subscription without wildcards so it matches exactly", () => {
    const sub = makeSubscription("$share/g/a/b", 0);
    expect(sub.patternParts).toEqual(["a", "b"]);
    expect(sub.topic).toBe("$share/g/a/b");
  });

  it("compiles a $queue subscription without its prefix", () => {
    const sub = makeSubscription("$queue/a/+", 0);
    expect(sub.patternParts).toEqual(["a", "+"]);
    expect(sub.topic).toBe("$queue/a/+");
  });

  it("does not compile a plain topic", () => {
    const sub = makeSubscription("a/b", 2);
    expect(sub.patternParts).toBeUndefined();
    expect(sub).toMatchObject({ topic: "a/b", qos: 2, muted: false });
  });

  it("compiles a wildcard subscription as is", () => {
    expect(makeSubscription("a/+/c", 0).patternParts).toEqual(["a", "+", "c"]);
  });
});

describe("subscriptionMatches", () => {
  const matches = (filter: string, topic: string) =>
    subscriptionMatches(makeSubscription(filter, 0), topic, topic.split("/"));

  it("matches a plain topic only when it is identical", () => {
    expect(matches("a/b", "a/b")).toBe(true);
    expect(matches("a/b", "a/b/c")).toBe(false);
  });

  it("matches a wildcard subscription by its pattern", () => {
    expect(matches("sensors/#", "sensors/temp/1")).toBe(true);
    expect(matches("a/+/c", "a/b/c")).toBe(true);
    expect(matches("a/+/c", "a/b/d")).toBe(false);
  });

  it("matches a shared subscription against the topic without its prefix", () => {
    expect(matches("$share/g/a/b", "a/b")).toBe(true);
    expect(matches("$share/g/sensors/#", "sensors/temp")).toBe(true);
    expect(matches("$share/g/a/b", "a/c")).toBe(false);
  });
});
