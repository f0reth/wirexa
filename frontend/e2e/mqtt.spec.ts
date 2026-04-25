import { type Page, expect, test } from "@playwright/test";

async function createBrokerProfile(page: Page, name: string) {
  await page.getByRole("button", { name: "New Broker" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Name", { exact: true }).fill(name);
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect(dialog).not.toBeVisible();
}

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  // MQTT パネルはデフォルトで表示される
});

// ── 観点H-1: ブローカープロファイルの作成・削除 ──────────────────────────────

test("can create a broker profile", async ({ page }) => {
  await page.getByRole("button", { name: "New Broker" }).click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  await dialog.getByLabel("Name", { exact: true }).fill("Test Broker");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  await expect(dialog).not.toBeVisible();
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

test("can delete a broker profile", async ({ page }) => {
  await createBrokerProfile(page, "Delete Me");
  await expect(page.getByText("Delete Me")).toBeVisible();

  // アクションボタンはホバーで表示される
  await page.getByText("Delete Me").hover();
  await page.getByRole("button", { name: "Delete broker", exact: true }).click();

  const confirmDialog = page.getByRole("dialog");
  await expect(confirmDialog).toBeVisible();

  await confirmDialog.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText("Delete Me")).not.toBeVisible();
});

// ── 観点H-2: Subscribe/Publish タブ切り替え ───────────────────────────────────

test("can switch between subscribe and publish tabs", async ({ page }) => {
  await createBrokerProfile(page, "Tab Test Broker");

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
}) => {
  await createBrokerProfile(page, "Offline Broker");

  const topicInput = page.getByPlaceholder("Topic (e.g., sensors/#)");
  await expect(topicInput).toBeVisible();

  await topicInput.fill("test/topic");

  // オフライン接続のため Subscribe ボタンは無効
  await expect(
    page.getByRole("button", { name: "Subscribe" }),
  ).toBeDisabled();
});

// ── 観点H-5: QoS の選択（0/1/2） ──────────────────────────────────────────────

test("can select QoS level 0, 1, and 2", async ({ page }) => {
  await createBrokerProfile(page, "QoS Test Broker");

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
