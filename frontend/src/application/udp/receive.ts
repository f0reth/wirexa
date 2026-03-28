import { createSignal } from "solid-js";
import { createStore } from "solid-js/store";
import { UDP_MAX_MESSAGES } from "../../config/limits";
import type {
  PayloadEncoding,
  UdpListenSession,
  UdpReceivedMessage,
} from "../../domain/udp/types";

export interface UdpReceiveApi {
  startListen(port: number, encoding: string): Promise<UdpListenSession>;
  stopListen(sessionId: string): Promise<void>;
  getListeners(): Promise<UdpListenSession[]>;
  onMessage(cb: (msg: UdpReceivedMessage) => void): () => void;
}

export function createUdpReceiveState(api: UdpReceiveApi) {
  const [sessions, setSessions] = createStore<UdpListenSession[]>([]);
  const [messages, setMessages] = createStore<UdpReceivedMessage[]>([]);
  const [listenPort, setListenPort] = createSignal(0);
  const [listenEncoding, setListenEncoding] =
    createSignal<PayloadEncoding>("text");
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);

  api.onMessage((msg) => {
    setMessages((prev) => [msg, ...prev].slice(0, UDP_MAX_MESSAGES));
  });

  async function startListen(): Promise<void> {
    setLoading(true);
    setError(null);
    try {
      const session = await api.startListen(listenPort(), listenEncoding());
      setSessions((prev) => [...prev, session]);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  async function stopListen(sessionId: string): Promise<void> {
    await api.stopListen(sessionId);
    setSessions((prev) => prev.filter((s) => s.id !== sessionId));
  }

  function clearMessages(): void {
    setMessages([]);
  }

  return {
    sessions,
    messages,
    listenPort,
    setListenPort,
    listenEncoding,
    setListenEncoding,
    loading,
    error,
    startListen,
    stopListen,
    clearMessages,
  };
}
