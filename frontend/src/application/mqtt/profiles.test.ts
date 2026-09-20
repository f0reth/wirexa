// @vitest-environment jsdom
import { createRoot } from "solid-js";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BrokerProfile } from "../../domain/mqtt/types";
import {
  createEmptyProfile,
  createProfilesState,
  type ProfileApi,
} from "./profiles";

const PROFILE_ORDER_KEY = "mqtt:profileOrder";

function makeProfile(id: string, over: Partial<BrokerProfile> = {}) {
  return {
    id,
    name: id,
    broker: "mqtt://localhost:1883",
    clientId: "",
    username: "",
    password: "",
    useTls: false,
    ...over,
  };
}

// makeApi は Go 側と同じく「空 ID は採番、非空 ID はそのまま」返す偽 API。
function makeApi(initial: BrokerProfile[] = []): ProfileApi {
  let seq = 0;
  return {
    getProfiles: vi.fn(async () => initial),
    saveProfile: vi.fn(async (profile: BrokerProfile) => ({
      ...profile,
      id: profile.id || `server-${++seq}`,
    })),
    deleteProfile: vi.fn(async () => {}),
  };
}

function withState(
  api: ProfileApi,
  fn: (state: ReturnType<typeof createProfilesState>) => Promise<void>,
): Promise<void> {
  return createRoot(async (dispose) => {
    await fn(createProfilesState(api));
    dispose();
  });
}

beforeEach(() => {
  localStorage.clear();
});

describe("createEmptyProfile", () => {
  it("leaves the id empty so the server assigns one", () => {
    expect(createEmptyProfile().id).toBe("");
  });
});

describe("createProfilesState saveProfile", () => {
  it("puts the server-assigned id into the state and the order key", async () => {
    await withState(makeApi(), async (state) => {
      const saved = await state.saveProfile(createEmptyProfile());

      expect(saved.id).toBe("server-1");
      expect(state.profiles()).toEqual([saved]);
      expect(
        JSON.parse(localStorage.getItem(PROFILE_ORDER_KEY) ?? "[]"),
      ).toEqual(["server-1"]);
    });
  });

  it("updates in place when an existing profile is saved", async () => {
    const api = makeApi([makeProfile("p1"), makeProfile("p2")]);
    await withState(api, async (state) => {
      await state.loadProfiles();
      await state.saveProfile(makeProfile("p1", { name: "Renamed" }));

      expect(state.profiles().map((p) => p.id)).toEqual(["p1", "p2"]);
      expect(state.profiles()[0].name).toBe("Renamed");
    });
  });
});
