import type { Diagnostic } from "@codemirror/lint";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createEditorState } from "./editor";
import type { ParseResult } from "./ports";

const okParse = (): ParseResult => ({ ok: true, spec: {} });

// updateContent のデバウンスのタイマーを次のテストへ持ち越さないよう、全テストで偽タイマーを使う。
beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("createEditorState dirty tracking", () => {
  it("is not dirty right after loadContent", () => {
    const s = createEditorState();
    s.loadContent("openapi: 3.0.0", okParse);
    expect(s.editorContent()).toBe("openapi: 3.0.0");
    expect(s.isDirty()).toBe(false);
  });

  it("becomes dirty when the content is edited", () => {
    const s = createEditorState();
    s.loadContent("a", okParse);
    s.updateContent("a-edited", okParse);
    expect(s.isDirty()).toBe(true);
  });

  it("is not dirty when edited back to the loaded content", () => {
    const s = createEditorState();
    s.loadContent("a", okParse);
    s.updateContent("b", okParse);
    expect(s.isDirty()).toBe(true);
    s.updateContent("a", okParse);
    expect(s.isDirty()).toBe(false);
  });

  it("markSaved clears the dirty flag at the current content", () => {
    const s = createEditorState();
    s.loadContent("a", okParse);
    s.updateContent("b", okParse);
    expect(s.isDirty()).toBe(true);
    s.markSaved();
    expect(s.isDirty()).toBe(false);
    // 以降の編集で再び dirty になる
    s.updateContent("c", okParse);
    expect(s.isDirty()).toBe(true);
  });

  it("loadContent resets the baseline so a reload is not dirty", () => {
    const s = createEditorState();
    s.loadContent("a", okParse);
    s.updateContent("b", okParse);
    expect(s.isDirty()).toBe(true);
    // 別ファイルを読み込むと dirty はリセットされる
    s.loadContent("other", okParse);
    expect(s.isDirty()).toBe(false);
  });

  it("debounced parsing does not affect dirty state timing", () => {
    const s = createEditorState();
    s.loadContent("a", okParse);
    s.updateContent("b", okParse);
    // dirty はデバウンス完了を待たず即座に反映される
    expect(s.isDirty()).toBe(true);
    vi.advanceTimersByTime(500);
    expect(s.isDirty()).toBe(true);
  });
});

const error: Diagnostic = {
  from: 0,
  to: 1,
  severity: "error",
  message: "bad yaml",
};
const ngParse = (): ParseResult => ({ ok: false, errors: [error] });

describe("createEditorState parse results", () => {
  it("exposes the spec and clears errors on a successful load", () => {
    const s = createEditorState();
    s.loadContent("x", ngParse);
    expect(s.parsedSpec()).toBeNull();
    expect(s.parseErrors()).toEqual([error]);

    const spec = { openapi: "3.0.0" };
    s.loadContent("y", () => ({ ok: true, spec }));

    expect(s.parsedSpec()).toBe(spec);
    expect(s.parseErrors()).toEqual([]);
  });

  it("exposes parse errors and clears the spec on failure after an edit", () => {
    const s = createEditorState();
    s.loadContent("a", () => ({ ok: true, spec: { openapi: "3.0.0" } }));

    s.updateContent("broken", ngParse);
    // デバウンス中は前回の結果のまま。
    expect(s.parsedSpec()).not.toBeNull();

    vi.advanceTimersByTime(500);
    expect(s.parsedSpec()).toBeNull();
    expect(s.parseErrors()).toEqual([error]);
  });
});

describe("createEditorState debounce", () => {
  it("parses only the last edit after the debounce", () => {
    const s = createEditorState();
    const parse = vi.fn(okParse);

    s.updateContent("a", parse);
    vi.advanceTimersByTime(499);
    s.updateContent("ab", parse);
    vi.advanceTimersByTime(499);
    expect(parse).not.toHaveBeenCalled();

    s.updateContent("abc", parse);
    vi.advanceTimersByTime(500);
    expect(parse).toHaveBeenCalledTimes(1);
    expect(parse).toHaveBeenCalledWith("abc");
  });

  it("cancels a pending parse when a file is loaded", () => {
    const s = createEditorState();
    const staleParse = vi.fn(ngParse);
    const spec = { openapi: "3.1.0" };

    s.updateContent("stale edit", staleParse);
    s.loadContent("loaded", () => ({ ok: true, spec }));
    vi.advanceTimersByTime(1000);

    expect(staleParse).not.toHaveBeenCalled();
    expect(s.parsedSpec()).toBe(spec);
    expect(s.parseErrors()).toEqual([]);
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("createEditorState preview", () => {
  it("starts in preview mode and toggles", () => {
    const s = createEditorState();
    expect(s.isPreviewing()).toBe(true);
    s.togglePreview();
    expect(s.isPreviewing()).toBe(false);
    s.togglePreview();
    expect(s.isPreviewing()).toBe(true);
  });
});
