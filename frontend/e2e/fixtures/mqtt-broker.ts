// フルスタック e2e 用の MQTT ブローカー (tools/e2e-broker)。playwright.integration.config.ts の
// webServer が実行ごとに起動し直すので、retained メッセージや購読は次の実行に持ち越されない。
// 1 回の実行の中では共有されるので、トピックはテストごとに変える。

/** ブローカーの MQTT (TCP) の待ち受けポート。 */
export const mqttBrokerPort = 18830;

/** ブローカーの操作用 HTTP の待ち受けポート。 */
export const mqttBrokerControlPort = 18831;

/**
 * 何も待ち受けていないポート。接続が拒否される宛先として使う。ほかのプロセスが使っていると、
 * 接続失敗のテストが落ちる。
 */
export const mqttUnusedPort = 18832;

/** ブローカーの MQTT (WebSocket) の待ち受けポート。 */
export const mqttBrokerWsPort = 18833;

/**
 * ブローカーの MQTT (TLS) の待ち受けポート。証明書は自己署名で、アプリからは信頼されない
 * (証明書の検証で接続が失敗する宛先として使う)。
 */
export const mqttBrokerTlsPort = 18834;

/** ブローカーの操作用 HTTP のベース URL。 */
export const mqttBrokerControlUrl = `http://127.0.0.1:${mqttBrokerControlPort}`;

/**
 * アプリ以外のクライアントとしてブローカーから publish する (mochi のインラインクライアント)。
 * payload にバイト列を渡すと、そのまま (UTF-8 でないバイナリも) 送る。
 * retain 付きの空ペイロードは、そのトピックの retained メッセージを消す。
 */
export async function publishFromBroker(
  topic: string,
  payload: string | Uint8Array<ArrayBuffer>,
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

/** 操作用 HTTP を呼ぶ。失敗したら、何をしようとして失敗したかを付けて投げる。 */
async function control(
  method: "POST" | "DELETE",
  path: string,
  action: string,
): Promise<void> {
  const res = await fetch(`${mqttBrokerControlUrl}${path}`, { method });
  if (!res.ok) {
    throw new Error(`${action} failed: ${res.status} ${await res.text()}`);
  }
}

/** ブローカーから見た、接続中のクライアント 1 件 (GET /clients)。 */
export interface BrokerClient {
  id: string;
  username: string;
  password: string;
  /** 接続したリスナー。TCP は "e2e"、WebSocket は "e2e-ws"、TLS は "e2e-tls"。 */
  listener: string;
  /** フィルターの順に並ぶ。 */
  subscriptions: Array<{ filter: string; qos: number }>;
}

/**
 * ブローカーに接続中のクライアントと、その購読を返す (ID の順)。アプリの接続のほかに、スキャン中なら
 * スキャン用の接続も含む。切断はブローカーが処理するまで反映されないので、expect.poll で待つ。
 */
export async function brokerClients(): Promise<BrokerClient[]> {
  const res = await fetch(`${mqttBrokerControlUrl}/clients`);
  if (!res.ok) {
    throw new Error(`list clients failed: ${res.status} ${await res.text()}`);
  }
  return (await res.json()) as BrokerClient[];
}

/**
 * ブローカーが接続中のクライアントをすべて切る (アプリの接続とスキャン用の接続)。Go は
 * mqtt:connection-lost と mqtt:scan-stopped を出し、接続は paho が自動で張り直す。
 */
export async function disconnectBrokerClients(): Promise<void> {
  await control("POST", "/disconnect-clients", "disconnect clients");
}

/**
 * 以後、filter への購読をブローカーが拒否する (SUBACK の失敗)。購読済みの購読はそのまま残り、
 * 再接続時の張り直しが拒否される。ブローカーは 1 回の実行の中で共有されるので、使った spec は
 * afterEach で allowBrokerFilters を呼ぶ。
 */
export async function denyBrokerFilter(filter: string): Promise<void> {
  const params = new URLSearchParams({ filter });
  await control("POST", `/deny?${params}`, `deny ${filter}`);
}

/** denyBrokerFilter で登録した拒否をすべて消す。 */
export async function allowBrokerFilters(): Promise<void> {
  await control("DELETE", "/deny", "allow filters");
}
