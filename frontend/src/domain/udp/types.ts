export type PayloadEncoding = "text" | "hex" | "base64";

export const PAYLOAD_ENCODINGS: PayloadEncoding[] = ["text", "hex", "base64"];

export interface UdpTarget {
  id: string;
  name: string;
  host: string;
  port: number;
  encoding: PayloadEncoding;
}

export interface UdpSendRequest {
  host: string;
  port: number;
  payload: string;
  encoding: PayloadEncoding;
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
