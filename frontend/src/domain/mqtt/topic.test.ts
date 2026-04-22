import { describe, expect, it } from "vitest";
import { compilePattern, topicMatches, topicMatchesParts } from "./topic";

describe("compilePattern", () => {
  it("splits a single-level topic", () => {
    expect(compilePattern("sensors")).toEqual(["sensors"]);
  });

  it("splits a multi-level topic by /", () => {
    expect(compilePattern("sensors/temp/1")).toEqual(["sensors", "temp", "1"]);
  });

  it("handles # wildcard", () => {
    expect(compilePattern("sensors/#")).toEqual(["sensors", "#"]);
  });

  it("handles + wildcard", () => {
    expect(compilePattern("sensors/+/data")).toEqual(["sensors", "+", "data"]);
  });

  it("handles standalone # (all topics)", () => {
    expect(compilePattern("#")).toEqual(["#"]);
  });

  it("handles empty string as single empty part", () => {
    expect(compilePattern("")).toEqual([""]);
  });

  it("handles multiple wildcards", () => {
    expect(compilePattern("+/+/#")).toEqual(["+", "+", "#"]);
  });
});

describe("topicMatchesParts", () => {
  describe("exact matching", () => {
    it("matches identical single-level topics", () => {
      expect(topicMatchesParts(["sensors"], ["sensors"])).toBe(true);
    });

    it("matches identical multi-level topics", () => {
      expect(topicMatchesParts(["a", "b", "c"], ["a", "b", "c"])).toBe(true);
    });

    it("does not match different topics", () => {
      expect(topicMatchesParts(["sensors"], ["actuators"])).toBe(false);
    });

    it("does not match when pattern is shorter than topic", () => {
      expect(topicMatchesParts(["sensors"], ["sensors", "temp"])).toBe(false);
    });

    it("does not match when topic is shorter than pattern", () => {
      expect(topicMatchesParts(["sensors", "temp"], ["sensors"])).toBe(false);
    });

    it("does not match different last segment", () => {
      expect(topicMatchesParts(["a", "b", "c"], ["a", "b", "d"])).toBe(false);
    });
  });

  describe("+ wildcard (single-level)", () => {
    it("matches any single segment at leaf", () => {
      expect(topicMatchesParts(["sensors", "+"], ["sensors", "temp"])).toBe(
        true,
      );
      expect(topicMatchesParts(["sensors", "+"], ["sensors", "humidity"])).toBe(
        true,
      );
    });

    it("does not match multiple levels with single +", () => {
      expect(topicMatchesParts(["+"], ["a", "b"])).toBe(false);
    });

    it("matches + at the beginning", () => {
      expect(topicMatchesParts(["+", "data"], ["any", "data"])).toBe(true);
    });

    it("matches + in the middle", () => {
      expect(topicMatchesParts(["a", "+", "c"], ["a", "b", "c"])).toBe(true);
    });

    it("does not match + in the middle with wrong prefix", () => {
      expect(topicMatchesParts(["a", "+", "c"], ["x", "b", "c"])).toBe(false);
    });

    it("does not match + in the middle with wrong suffix", () => {
      expect(topicMatchesParts(["a", "+", "c"], ["a", "b", "d"])).toBe(false);
    });

    it("matches multiple + wildcards", () => {
      expect(topicMatchesParts(["+", "+"], ["x", "y"])).toBe(true);
    });

    it("does not match multiple + when topic has fewer levels", () => {
      expect(topicMatchesParts(["+", "+"], ["x"])).toBe(false);
    });

    it("matches standalone +", () => {
      expect(topicMatchesParts(["+"], ["anything"])).toBe(true);
    });
  });

  describe("# wildcard (multi-level)", () => {
    it("matches any single-level topic", () => {
      expect(topicMatchesParts(["#"], ["sensors"])).toBe(true);
    });

    it("matches any multi-level topic", () => {
      expect(topicMatchesParts(["#"], ["a", "b", "c"])).toBe(true);
    });

    it("matches empty topic (zero levels)", () => {
      expect(topicMatchesParts(["#"], [])).toBe(true);
    });

    it("matches prefix then any remaining levels", () => {
      expect(topicMatchesParts(["sensors", "#"], ["sensors", "temp"])).toBe(
        true,
      );
      expect(
        topicMatchesParts(["sensors", "#"], ["sensors", "temp", "1"]),
      ).toBe(true);
    });

    it("matches prefix when topic ends at the prefix level", () => {
      // sensors/# should match sensors (# matches zero additional levels)
      expect(topicMatchesParts(["sensors", "#"], ["sensors"])).toBe(true);
    });

    it("does not match when prefix differs", () => {
      expect(topicMatchesParts(["sensors", "#"], ["actuators", "temp"])).toBe(
        false,
      );
    });
  });

  describe("edge cases", () => {
    it("empty pattern matches empty topic", () => {
      expect(topicMatchesParts([], [])).toBe(true);
    });

    it("empty pattern does not match non-empty topic", () => {
      expect(topicMatchesParts([], ["a"])).toBe(false);
    });

    it("non-empty pattern does not match empty topic (unless #)", () => {
      expect(topicMatchesParts(["a"], [])).toBe(false);
    });

    it("mixed + and # wildcards", () => {
      expect(topicMatchesParts(["+", "#"], ["x"])).toBe(true);
      expect(topicMatchesParts(["+", "#"], ["x", "y", "z"])).toBe(true);
    });
  });
});

describe("topicMatches", () => {
  it("matches an exact topic", () => {
    expect(topicMatches("home/sensor/temp", "home/sensor/temp")).toBe(true);
  });

  it("does not match a different topic", () => {
    expect(topicMatches("home/sensor/temp", "home/sensor/humidity")).toBe(
      false,
    );
  });

  it("matches with + wildcard in the middle", () => {
    expect(topicMatches("home/+/temp", "home/sensor/temp")).toBe(true);
    expect(topicMatches("home/+/temp", "home/actuator/temp")).toBe(true);
  });

  it("does not match + across multiple levels", () => {
    expect(topicMatches("home/+", "home/sensor/temp")).toBe(false);
  });

  it("matches with # wildcard at the end", () => {
    expect(topicMatches("home/#", "home/sensor/temp")).toBe(true);
    expect(topicMatches("home/#", "home/sensor/temp/1")).toBe(true);
  });

  it("matches # alone against any topic", () => {
    expect(topicMatches("#", "any/topic/here")).toBe(true);
    expect(topicMatches("#", "single")).toBe(true);
  });

  it("does not match # when prefix differs", () => {
    expect(topicMatches("home/#", "away/sensor")).toBe(false);
  });

  it("matches sensor topic with MQTT-style patterns", () => {
    expect(
      topicMatches("factory/+/machine/+/status", "factory/1/machine/5/status"),
    ).toBe(true);
    expect(
      topicMatches("factory/+/machine/+/status", "factory/1/machine/status"),
    ).toBe(false);
  });

  it("matches single-level exact topic", () => {
    expect(topicMatches("test", "test")).toBe(true);
    expect(topicMatches("test", "other")).toBe(false);
  });

  it("matches empty string topic with # pattern", () => {
    expect(topicMatches("#", "")).toBe(true);
  });

  it("handles trailing slash in topic", () => {
    expect(topicMatches("sensors/+", "sensors/")).toBe(true);
  });

  it("handles leading slash in topic", () => {
    expect(topicMatches("/sensors", "/sensors")).toBe(true);
  });
});
