import type { Page } from "@playwright/test";
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

/** New Broker のダイアログで e2e ブローカーを指すプロファイルを作り、action のボタンで閉じる。 */
async function newBroker(
  page: Page,
  name: string,
  action: "Save" | "Save & Connect" = "Save",
): Promise<void> {
  await page.getByRole("button", { name: "New Broker" }).click();
  const dialog = page.getByRole("dialog", { name: "New Profile" });
  await dialog.getByLabel("Name", { exact: true }).fill(name);
  await dialog.getByPlaceholder("localhost").fill("127.0.0.1");
  await dialog.getByPlaceholder("1883").fill(String(mqttBrokerPort));
  await dialog.getByRole("button", { name: action, exact: true }).click();
  await expect(dialog).toBeHidden();
}

/** e2e ブローカーを指すプロファイルを作って接続し、Connected になるまで待つ。 */
async function connectNewBroker(app: App, name: string): Promise<void> {
  await newBroker(app.page, name);
  await app.connectBroker(name);
}

/** Subscriptions パネルから購読し、購読の行が出るまで待つ (Go の Subscribe は SUBACK を待つ)。 */
async function subscribe(app: App, topic: string): Promise<void> {
  const panel = app.mqttSection("Subscriptions");
  await panel.getByPlaceholder("Topic (e.g., sensors/#)").fill(topic);
  await panel.getByRole("button", { name: "Subscribe", exact: true }).click();
  await expect(app.mqttSubscription(topic)).toBeVisible();
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
  await expect(page.getByPlaceholder("localhost")).toBeHidden();
  await expect(
    page.getByRole("button", { name: "Disconnect", exact: true }),
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

  await page.getByRole("button", { name: "Disconnect", exact: true }).click();

  await expect(page.getByText("Disconnected", { exact: true })).toBeVisible();
  await expect(app.brokerConnectButton).toBeVisible();
  await expect(app.broker(name).getByTitle("Disconnected")).toBeAttached();
  expect(await mqttConnections(page)).toEqual([]);

  // 切断したタブからもう一度つなげる。
  await app.brokerConnectButton.click();
  await expect(page.getByText("Connected", { exact: true })).toBeVisible();
});

test("Save & Connect saves the profile and connects to it", async ({
  page,
  app,
}) => {
  const name = "E2E MQTT Save And Connect";
  await newBroker(page, name, "Save & Connect");

  await expect(page.getByText("Connected", { exact: true })).toBeVisible();
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
  await subscribe(app, "e2e/receive/#");

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
  await subscribe(app, topic);

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
  await panel.getByPlaceholder("Topic (e.g., sensors/#)").fill("a/#/b");
  await panel.getByRole("button", { name: "Subscribe", exact: true }).click();

  await expect(
    page.getByRole("alert").filter({ hasText: "Failed to subscribe to a/#/b" }),
  ).toContainText("invalid topic: # must occupy the last level entirely");
  await expect(panel.getByText("No subscriptions")).toBeVisible();

  // 検証で弾いただけなので接続は保たれ、続けて購読できる。
  await expect(page.getByText("Connected", { exact: true })).toBeVisible();
  await subscribe(app, "e2e/invalid/ok");
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
  await subscribe(app, topic);
  try {
    await page.getByRole("button", { name: "Disconnect", exact: true }).click();
    await expect(app.brokerConnectButton).toBeVisible();
    await expect(app.mqttSubscription(topic)).toBeVisible();
    await publishFromBroker(topic, payload, { retain: true });

    await app.brokerConnectButton.click();
    await expect(page.getByText("Connected", { exact: true })).toBeVisible();

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
  await subscribe(app, "e2e/switch/alpha");
  await connectNewBroker(app, beta);
  await subscribe(app, "e2e/switch/beta");
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

    await panel.getByRole("button", { name: "Scan", exact: true }).click();

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
    await panel.getByRole("button", { name: "Stop", exact: true }).click();
    await expect(
      panel.getByRole("button", { name: "Scan", exact: true }),
    ).toBeVisible();
    const [conn] = await mqttConnections(page);
    expect(conn.subscriptions).toEqual([{ topic, qos: 0 }]);
  } finally {
    // retain 付きの空ペイロードで retained メッセージを消し、後のスキャンに残さない。
    await publishFromBroker(topic, "", { retain: true });
  }
});
