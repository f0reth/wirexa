import { type ConnectionState, isConnected } from "../../domain/mqtt/types";
import type { Notifier } from "../../domain/ui/ports";
import { runGuarded } from "../ui/guard";
import type { PublishDraft } from "./presets";

export interface MqttPublishApi {
  publish(
    connectionId: string,
    topic: string,
    payload: string,
    qos: number,
    retain: boolean,
  ): Promise<void>;
}

/** フォームの内容を送信できるかを返す。 */
export function canPublish(draft: PublishDraft): boolean {
  if (!draft.topic.trim()) return false;
  // retain 付きの空ペイロードは retained メッセージの削除を意味するので許可する。
  return draft.retain || draft.payload.trim().length > 0;
}

export function createPublishState(
  api: MqttPublishApi,
  activeConnection: () => ConnectionState | null,
  draft: () => PublishDraft,
  notifier: Notifier,
) {
  /** フォームの内容をアクティブな接続へ送信する。送信できない内容や未接続なら何もしない。 */
  async function publishDraft(): Promise<void> {
    const conn = activeConnection();
    // オフラインのタブも接続 ID (offline-<profileId>) を持つので、ID の有無ではなく接続中かを見る。
    if (!conn || !isConnected(conn)) return;
    const d = draft();
    if (!canPublish(d)) return;
    await runGuarded(notifier, "Failed to publish message", () =>
      api.publish(conn.connectionId, d.topic, d.payload, d.qos, d.retain),
    );
  }

  return { publishDraft };
}
