import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { BrokerProfile, ConnectionStatus } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import { type MqttEventName, WailsEvents } from "../../shared/wails-events";
import type { Logger } from "../logger";
import type { ConnectionStateExt, MqttEventListener } from "./connections";
import { makeSubscription } from "./subscription";
import {
  createSubscriptionsState,
  type SubscriptionApi,
} from "./subscriptions";

const profile: BrokerProfile = {
  id: "p1",
  name: "p1",
  broker: "tcp://p1:1883",
  clientId: "",
  username: "",
  password: "",
  useTls: false,
};

function runtime() {
  return {
    subscriptions: [makeSubscription("a", 0), makeSubscription("b/#", 1)],
    messages: [],
    selectedMessage: null,
    autoFollow: false,
    brokerTopics: [],
    brokerTopicsSet: new Set<string>(),
    isScanning: false,
  };
}

function onlineTab(connected: boolean): ConnectionStateExt {
  return {
    type: "online",
    connectionId: "c1",
    profileId: profile.id,
    profile,
    connected,
    closed: false,
    ...runtime(),
  };
}

function offlineTab(): ConnectionStateExt {
  return {
    type: "offline",
    connectionId: "offline-p1",
    profileId: profile.id,
    profile,
    ...runtime(),
  };
}

/** 1 つのタブをアクティブにした購読の state と、API・通知の記録を作る。 */
function harness(initial: ConnectionStateExt) {
  let conn = initial;
  const api = {
    subscribe: vi.fn(async () => {}),
    unsubscribe: vi.fn(async (_id: string, _topic: string) => {}),
    getConnections: vi.fn(async (): Promise<ConnectionStatus[]> => []),
    startTopicScan: vi.fn(async (_id: string) => {}),
    stopTopicScan: vi.fn(async (_id: string) => {}),
  } satisfies SubscriptionApi;
  const notifier: Notifier = {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
  const logger = { info: vi.fn(), error: vi.fn() } satisfies Logger;
  const handlers = new Map<MqttEventName, (data: unknown) => void>();
  const onEvent: MqttEventListener = (event, handler) => {
    // ペイロードの型は emit に渡す値で決まるので、ここでは unknown で持つ。
    handlers.set(event, handler as (data: unknown) => void);
    return () => handlers.delete(event);
  };
  const emit = (event: MqttEventName, data: unknown) =>
    handlers.get(event)?.(data);
  return createRoot((dispose) => {
    const state = createSubscriptionsState(
      () => conn,
      (id, updater) => {
        if (id === conn.connectionId) conn = updater(conn);
      },
      api,
      onEvent,
      logger,
      notifier,
    );
    const subscriptionId = (topic: string) => {
      const sub = conn.subscriptions.find((s) => s.topic === topic);
      if (!sub) throw new Error(`no subscription ${topic}`);
      return sub.id;
    };
    const topics = () => conn.subscriptions.map((s) => s.topic);
    return {
      state,
      api,
      notifier,
      logger,
      subscriptionId,
      topics,
      emit,
      handlers,
      conn: () => conn,
      dispose,
    };
  });
}

/** テストから解決・拒否できる Promise。 */
function deferred() {
  let resolve = () => {};
  let reject = (_err: Error) => {};
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("createSubscriptionsState setIsScanning", () => {
  it("starts and stops the scan through the scan RPCs, not a # subscription", async () => {
    const h = harness({
      ...onlineTab(true),
      brokerTopics: ["old"],
      brokerTopicsSet: new Set(["old"]),
    });

    await h.state.setIsScanning(true);

    expect(h.api.startTopicScan).toHaveBeenCalledWith("c1");
    expect(h.conn()).toMatchObject({ isScanning: true, brokerTopics: [] });
    expect(h.conn().brokerTopicsSet.size).toBe(0);

    await h.state.setIsScanning((prev) => !prev);

    expect(h.api.stopTopicScan).toHaveBeenCalledWith("c1");
    expect(h.conn().isScanning).toBe(false);
    expect(h.api.subscribe).not.toHaveBeenCalled();
    expect(h.api.unsubscribe).not.toHaveBeenCalled();
    expect(h.topics()).toEqual(["a", "b/#"]);
    h.dispose();
  });

  it("reverts isScanning and notifies when starting fails", async () => {
    const h = harness(onlineTab(true));
    h.api.startTopicScan.mockRejectedValueOnce(new Error("refused"));

    await h.state.setIsScanning(true);

    expect(h.conn().isScanning).toBe(false);
    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to start topic scan",
      "refused",
    );
    h.dispose();
  });

  it("stops right away while a start is in flight and ignores its failure", async () => {
    const h = harness(onlineTab(true));
    const start = deferred();
    h.api.startTopicScan.mockReturnValueOnce(start.promise);

    const starting = h.state.setIsScanning(true);
    await vi.waitFor(() => expect(h.api.startTopicScan).toHaveBeenCalled());
    const stopping = h.state.setIsScanning(false);

    // 開始の完了を待たずに止める (バックエンドが開始中のスキャンを打ち切る)。
    expect(h.api.stopTopicScan).toHaveBeenCalledWith("c1");
    expect(h.conn().isScanning).toBe(false);

    start.reject(new Error("topic scan was stopped"));
    await Promise.all([starting, stopping]);

    expect(h.notifier.error).not.toHaveBeenCalled();
    expect(h.conn().isScanning).toBe(false);
    expect(h.api.stopTopicScan).toHaveBeenCalledTimes(1);
    h.dispose();
  });

  it("stops again when a start that was stopped in flight succeeds", async () => {
    const h = harness(onlineTab(true));
    const start = deferred();
    h.api.startTopicScan.mockReturnValueOnce(start.promise);

    const starting = h.state.setIsScanning(true);
    await vi.waitFor(() => expect(h.api.startTopicScan).toHaveBeenCalled());
    await h.state.setIsScanning(false);
    // 停止が開始より先にバックエンドへ届いていた場合。
    start.resolve();
    await starting;

    expect(h.api.stopTopicScan).toHaveBeenCalledTimes(2);
    expect(h.conn().isScanning).toBe(false);
    h.dispose();
  });

  it("sends a restart only after the earlier start has finished", async () => {
    const h = harness(onlineTab(true));
    const first = deferred();
    h.api.startTopicScan.mockReturnValueOnce(first.promise);

    const starting = h.state.setIsScanning(true);
    await vi.waitFor(() => expect(h.api.startTopicScan).toHaveBeenCalled());
    const stopping = h.state.setIsScanning(false);
    const restarting = h.state.setIsScanning(true);
    await stopping;

    // 1 回目の開始が終わるまで 2 回目は送らない (打ち切られる前の開始に合流させない)。
    expect(h.api.startTopicScan).toHaveBeenCalledTimes(1);
    expect(h.conn().isScanning).toBe(true);

    first.reject(new Error("topic scan was stopped"));
    await Promise.all([starting, restarting]);

    expect(h.api.startTopicScan).toHaveBeenCalledTimes(2);
    // 1 回目の失敗では isScanning を戻さず、通知もしない。
    expect(h.conn().isScanning).toBe(true);
    expect(h.notifier.error).not.toHaveBeenCalled();
    h.dispose();
  });

  it("does not send a start that was stopped while waiting for an earlier one", async () => {
    const h = harness(onlineTab(true));
    const first = deferred();
    h.api.startTopicScan.mockReturnValueOnce(first.promise);

    const starting = h.state.setIsScanning(true);
    await vi.waitFor(() => expect(h.api.startTopicScan).toHaveBeenCalled());
    const calls = [
      h.state.setIsScanning(false),
      h.state.setIsScanning(true),
      h.state.setIsScanning(false),
    ];
    first.reject(new Error("topic scan was stopped"));
    await Promise.all([starting, ...calls]);

    expect(h.api.startTopicScan).toHaveBeenCalledTimes(1);
    expect(h.conn().isScanning).toBe(false);
    h.dispose();
  });
});

describe("createSubscriptionsState removeSubscription", () => {
  /** バックエンドが c1 の接続で持っている購読 (GetConnections の 1 件)。 */
  const backendStatus = (topics: string[]): ConnectionStatus => ({
    id: "c1",
    name: profile.name,
    broker: profile.broker,
    connected: true,
    profileId: profile.id,
    subscriptions: topics.map((topic) => ({ topic, qos: 0 })),
    scanning: false,
  });

  it("unsubscribes on an online tab that is still connecting", async () => {
    const h = harness(onlineTab(false));

    await h.state.removeSubscription(h.subscriptionId("a"));

    expect(h.api.unsubscribe).toHaveBeenCalledWith("c1", "a");
    expect(h.topics()).toEqual(["b/#"]);
    h.dispose();
  });

  it("removes the row without a toast when unsubscribing fails while not connected", async () => {
    const h = harness(onlineTab(false));
    h.api.unsubscribe.mockRejectedValueOnce(new Error("connection not found"));

    await h.state.removeSubscription(h.subscriptionId("a"));

    expect(h.notifier.error).not.toHaveBeenCalled();
    expect(h.logger.error).toHaveBeenCalledWith(
      "MQTT unsubscribe failed",
      expect.objectContaining({ connection_id: "c1", topic: "a" }),
    );
    expect(h.topics()).toEqual(["b/#"]);
    h.dispose();
  });

  // 応答を確認できなかった解除では、バックエンドは購読を外してからエラーを返す。
  it("notifies and removes the row when a failed unsubscribe left no subscription in the backend", async () => {
    const h = harness(onlineTab(true));
    h.api.unsubscribe.mockRejectedValueOnce(new Error("timeout"));
    h.api.getConnections.mockResolvedValueOnce([backendStatus(["b/#"])]);

    await h.state.removeSubscription(h.subscriptionId("a"));

    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to unsubscribe from a",
      "timeout",
    );
    expect(h.topics()).toEqual(["b/#"]);
    h.dispose();
  });

  it("keeps the row when a failed unsubscribe left the subscription in the backend", async () => {
    const h = harness(onlineTab(true));
    h.api.unsubscribe.mockRejectedValueOnce(new Error("broker refused"));
    h.api.getConnections.mockResolvedValueOnce([backendStatus(["a", "b/#"])]);

    await h.state.removeSubscription(h.subscriptionId("a"));

    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to unsubscribe from a",
      "broker refused",
    );
    expect(h.topics()).toEqual(["a", "b/#"]);
    h.dispose();
  });

  it("keeps the row when the backend subscriptions cannot be checked after a failed unsubscribe", async () => {
    const h = harness(onlineTab(true));
    h.api.unsubscribe.mockRejectedValueOnce(new Error("broker refused"));
    h.api.getConnections.mockRejectedValueOnce(new Error("rpc down"));

    await h.state.removeSubscription(h.subscriptionId("a"));

    expect(h.notifier.error).toHaveBeenCalledTimes(1);
    expect(h.topics()).toEqual(["a", "b/#"]);
    h.dispose();
  });

  it("does not unsubscribe on an offline tab", async () => {
    const h = harness(offlineTab());

    await h.state.removeSubscription(h.subscriptionId("a"));

    expect(h.api.unsubscribe).not.toHaveBeenCalled();
    expect(h.topics()).toEqual(["b/#"]);
    h.dispose();
  });
});

describe("createSubscriptionsState mqtt:subscription-dropped", () => {
  const dropped = (topic: string, connectionId = "c1") => ({
    connectionId,
    topic,
    error: "subscription rejected by broker",
  });

  it("removes only the dropped row and notifies once", () => {
    const h = harness(onlineTab(true));

    h.emit(WailsEvents.mqttSubscriptionDropped, dropped("b/#"));

    expect(h.topics()).toEqual(["a"]);
    expect(h.notifier.error).toHaveBeenCalledTimes(1);
    expect(h.notifier.error).toHaveBeenCalledWith(
      "Subscription to b/# was dropped",
      "subscription rejected by broker",
      { key: "c1:b/#" },
    );
    h.dispose();
  });

  it("does nothing for an unknown connection or topic", () => {
    const h = harness(onlineTab(true));

    h.emit(WailsEvents.mqttSubscriptionDropped, dropped("a", "other"));
    h.emit(WailsEvents.mqttSubscriptionDropped, dropped("c"));

    expect(h.topics()).toEqual(["a", "b/#"]);
    expect(h.notifier.error).not.toHaveBeenCalled();
    h.dispose();
  });

  it("does not add the row when the event arrives before the subscribe RPC returns", async () => {
    const h = harness(onlineTab(false));
    const subscribe = deferred();
    h.api.subscribe.mockReturnValueOnce(subscribe.promise);

    const adding = h.state.addSubscription("c", 0);
    await vi.waitFor(() => expect(h.api.subscribe).toHaveBeenCalled());
    h.emit(WailsEvents.mqttSubscriptionDropped, dropped("c"));
    subscribe.resolve();
    await adding;

    expect(h.topics()).toEqual(["a", "b/#"]);
    expect(h.notifier.error).toHaveBeenCalledTimes(1);
    expect(h.notifier.error).toHaveBeenCalledWith(
      "Subscription to c was dropped",
      "subscription rejected by broker",
      { key: "c1:c" },
    );

    // 印は残さないので、同じトピックをもう一度追加すると行が足される。
    await h.state.addSubscription("c", 0);

    expect(h.topics()).toEqual(["a", "b/#", "c"]);
    h.dispose();
  });

  it("stops listening when disposed", () => {
    const h = harness(onlineTab(true));
    h.dispose();

    expect(h.handlers.has(WailsEvents.mqttSubscriptionDropped)).toBe(false);
  });
});

describe("createSubscriptionsState addSubscription", () => {
  it("subscribes the typed topic without surrounding whitespace and clears the field", async () => {
    const h = harness(onlineTab(true));
    h.state.setNewTopic("  sensors/temp \t");
    h.state.setNewQos(1);

    await h.state.addSubscription();

    expect(h.api.subscribe).toHaveBeenCalledTimes(1);
    expect(h.api.subscribe).toHaveBeenCalledWith("c1", "sensors/temp", 1);
    expect(h.topics()).toEqual(["a", "b/#", "sensors/temp"]);
    expect(h.state.newTopic()).toBe("");
    h.dispose();
  });

  it("does not subscribe an already subscribed topic and clears the field", async () => {
    const h = harness(onlineTab(true));
    h.state.setNewTopic(" b/# ");

    await h.state.addSubscription();

    expect(h.api.subscribe).not.toHaveBeenCalled();
    expect(h.topics()).toEqual(["a", "b/#"]);
    expect(h.state.newTopic()).toBe("");
    h.dispose();
  });

  it("does nothing for a topic of only whitespace", async () => {
    const h = harness(onlineTab(true));
    h.state.setNewTopic("   ");

    await h.state.addSubscription();

    expect(h.api.subscribe).not.toHaveBeenCalled();
    expect(h.topics()).toEqual(["a", "b/#"]);
    expect(h.state.newTopic()).toBe("   ");
    h.dispose();
  });

  it("keeps the typed topic when subscribing fails", async () => {
    const h = harness(onlineTab(true));
    h.api.subscribe = vi.fn(async () => {
      throw new Error("not authorized");
    });
    h.state.setNewTopic("c");

    await h.state.addSubscription();

    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to subscribe to c",
      "not authorized",
    );
    expect(h.topics()).toEqual(["a", "b/#"]);
    expect(h.state.newTopic()).toBe("c");
    h.dispose();
  });

  // スキャン結果の一覧からの購読。入力途中のトピックを消さない。
  it("keeps the field when the topic is given as an argument", async () => {
    const h = harness(onlineTab(true));
    h.state.setNewTopic("typing");

    await h.state.addSubscription("found/topic", 0);
    // 既に購読しているトピックを渡した場合も同じ。
    await h.state.addSubscription("a", 0);

    expect(h.api.subscribe).toHaveBeenCalledTimes(1);
    expect(h.api.subscribe).toHaveBeenCalledWith("c1", "found/topic", 0);
    expect(h.topics()).toEqual(["a", "b/#", "found/topic"]);
    expect(h.state.newTopic()).toBe("typing");
    h.dispose();
  });
});
