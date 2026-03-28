import { createSignal } from "solid-js";
import type {
  PayloadEncoding,
  UdpSendRequest,
  UdpSendResult,
  UdpTarget,
} from "../../domain/udp/types";

export interface UdpSendApi {
  send(req: UdpSendRequest): Promise<UdpSendResult>;
}

export function createUdpSendState(api: UdpSendApi) {
  const [host, setHost] = createSignal("");
  const [port, setPort] = createSignal(0);
  const [payload, setPayload] = createSignal("");
  const [encoding, setEncoding] = createSignal<PayloadEncoding>("text");
  const [result, setResult] = createSignal<UdpSendResult | null>(null);
  const [loading, setLoading] = createSignal(false);

  async function send(): Promise<void> {
    setLoading(true);
    try {
      const res = await api.send({
        host: host(),
        port: port(),
        payload: payload(),
        encoding: encoding(),
      });
      setResult(res);
    } finally {
      setLoading(false);
    }
  }

  function loadTarget(target: UdpTarget): void {
    setHost(target.host);
    setPort(target.port);
    setEncoding(target.encoding);
    setResult(null);
  }

  return {
    host,
    setHost,
    port,
    setPort,
    payload,
    setPayload,
    encoding,
    setEncoding,
    result,
    loading,
    send,
    loadTarget,
  };
}
