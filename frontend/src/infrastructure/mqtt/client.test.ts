import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../wailsjs/go/adapters/MqttHandler", () => ({
  Connect: vi.fn(),
  DeleteProfile: vi.fn(),
  Disconnect: vi.fn(),
  GetConnections: vi.fn(),
  GetProfiles: vi.fn(),
  Publish: vi.fn(),
  SaveProfile: vi.fn(),
  Subscribe: vi.fn(),
  Unsubscribe: vi.fn(),
}));

import * as Handler from "../../../wailsjs/go/adapters/MqttHandler";
import {
  connect,
  deleteProfile,
  disconnect,
  getConnections,
  getProfiles,
  publish,
  saveProfile,
  subscribe,
  unsubscribe,
} from "./client";

// ---- helpers ----

function makeProfile(overrides: Record<string, unknown> = {}) {
  return {
    id: "profile-1",
    name: "Local",
    broker: "mqtt://localhost:1883",
    clientId: "test-client",
    username: "",
    password: "",
    useTls: false,
    ...overrides,
  };
}

function makeConnectionStatus(overrides: Record<string, unknown> = {}) {
  return {
    id: "conn-1",
    name: "Local",
    broker: "mqtt://localhost:1883",
    connected: true,
    ...overrides,
  };
}

// ---- tests ----

beforeEach(() => {
  vi.clearAllMocks();
});

describe("connect", () => {
  it("returns the connection id returned by the backend", async () => {
    vi.mocked(Handler.Connect).mockResolvedValue("conn-abc");
    const id = await connect(makeProfile());
    expect(id).toBe("conn-abc");
  });

  it("passes the profile fields to the backend", async () => {
    vi.mocked(Handler.Connect).mockResolvedValue("conn-1");
    const profile = makeProfile({
      name: "Remote",
      broker: "mqtts://remote:8883",
      clientId: "my-client",
      username: "user",
      password: "pass",
      useTls: true,
    });
    await connect(profile);
    expect(Handler.Connect).toHaveBeenCalledWith({
      name: "Remote",
      broker: "mqtts://remote:8883",
      clientId: "my-client",
      username: "user",
      password: "pass",
      useTls: true,
    });
  });

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.Connect).mockRejectedValue(
      new Error("broker unreachable"),
    );
    await expect(connect(makeProfile())).rejects.toThrow("broker unreachable");
  });
});

describe("disconnect", () => {
  it("calls Disconnect with the correct connection id", async () => {
    vi.mocked(Handler.Disconnect).mockResolvedValue(undefined);
    await disconnect("conn-abc");
    expect(Handler.Disconnect).toHaveBeenCalledWith("conn-abc");
  });
});

describe("subscribe", () => {
  it("calls Subscribe with the correct params", async () => {
    vi.mocked(Handler.Subscribe).mockResolvedValue(undefined);
    await subscribe("conn-1", "sensors/#", 1);
    expect(Handler.Subscribe).toHaveBeenCalledWith("conn-1", "sensors/#", 1);
  });

  it.each([0, 1, 2] as const)("passes QoS %d to the backend", async (qos) => {
    vi.mocked(Handler.Subscribe).mockResolvedValue(undefined);
    await subscribe("conn-1", "test/topic", qos);
    expect(Handler.Subscribe).toHaveBeenCalledWith("conn-1", "test/topic", qos);
  });
});

describe("unsubscribe", () => {
  it("calls Unsubscribe with the correct params", async () => {
    vi.mocked(Handler.Unsubscribe).mockResolvedValue(undefined);
    await unsubscribe("conn-1", "sensors/#");
    expect(Handler.Unsubscribe).toHaveBeenCalledWith("conn-1", "sensors/#");
  });
});

describe("publish", () => {
  it("calls Publish with the correct params", async () => {
    vi.mocked(Handler.Publish).mockResolvedValue(undefined);
    await publish("conn-1", "home/light", "on", 0, false);
    expect(Handler.Publish).toHaveBeenCalledWith(
      "conn-1",
      "home/light",
      "on",
      0,
      false,
    );
  });

  it("passes the retain flag correctly", async () => {
    vi.mocked(Handler.Publish).mockResolvedValue(undefined);
    await publish("conn-1", "home/light", "off", 1, true);
    expect(Handler.Publish).toHaveBeenCalledWith(
      "conn-1",
      "home/light",
      "off",
      1,
      true,
    );
  });

  it.each([0, 1, 2] as const)("passes QoS %d to the backend", async (qos) => {
    vi.mocked(Handler.Publish).mockResolvedValue(undefined);
    await publish("conn-1", "topic", "payload", qos, false);
    expect(Handler.Publish).toHaveBeenCalledWith(
      "conn-1",
      "topic",
      "payload",
      qos,
      false,
    );
  });
});

describe("getConnections", () => {
  it("returns an empty array when there are no active connections", async () => {
    vi.mocked(Handler.GetConnections).mockResolvedValue([]);
    const result = await getConnections();
    expect(result).toEqual([]);
  });

  it("returns a list of connection statuses", async () => {
    vi.mocked(Handler.GetConnections).mockResolvedValue([
      makeConnectionStatus() as never,
      makeConnectionStatus({ id: "conn-2", connected: false }) as never,
    ]);
    const result = await getConnections();
    expect(result).toHaveLength(2);
    expect(result[0].id).toBe("conn-1");
    expect(result[0].connected).toBe(true);
    expect(result[1].id).toBe("conn-2");
    expect(result[1].connected).toBe(false);
  });
});

describe("getProfiles", () => {
  it("returns an empty array when there are no saved profiles", async () => {
    vi.mocked(Handler.GetProfiles).mockResolvedValue([]);
    const result = await getProfiles();
    expect(result).toEqual([]);
  });

  it("returns a list of broker profiles", async () => {
    vi.mocked(Handler.GetProfiles).mockResolvedValue([
      makeProfile() as never,
      makeProfile({ id: "profile-2", name: "Remote" }) as never,
    ]);
    const result = await getProfiles();
    expect(result).toHaveLength(2);
    expect(result[0].id).toBe("profile-1");
    expect(result[1].name).toBe("Remote");
  });
});

describe("saveProfile", () => {
  it("passes all profile fields to the backend", async () => {
    vi.mocked(Handler.SaveProfile).mockResolvedValue(undefined);
    const profile = makeProfile({
      id: "profile-1",
      name: "Local",
      broker: "mqtt://localhost:1883",
      clientId: "test-client",
      username: "user",
      password: "pass",
      useTls: false,
    });
    await saveProfile(profile);
    expect(Handler.SaveProfile).toHaveBeenCalledWith({
      id: "profile-1",
      name: "Local",
      broker: "mqtt://localhost:1883",
      clientId: "test-client",
      username: "user",
      password: "pass",
      useTls: false,
    });
  });
});

describe("deleteProfile", () => {
  it("calls DeleteProfile with the correct id", async () => {
    vi.mocked(Handler.DeleteProfile).mockResolvedValue(undefined);
    await deleteProfile("profile-1");
    expect(Handler.DeleteProfile).toHaveBeenCalledWith("profile-1");
  });
});
