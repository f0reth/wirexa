import { describe, expect, it } from "vitest";
import { formatJson, formatTime } from "./format";

describe("formatJson", () => {
  it("formats an object with 2-space indentation", () => {
    expect(formatJson('{"a":1,"b":{"c":2}}')).toBe(
      '{\n  "a": 1,\n  "b": {\n    "c": 2\n  }\n}',
    );
  });

  it("formats an array", () => {
    expect(formatJson("[1,2]")).toBe("[\n  1,\n  2\n]");
  });

  it("returns null for malformed JSON", () => {
    expect(formatJson("{not json")).toBeNull();
  });

  it("returns null for an empty string", () => {
    expect(formatJson("")).toBeNull();
  });

  it("passes through a bare primitive, which is still valid JSON", () => {
    expect(formatJson("123")).toBe("123");
  });
});

describe("formatTime", () => {
  // ローカル時刻のコンポーネント指定で組むことで、実装が getHours() 系を
  // 使う限りタイムゾーンに依存しない。
  const local = (h: number, m: number, s: number, ms: number) =>
    new Date(2026, 0, 15, h, m, s, ms);

  it("formats a Date", () => {
    expect(formatTime(local(14, 23, 5, 123))).toBe("14:23:05.123");
  });

  it("formats an epoch number", () => {
    expect(formatTime(local(14, 23, 5, 123).getTime())).toBe("14:23:05.123");
  });

  it("zero-pads single-digit hours, minutes and seconds", () => {
    expect(formatTime(local(1, 2, 3, 456))).toBe("01:02:03.456");
  });

  it("zero-pads milliseconds to 3 digits", () => {
    expect(formatTime(local(0, 0, 0, 7))).toBe("00:00:00.007");
  });
});
