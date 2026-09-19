import { describe, expect, it } from "vitest";
import type { MqttMessageView } from "./connections";
import { collectFilterTopics, filterMessagesByTopic } from "./messages";

function makeMessage(topic: string, id = topic): MqttMessageView {
  return {
    id,
    direction: "incoming",
    topic,
    payload: "",
    payloadBase64: false,
    qos: 0,
    timestamp: new Date(0),
  };
}

describe("collectFilterTopics", () => {
  it("returns an empty list when nothing is subscribed", () => {
    expect(collectFilterTopics([], [makeMessage("a/b")])).toEqual([]);
  });

  it("lists subscribed topics sorted", () => {
    expect(
      collectFilterTopics([{ topic: "b/1" }, { topic: "a/1" }], []),
    ).toEqual(["a/1", "b/1"]);
  });

  it("adds concrete topics matched by a # subscription", () => {
    expect(
      collectFilterTopics(
        [{ topic: "sensors/#" }],
        [makeMessage("sensors/temp"), makeMessage("other/x")],
      ),
    ).toEqual(["sensors/#", "sensors/temp"]);
  });

  it("adds concrete topics matched by a + subscription", () => {
    expect(
      collectFilterTopics(
        [{ topic: "sensors/+/temp" }],
        [
          makeMessage("sensors/a/temp"),
          makeMessage("sensors/a/b/temp"),
          makeMessage("sensors/b/temp"),
        ],
      ),
    ).toEqual(["sensors/+/temp", "sensors/a/temp", "sensors/b/temp"]);
  });

  it("does not expand a subscription without wildcards", () => {
    expect(
      collectFilterTopics(
        [{ topic: "sensors/temp" }],
        [makeMessage("sensors/temp"), makeMessage("sensors/hum")],
      ),
    ).toEqual(["sensors/temp"]);
  });

  it("deduplicates topics reached through several subscriptions", () => {
    expect(
      collectFilterTopics(
        [{ topic: "sensors/#" }, { topic: "sensors/+" }],
        [makeMessage("sensors/temp"), makeMessage("sensors/temp", "2")],
      ),
    ).toEqual(["sensors/#", "sensors/+", "sensors/temp"]);
  });
});

describe("filterMessagesByTopic", () => {
  const messages = [
    makeMessage("sensors/a/temp", "1"),
    makeMessage("sensors/b/temp", "2"),
    makeMessage("other/x", "3"),
  ];

  it("returns the same array instance when the filter is empty", () => {
    expect(filterMessagesByTopic(messages, "")).toBe(messages);
  });

  it("filters by exact topic", () => {
    expect(
      filterMessagesByTopic(messages, "sensors/a/temp").map((m) => m.id),
    ).toEqual(["1"]);
  });

  it("filters by a # pattern", () => {
    expect(
      filterMessagesByTopic(messages, "sensors/#").map((m) => m.id),
    ).toEqual(["1", "2"]);
  });

  it("filters by a + pattern", () => {
    expect(
      filterMessagesByTopic(messages, "sensors/+/temp").map((m) => m.id),
    ).toEqual(["1", "2"]);
  });

  it("returns nothing when no topic matches", () => {
    expect(filterMessagesByTopic(messages, "missing/topic")).toEqual([]);
  });
});
