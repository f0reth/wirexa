import {
  Connect,
  DeleteProfile,
  Disconnect,
  GetConnections,
  GetProfiles,
  Publish,
  SaveProfile,
  StartTopicScan,
  StopTopicScan,
  Subscribe,
  Unsubscribe,
} from "../../../wailsjs/go/adapters/MQTTHandler";
import type { mqttdomain } from "../../../wailsjs/go/models";
import type { BrokerProfile, ConnectionStatus } from "../../domain/mqtt/types";

// 生成型 → domain 型の変換。キャストせずに戻り値の型注釈で検査させ、
// Go 側の型が変わってフロントの型と合わなくなったら tsc が検出する。
function fromWailsProfile(p: mqttdomain.BrokerProfile): BrokerProfile {
  return { ...p };
}

function fromWailsConnectionStatus(
  s: mqttdomain.ConnectionStatus,
): ConnectionStatus {
  return { ...s, subscriptions: s.subscriptions.map((x) => ({ ...x })) };
}

// profile の id は接続設定では profileId になるので、フィールドを列挙して渡す。
export function connect(profile: BrokerProfile): Promise<string> {
  return Connect({
    name: profile.name,
    broker: profile.broker,
    clientId: profile.clientId,
    username: profile.username,
    password: profile.password,
    useTls: profile.useTls,
    profileId: profile.id,
  });
}

export function disconnect(connectionId: string): Promise<void> {
  return Disconnect(connectionId);
}

export function subscribe(
  connectionId: string,
  topic: string,
  qos: number,
): Promise<void> {
  return Subscribe(connectionId, topic, qos);
}

export function unsubscribe(
  connectionId: string,
  topic: string,
): Promise<void> {
  return Unsubscribe(connectionId, topic);
}

export function startTopicScan(connectionId: string): Promise<void> {
  return StartTopicScan(connectionId);
}

export function stopTopicScan(connectionId: string): Promise<void> {
  return StopTopicScan(connectionId);
}

export function publish(
  connectionId: string,
  topic: string,
  payload: string,
  qos: number,
  retain: boolean,
): Promise<void> {
  return Publish(connectionId, topic, payload, qos, retain);
}

export async function getConnections(): Promise<ConnectionStatus[]> {
  const result = await GetConnections();
  return result.map(fromWailsConnectionStatus);
}

export async function getProfiles(): Promise<BrokerProfile[]> {
  const result = await GetProfiles();
  return result.map(fromWailsProfile);
}

export async function saveProfile(
  profile: BrokerProfile,
): Promise<BrokerProfile> {
  return fromWailsProfile(await SaveProfile(profile));
}

export function deleteProfile(id: string): Promise<void> {
  return DeleteProfile(id);
}
