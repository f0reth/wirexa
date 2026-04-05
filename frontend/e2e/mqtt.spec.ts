import { expect, test } from "./fixtures/app";

interface WailsMockWindow {
  __wailsMock: {
    store: { mqttConnections: Array<{ id: string }> };
  };
}

async function createConnectedBroker(
  page: import("@playwright/test").Page,
  wailsMock: import("./fixtures/app").WailsMockApi,
  name = "Test Broker",
): Promise<string> {
  await page.getByTitle("New Broker").click();
  await page.locator("#broker-name").fill(name);
  await page.getByRole("button", { name: "Save & Connect" }).click();

  // Wait for the online connection state to be created (BrokerManager shows "Disconnected")
  await expect(page.getByText("Disconnected")).toBeVisible();

  const connId = await page.evaluate(
    () =>
      (
        window as unknown as WailsMockWindow
      ).__wailsMock.store.mqttConnections.at(-1)?.id ?? "",
  );

  await wailsMock.emit("mqtt:connected", { connectionId: connId });
  return connId;
}

test.describe("MQTT Client UI", () => {
  test("ブローカー設定ダイアログ表示 - New Broker button opens dialog", async ({
    page,
    wailsMock: _wailsMock,
  }) => {
    await page.getByTitle("New Broker").click();

    await expect(
      page.getByRole("dialog", { name: "New Profile" }),
    ).toBeVisible();
  });

  test("プロファイル保存・選択 - create profile → select → offline connection shown", async ({
    page,
    wailsMock: _wailsMock,
  }) => {
    await page.getByTitle("New Broker").click();
    await page.locator("#broker-name").fill("My Test Broker");
    await page.getByRole("button", { name: "Save" }).click();

    await expect(page.getByText("My Test Broker")).toBeVisible();

    // Click profile to create offline connection
    await page.getByText("My Test Broker").click();

    await expect(page.getByText("Disconnected")).toBeVisible();
  });

  test("ブローカー接続 - connect → connected status shown", async ({
    page,
    wailsMock,
  }) => {
    await createConnectedBroker(page, wailsMock);

    await expect(page.getByText("Connected")).toBeVisible();
  });

  test("ブローカー切断 - disconnect → disconnected status shown", async ({
    page,
    wailsMock,
  }) => {
    await createConnectedBroker(page, wailsMock);
    await expect(page.getByText("Connected")).toBeVisible();

    await page.getByRole("button", { name: "Disconnect" }).click();

    await expect(page.getByText("Disconnected")).toBeVisible();
  });

  test("トピック購読 - subscribe to topic → shown in subscription list", async ({
    page,
    wailsMock,
  }) => {
    await createConnectedBroker(page, wailsMock);

    await page
      .getByPlaceholder("Topic (e.g., sensors/#)")
      .fill("sensor/temperature");
    await page.getByRole("button", { name: "Subscribe" }).click();

    await expect(page.getByText("sensor/temperature")).toBeVisible();
  });

  test("購読解除 - unsubscribe removes topic from list", async ({
    page,
    wailsMock,
  }) => {
    await createConnectedBroker(page, wailsMock);

    await page.getByPlaceholder("Topic (e.g., sensors/#)").fill("test/remove");
    await page.getByRole("button", { name: "Subscribe" }).click();
    await expect(page.getByText("test/remove")).toBeVisible();

    await page
      .locator('[class*="subscriptionItem"]')
      .filter({ hasText: "test/remove" })
      .locator('[class*="deleteButton"]')
      .click();

    await expect(page.getByText("test/remove")).not.toBeVisible();
  });

  test("メッセージパブリッシュ - topic and payload input → publish", async ({
    page,
    wailsMock,
  }) => {
    await createConnectedBroker(page, wailsMock);

    await page.getByRole("tab", { name: "Publish" }).click();

    await page.getByPlaceholder("Topic").fill("test/publish");
    await page.getByPlaceholder("Message payload").fill("Hello World");

    // Publish button is enabled because we are connected
    await page.getByRole("button", { name: "Publish" }).click();
  });

  test("メッセージ受信表示 - message event → displayed in message list", async ({
    page,
    wailsMock,
  }) => {
    const connId = await createConnectedBroker(page, wailsMock);

    await page.getByPlaceholder("Topic (e.g., sensors/#)").fill("data/sensor");
    await page.getByRole("button", { name: "Subscribe" }).click();
    await expect(page.getByText("data/sensor")).toBeVisible();

    await wailsMock.emit("mqtt:message", {
      connectionId: connId,
      topic: "data/sensor",
      payload: '{"value":42}',
      qos: 0,
      timestamp: Date.now(),
    });

    await expect(page.getByText('{"value":42}')).toBeVisible();
  });

  test("メッセージ詳細表示 - click message → detail panel shown", async ({
    page,
    wailsMock,
  }) => {
    const connId = await createConnectedBroker(page, wailsMock);

    await page.getByPlaceholder("Topic (e.g., sensors/#)").fill("detail/test");
    await page.getByRole("button", { name: "Subscribe" }).click();
    await expect(page.getByText("detail/test")).toBeVisible();

    await wailsMock.emit("mqtt:message", {
      connectionId: connId,
      topic: "detail/test",
      payload: "detail payload",
      qos: 1,
      timestamp: Date.now(),
    });

    await expect(page.getByText("detail payload")).toBeVisible();
    await page.getByText("detail payload").click();

    await expect(page.getByText("QoS 1")).toBeVisible();
  });
});
