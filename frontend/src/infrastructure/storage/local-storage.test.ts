// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  createActiveRequestStorage,
  createExpandedFoldersStorage,
  createLastProfileStorage,
  createPresetsStorage,
  createProfileOrderStorage,
  createTargetOrderStorage,
  createThemeStorage,
  loadFromStorage,
  removeFromStorage,
  saveToStorage,
} from "./local-storage";

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
});

describe("createLastProfileStorage", () => {
  it("loadLastProfileId returns null when empty", () => {
    const storage = createLastProfileStorage();
    expect(storage.loadLastProfileId()).toBeNull();
  });

  it("saveLastProfileId persists id under the correct key", () => {
    const storage = createLastProfileStorage();
    storage.saveLastProfileId("profile-1");
    expect(storage.loadLastProfileId()).toBe("profile-1");
  });

  it("removeLastProfileId clears the stored id", () => {
    const storage = createLastProfileStorage();
    storage.saveLastProfileId("profile-1");
    storage.removeLastProfileId();
    expect(storage.loadLastProfileId()).toBeNull();
  });
});

describe("createPresetsStorage", () => {
  it("load returns [] when empty", () => {
    const storage = createPresetsStorage();
    expect(storage.load()).toEqual([]);
  });

  it("save persists presets and load retrieves them", () => {
    const storage = createPresetsStorage();
    const preset = {
      id: "p1",
      name: "Test",
      topic: "a/b",
      payload: "msg",
      qos: 0 as const,
      retain: false,
    };
    storage.save([preset]);
    expect(storage.load()).toEqual([preset]);
  });

  it("save overwrites existing presets", () => {
    const storage = createPresetsStorage();
    const preset1 = {
      id: "p1",
      name: "First",
      topic: "a/b",
      payload: "msg1",
      qos: 0 as const,
      retain: false,
    };
    const preset2 = {
      id: "p2",
      name: "Second",
      topic: "c/d",
      payload: "msg2",
      qos: 1 as const,
      retain: true,
    };
    storage.save([preset1]);
    storage.save([preset2]);
    expect(storage.load()).toEqual([preset2]);
  });

  it("defaults retain to false for presets saved before the field existed", () => {
    localStorage.setItem(
      "mqtt:presets",
      JSON.stringify([
        { id: "p1", name: "Legacy", topic: "a/b", payload: "msg", qos: 0 },
      ]),
    );
    const storage = createPresetsStorage();
    expect(storage.load()).toEqual([
      {
        id: "p1",
        name: "Legacy",
        topic: "a/b",
        payload: "msg",
        qos: 0,
        retain: false,
      },
    ]);
  });
});

describe("createThemeStorage", () => {
  it("returns 'light' as default theme when storage is empty", () => {
    const storage = createThemeStorage();
    expect(storage.load()).toBe("light");
  });

  it("save persists theme and load retrieves it", () => {
    const storage = createThemeStorage();
    storage.save("dark");
    expect(storage.load()).toBe("dark");
  });

  it("save overwrites existing theme", () => {
    const storage = createThemeStorage();
    storage.save("dark");
    storage.save("light");
    expect(storage.load()).toBe("light");
  });
});

describe("createActiveRequestStorage", () => {
  it("load returns null when empty", () => {
    const storage = createActiveRequestStorage();
    expect(storage.load()).toBeNull();
  });

  it("save persists requestId and collectionId and load retrieves them", () => {
    const storage = createActiveRequestStorage();
    storage.save("req-1", "col-1");
    expect(storage.load()).toEqual({
      requestId: "req-1",
      collectionId: "col-1",
    });
  });

  it("clear removes the stored entry", () => {
    const storage = createActiveRequestStorage();
    storage.save("req-1", "col-1");
    storage.clear();
    expect(storage.load()).toBeNull();
  });
});

describe("createExpandedFoldersStorage", () => {
  it("load returns {} when empty", () => {
    const storage = createExpandedFoldersStorage();
    expect(storage.load()).toEqual({});
  });

  it("save persists expanded ids and load retrieves them", () => {
    const storage = createExpandedFoldersStorage();
    storage.save({ "folder-1": true, "col-1": false });
    expect(storage.load()).toEqual({ "folder-1": true, "col-1": false });
  });

  it("keeps using the existing key", () => {
    localStorage.setItem(
      "wirexa:http:expandedFolders",
      JSON.stringify({ "folder-1": true }),
    );
    const storage = createExpandedFoldersStorage();
    expect(storage.load()).toEqual({ "folder-1": true });
    storage.save({ "folder-2": false });
    expect(
      JSON.parse(localStorage.getItem("wirexa:http:expandedFolders") ?? "null"),
    ).toEqual({ "folder-2": false });
  });
});

describe("createProfileOrderStorage", () => {
  it("load returns [] when empty", () => {
    const storage = createProfileOrderStorage();
    expect(storage.load()).toEqual([]);
  });

  it("save persists the order and load retrieves it", () => {
    const storage = createProfileOrderStorage();
    storage.save(["p2", "p1"]);
    expect(storage.load()).toEqual(["p2", "p1"]);
  });

  it("keeps using the existing key", () => {
    localStorage.setItem("mqtt:profileOrder", JSON.stringify(["p1", "p2"]));
    const storage = createProfileOrderStorage();
    expect(storage.load()).toEqual(["p1", "p2"]);
    storage.save(["p3"]);
    expect(
      JSON.parse(localStorage.getItem("mqtt:profileOrder") ?? "null"),
    ).toEqual(["p3"]);
  });
});

describe("createTargetOrderStorage", () => {
  it("load returns [] when empty", () => {
    const storage = createTargetOrderStorage();
    expect(storage.load()).toEqual([]);
  });

  it("save persists the order and load retrieves it", () => {
    const storage = createTargetOrderStorage();
    storage.save(["t2", "t1"]);
    expect(storage.load()).toEqual(["t2", "t1"]);
  });

  it("keeps using the existing key", () => {
    localStorage.setItem("udp:targetOrder", JSON.stringify(["t1", "t2"]));
    const storage = createTargetOrderStorage();
    expect(storage.load()).toEqual(["t1", "t2"]);
    storage.save(["t3"]);
    expect(
      JSON.parse(localStorage.getItem("udp:targetOrder") ?? "null"),
    ).toEqual(["t3"]);
  });
});

describe("loadFromStorage", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("returns the fallback when key does not exist", () => {
    expect(loadFromStorage("missing", 42)).toBe(42);
    expect(loadFromStorage("missing", "default")).toBe("default");
    expect(loadFromStorage("missing", null)).toBeNull();
  });

  it("returns the parsed value when key exists", () => {
    localStorage.setItem("key", JSON.stringify({ foo: "bar" }));
    expect(loadFromStorage("key", null)).toEqual({ foo: "bar" });
  });

  it("returns the fallback when stored value is corrupted JSON", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    localStorage.setItem("key", "not-valid-json{");
    expect(loadFromStorage("key", "fallback")).toBe("fallback");
  });

  it("returns a stored array", () => {
    localStorage.setItem("list", JSON.stringify([1, 2, 3]));
    expect(loadFromStorage("list", [])).toEqual([1, 2, 3]);
  });

  it("returns an array fallback for a missing key", () => {
    expect(loadFromStorage("missing", [] as number[])).toEqual([]);
  });

  it("returns a stored string value", () => {
    localStorage.setItem("str", JSON.stringify("hello"));
    expect(loadFromStorage("str", "")).toBe("hello");
  });

  it("returns a stored boolean value", () => {
    localStorage.setItem("flag", JSON.stringify(false));
    expect(loadFromStorage("flag", true)).toBe(false);
  });

  it("returns a stored number", () => {
    localStorage.setItem("num", JSON.stringify(42));
    expect(loadFromStorage("num", 0)).toBe(42);
  });

  it("returns the fallback for truncated JSON", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    localStorage.setItem("bad", '{"key": "val');
    expect(loadFromStorage("bad", "default")).toBe("default");
  });

  // ストレージへのアクセスが拒否された環境では getItem 自体が SecurityError を投げる。
  it("returns the fallback when getItem throws", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    expect(loadFromStorage("key", "fallback")).toBe("fallback");
    expect(console.warn).toHaveBeenCalled();
  });

  it("returns the fallback when the value fails the shape check", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    localStorage.setItem("key", JSON.stringify({ foo: 1 }));
    const isString = (v: unknown): v is string => typeof v === "string";
    expect(loadFromStorage("key", "fallback", isString)).toBe("fallback");
    expect(console.warn).toHaveBeenCalled();
  });
});

// JSON としては正しいが形が違う値（手で書き換えた・旧版の別形式など）でも起動時に落とさない。
describe("stored values with an unexpected shape", () => {
  beforeEach(() => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  it.each([
    "null",
    "{}",
    '"x"',
    "1",
  ])("falls back to [] when presets are %s", (raw) => {
    localStorage.setItem("mqtt:presets", raw);
    expect(createPresetsStorage().load()).toEqual([]);
  });

  it("drops malformed presets and keeps the valid ones", () => {
    const valid = {
      id: "p1",
      name: "ok",
      topic: "a",
      payload: "",
      qos: 1,
      retain: true,
    };
    localStorage.setItem(
      "mqtt:presets",
      JSON.stringify([
        null,
        "p0",
        { ...valid, id: 2 },
        { ...valid, qos: 3 },
        { ...valid, retain: "yes" },
        valid,
      ]),
    );
    expect(createPresetsStorage().load()).toEqual([valid]);
  });

  it.each([
    '"blue"',
    "null",
    "1",
  ])("falls back to light when the theme is %s", (raw) => {
    localStorage.setItem("app:theme", raw);
    expect(createThemeStorage().load()).toBe("light");
  });

  it("loads a stored dark theme", () => {
    localStorage.setItem("app:theme", '"dark"');
    expect(createThemeStorage().load()).toBe("dark");
  });

  it.each([
    ["mqtt:profileOrder", createProfileOrderStorage],
    ["udp:targetOrder", createTargetOrderStorage],
  ])("falls back to [] when %s is not a string array", (key, create) => {
    for (const raw of ["null", "{}", '"p1"', "[1,2]", '["p1",null]']) {
      localStorage.setItem(key, raw);
      expect(create().load()).toEqual([]);
    }
  });

  it.each([
    "null",
    "[]",
    '"x"',
    '{"f1":"yes"}',
  ])("falls back to {} when expanded folders are %s", (raw) => {
    localStorage.setItem("wirexa:http:expandedFolders", raw);
    expect(createExpandedFoldersStorage().load()).toEqual({});
  });

  it.each([
    '"r1"',
    "[]",
    '{"requestId":"r1"}',
    '{"requestId":1,"collectionId":"c"}',
  ])("falls back to null when the active request is %s", (raw) => {
    localStorage.setItem("wirexa:http:activeRequest", raw);
    expect(createActiveRequestStorage().load()).toBeNull();
  });

  it.each([
    "1",
    "{}",
    "[]",
  ])("falls back to null when the last profile id is %s", (raw) => {
    localStorage.setItem("mqtt:lastActiveProfileId", raw);
    expect(createLastProfileStorage().loadLastProfileId()).toBeNull();
  });
});

describe("saveToStorage", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("saves an object and returns true", () => {
    const result = saveToStorage("key", { foo: "bar" });
    expect(result).toBe(true);
    expect(localStorage.getItem("key")).toBe(JSON.stringify({ foo: "bar" }));
  });

  it("saves an array and returns true", () => {
    const result = saveToStorage("list", [1, 2, 3]);
    expect(result).toBe(true);
    expect(JSON.parse(localStorage.getItem("list") ?? "null")).toEqual([
      1, 2, 3,
    ]);
  });

  it("saves a primitive number and returns true", () => {
    expect(saveToStorage("num", 99)).toBe(true);
    expect(JSON.parse(localStorage.getItem("num") ?? "null")).toBe(99);
  });

  it("saves a boolean and returns true", () => {
    expect(saveToStorage("flag", true)).toBe(true);
  });

  it("saves a null value and returns true", () => {
    expect(saveToStorage("nullkey", null)).toBe(true);
    expect(localStorage.getItem("nullkey")).toBe("null");
  });

  it("returns false and does not throw when localStorage.setItem throws", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.spyOn(Storage.prototype, "setItem").mockImplementationOnce(() => {
      throw new Error("QuotaExceededError");
    });
    expect(saveToStorage("key", "value")).toBe(false);
  });

  it("overwrites an existing key", () => {
    saveToStorage("key", "first");
    saveToStorage("key", "second");
    expect(JSON.parse(localStorage.getItem("key") ?? "null")).toBe("second");
  });

  it("returns false when JSON.stringify throws (circular reference)", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    const circular: Record<string, unknown> = {};
    circular.self = circular;
    expect(saveToStorage("key", circular)).toBe(false);
  });
});

describe("removeFromStorage", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("removes an existing item", () => {
    localStorage.setItem("key", "value");
    removeFromStorage("key");
    expect(localStorage.getItem("key")).toBeNull();
  });

  it("does not throw for a non-existent key", () => {
    expect(() => removeFromStorage("nonexistent")).not.toThrow();
  });

  it("does not throw when localStorage.removeItem throws", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.spyOn(Storage.prototype, "removeItem").mockImplementationOnce(() => {
      throw new Error("Storage error");
    });
    expect(() => removeFromStorage("key")).not.toThrow();
  });

  it("removes only the targeted key", () => {
    localStorage.setItem("a", "1");
    localStorage.setItem("b", "2");
    removeFromStorage("a");
    expect(localStorage.getItem("a")).toBeNull();
    expect(localStorage.getItem("b")).toBe("2");
  });
});
