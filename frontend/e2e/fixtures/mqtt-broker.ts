// フルスタック e2e 用の MQTT ブローカー (tools/e2e-broker)。playwright.integration.config.ts の
// webServer が実行ごとに起動し直すので、retained メッセージや購読は次の実行に持ち越されない。
// 1 回の実行の中では共有されるので、トピックはテストごとに変える。

/** ブローカーの MQTT (TCP) の待ち受けポート。 */
export const mqttBrokerPort = 18830;

/** ブローカーの操作用 HTTP の待ち受けポート。 */
export const mqttBrokerControlPort = 18831;

/** ブローカーの操作用 HTTP のベース URL。 */
export const mqttBrokerControlUrl = `http://127.0.0.1:${mqttBrokerControlPort}`;

/**
 * アプリ以外のクライアントとしてブローカーから publish する (mochi のインラインクライアント)。
 * retain 付きの空ペイロードは、そのトピックの retained メッセージを消す。
 */
export async function publishFromBroker(
  topic: string,
  payload: string,
  options: { qos?: 0 | 1 | 2; retain?: boolean } = {},
): Promise<void> {
  const params = new URLSearchParams({
    topic,
    qos: String(options.qos ?? 0),
    retain: String(options.retain ?? false),
  });
  const res = await fetch(`${mqttBrokerControlUrl}/publish?${params}`, {
    method: "POST",
    body: payload,
  });
  if (!res.ok) {
    throw new Error(`publish ${topic} failed: ${res.status} ${await res.text()}`);
  }
}
