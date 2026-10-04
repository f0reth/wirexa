import { expect, test, WailsEvents } from "../../fixtures/ui";

// MQTT パネルはデフォルトで表示される

const EMPTY_STATE =
  "No active connection. Select a broker from the sidebar to connect.";

// ── 観点H: ブローカープロファイルの作成・削除 ────────────────────────────────

test("can create a broker profile", async ({ app, fake }) => {
  await app.createBrokerProfile("Test Broker");

  // 新規作成は ID を空で送り、採番はバックエンドに任せる。URL はダイアログの既定値。
  expect(await fake.args("SaveProfile")).toEqual([
    [
      {
        id: "",
        name: "Test Broker",
        broker: "mqtt://localhost:1883",
        clientId: "",
        username: "",
        password: "",
        useTls: false,
      },
    ],
  ]);
  await expect(app.broker("Test Broker")).toContainText(
    "mqtt://localhost:1883",
  );
  // 作ったブローカーが選ばれ、接続バーが出る。
  await expect(app.brokerConnectButton).toBeVisible();
  await expect(app.brokerHostInput()).toHaveValue("localhost");
  await expect(app.brokerPortInput()).toHaveValue("1883");
});

test("new broker dialog save button is disabled when name is empty", async ({
  page,
  app,
}) => {
  await page.getByRole("button", { name: "New Broker" }).click();

  const dialog = app.brokerDialog("New Profile");
  await expect(dialog).toBeVisible();

  // 名前が空のため Save ボタンは無効
  await expect(
    dialog.getByRole("button", { name: "Save", exact: true }),
  ).toBeDisabled();
});

// 保存した broker URL を読み戻せない値（指数表記・小数のポート、コロンを含むホスト）は保存させない。
test("new broker dialog rejects a port or host that cannot be read back", async ({
  page,
  app,
}) => {
  await page.getByRole("button", { name: "New Broker" }).click();

  const dialog = app.brokerDialog("New Profile");
  await expect(dialog).toBeVisible();
  const save = dialog.getByRole("button", { name: "Save", exact: true });
  const host = app.brokerHostInput(dialog);
  const port = app.brokerPortInput(dialog);

  await dialog.getByLabel("Name", { exact: true }).fill("Validation Broker");
  await host.fill("localhost");
  await port.fill("1883");
  await expect(save).toBeEnabled();

  await port.fill("1e3");
  await expect(save).toBeDisabled();
  await port.fill("1883.0");
  await expect(save).toBeDisabled();

  await port.fill("1883");
  await host.fill("::1");
  await expect(save).toBeDisabled();
  await host.fill("broker.example.com");
  await expect(save).toBeEnabled();
});

test("can delete a broker profile", async ({ page, app, fake }) => {
  await app.createBrokerProfile("Delete Me");
  await expect(app.broker("Delete Me")).toBeVisible();
  const [created] = (await fake.snapshot()).mqttProfiles;

  await (await app.brokerRowAction("Delete Me", "Delete broker")).click();

  await app.confirmDelete();

  await expect(app.broker("Delete Me")).toHaveCount(0);
  expect(await fake.args("DeleteProfile")).toEqual([[created.id]]);
  expect((await fake.snapshot()).mqttProfiles).toEqual([]);
  // 最後の 1 件を消したので、空状態に戻る。
  await expect(page.getByText("No brokers yet")).toBeVisible();
  await expect(page.getByText(EMPTY_STATE)).toBeVisible();
});

// ── 観点H: Subscribe / Publish タブの切り替え ────────────────────────────────

test("can switch between subscribe and publish tabs", async ({ page, app }) => {
  await app.createBrokerProfile("Tab Test Broker");

  const subscribeTab = page.getByRole("tab", { name: "Subscribe" });
  const publishTab = page.getByRole("tab", { name: "Publish" });

  // 初期状態: Subscribe タブがアクティブ
  await expect(subscribeTab).toHaveAttribute("aria-selected", "true");
  await expect(publishTab).toHaveAttribute("aria-selected", "false");

  // Publish タブに切り替え
  await publishTab.click();
  await expect(publishTab).toHaveAttribute("aria-selected", "true");
  await expect(subscribeTab).toHaveAttribute("aria-selected", "false");

  // Subscribe タブに戻す
  await subscribeTab.click();
  await expect(subscribeTab).toHaveAttribute("aria-selected", "true");
  await expect(publishTab).toHaveAttribute("aria-selected", "false");
});

// ── 観点H: 未接続のブローカーでの購読 ────────────────────────────────────────

test("subscribe button is disabled when broker is not connected", async ({
  app,
}) => {
  await app.createBrokerProfile("Offline Broker");

  await expect(app.mqttTopicInput).toBeVisible();

  await app.mqttTopicInput.fill("test/topic");

  // オフライン接続のため Subscribe ボタンは無効
  await expect(app.mqttSubscribeButton).toBeDisabled();
});

// ── 観点H: 購読の QoS の選択 ─────────────────────────────────────────────────
// 未接続のブローカーなので、トリガーの表示だけを見る。選んだ値が Subscribe に渡ることは
// messages.spec.ts の "subscribing and unsubscribing reach the backend" が見る。

test("can select QoS level 0, 1, and 2", async ({ page, app }) => {
  await app.createBrokerProfile("QoS Test Broker");

  // トリガーには現在の値だけが出る。
  const qosTrigger = page.getByTestId("qos-select").getByRole("button");
  await expect(qosTrigger).toHaveText("0");

  // QoS 1 を選択
  await qosTrigger.click();
  await page.getByRole("button", { name: "QoS 1" }).click();
  await expect(qosTrigger).toHaveText("1");

  // QoS 2 を選択
  await qosTrigger.click();
  await page.getByRole("button", { name: "QoS 2" }).click();
  await expect(qosTrigger).toHaveText("2");

  // QoS 0 に戻す
  await qosTrigger.click();
  await page.getByRole("button", { name: "QoS 0" }).click();
  await expect(qosTrigger).toHaveText("0");
});

// ── 観点H: Publish の retain フラグ ──────────────────────────────────────────

test("can toggle the retain flag and it is stored on the selected preset", async ({
  page,
  app,
}) => {
  await app.createBrokerProfile("Retain Test Broker");
  await page.getByRole("tab", { name: "Publish" }).click();

  const retain = page.getByRole("checkbox", { name: "Retain" });
  await expect(retain).not.toBeChecked();

  // ON にすると選択中プリセットに反映され、一覧に Retained バッジが出る
  await retain.check();
  await expect(retain).toBeChecked();
  await expect(page.getByText("Retained")).toBeVisible();

  await retain.uncheck();
  await expect(retain).not.toBeChecked();
  await expect(page.getByText("Retained")).toBeHidden();
});

// ── 観点F: ブローカーの並び順 (localStorage の mqtt:profileOrder) ────────────
// 並びはバックエンドではなく localStorage に保存される。偽バックエンドは seed の順
// (Alpha, Beta) で返すので、リロード後に Beta が先なら保存した並びが使われている。

test.describe("broker order", () => {
  const ALPHA = { id: "profile-alpha", name: "Broker Alpha" };
  const BETA = { id: "profile-beta", name: "Broker Beta" };

  test.use({ seed: { mqttProfiles: [ALPHA, BETA] } });

  test("reordered brokers are kept in mqtt:profileOrder after reload", async ({
    page,
    app,
  }) => {
    const rows = page.getByRole("button").filter({ hasText: /^Broker / });
    await expect(rows).toHaveText([/Broker Alpha/, /Broker Beta/]);

    await app.dragListRowBefore(app.broker(BETA.name), app.broker(ALPHA.name));

    await expect(rows).toHaveText([/Broker Beta/, /Broker Alpha/]);
    const stored = await page.evaluate(() =>
      localStorage.getItem("mqtt:profileOrder"),
    );
    expect(JSON.parse(stored ?? "null")).toEqual([BETA.id, ALPHA.id]);

    await page.reload();
    await expect(rows).toHaveText([/Broker Beta/, /Broker Alpha/]);
  });
});

// ── 観点H: Broker Topics のスキャン ──────────────────────────────────────────
// スキャンは購読ではなく StartTopicScan / StopTopicScan で行う (Go は専用の接続で # を購読する)。
// ブローカーは無いので、見つかったトピックは Go が発火する mqtt:scan-topic を、スキャン用の
// 接続の切断は mqtt:scan-stopped を偽バックエンドから流して模す。

test.describe("broker topics scan", () => {
  const BROKER = { id: "profile-local", name: "Local Broker" };

  test.use({ seed: { mqttProfiles: [BROKER], mqttConnect: "ok" } });

  test("scan lists the topics found by the backend without subscribing", async ({
    page,
    app,
    fake,
  }) => {
    await app.connectBroker(BROKER.name);
    const connectionId = (await fake.snapshot()).mqttConnections[0].id;
    const panel = app.mqttSection("Broker Topics");
    const scan = app.mqttScanButton;
    const stop = app.mqttStopScanButton;
    await expect(panel.getByText("No topics found")).toBeVisible();

    await scan.click();

    await expect(stop).toBeVisible();
    await fake.waitForCalls("StartTopicScan");
    expect(await fake.args("StartTopicScan")).toEqual([[connectionId]]);
    expect((await fake.snapshot()).mqttConnections[0]).toMatchObject({
      scanning: true,
      subscriptions: [],
    });

    await fake.emitAll(WailsEvents.mqttScanTopic, [
      { connectionId, topic: "sensors/temp" },
      { connectionId, topic: "sensors/humidity" },
      { connectionId, topic: "sensors/temp" },
    ]);

    await expect(panel.getByText("sensors/temp", { exact: true })).toHaveCount(
      1,
    );
    await expect(
      panel.getByText("sensors/humidity", { exact: true }),
    ).toBeVisible();
    // スキャンはトピックを集めるだけで、購読もメッセージも増やさない。
    await expect(app.mqttMessages).toHaveCount(0);
    expect(await fake.calls("Subscribe")).toBe(0);

    await stop.click();

    await expect(scan).toBeVisible();
    await fake.waitForCalls("StopTopicScan");
    expect(await fake.args("StopTopicScan")).toEqual([[connectionId]]);
    expect((await fake.snapshot()).mqttConnections[0].scanning).toBe(false);
    expect(await fake.calls("Unsubscribe")).toBe(0);
    // 止めても見つけたトピックは残る。
    await expect(panel.getByText("sensors/temp", { exact: true })).toBeVisible();

    // スキャン用の接続が切れたら、ボタンは Scan に戻って通知が出る。
    await scan.click();
    await expect(stop).toBeVisible();
    await fake.emit(WailsEvents.mqttScanStopped, {
      connectionId,
      error: "EOF",
    });

    await expect(scan).toBeVisible();
    await expect(
      page.getByRole("alert").filter({ hasText: "MQTT topic scan stopped" }),
    ).toContainText("EOF");
  });
});
