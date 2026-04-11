import type { Diagnostic } from "@codemirror/lint";
import { createSignal } from "solid-js";

export type ParseResult =
  | { ok: true; spec: object }
  | { ok: false; errors: Diagnostic[] };

export function createEditorState() {
  const [editorContent, setEditorContent] = createSignal("");
  const [parsedSpec, setParsedSpec] = createSignal<object | null>(null);
  const [parseErrors, setParseErrors] = createSignal<Diagnostic[]>([]);
  const [isPreviewing, setIsPreviewing] = createSignal(true);

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
    const result = onParse(text);
    if (result.ok) {
      setParsedSpec(result.spec);
      setParseErrors([]);
    } else {
      setParsedSpec(null);
      setParseErrors(result.errors);
    }
  }

  function togglePreview() {
    setIsPreviewing((v) => !v);
  }

  return {
    editorContent,
    parsedSpec,
    parseErrors,
    isPreviewing,
    updateContent,
    loadContent,
    togglePreview,
  };
}

export type EditorState = ReturnType<typeof createEditorState>;
