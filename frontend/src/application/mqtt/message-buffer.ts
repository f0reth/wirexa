import type {
  MqttRawMessage,
  Qos,
  ScannedTopic,
} from "../../domain/mqtt/types";
import { generateId } from "../../shared/id";
import type { ConnectionStateExt, MqttMessageView } from "./connection-state";
import { subscriptionMatches } from "./subscription";

// 1 フレームの間に溜める件数の上限。超えた分は捨てる。
const MAX_BUFFERED = 5000;

function groupByConnection<T extends { connectionId: string }>(
  items: T[],
): Map<string, T[]> {
  const grouped = new Map<string, T[]>();
  for (const item of items) {
    let arr = grouped.get(item.connectionId);
    if (!arr) {
      arr = [];
      grouped.set(item.connectionId, arr);
    }
    arr.push(item);
  }
  return grouped;
}

/**
 * 受信したメッセージとスキャンで見つかったトピックを溜め、1 フレームに 1 回まとめて接続の状態へ反映する。
 */
export function createEventBuffer(
  updateConnection: (
    connId: string,
    updater: (state: ConnectionStateExt) => ConnectionStateExt,
  ) => void,
  maxMessages: number,
  maxTopics: number,
) {
  // Micro-batch: buffer incoming messages and flush once per animation frame.
  const messageBuffer: MqttRawMessage[] = [];
  // スキャンで見つかったトピック (mqtt:scan-topic)。メッセージと同じフレームでまとめて反映する。
  const topicBuffer: ScannedTopic[] = [];
  let flushScheduled = false;

  function scheduleFlush() {
    if (flushScheduled) return;
    flushScheduled = true;
    requestAnimationFrame(() => {
      flushScheduled = false;
      flushTopics();
      flushMessages();
    });
  }

  function flushTopics() {
    if (topicBuffer.length === 0) return;

    const grouped = groupByConnection(topicBuffer);
    topicBuffer.length = 0;

    for (const [connId, scanned] of grouped) {
      updateConnection(connId, (state) => {
        // 停止と行き違いで届いたトピックは捨てる。
        if (!state.isScanning) return state;
        const added = [
          ...new Set(
            scanned
              .map((s) => s.topic)
              .filter((t) => !state.brokerTopicsSet.has(t)),
          ),
        ];
        if (added.length === 0) return state;

        let brokerTopics = [...state.brokerTopics, ...added];
        const brokerTopicsSet = new Set(state.brokerTopicsSet);
        for (const t of added) brokerTopicsSet.add(t);
        if (brokerTopics.length > maxTopics) {
          const excess = brokerTopics.length - maxTopics;
          for (const t of brokerTopics.slice(0, excess)) {
            brokerTopicsSet.delete(t);
          }
          brokerTopics = brokerTopics.slice(excess);
        }
        return { ...state, brokerTopics, brokerTopicsSet };
      });
    }
  }

  function flushMessages() {
    if (messageBuffer.length === 0) return;

    const grouped = groupByConnection(messageBuffer);
    messageBuffer.length = 0;

    for (const [connId, batch] of grouped) {
      updateConnection(connId, (state) => {
        const pendingMessages: MqttMessageView[] = [];

        for (const data of batch) {
          const msgParts = data.topic.split("/");
          // 重なる購読（例: muted の sensors/# と sensors/temp）があれば、
          // muted でない購読が 1 つでも一致すれば表示する。
          // 照合は受信した時点ではなく、反映する時点の購読で行う。
          const delivered = state.subscriptions.some(
            (s) => !s.muted && subscriptionMatches(s, data.topic, msgParts),
          );
          if (delivered) {
            pendingMessages.push({
              id: generateId(),
              topic: data.topic,
              payload: data.payload,
              payloadBase64: data.payloadBase64 ?? false,
              // イベントの qos は number で届く。
              qos: data.qos as Qos,
              timestamp: new Date(data.timestamp),
              direction: "incoming",
            });
          }
        }

        if (pendingMessages.length === 0) return state;
        const combined = [...state.messages, ...pendingMessages];
        if (combined.length > maxMessages) {
          combined.splice(0, combined.length - maxMessages);
        }
        return { ...state, messages: combined };
      });
    }
  }

  function pushMessage(message: MqttRawMessage) {
    if (messageBuffer.length < MAX_BUFFERED) {
      messageBuffer.push(message);
    }
    scheduleFlush();
  }

  function pushTopic(topic: ScannedTopic) {
    if (topicBuffer.length < MAX_BUFFERED) {
      topicBuffer.push(topic);
    }
    scheduleFlush();
  }

  return { pushMessage, pushTopic };
}
