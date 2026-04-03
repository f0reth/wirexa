import { createSignal } from "solid-js";
import { createStore, produce } from "solid-js/store";
import type {
  FixedLengthField,
  PayloadEncoding,
  UdpSendRequest,
  UdpSendResult,
  UdpTarget,
} from "../../domain/udp/types";
import { log } from "../../infrastructure/logger/client";

export interface UdpSendApi {
  send(req: UdpSendRequest): Promise<UdpSendResult>;
}

export function createUdpSendState(api: UdpSendApi) {
  const [selectedTarget, setSelectedTarget] = createSignal<UdpTarget | null>(
    null,
  );
  const [host, setHost] = createSignal("");
  const [port, setPort] = createSignal(0);
  const [textPayload, setTextPayload] = createSignal("");
  const [jsonPayload, setJsonPayload] = createSignal("");
  const [encoding, setEncoding] = createSignal<PayloadEncoding>("text");

  const payload = () => (encoding() === "json" ? jsonPayload() : textPayload());
  const setPayload = (v: string) => {
    if (encoding() === "json") setJsonPayload(v);
    else setTextPayload(v);
  };
  const [messageLength, setMessageLength] = createSignal(0);
  const [fixedLengthFields, setFixedLengthFields] = createStore<
    FixedLengthField[]
  >([]);
  const [loading, setLoading] = createSignal(false);

  const addField = (field?: Partial<FixedLengthField>) => {
    const newField: FixedLengthField = {
      id: crypto.randomUUID(),
      name: field?.name ?? "",
      length: field?.length ?? 1,
      value: field?.value ?? "",
    };
    setFixedLengthFields(fixedLengthFields.length, newField);
  };

  const updateField = (id: string, updates: Partial<FixedLengthField>) => {
    const index = fixedLengthFields.findIndex((f) => f.id === id);
    if (index !== -1) {
      setFixedLengthFields(index, updates);
    }
  };

  const removeField = (id: string) => {
    setFixedLengthFields(
      produce((fields) => {
        const index = fields.findIndex((f) => f.id === id);
        if (index !== -1) fields.splice(index, 1);
      }),
    );
  };

  async function send(): Promise<void> {
    setLoading(true);
    const h = host();
    const p = port();
    log({
      level: "INFO",
      source: "frontend:udp",
      message: "UDP packet sending",
      attrs: { host: h, port: p, encoding: encoding() },
    });
    try {
      const request: UdpSendRequest = {
        host: h,
        port: p,
        payload: payload(),
        encoding: encoding(),
        messageLength: messageLength(),
        fixedLengthPayload:
          encoding() === "fixed"
            ? {
                fields: fixedLengthFields.map((f) => ({
                  id: f.id,
                  name: f.name,
                  length: f.length,
                  value: f.value,
                })),
              }
            : { fields: [] },
      };

      const result = await api.send(request);
      log({
        level: "INFO",
        source: "frontend:udp",
        message: "UDP packet sent",
        attrs: { host: h, port: p, bytes: result.bytesSent },
      });
    } catch (err) {
      log({
        level: "ERROR",
        source: "frontend:udp",
        message: "UDP send failed",
        attrs: { host: h, port: p, error: String(err) },
      });
      throw err;
    } finally {
      setLoading(false);
    }
  }

  function loadTarget(target: UdpTarget): void {
    setSelectedTarget(target);
    setHost(target.host);
    setPort(target.port);
  }

  return {
    selectedTarget,
    host,
    setHost,
    port,
    setPort,
    payload,
    setPayload,
    encoding,
    setEncoding,
    messageLength,
    setMessageLength,
    fixedLengthFields,
    addField,
    updateField,
    removeField,
    loading,
    send,
    loadTarget,
  };
}
