// 固定長フィールド入力欄の表示都合（ラベル・placeholder・HTML input type・バッジ文言）。
// 検証やバイト数計算は application/udp/field-validation.ts が持ち、ここはその結果を
// 画面の文言へ対応付けるだけにする。

import type {
  ByteCountInfo,
  FieldValueKind,
} from "../../../application/udp/field-validation";

export const FIELD_VALUE_LABELS: Record<FieldValueKind, string> = {
  ascii: "Value (ASCII)",
  hex: "Value (hex)",
  integer: "Value",
  "wide-integer": "Value",
  float: "Value",
};

export const FIELD_VALUE_PLACEHOLDERS: Record<FieldValueKind, string> = {
  ascii: "hello",
  hex: "0a 1b 2c",
  integer: "0",
  "wide-integer": "0",
  float: "1.0",
};

/** wide-integer（int64 / uint64）は number 入力だと桁が落ちるため text にする。 */
export const FIELD_VALUE_INPUT_TYPES: Record<
  FieldValueKind,
  "text" | "number"
> = {
  ascii: "text",
  hex: "text",
  integer: "number",
  "wide-integer": "text",
  float: "number",
};

export function byteCountLabel(info: ByteCountInfo): string {
  switch (info.kind) {
    case "non-ascii":
      return "non-ASCII";
    case "invalid-hex":
      return "invalid hex";
    case "out-of-range":
      return "out of range";
    case "var-length":
      return `${info.bytes}/${info.length}`;
    case "fixed-size":
      return `${info.bytes} bytes`;
  }
}
