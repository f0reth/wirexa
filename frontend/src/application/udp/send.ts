import { createSignal } from "solid-js";
import { createStore, produce } from "solid-js/store";
import type {
  Endianness,
  FixedLengthField,
  PayloadEncoding,
  UdpSendRequest,
  UdpSendResult,
  UdpTarget,
} from "../../domain/udp/types";
import { FIELD_TYPE_SIZES } from "../../domain/udp/types";
import { log } from "../../infrastructure/logger/client";
import { withLoading } from "../../shared/async-op";

/** UI 管理用 id を付加したアプリケーション層のフィールド型。 */
export type FixedLengthFieldState = FixedLengthField & { id: string };

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
  const [endianness, setEndianness] = createSignal<Endianness>("big");
  const [fixedLengthFields, setFixedLengthFields] = createStore<
    FixedLengthFieldState[]
  >([]);
  const [loading, setLoading] = createSignal(false);

  const addField = (field?: Partial<FixedLengthFieldState>) => {
    const fieldType = field?.fieldType ?? "string";
    const fixedSize = FIELD_TYPE_SIZES[fieldType];
    const newField: FixedLengthFieldState = {
      id: crypto.randomUUID(),
      name: field?.name ?? "",
      fieldType,
      length: field?.length ?? (fixedSize !== undefined ? fixedSize : 1),
      value: field?.value ?? "",
    };
    setFixedLengthFields(fixedLengthFields.length, newField);
  };

  const updateField = (id: string, updates: Partial<FixedLengthFieldState>) => {
    const index = fixedLengthFields.findIndex((f) => f.id === id);
    if (index !== -1) {
      // 型が変わった場合は length を自動更新
      if (updates.fieldType !== undefined) {
        const fixedSize = FIELD_TYPE_SIZES[updates.fieldType];
        if (fixedSize !== undefined) {
          updates = { ...updates, length: fixedSize };
        }
      }
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
    const h = host();
    const p = port();
    log({
      level: "INFO",
      source: "frontend:udp",
      message: "UDP packet sending",
      attrs: { host: h, port: p, encoding: encoding() },
    });
    try {
      const result = await withLoading(setLoading, () => {
        const request: UdpSendRequest = {
          host: h,
          port: p,
          payload: payload(),
          encoding: encoding(),
          messageLength: messageLength(),
          endianness: endianness(),
          fixedLengthPayload:
            encoding() === "fixed"
              ? {
                  fields: fixedLengthFields.map((f) => ({
                    name: f.name,
                    fieldType: f.fieldType,
                    length: f.length,
                    value: f.value,
                  })),
                }
              : { fields: [] },
        };
        return api.send(request);
      });
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
    endianness,
    setEndianness,
    fixedLengthFields,
    addField,
    updateField,
    removeField,
    loading,
    send,
    loadTarget,
  };
}
