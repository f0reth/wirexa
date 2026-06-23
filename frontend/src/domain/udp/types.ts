// フロントエンド domain 層の型定義（ユニオン型・型ガードで UI に意味付けする層）。
// 配線型の正は Go ドメイン型 internal/domain/udp/types.go で、これが RPC 境界に直接公開され
// Wails 生成型 wailsjs/go/models.ts に反映される。Go 側を変更したら `wails generate module` を
// 実行する（再生成忘れは CI のバインディング鮮度チェックが検知する）。
// 生成型 → この型の変換は infrastructure/udp/client.ts でスプレッド素通し＋ユニオン検証で行う。

export type PayloadEncoding = "text" | "json" | "fixed";

export const PAYLOAD_ENCODINGS: PayloadEncoding[] = ["text", "json", "fixed"];

export type FieldType =
  | "string"
  | "bytes"
  | "uint8"
  | "uint16"
  | "uint32"
  | "uint64"
  | "int8"
  | "int16"
  | "int32"
  | "int64"
  | "float32"
  | "float64";

export const FIELD_TYPES: FieldType[] = [
  "string",
  "bytes",
  "uint8",
  "uint16",
  "uint32",
  "uint64",
  "int8",
  "int16",
  "int32",
  "int64",
  "float32",
  "float64",
];

export type Endianness = "big" | "little";

export const ENDIANNESSES: Endianness[] = ["big", "little"];

/** 数値型の固定バイトサイズ。可変長型（string, bytes）は undefined。 */
export const FIELD_TYPE_SIZES: Partial<Record<FieldType, number>> = {
  uint8: 1,
  int8: 1,
  uint16: 2,
  int16: 2,
  uint32: 4,
  int32: 4,
  float32: 4,
  uint64: 8,
  int64: 8,
  float64: 8,
};

/**
 * 数値型フィールドの値が型の有効範囲内かを検証する。
 * 空文字列は true を返す（未入力扱い）。
 * string / bytes 型には使用しないこと。
 */
export function isValidNumericFieldValue(
  value: string,
  fieldType: FieldType,
): boolean {
  if (value === "") return true;
  switch (fieldType) {
    case "uint8":
      return /^\d+$/.test(value) && Number(value) <= 255;
    case "uint16":
      return /^\d+$/.test(value) && Number(value) <= 65535;
    case "uint32":
      return /^\d+$/.test(value) && Number(value) <= 4294967295;
    case "uint64": {
      if (!/^\d+$/.test(value)) return false;
      try {
        return BigInt(value) <= 18446744073709551615n;
      } catch {
        return false;
      }
    }
    case "int8": {
      if (!/^-?\d+$/.test(value)) return false;
      const n = Number(value);
      return n >= -128 && n <= 127;
    }
    case "int16": {
      if (!/^-?\d+$/.test(value)) return false;
      const n = Number(value);
      return n >= -32768 && n <= 32767;
    }
    case "int32": {
      if (!/^-?\d+$/.test(value)) return false;
      const n = Number(value);
      return n >= -2147483648 && n <= 2147483647;
    }
    case "int64": {
      if (!/^-?\d+$/.test(value)) return false;
      try {
        const n = BigInt(value);
        return n >= -9223372036854775808n && n <= 9223372036854775807n;
      } catch {
        return false;
      }
    }
    case "float32": {
      const n = Number(value);
      if (Number.isNaN(n) || !Number.isFinite(n)) return false;
      return Math.abs(n) <= 3.4028234663852886e38;
    }
    case "float64": {
      const n = Number(value);
      return !Number.isNaN(n) && Number.isFinite(n);
    }
    default:
      return true;
  }
}

export interface UdpTarget {
  id: string;
  name: string;
  host: string;
  port: number;
}

export interface FixedLengthField {
  name: string;
  fieldType: FieldType;
  length: number; // FieldTypeString, FieldTypeBytes のみ使用
  value: string;
}

export interface FixedLengthPayload {
  fields: FixedLengthField[];
}

export interface UdpSendRequest {
  host: string;
  port: number;
  encoding: PayloadEncoding;
  payload: string;
  messageLength: number;
  fixedLengthPayload: FixedLengthPayload;
  endianness: Endianness;
}

export interface UdpSendResult {
  bytesSent: number;
}

export function isPayloadEncoding(v: string): v is PayloadEncoding {
  return (PAYLOAD_ENCODINGS as string[]).includes(v);
}

export interface UdpListenSession {
  id: string;
  port: number;
  encoding: PayloadEncoding;
}

export interface UdpReceivedMessage {
  sessionId: string;
  port: number;
  remoteAddr: string;
  payload: string;
  encoding: PayloadEncoding;
  timestamp: number;
}
