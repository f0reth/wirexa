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
    return { state, api, notifier, logger, subscriptionId, topics, dispose };
  });
}

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
