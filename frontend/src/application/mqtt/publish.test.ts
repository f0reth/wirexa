import { describe, expect, it, vi } from "vitest";
import type { BrokerProfile, ConnectionState } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import type { PublishDraft } from "./presets";
import { canPublish, createPublishState, type MqttPublishApi } from "./publish";

const profile: BrokerProfile = {
  id: "p1",
  name: "p1",
  broker: "tcp://p1:1883",
  clientId: "",
  username: "",
  password: "",
  useTls: false,
};

function onlineTab(connected: boolean): ConnectionState {
  return {
    type: "online",
    connectionId: "c1",
    profileId: profile.id,
    profile,
    connected,
    closed: false,
  };
}

const offlineTab: ConnectionState = {
  type: "offline",
  connectionId: "offline-p1",
  profileId: profile.id,
  profile,
};

function makeDraft(patch: Partial<PublishDraft> = {}): PublishDraft {
  return { topic: "a/b", payload: "hello", qos: 1, retain: false, ...patch };
}

function harness(
  conn: ConnectionState | null,
  draft: PublishDraft,
  publish: MqttPublishApi["publish"] = async () => {},
) {
  const api = { publish: vi.fn(publish) } satisfies MqttPublishApi;
  const notifier: Notifier = {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
  const state = createPublishState(
    api,
    () => conn,
    () => draft,
    notifier,
  );
  return { api, notifier, state };
}

describe("canPublish", () => {
  it("accepts a topic with a payload", () => {
    expect(canPublish(makeDraft())).toBe(true);
  });

  it("rejects an empty or blank topic", () => {
    expect(canPublish(makeDraft({ topic: "" }))).toBe(false);
    expect(canPublish(makeDraft({ topic: "  " }))).toBe(false);
    expect(canPublish(makeDraft({ topic: "", retain: true }))).toBe(false);
  });

  it("rejects an empty or blank payload without retain", () => {
    expect(canPublish(makeDraft({ payload: "" }))).toBe(false);
    expect(canPublish(makeDraft({ payload: " \n" }))).toBe(false);
  });

  it("accepts an empty payload with retain (clears the retained message)", () => {
    expect(canPublish(makeDraft({ payload: "", retain: true }))).toBe(true);
  });
});

describe("createPublishState publishDraft", () => {
  it("sends the draft to the active connection", async () => {
    const h = harness(onlineTab(true), makeDraft({ retain: true }));

    await h.state.publishDraft();

    expect(h.api.publish).toHaveBeenCalledWith("c1", "a/b", "hello", 1, true);
    expect(h.notifier.error).not.toHaveBeenCalled();
  });

  it("does not send a draft that cannot be published", async () => {
    const h = harness(onlineTab(true), makeDraft({ topic: "" }));

    await h.state.publishDraft();

    expect(h.api.publish).not.toHaveBeenCalled();
  });

  it("does not send without a connected tab", async () => {
    for (const conn of [null, offlineTab, onlineTab(false)]) {
      const h = harness(conn, makeDraft());

      await h.state.publishDraft();

      expect(h.api.publish).not.toHaveBeenCalled();
    }
  });

  it("notifies when the publish fails", async () => {
    const h = harness(onlineTab(true), makeDraft(), async () => {
      throw new Error("not connected");
    });

    await h.state.publishDraft();

    expect(h.notifier.error).toHaveBeenCalledWith(
      "Failed to publish message",
      "not connected",
    );
  });
});
