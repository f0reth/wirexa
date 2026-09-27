import { createRoot } from "solid-js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createCopyButton } from "./copy-button";

describe("createCopyButton", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("navigator", {
      clipboard: { writeText: vi.fn(async (_text: string) => {}) },
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("writes to the clipboard and marks copied until resetMs elapses", async () => {
    await createRoot(async (dispose) => {
      const { copy, isCopied } = createCopyButton();
      expect(isCopied()).toBe(false);

      await copy("hello");
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith("hello");
      expect(isCopied()).toBe(true);

      vi.advanceTimersByTime(1499);
      expect(isCopied()).toBe(true);
      vi.advanceTimersByTime(1);
      expect(isCopied()).toBe(false);

      dispose();
    });
  });

  it("tracks copied state per key for list usage", async () => {
    await createRoot(async (dispose) => {
      const { copy, isCopied } = createCopyButton<number>();

      await copy("a", 1);
      expect(isCopied(1)).toBe(true);
      expect(isCopied(2)).toBe(false);

      // 単一タイマーなので別キーをコピーするとマーカーがそちらへ移る
      await copy("b", 2);
      expect(isCopied(2)).toBe(true);
      expect(isCopied(1)).toBe(false);

      dispose();
    });
  });

  it("honors a custom resetMs", async () => {
    await createRoot(async (dispose) => {
      const { copy, isCopied } = createCopyButton(500);
      await copy("x");
      vi.advanceTimersByTime(500);
      expect(isCopied()).toBe(false);
      dispose();
    });
  });

  it("clears the pending timer on cleanup (D-4 regression)", async () => {
    let api!: ReturnType<typeof createCopyButton>;
    const dispose = createRoot((d) => {
      api = createCopyButton();
      return d;
    });

    await api.copy("x");
    expect(api.isCopied()).toBe(true);
    expect(vi.getTimerCount()).toBe(1);

    dispose();
    // onCleanup が保留中タイマーを破棄している
    expect(vi.getTimerCount()).toBe(0);
    expect(() => vi.advanceTimersByTime(1500)).not.toThrow();
  });

  it("does not mark copied when the clipboard write fails", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.mocked(navigator.clipboard.writeText).mockRejectedValueOnce(
      new DOMException("denied", "NotAllowedError"),
    );
    await createRoot(async (dispose) => {
      const { copy, isCopied } = createCopyButton();

      await expect(copy("secret")).resolves.toBeUndefined();

      expect(isCopied()).toBe(false);
      expect(vi.getTimerCount()).toBe(0);
      expect(console.warn).toHaveBeenCalled();
      dispose();
    });
  });

  it("keeps the previous mark when a later copy fails", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    await createRoot(async (dispose) => {
      const { copy, isCopied } = createCopyButton<number>();
      await copy("a", 1);

      vi.mocked(navigator.clipboard.writeText).mockRejectedValueOnce(
        new Error("denied"),
      );
      await copy("b", 2);

      expect(isCopied(1)).toBe(true);
      expect(isCopied(2)).toBe(false);
      dispose();
    });
  });

  it("restarts the reset timer on a repeated copy", async () => {
    await createRoot(async (dispose) => {
      const { copy, isCopied } = createCopyButton();
      await copy("x");
      vi.advanceTimersByTime(1000);

      await copy("x");
      vi.advanceTimersByTime(1000);
      expect(isCopied()).toBe(true);
      expect(vi.getTimerCount()).toBe(1);

      vi.advanceTimersByTime(500);
      expect(isCopied()).toBe(false);
      dispose();
    });
  });

  it("does not start a timer when the write finishes after cleanup", async () => {
    let finish: () => void = () => {};
    vi.mocked(navigator.clipboard.writeText).mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    );
    let api!: ReturnType<typeof createCopyButton>;
    const dispose = createRoot((d) => {
      api = createCopyButton();
      return d;
    });

    const copying = api.copy("x");
    dispose();
    finish();
    await copying;

    expect(api.isCopied()).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
  });
});
