import { describe, expect, it } from "vitest";
import {
  isPayloadEncoding,
  isValidNumericFieldValue,
  PAYLOAD_ENCODINGS,
} from "./types";

describe("isPayloadEncoding", () => {
  it.each(PAYLOAD_ENCODINGS)("returns true for %s", (encoding) => {
    expect(isPayloadEncoding(encoding)).toBe(true);
  });

  it("returns false for unknown encodings", () => {
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

describe("isValidNumericFieldValue", () => {
  it("returns true for empty string (uninput)", () => {
    expect(isValidNumericFieldValue("", "uint8")).toBe(true);
    expect(isValidNumericFieldValue("", "int64")).toBe(true);
    expect(isValidNumericFieldValue("", "float32")).toBe(true);
  });

  describe("uint types", () => {
    it("uint8: valid range 0-255", () => {
      expect(isValidNumericFieldValue("0", "uint8")).toBe(true);
      expect(isValidNumericFieldValue("255", "uint8")).toBe(true);
      expect(isValidNumericFieldValue("256", "uint8")).toBe(false);
      expect(isValidNumericFieldValue("-1", "uint8")).toBe(false);
      expect(isValidNumericFieldValue("1.5", "uint8")).toBe(false);
    });
    it("uint16: valid range 0-65535", () => {
      expect(isValidNumericFieldValue("65535", "uint16")).toBe(true);
      expect(isValidNumericFieldValue("65536", "uint16")).toBe(false);
    });
    it("uint32: valid range 0-4294967295", () => {
      expect(isValidNumericFieldValue("4294967295", "uint32")).toBe(true);
      expect(isValidNumericFieldValue("4294967296", "uint32")).toBe(false);
    });
    it("uint64: valid range 0-18446744073709551615", () => {
      expect(isValidNumericFieldValue("18446744073709551615", "uint64")).toBe(
        true,
      );
      expect(isValidNumericFieldValue("18446744073709551616", "uint64")).toBe(
        false,
      );
      expect(isValidNumericFieldValue("-1", "uint64")).toBe(false);
    });
  });

  describe("int types", () => {
    it("int8: valid range -128 to 127", () => {
      expect(isValidNumericFieldValue("-128", "int8")).toBe(true);
      expect(isValidNumericFieldValue("127", "int8")).toBe(true);
      expect(isValidNumericFieldValue("-129", "int8")).toBe(false);
      expect(isValidNumericFieldValue("128", "int8")).toBe(false);
    });
    it("int16: valid range -32768 to 32767", () => {
      expect(isValidNumericFieldValue("-32768", "int16")).toBe(true);
      expect(isValidNumericFieldValue("32767", "int16")).toBe(true);
      expect(isValidNumericFieldValue("32768", "int16")).toBe(false);
    });
    it("int32: valid range -2147483648 to 2147483647", () => {
      expect(isValidNumericFieldValue("-2147483648", "int32")).toBe(true);
      expect(isValidNumericFieldValue("2147483647", "int32")).toBe(true);
      expect(isValidNumericFieldValue("2147483648", "int32")).toBe(false);
    });
    it("int64: valid range -9223372036854775808 to 9223372036854775807", () => {
      expect(isValidNumericFieldValue("-9223372036854775808", "int64")).toBe(
        true,
      );
      expect(isValidNumericFieldValue("9223372036854775807", "int64")).toBe(
        true,
      );
      expect(isValidNumericFieldValue("9223372036854775808", "int64")).toBe(
        false,
      );
    });
  });

  describe("float types", () => {
    it("float32: valid finite values within float32 range", () => {
      expect(isValidNumericFieldValue("0", "float32")).toBe(true);
      expect(isValidNumericFieldValue("1.5", "float32")).toBe(true);
      expect(isValidNumericFieldValue("-3.4e38", "float32")).toBe(true);
      expect(isValidNumericFieldValue("3.5e38", "float32")).toBe(false);
      expect(isValidNumericFieldValue("abc", "float32")).toBe(false);
      expect(isValidNumericFieldValue("Infinity", "float32")).toBe(false);
    });
    it("float64: valid finite values", () => {
      expect(isValidNumericFieldValue("1.7e308", "float64")).toBe(true);
      expect(isValidNumericFieldValue("abc", "float64")).toBe(false);
      expect(isValidNumericFieldValue("Infinity", "float64")).toBe(false);
    });
    it("float32: rejects -Infinity", () => {
      expect(isValidNumericFieldValue("-Infinity", "float32")).toBe(false);
    });
    it("float64: rejects -Infinity", () => {
      expect(isValidNumericFieldValue("-Infinity", "float64")).toBe(false);
    });
  });

  describe("int type decimal rejection", () => {
    it("int8: rejects decimal value", () => {
      expect(isValidNumericFieldValue("1.5", "int8")).toBe(false);
    });
    it("int16: rejects decimal value", () => {
      expect(isValidNumericFieldValue("1.5", "int16")).toBe(false);
    });
    it("int32: rejects decimal value", () => {
      expect(isValidNumericFieldValue("1.5", "int32")).toBe(false);
    });
    it("int64: rejects decimal value", () => {
      expect(isValidNumericFieldValue("1.5", "int64")).toBe(false);
    });
  });

  describe("uint type + prefix rejection", () => {
    it("uint8: rejects value with + prefix", () => {
      expect(isValidNumericFieldValue("+5", "uint8")).toBe(false);
    });
    it("uint16: rejects value with + prefix", () => {
      expect(isValidNumericFieldValue("+5", "uint16")).toBe(false);
    });
    it("uint32: rejects value with + prefix", () => {
      expect(isValidNumericFieldValue("+5", "uint32")).toBe(false);
    });
  });

  describe("default branch (string/bytes type)", () => {
    it("returns true for string type (default branch)", () => {
      expect(isValidNumericFieldValue("anything", "string")).toBe(true);
    });
    it("returns true for bytes type (default branch)", () => {
      expect(isValidNumericFieldValue("anything", "bytes")).toBe(true);
    });
  });
});
