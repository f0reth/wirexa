import * as dgram from "node:dgram";
import type { AddressInfo } from "node:net";

export interface UdpServer {
  port: number;
  /** 最初に受信したペイロード。 */
  firstMessage: Promise<string>;
  close(): Promise<void>;
}

/**
 * UDP 受信サーバーを起動し、実際に割り当てられたポートを返す。
 * bind したまま addr を読むので、ポート番号を返してから bind し直す TOCTOU が無い。
 */
export async function startUdpServer(): Promise<UdpServer> {
  const socket = dgram.createSocket("udp4");
  const firstMessage = new Promise<string>((resolve) => {
    socket.once("message", (msg) => resolve(msg.toString()));
  });
  await new Promise<void>((resolve) => socket.bind(0, "127.0.0.1", resolve));

  return {
    port: (socket.address() as AddressInfo).port,
    firstMessage,
    close: () => new Promise<void>((resolve) => socket.close(() => resolve())),
  };
}

/**
 * アプリ側が bind するためのポート番号を確保する。
 * 一度 bind して番号を読み、閉じてから返す。閉じてからアプリが bind するまでの隙間は
 * 原理的に他プロセスが奪えるが、アプリに bind させる以上これは避けられない。
 */
export async function reserveUdpPort(): Promise<number> {
  const socket = dgram.createSocket("udp4");
  await new Promise<void>((resolve) => socket.bind(0, "127.0.0.1", resolve));
  const port = (socket.address() as AddressInfo).port;
  await new Promise<void>((resolve) => socket.close(() => resolve()));
  return port;
}

export async function sendUdpPacket(
  payload: string,
  port: number,
  host = "127.0.0.1",
): Promise<void> {
  const client = dgram.createSocket("udp4");
  await new Promise<void>((resolve, reject) => {
    client.send(payload, port, host, (err) => {
      client.close();
      if (err) reject(err);
      else resolve();
    });
  });
}
