import type { Diagnostic } from "@codemirror/lint";
import { createSignal } from "solid-js";
import type { ParseResult } from "./ports";

export type { ParseResult };

export function createEditorState() {
  const [editorContent, setEditorContent] = createSignal("");
  // savedContent はディスク上（または初期読込時）の内容。dirty 判定のベースライン。
  const [savedContent, setSavedContent] = createSignal("");
  const [parsedSpec, setParsedSpec] = createSignal<object | null>(null);
  const [parseErrors, setParseErrors] = createSignal<Diagnostic[]>([]);
  const [isPreviewing, setIsPreviewing] = createSignal(true);

  const isDirty = () => editorContent() !== savedContent();

  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  function updateContent(text: string, onParse: (text: string) => ParseResult) {
    setEditorContent(text);
    if (debounceTimer !== null) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      const result = onParse(text);
      if (result.ok) {
        setParsedSpec(result.spec);
        setParseErrors([]);
      } else {
        setParsedSpec(null);
        setParseErrors(result.errors);
      }
    }, 500);
  }

  function loadContent(text: string, onParse: (text: string) => ParseResult) {
    if (debounceTimer !== null) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
    setEditorContent(text);
    setSavedContent(text);
    const result = onParse(text);
    if (result.ok) {
      setParsedSpec(result.spec);
      setParseErrors([]);
    } else {
      setParsedSpec(null);
      setParseErrors(result.errors);
    }
  }

  // markSaved は現在の内容を保存済みベースラインに引き上げる（保存成功時に呼ぶ）。
  function markSaved() {
    setSavedContent(editorContent());
  }

  function togglePreview() {
    setIsPreviewing((v) => !v);
  }

  return {
    editorContent,
    parsedSpec,
    parseErrors,
    isPreviewing,
    isDirty,
    updateContent,
    loadContent,
    markSaved,
    togglePreview,
  };
}

export type EditorState = ReturnType<typeof createEditorState>;
