import { test as base, expect } from "@playwright/test";

interface MockStoreWindow {
  __wailsMock: {
    store: {
      collections: unknown[];
      httpResponse: unknown;
      mqttConnections: unknown[];
      mqttProfiles: unknown[];
      udpTargets: unknown[];
      udpListeners: unknown[];
      udpSendResult: unknown;
    };
    emit(eventName: string, data: unknown): void;
  };
}

export interface WailsMockApi {
  setCollections(collections: unknown[]): Promise<void>;
  setHttpResponse(response: unknown): Promise<void>;
  setMqttConnections(connections: unknown[]): Promise<void>;
  setMqttProfiles(profiles: unknown[]): Promise<void>;
  setUdpTargets(targets: unknown[]): Promise<void>;
  setUdpListeners(listeners: unknown[]): Promise<void>;
  emit(eventName: string, data: unknown): Promise<void>;
}

export const test = base.extend<{ wailsMock: WailsMockApi }>({
  wailsMock: async ({ page }, use) => {
    await page.goto("/");

    const api: WailsMockApi = {
      setCollections: (collections) =>
        page.evaluate((c) => {
          (window as unknown as MockStoreWindow).__wailsMock.store.collections =
            c;
        }, collections),

      setHttpResponse: (response) =>
        page.evaluate((r) => {
          (
            window as unknown as MockStoreWindow
          ).__wailsMock.store.httpResponse = r;
        }, response),

      setMqttConnections: (connections) =>
        page.evaluate((c) => {
          (
            window as unknown as MockStoreWindow
          ).__wailsMock.store.mqttConnections = c;
        }, connections),

      setMqttProfiles: (profiles) =>
        page.evaluate((p) => {
          (
            window as unknown as MockStoreWindow
          ).__wailsMock.store.mqttProfiles = p;
        }, profiles),

      setUdpTargets: (targets) =>
        page.evaluate((t) => {
          (window as unknown as MockStoreWindow).__wailsMock.store.udpTargets =
            t;
        }, targets),

      setUdpListeners: (listeners) =>
        page.evaluate((l) => {
          (
            window as unknown as MockStoreWindow
          ).__wailsMock.store.udpListeners = l;
        }, listeners),

      emit: (eventName, data) =>
        page.evaluate(
          ([name, d]) => {
            (window as unknown as MockStoreWindow).__wailsMock.emit(
              name as string,
              d,
            );
          },
          [eventName, data] as [string, unknown],
        ),
    };

    await use(api);
  },
});

export { expect };
