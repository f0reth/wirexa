import type { httpdomain, mqttdomain, udpdomain } from "./go/models";

export interface MockStore {
  collections: httpdomain.Collection[];
  httpResponse: httpdomain.HttpResponse | null;
  mqttConnections: mqttdomain.ConnectionStatus[];
  mqttProfiles: mqttdomain.BrokerProfile[];
  udpTargets: udpdomain.UdpTarget[];
  udpListeners: udpdomain.UdpListenSession[];
  udpSendResult: udpdomain.UdpSendResult | null;
  eventHandlers: Map<string, Array<(...data: unknown[]) => void>>;
}

export const store: MockStore = {
  collections: [],
  httpResponse: null,
  mqttConnections: [],
  mqttProfiles: [],
  udpTargets: [],
  udpListeners: [],
  udpSendResult: null,
  eventHandlers: new Map(),
};

interface WailsMockInterface {
  store: MockStore;
  emit(eventName: string, ...data: unknown[]): void;
}

if (typeof window !== "undefined") {
  (window as unknown as { __wailsMock: WailsMockInterface }).__wailsMock = {
    store,
    emit(eventName: string, ...data: unknown[]): void {
      const handlers = store.eventHandlers.get(eventName) ?? [];
      for (const h of handlers) {
        h(...data);
      }
    },
  };
}
