import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { TargetOrderStorage } from "../../domain/udp/ports";
import type { UdpTarget } from "../../domain/udp/types";
import type { Notifier } from "../../domain/ui/ports";
import { createTargetsState, type UdpTargetApi } from "./targets";

function makeNotifier(): Notifier {
  return {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
}

function makeTarget(id: string): UdpTarget {
  return { id, name: id, host: "127.0.0.1", port: 9000 };
}

/** すべて成功する API。個々のテストで失敗させたいメソッドだけ差し替える。 */
function makeApi(initial: UdpTarget[] = []): UdpTargetApi {
  return {
    getTargets: vi.fn(async () => initial),
    saveTarget: vi.fn(async (target: UdpTarget) => target),
    deleteTarget: vi.fn(async () => {}),
  };
}

/** 並び順をメモリに持つストレージ。 */
function makeOrderStorage(initial: string[] = []) {
  let stored = [...initial];
  const storage: TargetOrderStorage = {
    load: () => [...stored],
    save: (ids) => {
      stored = [...ids];
    },
  };
  return { storage, stored: () => stored };
}

function withState(
  api: UdpTargetApi,
  fn: (
    state: ReturnType<typeof createTargetsState>,
    notifier: Notifier,
  ) => Promise<void>,
  orderStorage: TargetOrderStorage = makeOrderStorage().storage,
): Promise<void> {
  const notifier = makeNotifier();
  return createRoot(async (dispose) => {
    await fn(createTargetsState(api, notifier, orderStorage), notifier);
    dispose();
  });
}

describe("createTargetsState order", () => {
  it("applies the stored order on refresh", async () => {
    const api = makeApi([makeTarget("t1"), makeTarget("t2"), makeTarget("t3")]);
    const { storage } = makeOrderStorage(["t3", "t1"]);

    await withState(
      api,
      async (state) => {
        await state.refreshTargets();
        // 並び順に無い t2 は末尾に回る。
        expect(state.targets.map((t) => t.id)).toEqual(["t3", "t1", "t2"]);
      },
      storage,
    );
  });

  it("saves the order when targets are reordered", async () => {
    const api = makeApi([makeTarget("t1"), makeTarget("t2")]);
    const { storage, stored } = makeOrderStorage();

    await withState(
      api,
      async (state) => {
        await state.refreshTargets();
        state.reorderTargets(0, 1);

        expect(state.targets.map((t) => t.id)).toEqual(["t2", "t1"]);
        expect(stored()).toEqual(["t2", "t1"]);
      },
      storage,
    );
  });

  it("does not save when the indices are out of range", async () => {
    const api = makeApi([makeTarget("t1")]);
    const { storage, stored } = makeOrderStorage(["t1"]);
    const save = vi.spyOn(storage, "save");

    await withState(
      api,
      async (state) => {
        await state.refreshTargets();
        state.reorderTargets(0, 5);

        expect(save).not.toHaveBeenCalled();
        expect(stored()).toEqual(["t1"]);
      },
      storage,
    );
  });
});

describe("createTargetsState failure contract", () => {
  it("notifies and rethrows when saving fails", async () => {
    const api = makeApi();
    api.saveTarget = vi.fn(async () => {
      throw new Error("disk full");
    });

    await withState(api, async (state, notifier) => {
      await expect(state.saveTarget(makeTarget("t1"))).rejects.toThrow(
        "disk full",
      );
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to save target",
        "disk full",
      );
    });
  });

  it("returns the saved target and refreshes the list", async () => {
    const api = makeApi([makeTarget("t1")]);
    api.saveTarget = vi.fn(async (t: UdpTarget) => ({ ...t, id: "server-1" }));

    await withState(api, async (state, notifier) => {
      const saved = await state.saveTarget({ ...makeTarget(""), name: "new" });

      expect(saved).toEqual({ ...makeTarget("server-1"), name: "new" });
      expect(api.getTargets).toHaveBeenCalledTimes(1);
      expect(state.targets.map((t) => t.id)).toEqual(["t1"]);
      expect(notifier.error).not.toHaveBeenCalled();
    });
  });

  it("deletes the target and refreshes the list", async () => {
    const api = makeApi([makeTarget("t2")]);

    await withState(api, async (state, notifier) => {
      await state.deleteTarget("t1");

      expect(api.deleteTarget).toHaveBeenCalledWith("t1");
      expect(api.getTargets).toHaveBeenCalledTimes(1);
      expect(state.targets.map((t) => t.id)).toEqual(["t2"]);
      expect(notifier.error).not.toHaveBeenCalled();
    });
  });

  // refreshTargets はガードせず、呼び出し側（起動時の読み込み）に失敗を伝える。
  it("propagates getTargets failures", async () => {
    const api = makeApi();
    api.getTargets = vi.fn(async () => {
      throw new Error("rpc down");
    });

    await withState(api, async (state, notifier) => {
      await expect(state.refreshTargets()).rejects.toThrow("rpc down");
      expect(notifier.error).not.toHaveBeenCalled();
    });
  });

  // 保存自体は成功しているが、直後の再読み込みも notifyOnError の中にあるため失敗として扱われる。
  it("reports failure when the refresh after save fails", async () => {
    const api = makeApi();
    api.getTargets = vi.fn(async () => {
      throw new Error("rpc down");
    });

    await withState(api, async (state, notifier) => {
      await expect(state.saveTarget(makeTarget("t1"))).rejects.toThrow(
        "rpc down",
      );
      expect(api.saveTarget).toHaveBeenCalled();
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to save target",
        "rpc down",
      );
    });
  });

  it("notifies and swallows the error when deleting fails", async () => {
    const api = makeApi();
    api.deleteTarget = vi.fn(async () => {
      throw new Error("gone");
    });

    await withState(api, async (state, notifier) => {
      await expect(state.deleteTarget("t1")).resolves.toBeUndefined();
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to delete target",
        "gone",
      );
    });
  });
});
