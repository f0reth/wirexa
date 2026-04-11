import * as YAML from "yaml";
import type { ParseResult } from "../../application/openapi/ports";

export function parseSpec(text: string): ParseResult {
  if (!text.trim()) {
    return { ok: false, errors: [] };
  }

  try {
    const spec = YAML.parse(text) as object;
    if (spec === null || typeof spec !== "object") {
      return {
        ok: false,
        errors: [
          {
            from: 0,
            to: text.length,
            severity: "error",
            message: "Document must be a YAML/JSON object",
          },
        ],
      };
    }
    return { ok: true, spec };
  } catch (err) {
    if (err instanceof YAML.YAMLParseError) {
      const pos = err.pos?.[0] ?? 0;
      const end = err.pos?.[1] ?? pos + 1;
      return {
        ok: false,
        errors: [
          {
            from: pos,
            to: end,
            severity: "error",
            message: err.message,
          },
        ],
      };
    }
    return {
      ok: false,
      errors: [
        {
          from: 0,
          to: text.length,
          severity: "error",
          message: String(err),
        },
      ],
    };
  }
}
