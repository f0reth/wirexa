import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  loadFromStorage,
  removeFromStorage,
  saveToStorage,
} from "./local-storage";

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
    localStorage.setItem("bad", '{"key": "val');
    expect(loadFromStorage("bad", "default")).toBe("default");
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
