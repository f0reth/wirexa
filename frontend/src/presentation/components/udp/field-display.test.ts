import { describe, expect, it } from "vitest";
import {
  byteCountLabel,
  FIELD_VALUE_INPUT_TYPES,
  FIELD_VALUE_LABELS,
  FIELD_VALUE_PLACEHOLDERS,
} from "./field-display";

describe("byteCountLabel", () => {
  it("shows the used/declared length for variable-length fields", () => {
    expect(byteCountLabel({ kind: "var-length", bytes: 3, length: 4 })).toBe(
      "3/4",
    );
  });

  it("shows the fixed size for numeric fields", () => {
    expect(byteCountLabel({ kind: "fixed-size", bytes: 8 })).toBe("8 bytes");
  });

  it("shows the reason a value is invalid", () => {
    expect(byteCountLabel({ kind: "non-ascii" })).toBe("non-ASCII");
    expect(byteCountLabel({ kind: "invalid-hex" })).toBe("invalid hex");
    expect(byteCountLabel({ kind: "out-of-range" })).toBe("out of range");
  });
});

describe("FIELD_VALUE_LABELS / FIELD_VALUE_PLACEHOLDERS / FIELD_VALUE_INPUT_TYPES", () => {
  it("labels the value input per field kind", () => {
    expect(FIELD_VALUE_LABELS).toEqual({
      ascii: "Value (ASCII)",
      hex: "Value (hex)",
      integer: "Value",
      "wide-integer": "Value",
      float: "Value",
    });
  });

  it("uses a placeholder matching the field kind", () => {
    expect(FIELD_VALUE_PLACEHOLDERS).toEqual({
      ascii: "hello",
      hex: "0a 1b 2c",
      integer: "0",
      "wide-integer": "0",
      float: "1.0",
    });
  });

  it("falls back to a text input where number input would lose precision", () => {
    expect(FIELD_VALUE_INPUT_TYPES["wide-integer"]).toBe("text");
    expect(FIELD_VALUE_INPUT_TYPES.ascii).toBe("text");
    expect(FIELD_VALUE_INPUT_TYPES.hex).toBe("text");
    expect(FIELD_VALUE_INPUT_TYPES.integer).toBe("number");
    expect(FIELD_VALUE_INPUT_TYPES.float).toBe("number");
  });
});
