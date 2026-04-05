import type { mqttdomain } from "../models";
import { store } from "../../store";

export function Connect(config: mqttdomain.ConnectionConfig): Promise<string> {
  const id = crypto.randomUUID();
  store.mqttConnections.push({
    id,
    name: config.name,
    broker: config.broker,
    connected: true,
  });
  return Promise.resolve(id);
}

export function DeleteProfile(id: string): Promise<void> {
  store.mqttProfiles = store.mqttProfiles.filter((p) => p.id !== id);
  return Promise.resolve();
}

export function Disconnect(connectionId: string): Promise<void> {
  const conn = store.mqttConnections.find((c) => c.id === connectionId);
  if (conn) {
    conn.connected = false;
  }
  return Promise.resolve();
}

export function GetConnections(): Promise<Array<mqttdomain.ConnectionStatus>> {
  return Promise.resolve([...store.mqttConnections]);
}

export function GetProfiles(): Promise<Array<mqttdomain.BrokerProfile>> {
  return Promise.resolve([...store.mqttProfiles]);
}

export function Publish(
  _connectionId: string,
  _topic: string,
  _payload: string,
  _qos: number,
  _retained: boolean,
): Promise<void> {
  return Promise.resolve();
}

export function SaveProfile(profile: mqttdomain.BrokerProfile): Promise<void> {
  const idx = store.mqttProfiles.findIndex((p) => p.id === profile.id);
  if (idx >= 0) {
    store.mqttProfiles[idx] = profile;
  } else {
    store.mqttProfiles.push(profile);
  }
  return Promise.resolve();
}

export function Shutdown(): Promise<void> {
  store.mqttConnections = [];
  return Promise.resolve();
}

export function Subscribe(
  _connectionId: string,
  _topic: string,
  _qos: number,
): Promise<void> {
  return Promise.resolve();
}

export function Unsubscribe(
  _connectionId: string,
  _topic: string,
): Promise<void> {
  return Promise.resolve();
}
