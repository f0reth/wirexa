import { createSignal } from "solid-js";
import type {
  FixedLengthField,
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
  const [encoding, setEncoding] = createSignal<PayloadEncoding>("fixed");
  const [fixedLengthFields, setFixedLengthFields] = createSignal<
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
    setFixedLengthFields([...fixedLengthFields(), newField]);
  };

  const updateField = (id: string, updates: Partial<FixedLengthField>) => {
    setFixedLengthFields(
      fixedLengthFields().map((f) => (f.id === id ? { ...f, ...updates } : f)),
    );
  };

  const removeField = (id: string) => {
    setFixedLengthFields(fixedLengthFields().filter((f) => f.id !== id));
  };

  async function send(): Promise<void> {
    setLoading(true);
    try {
      const request: UdpSendRequest = {
        host: host(),
        port: port(),
        encoding: encoding(),
        fixedLengthPayload: {
          fields: fixedLengthFields(),
        },
      };

      await api.send(request);
    } finally {
      setLoading(false);
    }
  }

  function loadTarget(target: UdpTarget): void {
    setHost(target.host);
    setPort(target.port);
    setEncoding(target.encoding);
  }

  return {
    host,
    setHost,
    port,
    setPort,
    encoding,
    setEncoding,
    fixedLengthFields,
    addField,
    updateField,
    removeField,
    loading,
    send,
    loadTarget,
  };
}
