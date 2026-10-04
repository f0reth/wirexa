import {
  type App,
  disconnectMqttConnections,
  expect,
  mqttConnections,
  test,
} from "../../fixtures/integration";
import {
  mqttBrokerPort,
  publishFromBroker,
} from "../../fixtures/mqtt-broker";

// 実 Go バックエンド越しに、実ブローカー (tools/e2e-broker) へ接続・購読・publish する。
// ブローカーは 1 回の実行の中で共有されるので、ブローカー名とトピックはテストごとに変える。
// 接続と購読は Go 側に残り、次のテストの読み込み時にオンラインのタブとして復元されるので、
// afterEach でバックエンドから直接切る。

const BROKER_URL = `mqtt://127.0.0.1:${mqttBrokerPort}`;

/** e2e ブローカーの宛先。app.createBrokerProfile に渡す。 */
const E2E_BROKER = { host: "127.0.0.1", port: mqttBrokerPort };

/** e2e ブローカーを指すプロファイルを作って接続し、Connected になるまで待つ。 */
async function connectNewBroker(app: App, name: string): Promise<void> {
  await app.createBrokerProfile(name, E2E_BROKER);
  await app.connectBroker(name);
}

/** 接続済みのブローカーへ切り替える。行をフォーカスして Enter で選ぶ (app.selectBroker と同じ理由)。 */
async function switchBroker(app: App, name: string): Promise<void> {
  const row = app.broker(name);
  await row.focus();
  await row.press("Enter");
}

test.afterEach(async ({ page }) => {
  await disconnectMqttConnections(page);
});

// ── 観点H・M: 接続と切断 ─────────────────────────────────────────────────────

test("connects to a local broker and shows Connected", async ({
  page,
  app,
}) => {
  const name = "E2E MQTT Connect";
  await connectNewBroker(app, name);

  // 接続バーは URL の入力欄を閉じて Disconnect を出す。
  await expect(app.brokerHostInput()).toBeHidden();
  await expect(
    app.brokerDisconnectButton,
  ).toBeVisible();
  await expect(app.broker(name).getByTitle("Connected")).toBeAttached();
  await expect(page.getByRole("tab", { name: "Subscribe" })).toBeVisible();

  const conns = await mqttConnections(page);
  expect(conns).toEqual([
    expect.objectContaining({ name, broker: BROKER_URL, connected: true }),
  ]);
});

test("disconnect returns the tab to Disconnected", async ({ page, app }) => {
  const name = "E2E MQTT Disconnect";
  await connectNewBroker(app, name);

  await app.brokerDisconnectButton.click();

  await expect(app.mqttStatus("Disconnected")).toBeVisible();
  await expect(app.brokerConnectButton).toBeVisible();
  await expect(app.broker(name).getByTitle("Disconnected")).toBeAttached();
  expect(await mqttConnections(page)).toEqual([]);

  // 切断したタブからもう一度つなげる。
  await app.brokerConnectButton.click();
  await expect(app.mqttStatus("Connected")).toBeVisible();
});

test("Save & Connect saves the profile and connects to it", async ({
  page,
  app,
}) => {
  const name = "E2E MQTT Save And Connect";
  await app.createBrokerProfile(name, {
    ...E2E_BROKER,
    action: "Save & Connect",
  });

  await expect(app.mqttStatus("Connected")).toBeVisible();
  await expect(app.broker(name)).toContainText(BROKER_URL);
  expect(await mqttConnections(page)).toEqual([
    expect.objectContaining({ name, broker: BROKER_URL, connected: true }),
  ]);
});

// ── 観点H・M: 購読と受信 ─────────────────────────────────────────────────────

test("published message on a subscribed topic appears in Messages", async ({
  page,
  app,
}) => {
  await connectNewBroker(app, "E2E MQTT Receive");
  await app.subscribeMqtt("e2e/receive/#");

  // アプリ以外のクライアントから publish する。
  await publishFromBroker("e2e/receive/temp", '{"temp":21.5}', { qos: 1 });
  await publishFromBroker("e2e/other/temp", "not-subscribed");

  const item = app.mqttMessage('{"temp":21.5}');
  await expect(item).toContainText("e2e/receive/temp");
  await item.click();
  await expect(page.getByText("incoming", { exact: true })).toBeVisible();
  await expect(page.getByText('{\n  "temp": 21.5\n}')).toBeVisible();
  // 購読していないトピックは届かない。
  await expect(app.mqttMessages).toHaveCount(1);
});

test("publishing to a subscribed topic loops back into Messages", async ({
  page,
  app,
}) => {
  const topic = "e2e/loopback/lamp";
  await connectNewBroker(app, "E2E MQTT Loopback");
  await app.subscribeMqtt(topic);

  await page.getByRole("tab", { name: "Publish" }).click();
  const form = app.mqttSection("Publish");
  await form.getByPlaceholder("Topic", { exact: true }).fill(topic);
  await form.getByPlaceholder("Message payload").fill("loopback-on");
  await form.getByRole("button", { name: "Publish", exact: true }).click();

  // 送信した側では一覧に足さないので、ここに出るのはブローカーから届いたもの。
  await page.getByRole("tab", { name: "Subscribe" }).click();
  await expect(app.mqttMessage("loopback-on")).toContainText(topic);
});

test("subscribing to 'a/#/b' shows a validation error toast", async ({
  page,
  app,
}) => {
  await connectNewBroker(app, "E2E MQTT Invalid Topic");
  const panel = app.mqttSection("Subscriptions");
  await app.mqttTopicInput.fill("a/#/b");
  await app.mqttSubscribeButton.click();

  await expect(
    page.getByRole("alert").filter({ hasText: "Failed to subscribe to a/#/b" }),
  ).toContainText("invalid topic: # must occupy the last level entirely");
  await expect(panel.getByText("No subscriptions")).toBeVisible();

  // 検証で弾いただけなので接続は保たれ、続けて購読できる。
  await expect(app.mqttStatus("Connected")).toBeVisible();
  await app.subscribeMqtt("e2e/invalid/ok");
  const [conn] = await mqttConnections(page);
  expect(conn.subscriptions).toEqual([{ topic: "e2e/invalid/ok", qos: 0 }]);
});

// 手動の Disconnect → Connect は新しい接続を作り、残っている購読の行をその接続へ張り直す。
// 購読の成立を画面から待てないので、切断中に retained メッセージを置き、購読と同時に届くようにする。
test("reconnecting after Disconnect keeps receiving subscribed topics", async ({
  page,
  app,
}) => {
  const topic = "e2e/reconnect/retained";
  const payload = "after-manual-reconnect";
  await connectNewBroker(app, "E2E MQTT Manual Reconnect");
  await app.subscribeMqtt(topic);
  try {
    await app.brokerDisconnectButton.click();
    await expect(app.brokerConnectButton).toBeVisible();
    await expect(app.mqttSubscription(topic)).toBeVisible();
    await publishFromBroker(topic, payload, { retain: true });

    await app.brokerConnectButton.click();
    await expect(app.mqttStatus("Connected")).toBeVisible();

    await expect(app.mqttMessage(payload).first()).toContainText(topic);
    await expect(
      page.getByRole("alert").filter({ hasText: "Failed to re-subscribe" }),
    ).toHaveCount(0);
    const [conn] = await mqttConnections(page);
    expect(conn.subscriptions).toEqual([{ topic, qos: 0 }]);
  } finally {
    // retain 付きの空ペイロードで retained メッセージを消し、後のテストに残さない。
    await publishFromBroker(topic, "", { retain: true });
  }
});

// ── 観点H: 複数ブローカーの購読の独立 ────────────────────────────────────────

test("switching brokers keeps each broker's subscriptions separate", async ({
  page,
  app,
}) => {
  const alpha = "E2E MQTT Switch Alpha";
  const beta = "E2E MQTT Switch Beta";
  await connectNewBroker(app, alpha);
  await app.subscribeMqtt("e2e/switch/alpha");
  await connectNewBroker(app, beta);
  await app.subscribeMqtt("e2e/switch/beta");
  await expect(app.mqttSubscription("e2e/switch/alpha")).toBeHidden();

  // 両方のトピックへ届けても、各ブローカーには自分の購読の分だけが出る。
  await publishFromBroker("e2e/switch/alpha", "to-alpha");
  await publishFromBroker("e2e/switch/beta", "to-beta");
  await expect(app.mqttMessage("to-beta")).toBeVisible();
  await expect(app.mqttMessage("to-alpha")).toHaveCount(0);

  await switchBroker(app, alpha);
  await expect(app.mqttSubscription("e2e/switch/alpha")).toBeVisible();
  await expect(app.mqttSubscription("e2e/switch/beta")).toBeHidden();
  await expect(app.mqttMessage("to-alpha")).toBeVisible();
  await expect(app.mqttMessage("to-beta")).toHaveCount(0);

  const subsByName = Object.fromEntries(
    (await mqttConnections(page)).map((c) => [
      c.name,
      c.subscriptions.map((s) => s.topic),
    ]),
  );
  expect(subsByName).toEqual({
    [alpha]: ["e2e/switch/alpha"],
    [beta]: ["e2e/switch/beta"],
  });
});

// ── 観点H: Broker Topics のスキャン ──────────────────────────────────────────
// スキャンは専用の接続で "#" を購読し、届いたトピックを一覧にする (元の接続の購読には現れない)。
// 購読の完了を画面から待てないので、先に retained メッセージを置いておき、"#" の購読と同時に
// 届くようにする。

test("scanning lists broker topics and subscribes from the list", async ({
  page,
  app,
}) => {
  const topic = "e2e/scan/retained";
  await publishFromBroker(topic, "kept", { retain: true });
  try {
    await connectNewBroker(app, "E2E MQTT Scan");
    const panel = app.mqttSection("Broker Topics");
    await expect(panel.getByText("No topics found")).toBeVisible();

    await app.mqttScanButton.click();

    await expect(panel.getByText(topic, { exact: true })).toBeVisible();
    // スキャンはトピックを集めるだけで、購読していないトピックのメッセージは一覧に出さない。
    await expect(app.mqttMessages).toHaveCount(0);
    // スキャンの "#" は専用の接続が購読するので、この接続の購読には無い。
    expect(await mqttConnections(page)).toEqual([
      expect.objectContaining({ scanning: true, subscriptions: [] }),
    ]);

    await panel.getByTitle("Subscribe").click();
    await expect(app.mqttSubscription(topic)).toBeVisible();
    await expect(panel.getByTitle("Already subscribed")).toBeDisabled();
    // 購読した retained メッセージが届く。スキャンの "#" は別の接続なので、この接続には 1 件だけ届く。
    await expect(app.mqttMessage("kept")).toContainText(topic);
    await expect(app.mqttMessages).toHaveCount(1);
    await app.mqttStopScanButton.click();
    await expect(app.mqttScanButton).toBeVisible();
    const [conn] = await mqttConnections(page);
    expect(conn.subscriptions).toEqual([{ topic, qos: 0 }]);
  } finally {
    // retain 付きの空ペイロードで retained メッセージを消し、後のスキャンに残さない。
    await publishFromBroker(topic, "", { retain: true });
  }
});
