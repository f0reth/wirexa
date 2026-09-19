// 固定長フィールド入力欄の表示・検証に使う純粋関数群。
// UI から独立させ、SendForm は結果を描画するだけにする。

import type { FieldType, FixedLengthField } from "../../domain/udp/types";
import {
  FIELD_TYPE_SIZES,
  isValidNumericFieldValue,
} from "../../domain/udp/types";

/** 検証に必要な最小限のフィールド情報。id 付きの状態型もそのまま渡せる。 */
export type FieldValueInput = Pick<FixedLengthField, "fieldType" | "value">;

/** 長さ比較まで行う場合に必要な情報。 */
export type FieldInput = FieldValueInput & Pick<FixedLengthField, "length">;

/** バイト数バッジの状態。CSS クラスへの対応付けは presentation 層が行う。 */
export type ByteCountStatus = "ok" | "warn" | "error";

export function isValidAscii(value: string): boolean {
  return [...value].every((c) => (c.codePointAt(0) ?? 0) <= 0x7f);
}

export function isValidHex(value: string): boolean {
  const cleaned = value.replace(/\s/g, "");
  return cleaned.length % 2 === 0 && /^[0-9a-fA-F]*$/.test(cleaned);
}

/** 空白を除いた hex 文字列のバイト数。奇数長は isValidHex が false になる前提。 */
export function hexByteCount(value: string): number {
  return value.replace(/\s/g, "").length / 2;
}

/** length 指定が意味を持つ可変長型かどうか。 */
export function isVarLengthFieldType(fieldType: FieldType): boolean {
  return fieldType === "string" || fieldType === "bytes";
}

export function isValidFieldValue(field: FieldValueInput): boolean {
  if (field.fieldType === "string") return isValidAscii(field.value);
  if (field.fieldType === "bytes") return isValidHex(field.value);
  return isValidNumericFieldValue(field.value, field.fieldType);
}

export function fieldByteCount(field: FieldValueInput): number {
  if (field.fieldType === "bytes") return hexByteCount(field.value);
  return field.value.length;
}

export function fieldByteCountStatus(field: FieldInput): ByteCountStatus {
  if (!isValidFieldValue(field)) return "error";
  if (
    isVarLengthFieldType(field.fieldType) &&
    fieldByteCount(field) > field.length
  )
    return "warn";
  return "ok";
}

export function fieldByteCountLabel(field: FieldInput): string {
  if (field.fieldType === "string") {
    if (!isValidAscii(field.value)) return "non-ASCII";
    return `${fieldByteCount(field)}/${field.length}`;
  }
  if (field.fieldType === "bytes") {
    if (!isValidHex(field.value)) return "invalid hex";
    return `${fieldByteCount(field)}/${field.length}`;
  }
  if (field.value !== "" && !isValidFieldValue(field)) return "out of range";
  return `${FIELD_TYPE_SIZES[field.fieldType]} bytes`;
}

export function fieldValueLabel(fieldType: FieldType): string {
  if (fieldType === "bytes") return "Value (hex)";
  if (fieldType === "string") return "Value (ASCII)";
  return "Value";
}

export function fieldValuePlaceholder(fieldType: FieldType): string {
  if (fieldType === "bytes") return "0a 1b 2c";
  if (fieldType === "string") return "hello";
  if (fieldType === "float32" || fieldType === "float64") return "1.0";
  return "0";
}

/** 64bit 整数は number 入力だと桁が落ちるため text 扱いにする。 */
export function fieldValueInputType(fieldType: FieldType): "text" | "number" {
  if (
    fieldType === "string" ||
    fieldType === "bytes" ||
    fieldType === "int64" ||
    fieldType === "uint64"
  )
    return "text";
  return "number";
}

/** 固定長ペイロード全体のバイト数。可変長型は宣言された length を使う。 */
export function totalFieldBytes(
  fields: readonly Pick<FixedLengthField, "fieldType" | "length">[],
): number {
  return fields.reduce((sum, field) => {
    const fixedSize = FIELD_TYPE_SIZES[field.fieldType];
    return sum + (fixedSize !== undefined ? fixedSize : field.length);
  }, 0);
}
