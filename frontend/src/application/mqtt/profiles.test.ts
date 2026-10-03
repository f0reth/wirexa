import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { ProfileOrderStorage } from "../../domain/mqtt/ports";
import type { BrokerProfile } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
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

function makeNotifier(): Notifier {
  return {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
}

function withState(
  api: ProfileApi,
  fn: (state: ReturnType<typeof createProfilesState>) => Promise<void>,
  orderStorage: ProfileOrderStorage = makeOrderStorage().storage,
  notifier: Notifier = makeNotifier(),
): Promise<void> {
  return createRoot(async (dispose) => {
    await fn(createProfilesState(api, notifier, orderStorage));
    dispose();
  });
}

describe("createEmptyProfile", () => {
  it("leaves the id empty so the server assigns one", () => {
    expect(createEmptyProfile()).toEqual({
      id: "",
      name: "",
      broker: "mqtt://localhost:1883",
      clientId: "",
      username: "",
      password: "",
      useTls: false,
    });
  });

  it("returns a fresh object each time", () => {
    expect(createEmptyProfile()).not.toBe(createEmptyProfile());
  });
});

describe("createProfilesState failures", () => {
  it("leaves the list untouched when saving fails", async () => {
    const api = makeApi([makeProfile("p1")]);
    api.saveProfile = vi.fn(async () => {
      throw new Error("disk full");
    });
    const { storage, stored } = makeOrderStorage(["p1"]);
    const notifier = makeNotifier();
    await withState(
      api,
      async (state) => {
        await state.loadProfiles();

        await expect(state.saveProfile(makeProfile("p2"))).rejects.toThrow(
          "disk full",
        );
        expect(state.profiles().map((p) => p.id)).toEqual(["p1"]);
        expect(stored()).toEqual(["p1"]);
        // 接続バーの入力のたびに保存が走るので、同じプロファイルの通知は key で 1 つにまとめる。
        expect(notifier.error).toHaveBeenCalledTimes(1);
        expect(notifier.error).toHaveBeenCalledWith(
          "Failed to save broker",
          "disk full",
          { key: "mqtt:save-profile:p2" },
        );
      },
      storage,
      notifier,
    );
  });

  it("keeps the profile when deleting fails", async () => {
    const api = makeApi([makeProfile("p1"), makeProfile("p2")]);
    api.deleteProfile = vi.fn(async () => {
      throw new Error("locked");
    });
    const { storage, stored } = makeOrderStorage(["p1", "p2"]);
    const notifier = makeNotifier();
    await withState(
      api,
      async (state) => {
        await state.loadProfiles();

        await expect(state.deleteProfile("p1")).rejects.toThrow("locked");
        expect(state.profiles().map((p) => p.id)).toEqual(["p1", "p2"]);
        expect(stored()).toEqual(["p1", "p2"]);
        expect(notifier.error).toHaveBeenCalledTimes(1);
        expect(notifier.error).toHaveBeenCalledWith(
          "Failed to delete broker",
          "locked",
        );
      },
      storage,
      notifier,
    );
  });

  it("does not notify when saving and deleting succeed", async () => {
    const notifier = makeNotifier();
    await withState(
      makeApi([makeProfile("p1")]),
      async (state) => {
        await state.loadProfiles();
        await state.saveProfile(makeProfile("p1", { name: "Renamed" }));
        await state.deleteProfile("p1");

        expect(state.profiles()).toEqual([]);
        expect(notifier.error).not.toHaveBeenCalled();
      },
      undefined,
      notifier,
    );
  });

  it("propagates a load failure and keeps the list empty", async () => {
    const api = makeApi();
    api.getProfiles = vi.fn(async () => {
      throw new Error("rpc down");
    });
    await withState(api, async (state) => {
      await expect(state.loadProfiles()).rejects.toThrow("rpc down");
      expect(state.profiles()).toEqual([]);
    });
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

  it("does not save when the indices are out of range", async () => {
    const api = makeApi([makeProfile("p1"), makeProfile("p2")]);
    const storage = { load: vi.fn(() => []), save: vi.fn() };
    await withState(
      api,
      async (state) => {
        await state.loadProfiles();
        state.reorderProfiles(0, 2);
        state.reorderProfiles(-1, 0);

        expect(state.profiles().map((p) => p.id)).toEqual(["p1", "p2"]);
        expect(storage.save).not.toHaveBeenCalled();
      },
      storage,
    );
  });
});
