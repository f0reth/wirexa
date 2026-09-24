import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { ProfileOrderStorage } from "../../domain/mqtt/ports";
import type { BrokerProfile } from "../../domain/mqtt/types";
import {
  createEmptyProfile,
  createProfilesState,
  type ProfileApi,
} from "./profiles";

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

/** 並び順をメモリに持つストレージ。 */
function makeOrderStorage(initial: string[] = []) {
  let stored = [...initial];
  const storage: ProfileOrderStorage = {
    load: () => [...stored],
    save: (ids) => {
      stored = [...ids];
    },
  };
  return { storage, stored: () => stored };
}

function withState(
  api: ProfileApi,
  fn: (state: ReturnType<typeof createProfilesState>) => Promise<void>,
  orderStorage: ProfileOrderStorage = makeOrderStorage().storage,
): Promise<void> {
  return createRoot(async (dispose) => {
    await fn(createProfilesState(api, orderStorage));
    dispose();
  });
}

describe("createEmptyProfile", () => {
  it("leaves the id empty so the server assigns one", () => {
    expect(createEmptyProfile().id).toBe("");
  });
});

describe("createProfilesState saveProfile", () => {
  it("puts the server-assigned id into the state and the stored order", async () => {
    const { storage, stored } = makeOrderStorage();
    await withState(
      makeApi(),
      async (state) => {
        const saved = await state.saveProfile(createEmptyProfile());

        expect(saved.id).toBe("server-1");
        expect(state.profiles()).toEqual([saved]);
        expect(stored()).toEqual(["server-1"]);
      },
      storage,
    );
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

describe("createProfilesState order", () => {
  it("applies the stored order on load", async () => {
    const api = makeApi([makeProfile("p1"), makeProfile("p2")]);
    const { storage } = makeOrderStorage(["p2", "p1"]);
    await withState(
      api,
      async (state) => {
        await state.loadProfiles();
        expect(state.profiles().map((p) => p.id)).toEqual(["p2", "p1"]);
      },
      storage,
    );
  });

  it("saves the order when profiles are reordered or deleted", async () => {
    const api = makeApi([makeProfile("p1"), makeProfile("p2")]);
    const { storage, stored } = makeOrderStorage();
    await withState(
      api,
      async (state) => {
        await state.loadProfiles();
        state.reorderProfiles(0, 1);
        expect(stored()).toEqual(["p2", "p1"]);

        await state.deleteProfile("p2");
        expect(stored()).toEqual(["p1"]);
      },
      storage,
    );
  });
});
