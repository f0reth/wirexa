// フロントエンド domain 層の型定義（正）。
// Wails 自動生成型（wailsjs/go/models.ts）との変換は infrastructure/udp/client.ts で行う。
// これらの型を変更した場合は internal/domain/udp/types.go も必ず合わせて更新すること。

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
