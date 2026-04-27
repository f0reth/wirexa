import { test, expect } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(
    page.getByPlaceholder("https://api.example.com/endpoint"),
  ).toBeVisible();
});

// ── 観点D-1: URL 空欄で送信ボタン無効 ────────────────────────────────────────

test("send button is disabled when URL is empty", async ({ page }) => {
  const urlInput = page.getByPlaceholder("https://api.example.com/endpoint");
  await expect(urlInput).toHaveValue("");
  await expect(page.getByRole("button", { name: "Send" })).toBeDisabled();
});

test("send button is enabled when a valid URL is entered", async ({ page }) => {
  await page
    .getByPlaceholder("https://api.example.com/endpoint")
    .fill("https://api.example.com/users");
  await expect(page.getByRole("button", { name: "Send" })).toBeEnabled();
});

// ── 観点D-2: 不正URLで送信ボタン無効 ────────────────────────────────────────

test("send button is disabled for malformed URL like 'not-a-url'", async ({
  page,
}) => {
  await page
    .getByPlaceholder("https://api.example.com/endpoint")
    .fill("not-a-url");
  await expect(page.getByRole("button", { name: "Send" })).toBeDisabled();
});

test("send button is re-disabled after clearing a valid URL", async ({
  page,
}) => {
  const urlInput = page.getByPlaceholder("https://api.example.com/endpoint");
  await urlInput.fill("https://api.example.com");
  await expect(page.getByRole("button", { name: "Send" })).toBeEnabled();
  await urlInput.clear();
  await expect(page.getByRole("button", { name: "Send" })).toBeDisabled();
});

// ── 観点D-3: ポート番号の範囲バリデーション ──────────────────────────────────

test("udp target port field has min=1 max=65535 constraints", async ({
  page,
}) => {
  await page.getByRole("button", { name: "UDP", exact: true }).click();
  await expect(page.getByText("Targets", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "New Target" }).click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  const portInput = dialog.getByLabel("Port");
  await expect(portInput).toHaveAttribute("type", "number");
  await expect(portInput).toHaveAttribute("min", "1");
  await expect(portInput).toHaveAttribute("max", "65535");

  // 範囲外の値はブラウザのネイティブバリデーションで invalid になる
  await portInput.fill("0");
  const underflowValid = await portInput.evaluate(
    (el: HTMLInputElement) => el.validity.valid,
  );
  expect(underflowValid).toBe(false);

  await portInput.fill("65536");
  const overflowValid = await portInput.evaluate(
    (el: HTMLInputElement) => el.validity.valid,
  );
  expect(overflowValid).toBe(false);

  // 範囲内の値は valid
  await portInput.fill("8080");
  const inRangeValid = await portInput.evaluate(
    (el: HTMLInputElement) => el.validity.valid,
  );
  expect(inRangeValid).toBe(true);
});

// ── 観点D-4: key-valueエディタの行追加・削除 ─────────────────────────────────

test("can add and remove rows in key-value editor (Params tab)", async ({
  page,
}) => {
  // Params タブはデフォルトでアクティブ
  const paramsPanel = page.locator("#tabpanel-params");
  const addButton = paramsPanel.getByRole("button", { name: "Add" });
  const paramInputs = paramsPanel.getByPlaceholder("Parameter");

  await expect(paramInputs).toHaveCount(0);

  await addButton.click();
  await expect(paramInputs).toHaveCount(1);

  await addButton.click();
  await expect(paramInputs).toHaveCount(2);

  await paramsPanel.getByRole("button", { name: "Remove row" }).first().click();
  await expect(paramInputs).toHaveCount(1);

  await paramsPanel.getByRole("button", { name: "Remove row" }).first().click();
  await expect(paramInputs).toHaveCount(0);
});

test("can add and remove rows in key-value editor (Headers tab)", async ({
  page,
}) => {
  await page.getByRole("tab", { name: "Headers" }).click();

  const headersPanel = page.locator("#tabpanel-headers");
  const addButton = headersPanel.getByRole("button", { name: "Add" });
  const headerInputs = headersPanel.getByPlaceholder("Header");

  await expect(headerInputs).toHaveCount(0);

  await addButton.click();
  await expect(headerInputs).toHaveCount(1);

  await headersPanel
    .getByRole("button", { name: "Remove row" })
    .first()
    .click();
  await expect(headerInputs).toHaveCount(0);
});

// ── 観点D-5: key-valueエディタの enabled トグル ───────────────────────────────

test("can toggle enabled state of a key-value editor row", async ({ page }) => {
  const paramsPanel = page.locator("#tabpanel-params");
  await paramsPanel.getByRole("button", { name: "Add" }).click();

  // 新規行のチェックボックスはデフォルトで ON
  const checkbox = paramsPanel.locator('input[type="checkbox"]').first();
  await expect(checkbox).toBeChecked();

  await checkbox.uncheck();
  await expect(checkbox).not.toBeChecked();

  await checkbox.check();
  await expect(checkbox).toBeChecked();
});
