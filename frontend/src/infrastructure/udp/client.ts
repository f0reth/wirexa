import {
  DeleteTarget,
  GetListeners,
  GetTargets,
  SaveTarget,
  Send,
  StartListen,
  StopListen,
} from "../../../wailsjs/go/adapters/UdpHandler";
import { udpdomain } from "../../../wailsjs/go/models";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import type {
  UdpListenSession,
  UdpReceivedMessage,
  UdpSendRequest,
  UdpSendResult,
  UdpTarget,
} from "../../domain/udp/types";

function toWailsRequest(req: UdpSendRequest): udpdomain.UdpSendRequest {
  return udpdomain.UdpSendRequest.createFrom(req);
}

function fromWailsSendResult(res: udpdomain.UdpSendResult): UdpSendResult {
  return { bytesSent: res.bytesSent };
}

function fromWailsTarget(t: udpdomain.UdpTarget): UdpTarget {
  return {
    id: t.id,
    name: t.name,
    host: t.host,
    port: t.port,
    encoding: t.encoding as UdpTarget["encoding"],
  };
}

function toWailsTarget(t: UdpTarget): udpdomain.UdpTarget {
  return udpdomain.UdpTarget.createFrom(t);
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
  s: udpdomain.UdpListenSession,
): UdpListenSession {
  return {
    id: s.id,
    port: s.port,
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
  return EventsOn("udp:message", cb);
}
