import { createRoot, createSignal } from "solid-js";
import { createStore } from "solid-js/store";
import { describe, expect, it, vi } from "vitest";
import type { ConnectionStateExt, MqttMessageView } from "./connections";
import {
  collectFilterTopics,
  createMessagesState,
  filterMessagesByTopic,
} from "./messages";
import { makeSubscription } from "./subscription";

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

  // 受信したメッセージのトピックには共有購読の接頭辞が付かない。
  it("adds concrete topics matched by a shared wildcard subscription", () => {
    expect(
      collectFilterTopics(
        [{ topic: "$share/g/sensors/#" }, { topic: "$queue/q/+" }],
        [makeMessage("sensors/temp"), makeMessage("q/1"), makeMessage("x/y")],
      ),
    ).toEqual(["$queue/q/+", "$share/g/sensors/#", "q/1", "sensors/temp"]);
  });

  it("does not expand a shared subscription without wildcards", () => {
    expect(
      collectFilterTopics([{ topic: "$share/g/a" }], [makeMessage("a")]),
    ).toEqual(["$share/g/a"]);
  });
});

describe("filterMessagesByTopic", () => {
  const messages = [
    makeMessage("sensors/a/temp", "1"),
    makeMessage("sensors/b/temp", "2"),
    makeMessage("other/x", "3"),
  ];
  const subscriptions = [
    { topic: "sensors/#" },
    { topic: "sensors/+/temp" },
    { topic: "missing/topic" },
  ];

  it("returns the same array instance when the filter is empty", () => {
    expect(filterMessagesByTopic(messages, "", subscriptions)).toBe(messages);
  });

  it("filters by exact topic", () => {
    expect(
      filterMessagesByTopic(messages, "sensors/a/temp", subscriptions).map(
        (m) => m.id,
      ),
    ).toEqual(["1"]);
  });

  it("filters by a # pattern", () => {
    expect(
      filterMessagesByTopic(messages, "sensors/#", subscriptions).map(
        (m) => m.id,
      ),
    ).toEqual(["1", "2"]);
  });

  it("filters by a + pattern", () => {
    expect(
      filterMessagesByTopic(messages, "sensors/+/temp", subscriptions).map(
        (m) => m.id,
      ),
    ).toEqual(["1", "2"]);
  });

  it("returns nothing when no topic matches", () => {
    expect(
      filterMessagesByTopic(messages, "missing/topic", subscriptions),
    ).toEqual([]);
  });

  describe("shared subscriptions", () => {
    const received = [
      makeMessage("sensors/temp", "1"),
      makeMessage("a", "2"),
      makeMessage("q/1", "3"),
      makeMessage("other/x", "4"),
    ];
    const shared = [
      { topic: "$share/g/sensors/#" },
      { topic: "$share/g/a" },
      { topic: "$queue/q/+" },
    ];

    it("matches a $share wildcard subscription without its prefix", () => {
      expect(
        filterMessagesByTopic(received, "$share/g/sensors/#", shared).map(
          (m) => m.id,
        ),
      ).toEqual(["1"]);
    });

    it("matches a $share subscription without wildcards without its prefix", () => {
      expect(
        filterMessagesByTopic(received, "$share/g/a", shared).map((m) => m.id),
      ).toEqual(["2"]);
    });

    it("matches a $queue subscription without its prefix", () => {
      expect(
        filterMessagesByTopic(received, "$queue/q/+", shared).map((m) => m.id),
      ).toEqual(["3"]);
    });

    // 実トピックにまで接頭辞を外すと、$queue/q/1 を q/1 で照合して 0 件になる。
    it("filters a concrete topic by exact match without stripping a prefix", () => {
      const msgs = [makeMessage("$queue/q/1", "1"), makeMessage("q/1", "2")];
      expect(
        filterMessagesByTopic(msgs, "$queue/q/1", [
          { topic: "$share/g/$queue/q/+" },
        ]).map((m) => m.id),
      ).toEqual(["1"]);
    });

    it("keeps messages whose concrete topic equals the subscription string", () => {
      const msgs = [makeMessage("$queue/a", "1"), makeMessage("a", "2")];
      expect(
        filterMessagesByTopic(msgs, "$queue/a", [
          { topic: "$queue/a" },
          { topic: "$share/g/$queue/+" },
        ]).map((m) => m.id),
      ).toEqual(["1", "2"]);
    });
  });
});

function makeConnection(
  connectionId: string,
  over: Partial<ConnectionStateExt> = {},
): ConnectionStateExt {
  return {
    type: "offline",
    connectionId,
    profileId: connectionId,
    profile: {
      id: connectionId,
      name: connectionId,
      broker: "mqtt://localhost:1883",
      clientId: "",
      username: "",
      password: "",
      useTls: false,
    },
    subscriptions: [],
    messages: [],
    selectedMessage: null,
    autoFollow: false,
    brokerTopics: [],
    brokerTopicsSet: new Set(),
    isScanning: false,
    ...over,
  } as ConnectionStateExt;
}

/** connections.ts と同じ形の接続ストアの上に messages state を作る。 */
function setupMessages(initial: ConnectionStateExt[]) {
  return createRoot((dispose) => {
    const [connections, setConnections] = createStore<
      Record<string, ConnectionStateExt>
    >(Object.fromEntries(initial.map((c) => [c.connectionId, c])));
    const [activeId, setActiveId] = createSignal<string | null>(
      initial[0]?.connectionId ?? null,
    );
    const updateConnection = vi.fn(
      (id: string, fn: (s: ConnectionStateExt) => ConnectionStateExt) => {
        const existing = connections[id];
        if (existing) setConnections(id, fn(existing));
      },
    );
    const state = createMessagesState(() => {
      const id = activeId();
      return id ? (connections[id] ?? null) : null;
    }, updateConnection);
    const push = (id: string, msg: MqttMessageView) =>
      updateConnection(id, (s) => ({ ...s, messages: [...s.messages, msg] }));
    return {
      state,
      connections,
      setActiveId,
      updateConnection,
      push,
      dispose,
    };
  });
}

describe("createMessagesState", () => {
  it("follows the newest message while autoFollow is on", () => {
    const h = setupMessages([makeConnection("c1")]);
    h.push("c1", makeMessage("a", "m1"));
    expect(h.state.selectedMessage()).toBeNull();

    h.state.setAutoFollow(true);
    expect(h.state.selectedMessage()?.id).toBe("m1");

    h.push("c1", makeMessage("a", "m2"));
    expect(h.state.selectedMessage()?.id).toBe("m2");

    h.state.setAutoFollow(false);
    h.push("c1", makeMessage("a", "m3"));
    expect(h.state.selectedMessage()?.id).toBe("m2");
    h.dispose();
  });

  it("does not update the connection when the newest message is already selected", () => {
    const last = makeMessage("a", "m1");
    const h = setupMessages([
      makeConnection("c1", {
        messages: [last],
        selectedMessage: last,
        autoFollow: true,
      }),
    ]);

    // 初回の effect は選択済みの末尾を見て何もしない。
    expect(h.updateConnection).not.toHaveBeenCalled();
    h.dispose();
  });

  it("computes setAutoFollow from the current value", () => {
    const h = setupMessages([makeConnection("c1")]);

    h.state.setAutoFollow((prev) => !prev);
    expect(h.state.autoFollow()).toBe(true);
    h.state.setAutoFollow((prev) => !prev);
    expect(h.state.autoFollow()).toBe(false);
    h.dispose();
  });

  it("clears the selection together with the messages", () => {
    const msg = makeMessage("a", "m1");
    const h = setupMessages([
      makeConnection("c1", { messages: [msg], selectedMessage: msg }),
    ]);

    h.state.clearMessages();

    expect(h.state.messages()).toEqual([]);
    expect(h.state.selectedMessage()).toBeNull();
    h.dispose();
  });

  it("reads and writes only the active connection", () => {
    const h = setupMessages([
      makeConnection("c1", { messages: [makeMessage("a", "m1")] }),
      makeConnection("c2", { messages: [makeMessage("b", "m2")] }),
    ]);

    h.setActiveId("c2");
    expect(h.state.messages().map((m) => m.id)).toEqual(["m2"]);
    h.state.setSelectedMessage(h.state.messages()[0]);

    expect(h.connections.c2.selectedMessage?.id).toBe("m2");
    expect(h.connections.c1.selectedMessage).toBeNull();
    h.dispose();
  });

  it("does nothing without an active connection", () => {
    const h = setupMessages([makeConnection("c1")]);
    h.setActiveId(null);
    h.updateConnection.mockClear();

    h.state.setSelectedMessage(makeMessage("a"));
    h.state.setAutoFollow(true);
    h.state.clearMessages();

    expect(h.updateConnection).not.toHaveBeenCalled();
    expect(h.state.messages()).toEqual([]);
    expect(h.state.selectedMessage()).toBeNull();
    expect(h.state.autoFollow()).toBe(false);
    h.dispose();
  });
});

describe("createMessagesState topic filter", () => {
  /** sensors/# を購読し、temp と hum を 1 件ずつ受信済みの接続。 */
  function setupSensors(over: Partial<ConnectionStateExt> = {}) {
    return setupMessages([
      makeConnection("c1", {
        subscriptions: [makeSubscription("sensors/#", 0)],
        messages: [
          makeMessage("sensors/temp", "t1"),
          makeMessage("sensors/hum", "h1"),
        ],
        ...over,
      }),
    ]);
  }

  it("lists subscribed topics and the concrete topics they matched", () => {
    const h = setupSensors();

    expect(h.state.filterTopics()).toEqual([
      "sensors/#",
      "sensors/hum",
      "sensors/temp",
    ]);
    h.dispose();
  });

  it("returns the messages themselves while no filter is set", () => {
    const h = setupSensors();

    expect(h.state.visibleMessages()).toBe(h.state.messages());

    h.state.setTopicFilter("sensors/temp");
    expect(h.state.visibleMessages().map((m) => m.id)).toEqual(["t1"]);
    h.dispose();
  });

  it("keeps the selection when a message outside the filter arrives", () => {
    const h = setupSensors({ autoFollow: true });
    h.state.setTopicFilter("sensors/temp");
    expect(h.state.selectedMessage()?.id).toBe("t1");
    h.updateConnection.mockClear();

    // 追従先が全メッセージの末尾だと、ここで選択がフィルター外の h2 に移る。
    expect(() => h.push("c1", makeMessage("sensors/hum", "h2"))).not.toThrow();

    expect(h.state.selectedMessage()?.id).toBe("t1");
    // push 自身の 1 回だけで、追従 effect は書き込まない。
    expect(h.updateConnection).toHaveBeenCalledTimes(1);
    h.dispose();
  });

  it("follows the newest message matching the filter", () => {
    const h = setupSensors({ autoFollow: true });
    h.state.setTopicFilter("sensors/temp");

    h.push("c1", makeMessage("sensors/temp", "t2"));

    expect(h.state.selectedMessage()?.id).toBe("t2");
    h.dispose();
  });

  it("follows the newest message of all again once the filter is cleared", () => {
    const h = setupSensors({ autoFollow: true });
    h.state.setTopicFilter("sensors/temp");
    expect(h.state.selectedMessage()?.id).toBe("t1");

    h.state.setTopicFilter("");

    expect(h.state.selectedMessage()?.id).toBe("h1");
    h.dispose();
  });

  it("keeps the selection while no message matches the filter", () => {
    const h = setupMessages([
      makeConnection("c1", {
        subscriptions: [
          makeSubscription("sensors/#", 0),
          makeSubscription("alerts/fire", 0),
        ],
        messages: [makeMessage("sensors/temp", "t1")],
        autoFollow: true,
      }),
    ]);
    expect(h.state.selectedMessage()?.id).toBe("t1");

    h.state.setTopicFilter("alerts/fire");

    expect(h.state.visibleMessages()).toEqual([]);
    expect(h.state.selectedMessage()?.id).toBe("t1");
    h.dispose();
  });

  it("shows the messages of a shared subscription chosen as the filter", () => {
    const h = setupSensors({
      subscriptions: [makeSubscription("$share/g/sensors/#", 0)],
    });

    expect(h.state.filterTopics()).toEqual([
      "$share/g/sensors/#",
      "sensors/hum",
      "sensors/temp",
    ]);

    h.state.setTopicFilter("$share/g/sensors/#");
    expect(h.state.visibleMessages().map((m) => m.id)).toEqual(["t1", "h1"]);
    h.dispose();
  });

  it("clears a filter that is no longer among the choices", () => {
    const h = setupSensors();
    h.state.setTopicFilter("sensors/temp");

    h.updateConnection("c1", (s) => ({ ...s, subscriptions: [] }));

    expect(h.state.topicFilter()).toBe("");
    expect(h.state.visibleMessages()).toBe(h.state.messages());
    h.dispose();
  });
});
