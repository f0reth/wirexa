import { describe, expect, it } from "vitest";
import { moveItem } from "./array";

describe("moveItem", () => {
  it("moves an item forward", () => {
    expect(moveItem(["a", "b", "c", "d"], 0, 2)).toEqual(["b", "c", "a", "d"]);
  });

  it("moves an item backward", () => {
    expect(moveItem(["a", "b", "c", "d"], 3, 1)).toEqual(["a", "d", "b", "c"]);
  });

  it("returns an unchanged order when from === to", () => {
    expect(moveItem(["a", "b", "c"], 1, 1)).toEqual(["a", "b", "c"]);
  });

  it("does not mutate the input array", () => {
    const input = ["a", "b", "c"];
    moveItem(input, 0, 2);
    expect(input).toEqual(["a", "b", "c"]);
  });

  it("returns null when from is out of range", () => {
    expect(moveItem(["a", "b"], -1, 0)).toBeNull();
    expect(moveItem(["a", "b"], 2, 0)).toBeNull();
  });

  it("returns null when to is out of range", () => {
    expect(moveItem(["a", "b"], 0, -1)).toBeNull();
    expect(moveItem(["a", "b"], 0, 2)).toBeNull();
  });

  it("returns null for an empty array", () => {
    expect(moveItem([], 0, 0)).toBeNull();
  });
});
