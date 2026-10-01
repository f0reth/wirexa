import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { BrokerProfile } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import type { Logger } from "../logger";
import type { ConnectionStateExt } from "./connections";
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
  return createRoot((dispose) => {
    const state = createSubscriptionsState(
      () => conn,
      (id, updater) => {
        if (id === conn.connectionId) conn = updater(conn);
      },
      api,
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

  it("notifies when unsubscribing fails on a connected tab", async () => {
    const h = harness(onlineTab(true));
    h.api.unsubscribe.mockRejectedValueOnce(new Error("timeout"));

    await h.state.removeSubscription(h.subscriptionId("a"));

    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to unsubscribe from a",
      "timeout",
    );
    expect(h.topics()).toEqual(["b/#"]);
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
