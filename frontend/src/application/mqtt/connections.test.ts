import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { ConnectionPersistence } from "../../domain/mqtt/ports";
import type { BrokerProfile, ConnectionStatus } from "../../domain/mqtt/types";
import type { Logger } from "../logger";
import {
  createConnectionsState,
  type MqttConnectionApi,
  type MqttEventListener,
} from "./connections";

const noopLogger: Logger = { info: () => {}, error: () => {} };
const noopEvent: MqttEventListener = () => () => {};

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
  return createRoot((dispose) => {
    const state = createConnectionsState(
      makeApi(async () => live),
      noopEvent,
      makePersistence(lastProfileId),
      () => profiles,
      async () => {},
      noopLogger,
      1000,
      1000,
    );
    return { state, dispose };
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
        async () => {},
        noopLogger,
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
