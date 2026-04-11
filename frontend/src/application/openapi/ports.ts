import type { Diagnostic } from "@codemirror/lint";

export type ParseResult =
  | { ok: true; spec: object }
  | { ok: false; errors: Diagnostic[] };
