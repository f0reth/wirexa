import { type Accessor, createContext, type JSX, useContext } from "solid-js";
import { createUdpReceiveState } from "../../application/udp/receive";
import { createUdpSendState } from "../../application/udp/send";
import { createTargetsState } from "../../application/udp/targets";
import type {
  FixedLengthField,
  PayloadEncoding,
  UdpListenSession,
  UdpReceivedMessage,
  UdpTarget,
} from "../../domain/udp/types";
import * as udpClient from "../../infrastructure/udp/client";

export interface UdpSendContextValue {
  host: Accessor<string>;
  setHost: (v: string) => void;
  port: Accessor<number>;
  setPort: (v: number) => void;
  payload: Accessor<string>;
  setPayload: (v: string) => void;
  encoding: Accessor<PayloadEncoding>;
  setEncoding: (v: PayloadEncoding) => void;
  messageLength: Accessor<number>;
  setMessageLength: (v: number) => void;
  fixedLengthFields: Accessor<FixedLengthField[]>;
  addField: (field?: Partial<FixedLengthField>) => void;
  updateField: (id: string, updates: Partial<FixedLengthField>) => void;
  removeField: (id: string) => void;
  loading: Accessor<boolean>;
  send: () => Promise<void>;
  loadTarget: (target: UdpTarget) => void;
}

export interface UdpTargetsContextValue {
  targets: UdpTarget[];
  refreshTargets: () => Promise<void>;
  saveTarget: (t: UdpTarget) => Promise<UdpTarget>;
  deleteTarget: (id: string) => Promise<void>;
}

export interface UdpReceiveContextValue {
  sessions: UdpListenSession[];
  messages: UdpReceivedMessage[];
  listenPort: Accessor<number>;
  setListenPort: (v: number) => void;
  listenEncoding: Accessor<PayloadEncoding>;
  setListenEncoding: (v: PayloadEncoding) => void;
  loading: Accessor<boolean>;
  error: Accessor<string | null>;
  startListen: () => Promise<void>;
  stopListen: (sessionId: string) => Promise<void>;
  clearMessages: () => void;
}

const UdpSendContext = createContext<UdpSendContextValue>();
const UdpTargetsContext = createContext<UdpTargetsContextValue>();
const UdpReceiveContext = createContext<UdpReceiveContextValue>();

export function UdpProvider(props: { children: JSX.Element }) {
  const sendState = createUdpSendState(udpClient);
  const targetsState = createTargetsState(udpClient);
  const receiveState = createUdpReceiveState(udpClient);

  return (
    <UdpSendContext.Provider value={sendState}>
      <UdpTargetsContext.Provider value={targetsState}>
        <UdpReceiveContext.Provider value={receiveState}>
          {props.children}
        </UdpReceiveContext.Provider>
      </UdpTargetsContext.Provider>
    </UdpSendContext.Provider>
  );
}

export function useUdpSend(): UdpSendContextValue {
  const ctx = useContext(UdpSendContext);
  if (!ctx) throw new Error("useUdpSend must be used within UdpProvider");
  return ctx;
}

export function useUdpTargets(): UdpTargetsContextValue {
  const ctx = useContext(UdpTargetsContext);
  if (!ctx) throw new Error("useUdpTargets must be used within UdpProvider");
  return ctx;
}

export function useUdpReceive(): UdpReceiveContextValue {
  const ctx = useContext(UdpReceiveContext);
  if (!ctx) throw new Error("useUdpReceive must be used within UdpProvider");
  return ctx;
}
