import { afterEach, describe, expect, it, vi } from "vitest";
import { createEditorState } from "./editor";
import type { ParseResult } from "./ports";

const okParse = (): ParseResult => ({ ok: true, spec: {} });

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
    vi.useFakeTimers();
    const s = createEditorState();
    s.loadContent("a", okParse);
    s.updateContent("b", okParse);
    // dirty はデバウンス完了を待たず即座に反映される
    expect(s.isDirty()).toBe(true);
    vi.advanceTimersByTime(500);
    expect(s.isDirty()).toBe(true);
  });
});
