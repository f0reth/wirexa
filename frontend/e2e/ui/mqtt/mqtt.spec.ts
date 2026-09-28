import { expect, test } from "../../fixtures/ui";

// MQTT パネルはデフォルトで表示される

// ── 観点H-1: ブローカープロファイルの作成・削除 ──────────────────────────────

test("can create a broker profile", async ({ page, app }) => {
  await app.createBrokerProfile("Test Broker");
  await expect(page.getByText("Test Broker")).toBeVisible();
});

test("new broker dialog save button is disabled when name is empty", async ({
  page,
}) => {
  await page.getByRole("button", { name: "New Broker" }).click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  // 名前が空のため Save ボタンは無効
  await expect(
    dialog.getByRole("button", { name: "Save", exact: true }),
  ).toBeDisabled();
});

// 保存した broker URL を読み戻せない値（指数表記・小数のポート、コロンを含むホスト）は保存させない。
test("new broker dialog rejects a port or host that cannot be read back", async ({
  page,
}) => {
  await page.getByRole("button", { name: "New Broker" }).click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  const save = dialog.getByRole("button", { name: "Save", exact: true });
  const host = dialog.locator("#broker-host");
  const port = dialog.locator("#broker-port");

  await dialog.locator("#broker-name").fill("Validation Broker");
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

test("can delete a broker profile", async ({ page, app }) => {
  await app.createBrokerProfile("Delete Me");
  await expect(page.getByText("Delete Me")).toBeVisible();

  // アクションボタンはホバーで表示される
  await page.getByText("Delete Me").hover();
  await page.getByRole("button", { name: "Delete broker", exact: true }).click();

  await app.confirmDelete();

  await expect(page.getByText("Delete Me")).toBeHidden();
});

// ── 観点H-2: Subscribe/Publish タブ切り替え ───────────────────────────────────

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

// ── 観点H-4: トピックのサブスクライブ UIフロー ────────────────────────────────

test("subscribe button is disabled when broker is not connected", async ({
  page,
  app,
}) => {
  await app.createBrokerProfile("Offline Broker");

  const topicInput = page.getByPlaceholder("Topic (e.g., sensors/#)");
  await expect(topicInput).toBeVisible();

  await topicInput.fill("test/topic");

  // オフライン接続のため Subscribe ボタンは無効
  await expect(page.getByRole("button", { name: "Subscribe" })).toBeDisabled();
});

// ── 観点H-5: QoS の選択（0/1/2） ──────────────────────────────────────────────

test("can select QoS level 0, 1, and 2", async ({ page, app }) => {
  await app.createBrokerProfile("QoS Test Broker");

  const qosTrigger = page.getByTestId("qos-select").getByRole("button");
  await expect(qosTrigger).toBeVisible();

  // QoS 1 を選択
  await qosTrigger.click();
  await page.getByRole("button", { name: "QoS 1" }).click();
  await expect(qosTrigger).toContainText("1");

  // QoS 2 を選択
  await qosTrigger.click();
  await page.getByRole("button", { name: "QoS 2" }).click();
  await expect(qosTrigger).toContainText("2");

  // QoS 0 に戻す
  await qosTrigger.click();
  await page.getByRole("button", { name: "QoS 0" }).click();
  await expect(qosTrigger).toContainText("0");
});

// ── 観点H-6: Publish の retain フラグ ────────────────────────────────────────

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
  }) => {
    const row = (name: string) =>
      page.getByRole("button").filter({ hasText: name });
    const rows = page.getByRole("button").filter({ hasText: /^Broker / });
    await expect(rows).toHaveText([/Broker Alpha/, /Broker Beta/]);

    // HTML5 DnD (list-reorder.tsx)。行の上半分に落とすとその行の前に入る。
    // ドラッグが始まると挿入ゾーンが広がって行が下へずれるので、上端から少し離して落とす。
    const target = row(ALPHA.name);
    const box = await target.boundingBox();
    if (!box) throw new Error("broker row is not visible");
    await row(BETA.name).dragTo(target, {
      targetPosition: { x: box.width / 2, y: box.height / 3 },
    });

    await expect(rows).toHaveText([/Broker Beta/, /Broker Alpha/]);
    const stored = await page.evaluate(() =>
      localStorage.getItem("mqtt:profileOrder"),
    );
    expect(JSON.parse(stored ?? "null")).toEqual([BETA.id, ALPHA.id]);

    await page.reload();
    await expect(rows).toHaveText([/Broker Beta/, /Broker Alpha/]);
  });
});
