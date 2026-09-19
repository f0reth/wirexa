import { describe, expect, it, vi } from "vitest";
import type { Notifier } from "../../domain/ui/ports";
import { notifyOnError, runGuarded } from "./guard";

function makeNotifier(): Notifier {
  return {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
}

describe("runGuarded", () => {
  it("runs the fn side effects and does not notify on success", async () => {
    const notifier = makeNotifier();
    let ran = false;
    await runGuarded(notifier, "Failed", async () => {
      ran = true;
    });
    expect(ran).toBe(true);
    expect(notifier.error).not.toHaveBeenCalled();
  });

  it("notifies with the Error message when fn throws an Error", async () => {
    const notifier = makeNotifier();
    await runGuarded(notifier, "Failed to add folder", async () => {
      throw new Error("boom");
    });
    expect(notifier.error).toHaveBeenCalledWith("Failed to add folder", "boom");
  });

  it("notifies with String(err) when fn throws a non-Error", async () => {
    const notifier = makeNotifier();
    await runGuarded(notifier, "Failed", async () => {
      throw "nope";
    });
    expect(notifier.error).toHaveBeenCalledWith("Failed", "nope");
  });

  it("accepts an fn that returns a value", async () => {
    const notifier = makeNotifier();
    await runGuarded(notifier, "Failed", () => Promise.resolve("ignored"));
    expect(notifier.error).not.toHaveBeenCalled();
  });
});

describe("notifyOnError", () => {
  it("returns the value and does not notify on success", async () => {
    const notifier = makeNotifier();
    const value = await notifyOnError(notifier, "Failed", async () => "ok");
    expect(value).toBe("ok");
    expect(notifier.error).not.toHaveBeenCalled();
  });

  it("notifies and rethrows so the caller can branch on failure", async () => {
    const notifier = makeNotifier();
    await expect(
      notifyOnError(notifier, "Failed to create collection", async () => {
        throw new Error("boom");
      }),
    ).rejects.toThrow("boom");
    expect(notifier.error).toHaveBeenCalledWith(
      "Failed to create collection",
      "boom",
    );
  });
});
