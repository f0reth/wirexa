export type PayloadEncoding = "text" | "json" | "fixed";

export const PAYLOAD_ENCODINGS: PayloadEncoding[] = ["text", "json", "fixed"];

export interface UdpTarget {
  id: string;
  name: string;
  host: string;
  port: number;
}

export interface FixedLengthField {
  name: string;
  length: number;
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
