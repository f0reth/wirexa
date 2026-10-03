import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { PresetStorage } from "../../domain/mqtt/ports";
import type { PublishPreset } from "../../domain/mqtt/types";
import { createPresetsState } from "./presets";

function makeStorage(initial: PublishPreset[] = []): PresetStorage {
  let stored = initial;
  return {
    load: () => stored,
    save: vi.fn((presets: PublishPreset[]) => {
      stored = presets;
    }),
  };
}

function makePreset(id: string, over: Partial<PublishPreset> = {}) {
  return {
    id,
    name: id,
    topic: `topic/${id}`,
    payload: `payload-${id}`,
    qos: 1 as const,
    retain: true,
    ...over,
  };
}

function withState(fn: (state: ReturnType<typeof createPresetsState>) => void) {
  createRoot((dispose) => {
    fn(createPresetsState(makeStorage([makePreset("p1"), makePreset("p2")])));
    dispose();
  });
}

describe("createPresetsState draft", () => {
  it("starts with an empty draft and no selection", () => {
    withState((state) => {
      expect(state.selectedPresetId()).toBeNull();
      expect(state.draft()).toEqual({
        topic: "",
        payload: "",
        qos: 0,
        retain: false,
      });
    });
  });

  it("loads the preset into the draft when it is selected", () => {
    withState((state) => {
      state.selectPreset("p2");
      expect(state.draft()).toEqual({
        topic: "topic/p2",
        payload: "payload-p2",
        qos: 1,
        retain: true,
      });
    });
  });

  it("writes draft edits back to the selected preset", () => {
    withState((state) => {
      state.selectPreset("p1");
      state.updateDraft({ topic: "sensors/temp", retain: false });

      expect(state.draft().topic).toBe("sensors/temp");
      expect(state.draft().retain).toBe(false);
      const preset = state.presets().find((p) => p.id === "p1");
      expect(preset?.topic).toBe("sensors/temp");
      expect(preset?.retain).toBe(false);
      // 他のプリセットは変わらない。
      expect(state.presets().find((p) => p.id === "p2")?.topic).toBe(
        "topic/p2",
      );
    });
  });

  it("keeps the draft editable while no preset is selected", () => {
    withState((state) => {
      state.updateDraft({ topic: "ad-hoc", payload: "once" });

      expect(state.draft().topic).toBe("ad-hoc");
      expect(state.presets().map((p) => p.topic)).toEqual([
        "topic/p1",
        "topic/p2",
      ]);
    });
  });

  it("clears the draft when a new preset is added", () => {
    withState((state) => {
      state.selectPreset("p1");
      state.addPreset();

      expect(state.draft()).toEqual({
        topic: "",
        payload: "",
        qos: 0,
        retain: false,
      });
      expect(state.selectedPresetId()).not.toBe("p1");
    });
  });

  it("keeps the draft when the selected preset is removed", () => {
    withState((state) => {
      state.selectPreset("p1");
      state.removePreset("p1");

      expect(state.selectedPresetId()).toBeNull();
      expect(state.draft().topic).toBe("topic/p1");
    });
  });

  it("does not select a missing preset", () => {
    const { state, storage, dispose } = setup();
    state.selectPreset("p1");

    state.selectPreset("missing");
    expect(state.selectedPresetId()).toBe("p1");
    expect(state.draft().topic).toBe("topic/p1");

    // 選択が変わっていないので、編集は p1 に書き戻される。
    state.updateDraft({ topic: "edited" });
    expect(storage.save).toHaveBeenLastCalledWith([
      makePreset("p1", { topic: "edited" }),
      makePreset("p2"),
    ]);
    dispose();
  });
});

/** state とストレージの spy を返す。 */
function setup(initial = [makePreset("p1"), makePreset("p2")]) {
  const storage = makeStorage(initial);
  return createRoot((dispose) => ({
    state: createPresetsState(storage),
    storage: vi.mocked(storage),
    dispose,
  }));
}

describe("createPresetsState persistence", () => {
  it("names an added preset 'no name', selects it and saves it", () => {
    const { state, storage, dispose } = setup([]);

    state.addPreset();
    state.addPreset();

    expect(state.presets().map((p) => p.name)).toEqual(["no name", "no name"]);
    expect(state.presets()[0].id).not.toBe(state.presets()[1].id);
    expect(state.presets()[0]).toMatchObject({
      topic: "",
      payload: "",
      qos: 0,
      retain: false,
    });
    expect(state.selectedPresetId()).toBe(state.presets()[1].id);
    expect(storage.save).toHaveBeenCalledTimes(2);
    expect(storage.save).toHaveBeenLastCalledWith(state.presets());
    dispose();
  });

  it("saves updates and removals", () => {
    const { state, storage, dispose } = setup();

    state.updatePreset("p2", { name: "renamed" });
    expect(storage.save).toHaveBeenLastCalledWith([
      makePreset("p1"),
      makePreset("p2", { name: "renamed" }),
    ]);

    state.removePreset("p1");
    expect(storage.save).toHaveBeenLastCalledWith([
      makePreset("p2", { name: "renamed" }),
    ]);
    dispose();
  });

  it("does not save draft edits while no preset is selected", () => {
    const { state, storage, dispose } = setup();

    state.updateDraft({ topic: "ad-hoc" });

    expect(storage.save).not.toHaveBeenCalled();
    dispose();
  });

  it("reorders presets and saves the new order", () => {
    const { state, storage, dispose } = setup();

    state.reorderPresets(0, 1);

    expect(state.presets().map((p) => p.id)).toEqual(["p2", "p1"]);
    expect(storage.save).toHaveBeenLastCalledWith(state.presets());
    dispose();
  });

  it("ignores out-of-range reorder", () => {
    const { state, storage, dispose } = setup();

    state.reorderPresets(0, 2);
    state.reorderPresets(-1, 0);

    expect(state.presets().map((p) => p.id)).toEqual(["p1", "p2"]);
    expect(storage.save).not.toHaveBeenCalled();
    dispose();
  });
});
