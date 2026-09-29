import { createRoot } from "solid-js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ConnectionPersistence } from "../../domain/mqtt/ports";
import type { BrokerProfile, ConnectionStatus } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import { WailsEvents } from "../../shared/wails-events";
import type { Logger } from "../logger";
import {
  createConnectionsState,
  type MqttConnectionApi,
  type MqttEventListener,
  type MqttEventName,
} from "./connections";

const noopLogger: Logger = { info: () => {}, error: () => {} };
const noopEvent: MqttEventListener = () => () => {};

function makeNotifier(): Notifier {
  return {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
}

function makeProfile(id: string, name = id): BrokerProfile {
  return {
    id,
    name,
    broker: `tcp://${name}:1883`,
    clientId: "",
    username: "",
    password: "",
    useTls: false,
  };
}

function makeApi(
  getConnections: () => Promise<ConnectionStatus[]>,
): MqttConnectionApi {
  return {
    connect: vi.fn(async () => "new-id"),
    disconnect: vi.fn(async () => {}),
    subscribe: vi.fn(async () => {}),
    unsubscribe: vi.fn(async () => {}),
    getConnections: vi.fn(getConnections),
  };
}

function makePersistence(lastProfileId: string | null): ConnectionPersistence {
  return {
    loadLastProfileId: () => lastProfileId,
    saveLastProfileId: () => {},
    removeLastProfileId: () => {},
  };
}

function setup(
  live: ConnectionStatus[],
  profiles: BrokerProfile[],
  lastProfileId: string | null = null,
) {
  const notifier = makeNotifier();
  return createRoot((dispose) => {
    const state = createConnectionsState(
      makeApi(async () => live),
      noopEvent,
      makePersistence(lastProfileId),
      () => profiles,
      async (p) => p,
      noopLogger,
      notifier,
      1000,
      1000,
    );
    return { state, notifier, dispose };
  });
}

describe("createConnectionsState restore", () => {
  it("restores live backend connections as online tabs with subscriptions", async () => {
    const { state, dispose } = setup(
      [
        {
          id: "conn-1",
          name: "Broker A",
          broker: "tcp://a:1883",
          connected: true,
          profileId: "p1",
          subscriptions: [
            { topic: "sensors/temp", qos: 1 },
            { topic: "sensors/#", qos: 0 },
          ],
        },
      ],
      [makeProfile("p1", "Broker A"), makeProfile("p2", "Broker B")],
    );

    await state.restore();

    // オンラインタブはバックエンド UUID をキーに復元される。
    const online = state.connections["conn-1"];
    expect(online).toBeDefined();
    expect(online.type).toBe("online");
    expect(online.profileId).toBe("p1");
    if (online.type === "online") {
      expect(online.connected).toBe(true);
    }
    expect(online.subscriptions.map((s) => s.topic).sort()).toEqual([
      "sensors/#",
      "sensors/temp",
    ]);
    // ワイルドカード購読には patternParts が付与される。
    const wildcard = online.subscriptions.find((s) => s.topic === "sensors/#");
    expect(wildcard?.patternParts).toEqual(["sensors", "#"]);

    // 生きた接続を持たないプロファイルはオフラインタブとして復元される。
    expect(state.connections["offline-p2"]?.type).toBe("offline");
    // オンライン化したプロファイルのオフラインタブは作られない。
    expect(state.connections["offline-p1"]).toBeUndefined();

    dispose();
  });

  it("selects the saved profile's online connection as active", async () => {
    const { state, dispose } = setup(
      [
        {
          id: "conn-1",
          name: "Broker A",
          broker: "tcp://a:1883",
          connected: true,
          profileId: "p1",
          subscriptions: [],
        },
      ],
      [makeProfile("p1"), makeProfile("p2")],
      "p1",
    );

    await state.restore();
    expect(state.activeConnectionId()).toBe("conn-1");
    dispose();
  });

  it("falls back to the offline tab when the saved profile has no live connection", async () => {
    const { state, dispose } = setup(
      [],
      [makeProfile("p1"), makeProfile("p2")],
      "p2",
    );

    await state.restore();
    expect(state.activeConnectionId()).toBe("offline-p2");
    dispose();
  });

  it("synthesizes a profile for a live connection whose profile was deleted", async () => {
    const { state, dispose } = setup(
      [
        {
          id: "conn-x",
          name: "Orphan",
          broker: "tcp://orphan:1883",
          connected: false,
          profileId: "gone",
          subscriptions: [],
        },
      ],
      [],
    );

    await state.restore();
    const conn = state.connections["conn-x"];
    expect(conn).toBeDefined();
    expect(conn.profile.name).toBe("Orphan");
    expect(conn.profile.broker).toBe("tcp://orphan:1883");
    dispose();
  });

  it("runs only once", async () => {
    const live: ConnectionStatus[] = [];
    const profiles = [makeProfile("p1")];
    await createRoot(async (dispose) => {
      const api = makeApi(async () => live);
      const state = createConnectionsState(
        api,
        noopEvent,
        makePersistence(null),
        () => profiles,
        async (p) => p,
        noopLogger,
        makeNotifier(),
        1000,
        1000,
      );
      await state.restore();
      await state.restore();
      expect(api.getConnections).toHaveBeenCalledTimes(1);
      dispose();
    });
  });
});

/** 登録されたハンドラを保持し、テストから発火できるイベント購読。 */
function makeEvents() {
  const handlers = new Map<MqttEventName, (data: unknown) => void>();
  const unsubscribed: MqttEventName[] = [];
  const onEvent: MqttEventListener = (event, handler) => {
    handlers.set(event, handler);
    return () => {
      unsubscribed.push(event);
      handlers.delete(event);
    };
  };
  const emit = (event: MqttEventName, data: unknown) =>
    handlers.get(event)?.(data);
  return { onEvent, emit, unsubscribed, handlers };
}

interface HarnessOptions {
  live?: ConnectionStatus[];
  profiles?: BrokerProfile[];
  lastProfileId?: string | null;
  maxMessages?: number;
  maxTopics?: number;
}

/** イベント・永続化・API の呼び出しをすべて記録できる state を作る。 */
function harness(opts: HarnessOptions = {}) {
  const events = makeEvents();
  const api = makeApi(async () => opts.live ?? []);
  const persistence = {
    loadLastProfileId: vi.fn(() => opts.lastProfileId ?? null),
    saveLastProfileId: vi.fn((_id: string) => {}),
    removeLastProfileId: vi.fn(() => {}),
  } satisfies ConnectionPersistence;
  const saveProfile = vi.fn(async (p: BrokerProfile) => p);
  const notifier = makeNotifier();
  const logger = { info: vi.fn(), error: vi.fn() } satisfies Logger;
  const profiles = opts.profiles ?? [makeProfile("p1")];
  return createRoot((dispose) => {
    const state = createConnectionsState(
      api,
      events.onEvent,
      persistence,
      () => profiles,
      saveProfile,
      logger,
      notifier,
      opts.maxMessages ?? 1000,
      opts.maxTopics ?? 1000,
    );
    return {
      state,
      api,
      events,
      persistence,
      saveProfile,
      notifier,
      logger,
      dispose,
    };
  });
}

function liveStatus(
  id: string,
  profileId: string,
  topics: string[] = [],
  connected = true,
): ConnectionStatus {
  return {
    id,
    name: profileId,
    broker: `tcp://${profileId}:1883`,
    connected,
    profileId,
    subscriptions: topics.map((topic) => ({ topic, qos: 0 })),
  };
}

function rawMessage(connectionId: string, topic: string, payload = topic) {
  return { connectionId, topic, payload, qos: 1, timestamp: 1_000 };
}

function setMuted(
  state: ReturnType<typeof harness>["state"],
  connId: string,
  topic: string,
) {
  state.updateConnection(connId, (s) => ({
    ...s,
    subscriptions: s.subscriptions.map((sub) =>
      sub.topic === topic ? { ...sub, muted: true } : sub,
    ),
  }));
}

describe("createConnectionsState incoming messages", () => {
  // node 環境には requestAnimationFrame が無いので、テストから 1 フレームずつ進める。
  let frames: FrameRequestCallback[] = [];
  function runFrame() {
    const pending = frames;
    frames = [];
    for (const cb of pending) cb(0);
  }

  beforeEach(() => {
    frames = [];
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      frames.push(cb);
      return frames.length;
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("appends only messages matching an unmuted subscription", async () => {
    const h = harness({
      live: [liveStatus("c1", "p1", ["sensors/temp", "alerts/#", "logs/+"])],
    });
    await h.state.restore();
    setMuted(h.state, "c1", "logs/+");

    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "sensors/temp"));
    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "alerts/fire/1"));
    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "logs/app"));
    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "other"));
    expect(h.state.connections.c1.messages).toHaveLength(0);
    runFrame();

    const messages = h.state.connections.c1.messages;
    expect(messages.map((m) => m.topic)).toEqual([
      "sensors/temp",
      "alerts/fire/1",
    ]);
    expect(messages[0]).toMatchObject({
      topic: "sensors/temp",
      payload: "sensors/temp",
      payloadBase64: false,
      qos: 1,
      direction: "incoming",
      timestamp: new Date(1_000),
    });
    expect(messages[0].id).not.toBe(messages[1].id);
    h.dispose();
  });

  it("keeps the base64 flag of binary payloads", async () => {
    const h = harness({ live: [liveStatus("c1", "p1", ["bin"])] });
    await h.state.restore();

    h.events.emit(WailsEvents.mqttMessage, {
      ...rawMessage("c1", "bin", "AAEC"),
      payloadBase64: true,
    });
    runFrame();

    expect(h.state.connections.c1.messages[0].payloadBase64).toBe(true);
    h.dispose();
  });

  it("batches messages per frame and per connection", async () => {
    const h = harness({
      live: [liveStatus("c1", "p1", ["a"]), liveStatus("c2", "p2", ["a"])],
      profiles: [makeProfile("p1"), makeProfile("p2")],
    });
    await h.state.restore();

    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "a", "1"));
    h.events.emit(WailsEvents.mqttMessage, rawMessage("c2", "a", "2"));
    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "a", "3"));
    h.events.emit(WailsEvents.mqttMessage, rawMessage("gone", "a", "4"));
    expect(frames).toHaveLength(1);
    runFrame();

    expect(h.state.connections.c1.messages.map((m) => m.payload)).toEqual([
      "1",
      "3",
    ]);
    expect(h.state.connections.c2.messages.map((m) => m.payload)).toEqual([
      "2",
    ]);

    // 反映後の受信は次のフレームを予約する。
    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "a", "5"));
    expect(frames).toHaveLength(1);
    h.dispose();
  });

  it("keeps a message when any matching subscription is unmuted", async () => {
    const h = harness({
      live: [liveStatus("c1", "p1", ["sensors/#", "sensors/temp"])],
    });
    await h.state.restore();
    setMuted(h.state, "c1", "sensors/#");

    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "sensors/temp"));
    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "sensors/humid"));
    runFrame();

    expect(h.state.connections.c1.messages.map((m) => m.topic)).toEqual([
      "sensors/temp",
    ]);
    h.dispose();
  });

  it("keeps at most maxMessages, dropping the oldest", async () => {
    const h = harness({
      live: [liveStatus("c1", "p1", ["t"])],
      maxMessages: 3,
    });
    await h.state.restore();

    for (const p of ["1", "2", "3"]) {
      h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "t", p));
    }
    runFrame();
    expect(h.state.connections.c1.messages.map((m) => m.payload)).toEqual([
      "1",
      "2",
      "3",
    ]);

    for (const p of ["4", "5"]) {
      h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "t", p));
    }
    runFrame();
    expect(h.state.connections.c1.messages.map((m) => m.payload)).toEqual([
      "3",
      "4",
      "5",
    ]);
    h.dispose();
  });

  it("caps broker topics while scanning, dropping the oldest", async () => {
    const h = harness({ live: [liveStatus("c1", "p1")], maxTopics: 2 });
    await h.state.restore();

    // スキャン中でなければトピックを集めない。
    h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "x"));
    runFrame();
    expect(h.state.connections.c1.brokerTopics).toEqual([]);

    h.state.updateConnection("c1", (s) => ({ ...s, isScanning: true }));
    for (const t of ["a", "b", "a", "c"]) {
      h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", t));
    }
    runFrame();

    const conn = h.state.connections.c1;
    expect(conn.brokerTopics).toEqual(["b", "c"]);
    expect([...conn.brokerTopicsSet].sort()).toEqual(["b", "c"]);
    // 購読していないトピックはメッセージとしては追加しない。
    expect(conn.messages).toEqual([]);
    h.dispose();
  });

  it("buffers at most 5000 messages between frames", async () => {
    const h = harness({
      live: [liveStatus("c1", "p1", ["t"])],
      maxMessages: 10_000,
    });
    await h.state.restore();

    for (let i = 0; i < 5001; i++) {
      h.events.emit(WailsEvents.mqttMessage, rawMessage("c1", "t", String(i)));
    }
    runFrame();

    const messages = h.state.connections.c1.messages;
    expect(messages).toHaveLength(5000);
    expect(messages[messages.length - 1].payload).toBe("4999");
    h.dispose();
  });
});

describe("createConnectionsState lifecycle events", () => {
  beforeEach(() => {
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  async function setupOnlineAndOffline() {
    const h = harness({
      live: [liveStatus("c1", "p1", [], false)],
      profiles: [makeProfile("p1"), makeProfile("p2")],
    });
    await h.state.restore();
    h.state.updateConnection("c1", (s) => ({ ...s, isScanning: true }));
    return h;
  }

  function onlineState(h: ReturnType<typeof harness>, id: string) {
    const conn = h.state.connections[id];
    if (conn.type !== "online") throw new Error("expected online");
    return conn;
  }

  it("marks only online connections connected", async () => {
    const h = await setupOnlineAndOffline();
    const offlineBefore = h.state.connections["offline-p2"];

    h.events.emit(WailsEvents.mqttConnected, { connectionId: "c1" });
    h.events.emit(WailsEvents.mqttConnected, { connectionId: "offline-p2" });

    expect(onlineState(h, "c1").connected).toBe(true);
    expect(h.state.connections["offline-p2"]).toEqual(offlineBefore);
    h.dispose();
  });

  it("stops scanning on disconnect", async () => {
    const h = await setupOnlineAndOffline();
    h.events.emit(WailsEvents.mqttConnected, { connectionId: "c1" });

    h.events.emit(WailsEvents.mqttDisconnected, { connectionId: "c1" });

    expect(onlineState(h, "c1")).toMatchObject({
      connected: false,
      isScanning: false,
    });
    expect(h.notifier.error).not.toHaveBeenCalled();
    h.dispose();
  });

  it("marks the connection lost and notifies once per connection", async () => {
    const h = await setupOnlineAndOffline();
    h.events.emit(WailsEvents.mqttConnected, { connectionId: "c1" });

    h.events.emit(WailsEvents.mqttConnectionLost, {
      connectionId: "c1",
      error: "EOF",
    });

    expect(h.notifier.error).toHaveBeenCalledWith(
      "MQTT connection lost",
      "EOF",
      { key: "c1" },
    );
    // 自動再接続に備えてスキャン状態は残す。
    expect(onlineState(h, "c1")).toMatchObject({
      connected: false,
      isScanning: true,
    });
    h.dispose();
  });

  it("marks the connection failed, stops scanning and notifies", async () => {
    const h = await setupOnlineAndOffline();

    h.events.emit(WailsEvents.mqttConnectionFailed, {
      connectionId: "c1",
      error: "refused",
    });

    expect(h.notifier.error).toHaveBeenCalledWith(
      "MQTT connection failed",
      "refused",
      { key: "c1" },
    );
    expect(onlineState(h, "c1")).toMatchObject({
      connected: false,
      isScanning: false,
    });
    h.dispose();
  });

  it("unsubscribes every event on dispose", () => {
    const h = harness();
    expect(h.events.handlers.size).toBe(5);

    h.dispose();

    expect([...h.events.unsubscribed].sort()).toEqual(
      [
        WailsEvents.mqttMessage,
        WailsEvents.mqttConnected,
        WailsEvents.mqttDisconnected,
        WailsEvents.mqttConnectionLost,
        WailsEvents.mqttConnectionFailed,
      ].sort(),
    );
    expect(h.events.handlers.size).toBe(0);
  });
});

describe("createConnectionsState connection operations", () => {
  it("replaces the offline tab with the new online connection", async () => {
    const h = harness({ profiles: [makeProfile("p1"), makeProfile("p2")] });
    await h.state.restore();

    await h.state.handleConnect("p1");

    expect(h.api.connect).toHaveBeenCalledWith(makeProfile("p1"));
    expect(h.state.connections["offline-p1"]).toBeUndefined();
    expect(h.state.connections["new-id"]).toMatchObject({
      type: "online",
      profileId: "p1",
      connected: false,
      subscriptions: [],
    });
    expect(h.state.connections["offline-p2"]).toBeDefined();
    expect(h.state.activeConnectionId()).toBe("new-id");
    h.dispose();
  });

  it("does not connect an unknown profile", async () => {
    const h = harness();

    await h.state.handleConnect("missing");

    expect(h.api.connect).not.toHaveBeenCalled();
    expect(h.notifier.error).not.toHaveBeenCalled();
    h.dispose();
  });

  it("disconnects the active connection when no id is given", async () => {
    const h = harness({ live: [liveStatus("c1", "p1")], lastProfileId: "p1" });
    await h.state.restore();
    h.state.updateConnection("c1", (s) => ({ ...s, isScanning: true }));

    await h.state.handleDisconnect();

    expect(h.api.disconnect).toHaveBeenCalledWith("c1");
    expect(h.state.connections.c1).toMatchObject({
      connected: false,
      isScanning: false,
    });
    h.dispose();
  });

  it("does nothing on disconnect without an active connection", async () => {
    const h = harness();

    await h.state.handleDisconnect();

    expect(h.api.disconnect).not.toHaveBeenCalled();
    h.dispose();
  });

  it("marks the connection disconnected even when the backend call fails", async () => {
    const h = harness({ live: [liveStatus("c1", "p1")] });
    await h.state.restore();
    h.api.disconnect = vi.fn(async () => {
      throw new Error("not connected");
    });

    await h.state.handleDisconnect("c1");

    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to disconnect",
      "not connected",
    );
    expect(h.state.connections.c1).toMatchObject({ connected: false });
    h.dispose();
  });

  it("moves state to the new connection id and re-subscribes", async () => {
    const h = harness({
      live: [liveStatus("c1", "p1", ["a", "b/#"])],
      lastProfileId: "p1",
    });
    await h.state.restore();
    h.state.updateConnection("c1", (s) => ({ ...s, autoFollow: true }));
    // 既に切断済みでも再接続は続ける。
    h.api.disconnect = vi.fn(async () => {
      throw new Error("already closed");
    });

    await h.state.handleReconnect("c1");

    expect(h.api.disconnect).toHaveBeenCalledWith("c1");
    expect(h.state.connections.c1).toBeUndefined();
    const moved = h.state.connections["new-id"];
    expect(moved).toMatchObject({
      type: "online",
      connectionId: "new-id",
      connected: false,
      autoFollow: true,
    });
    expect(moved.subscriptions.map((s) => s.topic)).toEqual(["a", "b/#"]);
    expect(h.state.activeConnectionId()).toBe("new-id");
    expect(h.api.subscribe).toHaveBeenCalledWith("new-id", "a", 0);
    expect(h.api.subscribe).toHaveBeenCalledWith("new-id", "b/#", 0);
    expect(h.notifier.error).not.toHaveBeenCalled();
    h.dispose();
  });

  it("keeps re-subscribing after one topic fails", async () => {
    const h = harness({ live: [liveStatus("c1", "p1", ["a", "b", "c"])] });
    await h.state.restore();
    h.api.subscribe = vi.fn(async (_id: string, topic: string) => {
      if (topic === "b") throw new Error("not authorized");
    });

    await h.state.handleReconnect("c1");

    expect(h.api.subscribe).toHaveBeenCalledTimes(3);
    expect(h.notifier.error).toHaveBeenCalledTimes(1);
    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to re-subscribe to b",
      "not authorized",
    );
    h.dispose();
  });

  it("skips re-subscribing a topic removed while an earlier one was in flight", async () => {
    const h = harness({ live: [liveStatus("c1", "p1", ["a", "b"])] });
    await h.state.restore();
    let resolveFirst = () => {};
    h.api.subscribe = vi.fn(
      (_id: string, topic: string) =>
        new Promise<void>((resolve) => {
          if (topic === "a") resolveFirst = resolve;
          else resolve();
        }),
    );

    const reconnecting = h.state.handleReconnect("c1");
    await vi.waitFor(() => expect(h.api.subscribe).toHaveBeenCalledTimes(1));
    h.state.updateConnection("new-id", (s) => ({
      ...s,
      subscriptions: s.subscriptions.filter((sub) => sub.topic !== "b"),
    }));
    resolveFirst();
    await reconnecting;

    expect(h.api.subscribe).toHaveBeenCalledTimes(1);
    expect(h.api.subscribe).toHaveBeenCalledWith("new-id", "a", 0);
    h.dispose();
  });

  it("stops re-subscribing once the new tab is closed", async () => {
    const h = harness({ live: [liveStatus("c1", "p1", ["a", "b"])] });
    await h.state.restore();
    let resolveFirst = () => {};
    h.api.subscribe = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveFirst = resolve;
        }),
    );

    const reconnecting = h.state.handleReconnect("c1");
    await vi.waitFor(() => expect(h.api.subscribe).toHaveBeenCalledTimes(1));
    h.state.closeConnection("new-id");
    resolveFirst();
    await reconnecting;

    expect(h.api.subscribe).toHaveBeenCalledTimes(1);
    h.dispose();
  });

  it("reconnects an offline tab without disconnecting and keeps the active id", async () => {
    const h = harness({
      profiles: [makeProfile("p1"), makeProfile("p2")],
      lastProfileId: "p2",
    });
    await h.state.restore();

    await h.state.handleReconnect("offline-p1");

    expect(h.api.disconnect).not.toHaveBeenCalled();
    expect(h.state.connections["offline-p1"]).toBeUndefined();
    expect(h.state.connections["new-id"]?.type).toBe("online");
    expect(h.state.activeConnectionId()).toBe("offline-p2");
    h.dispose();
  });

  it("notifies and keeps the tab when reconnecting fails", async () => {
    const h = harness({ live: [liveStatus("c1", "p1", ["a"])] });
    await h.state.restore();
    h.api.connect = vi.fn(async () => {
      throw new Error("refused");
    });

    await h.state.handleReconnect("c1");

    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to reconnect",
      "refused",
    );
    expect(h.state.connections.c1).toBeDefined();
    expect(h.api.subscribe).not.toHaveBeenCalled();
    h.dispose();
  });

  it("disconnects only a connected online tab when closing it", async () => {
    const h = harness({
      live: [liveStatus("c1", "p1"), liveStatus("c2", "p2", [], false)],
      profiles: [makeProfile("p1"), makeProfile("p2"), makeProfile("p3")],
    });
    await h.state.restore();

    h.state.closeConnection("c2");
    h.state.closeConnection("offline-p3");
    expect(h.api.disconnect).not.toHaveBeenCalled();

    h.state.closeConnection("c1");
    expect(h.api.disconnect).toHaveBeenCalledWith("c1");
    expect(Object.keys(h.state.connections)).toEqual([]);
    h.dispose();
  });

  it("notifies when disconnecting a closed tab fails", async () => {
    const h = harness({ live: [liveStatus("c1", "p1")] });
    await h.state.restore();
    h.api.disconnect = vi.fn(async () => {
      throw new Error("timeout");
    });

    h.state.closeConnection("c1");
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to disconnect",
      "timeout",
    );
    expect(h.state.connections.c1).toBeUndefined();
    h.dispose();
  });

  it("activates the first remaining tab after closing the active one", async () => {
    const h = harness({
      profiles: [makeProfile("p1"), makeProfile("p2"), makeProfile("p3")],
      lastProfileId: "p2",
    });
    await h.state.restore();

    h.state.closeConnection("offline-p2");
    expect(h.state.activeConnectionId()).toBe("offline-p1");

    // アクティブでないタブを閉じてもアクティブは変わらない。
    h.state.closeConnection("offline-p3");
    expect(h.state.activeConnectionId()).toBe("offline-p1");

    h.state.closeConnection("offline-p1");
    expect(h.state.activeConnectionId()).toBeNull();
    h.dispose();
  });

  it("switches the active connection", async () => {
    const h = harness({ profiles: [makeProfile("p1"), makeProfile("p2")] });
    await h.state.restore();

    h.state.switchConnection("offline-p2");

    expect(h.state.activeConnectionId()).toBe("offline-p2");
    expect(h.state.activeConnection()?.profileId).toBe("p2");
    h.dispose();
  });
});

describe("createConnectionsState tabs and profiles", () => {
  it("replaces existing tabs of the same profile", async () => {
    const h = harness({
      live: [liveStatus("c1", "p1")],
      profiles: [makeProfile("p1"), makeProfile("p2")],
    });
    await h.state.restore();

    h.state.createOfflineConnection(makeProfile("p1"));

    expect(h.state.connections.c1).toBeUndefined();
    expect(h.state.connections["offline-p1"]?.type).toBe("offline");
    expect(h.state.connections["offline-p2"]).toBeDefined();
    expect(h.state.activeConnectionId()).toBe("offline-p1");
    h.dispose();
  });

  it("does not change the broker of a connected tab", async () => {
    const h = harness({ live: [liveStatus("c1", "p1")] });
    await h.state.restore();

    h.state.updateConnectionBroker("c1", "tcp://changed:1883");

    expect(h.state.connections.c1.profile.broker).toBe("tcp://p1:1883");
    expect(h.saveProfile).not.toHaveBeenCalled();
    h.dispose();
  });

  it.each([
    ["an offline tab", "offline-p2", []],
    ["a disconnected online tab", "c1", [liveStatus("c1", "p1", [], false)]],
  ] as const)("updates and saves the broker of %s", async (_, connId, live) => {
    const h = harness({
      live: [...live],
      profiles: [makeProfile("p1"), makeProfile("p2")],
    });
    await h.state.restore();

    h.state.updateConnectionBroker(connId, "tcp://changed:1883");

    const conn = h.state.connections[connId];
    expect(conn.profile.broker).toBe("tcp://changed:1883");
    expect(h.saveProfile).toHaveBeenCalledWith(conn.profile);
    h.dispose();
  });

  it("ignores a broker update for an unknown tab", () => {
    const h = harness();

    h.state.updateConnectionBroker("missing", "tcp://changed:1883");

    expect(h.saveProfile).not.toHaveBeenCalled();
    h.dispose();
  });

  it("saves the active profile and forgets it when no tab is active", async () => {
    const h = harness({ profiles: [makeProfile("p1"), makeProfile("p2")] });
    await h.state.restore();
    h.persistence.removeLastProfileId.mockClear();

    h.state.switchConnection("offline-p2");
    expect(h.persistence.saveLastProfileId).toHaveBeenLastCalledWith("p2");

    h.state.switchConnection("offline-p1");
    expect(h.persistence.saveLastProfileId).toHaveBeenLastCalledWith("p1");

    h.state.closeConnection("offline-p1");
    h.state.closeConnection("offline-p2");
    expect(h.state.activeConnectionId()).toBeNull();
    expect(h.persistence.removeLastProfileId).toHaveBeenCalled();
    h.dispose();
  });

  it("keeps the saved profile until restore has read it", async () => {
    // 実際の保存領域と同じく、消したら読めなくなる永続化。
    let stored: string | null = "p2";
    const persistence: ConnectionPersistence = {
      loadLastProfileId: () => stored,
      saveLastProfileId: (id) => {
        stored = id;
      },
      removeLastProfileId: () => {
        stored = null;
      },
    };
    const { state, dispose } = createRoot((dispose) => ({
      state: createConnectionsState(
        makeApi(async () => []),
        noopEvent,
        persistence,
        () => [makeProfile("p1"), makeProfile("p2")],
        async (p) => p,
        noopLogger,
        makeNotifier(),
        1000,
        1000,
      ),
      dispose,
    }));
    // 起動直後はアクティブな接続が無いが、まだ復元していないので消さない。
    expect(stored).toBe("p2");

    await state.restore();

    expect(state.activeConnectionId()).toBe("offline-p2");
    expect(stored).toBe("p2");
    dispose();
  });
});

describe("createConnectionsState restore failures", () => {
  it("falls back to offline tabs when getConnections fails", async () => {
    const h = harness({
      profiles: [makeProfile("p1"), makeProfile("p2")],
      lastProfileId: "p2",
    });
    h.api.getConnections = vi.fn(async () => {
      throw new Error("rpc down");
    });

    await h.state.restore();

    expect(h.logger.error).toHaveBeenCalledWith("MQTT restore failed", {
      error: "Error: rpc down",
    });
    expect(Object.keys(h.state.connections).sort()).toEqual([
      "offline-p1",
      "offline-p2",
    ]);
    expect(h.state.activeConnectionId()).toBe("offline-p2");
    h.dispose();
  });

  it("leaves no active tab when the saved profile no longer exists", async () => {
    const h = harness({ lastProfileId: "deleted" });

    await h.state.restore();

    expect(h.state.activeConnectionId()).toBeNull();
    h.dispose();
  });
});

describe("createConnectionsState notifications", () => {
  it("notifies through the injected notifier when connect fails", async () => {
    const profiles = [makeProfile("p1")];
    const notifier = makeNotifier();
    await createRoot(async (dispose) => {
      const api = makeApi(async () => []);
      api.connect = vi.fn(async () => {
        throw new Error("no route to host");
      });
      const state = createConnectionsState(
        api,
        noopEvent,
        makePersistence(null),
        () => profiles,
        async (p) => p,
        noopLogger,
        notifier,
        1000,
        1000,
      );

      // 失敗しても例外は伝播せず、通知だけが出る。
      await state.handleConnect("p1");

      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to connect",
        "no route to host",
      );
      dispose();
    });
  });
});
