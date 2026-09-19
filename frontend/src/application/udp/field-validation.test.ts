import { describe, expect, it } from "vitest";
import {
  fieldByteCount,
  fieldByteCountInfo,
  fieldByteCountStatus,
  fieldValueKind,
  hexByteCount,
  isValidAscii,
  isValidFieldValue,
  isValidHex,
  isVarLengthFieldType,
  totalFieldBytes,
} from "./field-validation";

describe("isValidAscii", () => {
  it("returns true for an empty string", () => {
    expect(isValidAscii("")).toBe(true);
  });

  it("returns true for printable ASCII and control characters", () => {
    expect(isValidAscii("hello world")).toBe(true);
    expect(isValidAscii("\t\n")).toBe(true);
    expect(isValidAscii("\u007f")).toBe(true);
  });

  it("returns false for non-ASCII characters", () => {
    expect(isValidAscii("あ")).toBe(false);
    expect(isValidAscii("a\u0080")).toBe(false);
  });

  it("returns false for surrogate pairs such as emoji", () => {
    expect(isValidAscii("🙂")).toBe(false);
  });
});

describe("isValidHex", () => {
  it("returns true for an empty string", () => {
    expect(isValidHex("")).toBe(true);
  });

  it("accepts both cases and ignores whitespace", () => {
    expect(isValidHex("0aFF")).toBe(true);
    expect(isValidHex("0a 1b 2c")).toBe(true);
    expect(isValidHex(" 0a\t1b\n")).toBe(true);
  });

  it("returns false for odd-length input", () => {
    expect(isValidHex("abc")).toBe(false);
    expect(isValidHex("0a 1")).toBe(false);
  });

  it("returns false for non-hex characters", () => {
    expect(isValidHex("zz")).toBe(false);
    expect(isValidHex("0x0a")).toBe(false);
  });
});

describe("hexByteCount", () => {
  it("counts bytes ignoring whitespace", () => {
    expect(hexByteCount("")).toBe(0);
    expect(hexByteCount("0a1b")).toBe(2);
    expect(hexByteCount(" 0a 1b 2c ")).toBe(3);
  });

  it("returns a fractional count for odd-length input (isValidHex で弾く前提)", () => {
    expect(hexByteCount("0a1")).toBe(1.5);
  });
});

describe("isVarLengthFieldType", () => {
  it("returns true only for string and bytes", () => {
    expect(isVarLengthFieldType("string")).toBe(true);
    expect(isVarLengthFieldType("bytes")).toBe(true);
    expect(isVarLengthFieldType("uint8")).toBe(false);
    expect(isVarLengthFieldType("float64")).toBe(false);
  });
});

describe("isValidFieldValue", () => {
  it("validates string fields as ASCII", () => {
    expect(isValidFieldValue({ fieldType: "string", value: "hello" })).toBe(
      true,
    );
    expect(isValidFieldValue({ fieldType: "string", value: "あ" })).toBe(false);
  });

  it("validates bytes fields as hex", () => {
    expect(isValidFieldValue({ fieldType: "bytes", value: "0a1b" })).toBe(true);
    expect(isValidFieldValue({ fieldType: "bytes", value: "0a1" })).toBe(false);
  });

  it("validates numeric fields against their range", () => {
    expect(isValidFieldValue({ fieldType: "uint8", value: "255" })).toBe(true);
    expect(isValidFieldValue({ fieldType: "uint8", value: "256" })).toBe(false);
    expect(isValidFieldValue({ fieldType: "int8", value: "-128" })).toBe(true);
    expect(isValidFieldValue({ fieldType: "int8", value: "-129" })).toBe(false);
  });

  it("treats an empty numeric value as not yet entered", () => {
    expect(isValidFieldValue({ fieldType: "uint8", value: "" })).toBe(true);
  });
});

describe("fieldByteCount", () => {
  it("counts hex bytes for bytes fields and characters otherwise", () => {
    expect(fieldByteCount({ fieldType: "bytes", value: "0a 1b" })).toBe(2);
    expect(fieldByteCount({ fieldType: "string", value: "hello" })).toBe(5);
    expect(fieldByteCount({ fieldType: "uint8", value: "255" })).toBe(3);
  });
});

describe("fieldByteCountStatus", () => {
  it("reports error for an invalid value", () => {
    expect(
      fieldByteCountStatus({ fieldType: "string", value: "あ", length: 4 }),
    ).toBe("error");
    expect(
      fieldByteCountStatus({ fieldType: "bytes", value: "0a1", length: 4 }),
    ).toBe("error");
    expect(
      fieldByteCountStatus({ fieldType: "uint8", value: "256", length: 1 }),
    ).toBe("error");
  });

  it("reports warn when a variable-length value exceeds its declared length", () => {
    expect(
      fieldByteCountStatus({ fieldType: "string", value: "hello", length: 4 }),
    ).toBe("warn");
    expect(
      fieldByteCountStatus({ fieldType: "bytes", value: "0a1b2c", length: 2 }),
    ).toBe("warn");
  });

  it("reports ok when the value fits", () => {
    expect(
      fieldByteCountStatus({ fieldType: "string", value: "abcd", length: 4 }),
    ).toBe("ok");
    expect(
      fieldByteCountStatus({ fieldType: "uint16", value: "65535", length: 2 }),
    ).toBe("ok");
  });

  it("ignores length for numeric fields", () => {
    expect(
      fieldByteCountStatus({
        fieldType: "uint32",
        value: "4294967295",
        length: 1,
      }),
    ).toBe("ok");
  });
});

describe("fieldByteCountInfo", () => {
  it("reports the used/declared length for variable-length fields", () => {
    expect(
      fieldByteCountInfo({ fieldType: "string", value: "abc", length: 4 }),
    ).toEqual({ kind: "var-length", bytes: 3, length: 4 });
    expect(
      fieldByteCountInfo({ fieldType: "bytes", value: "0a 1b", length: 4 }),
    ).toEqual({ kind: "var-length", bytes: 2, length: 4 });
  });

  it("reports the reason a variable-length value is invalid", () => {
    expect(
      fieldByteCountInfo({ fieldType: "string", value: "あ", length: 4 }),
    ).toEqual({ kind: "non-ascii" });
    expect(
      fieldByteCountInfo({ fieldType: "bytes", value: "0a1", length: 4 }),
    ).toEqual({ kind: "invalid-hex" });
  });

  it("reports the fixed size for numeric fields", () => {
    expect(
      fieldByteCountInfo({ fieldType: "uint16", value: "1", length: 2 }),
    ).toEqual({ kind: "fixed-size", bytes: 2 });
    expect(
      fieldByteCountInfo({ fieldType: "float64", value: "", length: 8 }),
    ).toEqual({ kind: "fixed-size", bytes: 8 });
  });

  it("reports out of range for numeric fields outside their type", () => {
    expect(
      fieldByteCountInfo({ fieldType: "uint8", value: "256", length: 1 }),
    ).toEqual({ kind: "out-of-range" });
  });
});

describe("fieldValueKind", () => {
  it("separates ASCII, hex, integer and float inputs", () => {
    expect(fieldValueKind("string")).toBe("ascii");
    expect(fieldValueKind("bytes")).toBe("hex");
    expect(fieldValueKind("uint8")).toBe("integer");
    expect(fieldValueKind("int32")).toBe("integer");
    expect(fieldValueKind("float32")).toBe("float");
    expect(fieldValueKind("float64")).toBe("float");
  });

  it("separates 64bit integers, where a number input would lose precision", () => {
    expect(fieldValueKind("int64")).toBe("wide-integer");
    expect(fieldValueKind("uint64")).toBe("wide-integer");
  });
});

describe("totalFieldBytes", () => {
  it("returns 0 for no fields", () => {
    expect(totalFieldBytes([])).toBe(0);
  });

  it("uses the fixed size for numeric fields and length for variable-length ones", () => {
    expect(
      totalFieldBytes([
        { fieldType: "uint8", length: 99 },
        { fieldType: "uint32", length: 0 },
        { fieldType: "string", length: 10 },
        { fieldType: "bytes", length: 3 },
      ]),
    ).toBe(1 + 4 + 10 + 3);
  });
});
