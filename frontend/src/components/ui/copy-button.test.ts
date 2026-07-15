import { createRoot } from "solid-js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createCopyButton } from "./copy-button";

describe("createCopyButton", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("navigator", { clipboard: { writeText: vi.fn() } });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("writes to the clipboard and marks copied until resetMs elapses", () => {
    createRoot((dispose) => {
      const { copy, isCopied } = createCopyButton();
      expect(isCopied()).toBe(false);

      copy("hello");
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith("hello");
      expect(isCopied()).toBe(true);

      vi.advanceTimersByTime(1499);
      expect(isCopied()).toBe(true);
      vi.advanceTimersByTime(1);
      expect(isCopied()).toBe(false);

      dispose();
    });
  });

  it("tracks copied state per key for list usage", () => {
    createRoot((dispose) => {
      const { copy, isCopied } = createCopyButton<number>();

      copy("a", 1);
      expect(isCopied(1)).toBe(true);
      expect(isCopied(2)).toBe(false);

      // 単一タイマーなので別キーをコピーするとマーカーがそちらへ移る
      copy("b", 2);
      expect(isCopied(2)).toBe(true);
      expect(isCopied(1)).toBe(false);

      dispose();
    });
  });

  it("honors a custom resetMs", () => {
    createRoot((dispose) => {
      const { copy, isCopied } = createCopyButton(500);
      copy("x");
      vi.advanceTimersByTime(500);
      expect(isCopied()).toBe(false);
      dispose();
    });
  });

  it("clears the pending timer on cleanup (D-4 regression)", () => {
    let api!: ReturnType<typeof createCopyButton>;
    const dispose = createRoot((d) => {
      api = createCopyButton();
      return d;
    });

    api.copy("x");
    expect(api.isCopied()).toBe(true);
    expect(vi.getTimerCount()).toBe(1);

    dispose();
    // onCleanup が保留中タイマーを破棄している
    expect(vi.getTimerCount()).toBe(0);
    expect(() => vi.advanceTimersByTime(1500)).not.toThrow();
  });
});
