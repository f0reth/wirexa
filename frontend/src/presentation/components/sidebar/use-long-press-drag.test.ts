// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { makeLongPressDragHandlers } from "./use-long-press-drag";

const LONG_PRESS_MS = 250;

function mouseDown(x: number, y: number, button = 0) {
  return new MouseEvent("mousedown", { clientX: x, clientY: y, button });
}

function dispatchMove(x: number, y: number) {
  document.dispatchEvent(
    new MouseEvent("mousemove", { clientX: x, clientY: y }),
  );
}

function dispatchUp() {
  document.dispatchEvent(new MouseEvent("mouseup"));
}

describe("makeLongPressDragHandlers", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    // 途中で終わったテストのリスナーを次のテストへ持ち越さない。
    dispatchUp();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("activates at the press origin once LONG_PRESS_MS elapses", () => {
    const onActivate = vi.fn();
    const suppressRef = { suppress: false };
    const { handleMouseDown } = makeLongPressDragHandlers(
      suppressRef,
      onActivate,
    );

    handleMouseDown(mouseDown(10, 20));
    vi.advanceTimersByTime(LONG_PRESS_MS - 1);
    expect(onActivate).not.toHaveBeenCalled();
    expect(suppressRef.suppress).toBe(false);

    vi.advanceTimersByTime(1);
    expect(onActivate).toHaveBeenCalledWith(10, 20);
    expect(suppressRef.suppress).toBe(true);
  });

  it("activates immediately at the cursor once the move exceeds the threshold", () => {
    const onActivate = vi.fn();
    const { handleMouseDown } = makeLongPressDragHandlers(
      { suppress: false },
      onActivate,
    );

    handleMouseDown(mouseDown(0, 0));
    dispatchMove(6, 0);

    expect(onActivate).toHaveBeenCalledWith(6, 0);

    // 長押しタイマーはキャンセルされ、2 度目の発火は起きない。
    vi.advanceTimersByTime(LONG_PRESS_MS);
    expect(onActivate).toHaveBeenCalledTimes(1);
  });

  it("does not activate while the move stays within the threshold", () => {
    const onActivate = vi.fn();
    const { handleMouseDown } = makeLongPressDragHandlers(
      { suppress: false },
      onActivate,
    );

    handleMouseDown(mouseDown(0, 0));
    dispatchMove(3, 3); // 距離 ≈ 4.24 < 5
    expect(onActivate).not.toHaveBeenCalled();
    dispatchUp();
  });

  it("does not activate at exactly the threshold distance", () => {
    const onActivate = vi.fn();
    const { handleMouseDown } = makeLongPressDragHandlers(
      { suppress: false },
      onActivate,
    );

    handleMouseDown(mouseDown(10, 10));
    dispatchMove(13, 14); // 距離 = 5
    expect(onActivate).not.toHaveBeenCalled();

    dispatchMove(13, 15); // 距離 ≈ 5.83 > 5
    expect(onActivate).toHaveBeenCalledWith(13, 15);
  });

  it("leaves no document listeners behind after a move activation", () => {
    const onActivate = vi.fn();
    const removeSpy = vi.spyOn(document, "removeEventListener");
    const { handleMouseDown } = makeLongPressDragHandlers(
      { suppress: false },
      onActivate,
    );

    handleMouseDown(mouseDown(0, 0));
    dispatchMove(10, 0);
    expect(onActivate).toHaveBeenCalledTimes(1);
    expect(removeSpy).toHaveBeenCalledWith("mousemove", expect.any(Function));
    expect(removeSpy).toHaveBeenCalledWith("mouseup", expect.any(Function));

    dispatchMove(500, 500);
    vi.advanceTimersByTime(LONG_PRESS_MS);
    expect(onActivate).toHaveBeenCalledTimes(1);
  });

  it("cancels the long press when the mouse is released early", () => {
    const onActivate = vi.fn();
    const suppressRef = { suppress: false };
    const { handleMouseDown } = makeLongPressDragHandlers(
      suppressRef,
      onActivate,
    );

    handleMouseDown(mouseDown(0, 0));
    vi.advanceTimersByTime(LONG_PRESS_MS - 1);
    dispatchUp();
    vi.advanceTimersByTime(LONG_PRESS_MS);

    expect(onActivate).not.toHaveBeenCalled();
    expect(suppressRef.suppress).toBe(false);
  });

  it("ignores non-primary buttons", () => {
    const onActivate = vi.fn();
    const addSpy = vi.spyOn(document, "addEventListener");
    const { handleMouseDown } = makeLongPressDragHandlers(
      { suppress: false },
      onActivate,
    );

    handleMouseDown(mouseDown(0, 0, 2));
    vi.advanceTimersByTime(LONG_PRESS_MS);

    expect(onActivate).not.toHaveBeenCalled();
    expect(addSpy).not.toHaveBeenCalled();
  });

  it("leaves no document listeners behind after activating", () => {
    const onActivate = vi.fn();
    const { handleMouseDown } = makeLongPressDragHandlers(
      { suppress: false },
      onActivate,
    );

    handleMouseDown(mouseDown(0, 0));
    vi.advanceTimersByTime(LONG_PRESS_MS);
    expect(onActivate).toHaveBeenCalledTimes(1);

    // 発火後に残ったリスナが動くと 2 度目の activate が起きる。
    dispatchMove(500, 500);
    dispatchUp();
    expect(onActivate).toHaveBeenCalledTimes(1);
  });

  it("leaves no document listeners behind after an early release", () => {
    const onActivate = vi.fn();
    const { handleMouseDown } = makeLongPressDragHandlers(
      { suppress: false },
      onActivate,
    );

    handleMouseDown(mouseDown(0, 0));
    dispatchUp();

    dispatchMove(500, 500);
    expect(onActivate).not.toHaveBeenCalled();
  });
});
