import {
  DeleteTarget,
  GetListeners,
  GetTargets,
  SaveTarget,
  Send,
  StartListen,
  StopListen,
} from "../../../wailsjs/go/adapters/UDPHandler";
import { udpdomain } from "../../../wailsjs/go/models";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import type {
  UdpListenSession,
  UdpReceivedMessage,
  UdpSendRequest,
  UdpSendResult,
  UdpTarget,
} from "../../domain/udp/types";
import { WailsEvents } from "../../shared/wails-events";

function toWailsRequest(req: UdpSendRequest): udpdomain.UDPSendRequest {
  return udpdomain.UDPSendRequest.createFrom(req);
}

function fromWailsSendResult(res: udpdomain.UDPSendResult): UdpSendResult {
  return { ...res };
}

function fromWailsTarget(t: udpdomain.UDPTarget): UdpTarget {
  // スプレッドで素通しし、Go 側がプリミティブ項目を足しても黙って落ちないようにする。
  return { ...t };
}

function toWailsTarget(t: UdpTarget): udpdomain.UDPTarget {
  return udpdomain.UDPTarget.createFrom(t);
}

export async function send(req: UdpSendRequest): Promise<UdpSendResult> {
  const result = await Send(toWailsRequest(req));
  return fromWailsSendResult(result);
}

export async function getTargets(): Promise<UdpTarget[]> {
  const result = await GetTargets();
  return result.map(fromWailsTarget);
}

export async function saveTarget(target: UdpTarget): Promise<UdpTarget> {
  const result = await SaveTarget(toWailsTarget(target));
  return fromWailsTarget(result);
}

export async function deleteTarget(id: string): Promise<void> {
  return DeleteTarget(id);
}

function fromWailsListenSession(
  s: udpdomain.UDPListenSession,
): UdpListenSession {
  return {
    ...s,
    encoding: s.encoding as UdpListenSession["encoding"],
  };
}

export async function startListen(
  port: number,
  encoding: string,
): Promise<UdpListenSession> {
  const result = await StartListen(port, encoding);
  return fromWailsListenSession(result);
}

export async function stopListen(sessionId: string): Promise<void> {
  return StopListen(sessionId);
}

export async function getListeners(): Promise<UdpListenSession[]> {
  const result = await GetListeners();
  return result.map(fromWailsListenSession);
}

export function onMessage(cb: (msg: UdpReceivedMessage) => void): () => void {
  return EventsOn(WailsEvents.udpMessage, cb);
}
