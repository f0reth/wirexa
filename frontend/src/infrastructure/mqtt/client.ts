import {
  Connect,
  DeleteProfile,
  Disconnect,
  GetConnections,
  GetProfiles,
  Publish,
  SaveProfile,
  Subscribe,
  Unsubscribe,
} from "../../../wailsjs/go/adapters/MqttHandler";
import type { BrokerProfile } from "../../domain/mqtt/types";

export interface ConnectionStatus {
  id: string;
  name: string;
  broker: string;
  connected: boolean;
}

export async function connect(profile: BrokerProfile): Promise<string> {
  return Connect({
    name: profile.name,
    broker: profile.broker,
    clientId: profile.clientId,
    username: profile.username,
    password: profile.password,
    useTls: profile.useTls,
  }) as Promise<string>;
}

export function disconnect(connectionId: string): Promise<void> {
  return Disconnect(connectionId) as Promise<void>;
}

export function subscribe(
  connectionId: string,
  topic: string,
  qos: number,
): Promise<void> {
  return Subscribe(connectionId, topic, qos) as Promise<void>;
}

export function unsubscribe(
  connectionId: string,
  topic: string,
): Promise<void> {
  return Unsubscribe(connectionId, topic) as Promise<void>;
}

export function publish(
  connectionId: string,
  topic: string,
  payload: string,
  qos: number,
  retain: boolean,
): Promise<void> {
  return Publish(connectionId, topic, payload, qos, retain) as Promise<void>;
}

export function getConnections(): Promise<ConnectionStatus[]> {
  return GetConnections() as Promise<ConnectionStatus[]>;
}

export function getProfiles(): Promise<BrokerProfile[]> {
  return GetProfiles() as Promise<BrokerProfile[]>;
}

export function saveProfile(profile: BrokerProfile): Promise<void> {
  return SaveProfile({
    id: profile.id,
    name: profile.name,
    broker: profile.broker,
    clientId: profile.clientId,
    username: profile.username,
    password: profile.password,
    useTls: profile.useTls,
  });
}

export function deleteProfile(id: string): Promise<void> {
  return DeleteProfile(id);
}
