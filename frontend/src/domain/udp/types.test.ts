import { describe, expect, it } from "vitest";
import { isPayloadEncoding, PAYLOAD_ENCODINGS } from "./types";

describe("isPayloadEncoding", () => {
  it.each(PAYLOAD_ENCODINGS)("returns true for %s", (encoding) => {
    expect(isPayloadEncoding(encoding)).toBe(true);
  });

  it("returns false for unknown encodings", () => {
    expect(isPayloadEncoding("hex")).toBe(false);
    expect(isPayloadEncoding("base64")).toBe(false);
    expect(isPayloadEncoding("binary")).toBe(false);
    expect(isPayloadEncoding("")).toBe(false);
  });

  it("returns false for capitalized variants", () => {
    expect(isPayloadEncoding("Text")).toBe(false);
    expect(isPayloadEncoding("JSON")).toBe(false);
    expect(isPayloadEncoding("Fixed")).toBe(false);
  });

  it("returns false for partial matches", () => {
    expect(isPayloadEncoding("tex")).toBe(false);
    expect(isPayloadEncoding("fixe")).toBe(false);
  });
});
