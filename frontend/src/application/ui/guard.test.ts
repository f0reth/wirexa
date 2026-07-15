import { afterEach, describe, expect, it, vi } from "vitest";
import { runGuarded } from "./guard";
import { notify } from "./notifications";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("runGuarded", () => {
  it("runs the fn side effects and does not notify on success", async () => {
    const errorSpy = vi.spyOn(notify, "error");
    let ran = false;
    await runGuarded("Failed", async () => {
      ran = true;
    });
    expect(ran).toBe(true);
    expect(errorSpy).not.toHaveBeenCalled();
  });

  it("notifies with the Error message when fn throws an Error", async () => {
    const errorSpy = vi.spyOn(notify, "error").mockReturnValue("id");
    await runGuarded("Failed to add folder", async () => {
      throw new Error("boom");
    });
    expect(errorSpy).toHaveBeenCalledWith("Failed to add folder", "boom");
  });

  it("notifies with String(err) when fn throws a non-Error", async () => {
    const errorSpy = vi.spyOn(notify, "error").mockReturnValue("id");
    await runGuarded("Failed", async () => {
      throw "nope";
    });
    expect(errorSpy).toHaveBeenCalledWith("Failed", "nope");
  });

  it("accepts an fn that returns a value", async () => {
    const errorSpy = vi.spyOn(notify, "error");
    await runGuarded("Failed", () => Promise.resolve("ignored"));
    expect(errorSpy).not.toHaveBeenCalled();
  });
});
