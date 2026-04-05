import type { udpdomain } from "../models";
import { store } from "../../store";

export function DeleteTarget(id: string): Promise<void> {
  store.udpTargets = store.udpTargets.filter((t) => t.id !== id);
  return Promise.resolve();
}

export function GetListeners(): Promise<Array<udpdomain.UdpListenSession>> {
  return Promise.resolve([...store.udpListeners]);
}

export function GetTargets(): Promise<Array<udpdomain.UdpTarget>> {
  return Promise.resolve([...store.udpTargets]);
}

export function SaveTarget(
  target: udpdomain.UdpTarget,
): Promise<udpdomain.UdpTarget> {
  const saved: udpdomain.UdpTarget = {
    ...target,
    id: target.id || crypto.randomUUID(),
  };
  const idx = store.udpTargets.findIndex((t) => t.id === saved.id);
  if (idx >= 0) {
    store.udpTargets[idx] = saved;
  } else {
    store.udpTargets.push(saved);
  }
  return Promise.resolve(saved);
}

export function Send(
  _request: udpdomain.UdpSendRequest,
): Promise<udpdomain.UdpSendResult> {
  return Promise.resolve(store.udpSendResult ?? { bytesSent: 0 });
}

export function Shutdown(): Promise<void> {
  store.udpListeners = [];
  return Promise.resolve();
}

export function StartListen(
  port: number,
  encoding: string,
): Promise<udpdomain.UdpListenSession> {
  const session: udpdomain.UdpListenSession = {
    id: crypto.randomUUID(),
    port,
    encoding,
  };
  store.udpListeners.push(session);
  return Promise.resolve(session);
}

export function StopListen(id: string): Promise<void> {
  store.udpListeners = store.udpListeners.filter((s) => s.id !== id);
  return Promise.resolve();
}
