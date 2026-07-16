import type { Page } from "@playwright/test";
import { expect, test } from "../../fixtures/ui";

test.beforeEach(async ({ page }) => {
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(
    page.getByPlaceholder("https://api.example.com/endpoint"),
  ).toBeVisible();
});

// ── 観点I-1: HTTPメソッドの選択 ──────────────────────────────────────────────

test("can select different HTTP methods from dropdown", async ({ page }) => {
  const methodSelect = page.getByTestId("method-select");
  const trigger = methodSelect.getByRole("button").first();

  // 初期状態: GET
  await expect(trigger).toContainText("GET");

  // POST を選択
  await trigger.click();
  await methodSelect.getByRole("button", { name: "POST" }).click();
  await expect(trigger).toContainText("POST");

  // DELETE を選択
  await trigger.click();
  await methodSelect.getByRole("button", { name: "DELETE" }).click();
  await expect(trigger).toContainText("DELETE");

  // PUT を選択
  await trigger.click();
  await methodSelect.getByRole("button", { name: "PUT" }).click();
  await expect(trigger).toContainText("PUT");

  // GET に戻す
  await trigger.click();
  await methodSelect.getByRole("button", { name: "GET" }).click();
  await expect(trigger).toContainText("GET");
});

// ── 観点I-3: ボディタイプ切り替え ────────────────────────────────────────────

test("switching body type to JSON shows CodeMirror editor", async ({ page }) => {
  await page.getByRole("tab", { name: "Body" }).click();

  const bodyPanel = page.locator("#tabpanel-body");
  const bodyTypeTrigger = bodyPanel.getByRole("button").first();

  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "JSON" }).click();

  await expect(bodyPanel.locator(".cm-editor")).toBeVisible();
});

test("switching body type to Text shows textarea", async ({ page }) => {
  await page.getByRole("tab", { name: "Body" }).click();

  const bodyPanel = page.locator("#tabpanel-body");
  const bodyTypeTrigger = bodyPanel.getByRole("button").first();

  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "Text" }).click();

  await expect(bodyPanel.getByRole("textbox")).toBeVisible();
});

test("switching body type to Form Data shows key-value editor", async ({
  page,
}) => {
  await page.getByRole("tab", { name: "Body" }).click();

  const bodyPanel = page.locator("#tabpanel-body");
  const bodyTypeTrigger = bodyPanel.getByRole("button").first();

  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "Form Data" }).click();

  await expect(bodyPanel.getByRole("button", { name: "Add" })).toBeVisible();
});

// ボディタイプを選択して Body タブの key-value エディタを開く。
async function openFormBody(page: Page, bodyTypeLabel: string) {
  await page.getByRole("tab", { name: "Body" }).click();
  const bodyPanel = page.locator("#tabpanel-body");
  await bodyPanel.getByRole("button").first().click();
  await bodyPanel.getByRole("button", { name: bodyTypeLabel }).click();
  return bodyPanel;
}

// 行は文字列へ直列化して導出し直していたため、空キー行が直列化で捨てられ
// Add が無反応になっていた。行が実体の state であることを担保する回帰テスト。
for (const bodyTypeLabel of ["Form Data", "Form URL Encoded"]) {
  test(`can add rows to ${bodyTypeLabel} body`, async ({ page }) => {
    const bodyPanel = await openFormBody(page, bodyTypeLabel);
    const addButton = bodyPanel.getByRole("button", { name: "Add" });

    await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(0);

    await addButton.click();
    await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(1);

    await addButton.click();
    await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(2);

    const fields = bodyPanel.getByPlaceholder("Field");
    await fields.nth(0).fill("username");
    await fields.nth(1).fill("password");
    await expect(fields.nth(0)).toHaveValue("username");
    await expect(fields.nth(1)).toHaveValue("password");
  });
}

// チェックボックスは行の有効/無効を切り替えるものであり、行を削除してはいけない。
test("unchecking a Form Data row disables it without removing it", async ({
  page,
}) => {
  const bodyPanel = await openFormBody(page, "Form Data");

  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("token");

  const checkbox = bodyPanel.getByRole("checkbox");
  await expect(checkbox).toBeChecked();
  await checkbox.uncheck();

  await expect(checkbox).not.toBeChecked();
  await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(1);
  await expect(bodyPanel.getByPlaceholder("Field")).toHaveValue("token");
});

// キーを空にしても値ごと行が消えてはいけない。
test("clearing the key of a Form Data row keeps the row", async ({ page }) => {
  const bodyPanel = await openFormBody(page, "Form Data");

  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("k");
  await bodyPanel.getByPlaceholder("Value").fill("kept");

  await bodyPanel.getByPlaceholder("Field").fill("");

  await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(1);
  await expect(bodyPanel.getByPlaceholder("Value")).toHaveValue("kept");
});

// form-data と form-urlencoded は独立した行を持ち、切り替えで互いを壊さない。
test("Form Data and Form URL Encoded keep independent rows", async ({
  page,
}) => {
  const bodyPanel = await openFormBody(page, "Form Data");
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("from-form-data");

  await openFormBody(page, "Form URL Encoded");
  await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(0);
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("from-urlencoded");

  await openFormBody(page, "Form Data");
  await expect(bodyPanel.getByPlaceholder("Field")).toHaveValue(
    "from-form-data",
  );
});

test("switching body type back to none hides the editor", async ({ page }) => {
  await page.getByRole("tab", { name: "Body" }).click();

  const bodyPanel = page.locator("#tabpanel-body");
  const bodyTypeTrigger = bodyPanel.getByRole("button").first();

  // JSON を選択してエディタを表示
  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "JSON" }).click();
  await expect(bodyPanel.locator(".cm-editor")).toBeVisible();

  // none に戻す
  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "None" }).click();
  await expect(bodyPanel.locator(".cm-editor")).not.toBeVisible();
});

// ── 観点I-4: 認証設定の切り替え ──────────────────────────────────────────────

test("selecting Basic auth shows username and password fields", async ({
  page,
}) => {
  await page.getByRole("tab", { name: "Auth" }).click();

  const authPanel = page.locator("#tabpanel-auth");
  const authTypeTrigger = authPanel.getByRole("button").first();

  await authTypeTrigger.click();
  await authPanel.getByRole("button", { name: "Basic Auth" }).click();

  await expect(authPanel.getByPlaceholder("Username")).toBeVisible();
  await expect(authPanel.getByPlaceholder("Password")).toBeVisible();
});

test("selecting Bearer auth shows token field", async ({ page }) => {
  await page.getByRole("tab", { name: "Auth" }).click();

  const authPanel = page.locator("#tabpanel-auth");
  const authTypeTrigger = authPanel.getByRole("button").first();

  await authTypeTrigger.click();
  await authPanel.getByRole("button", { name: "Bearer Token" }).click();

  await expect(authPanel.getByPlaceholder("Token")).toBeVisible();
});

test("switching back to no auth hides credential fields", async ({ page }) => {
  await page.getByRole("tab", { name: "Auth" }).click();

  const authPanel = page.locator("#tabpanel-auth");
  const authTypeTrigger = authPanel.getByRole("button").first();

  // Basic Auth を選択
  await authTypeTrigger.click();
  await authPanel.getByRole("button", { name: "Basic Auth" }).click();
  await expect(authPanel.getByPlaceholder("Username")).toBeVisible();

  // None に戻す
  await authTypeTrigger.click();
  await authPanel.getByRole("button", { name: "None" }).click();
  await expect(authPanel.getByPlaceholder("Username")).not.toBeVisible();
  await expect(authPanel.getByPlaceholder("Password")).not.toBeVisible();
});

// ── 観点I-7: プロキシ・タイムアウト設定の入力UI ──────────────────────────────

test("can configure timeout in settings tab", async ({ page }) => {
  await page.getByRole("tab", { name: "Settings" }).click();

  const settingsPanel = page.locator("#tabpanel-settings");
  const timeoutInput = settingsPanel.getByLabel("Timeout (s)");

  await expect(timeoutInput).toBeVisible();
  await timeoutInput.fill("60");
  await expect(timeoutInput).toHaveValue("60");
});

test("selecting custom proxy mode shows proxy URL field", async ({ page }) => {
  await page.getByRole("tab", { name: "Settings" }).click();

  const settingsPanel = page.locator("#tabpanel-settings");

  // proxy ドロップダウンを開いて Custom を選択
  const proxyTrigger = settingsPanel.getByRole("button").first();
  await proxyTrigger.click();
  await settingsPanel.getByRole("button", { name: "Custom" }).click();

  await expect(settingsPanel.getByLabel("Proxy URL")).toBeVisible();
  await settingsPanel
    .getByLabel("Proxy URL")
    .fill("http://proxy.example.com:8080");
  await expect(settingsPanel.getByLabel("Proxy URL")).toHaveValue(
    "http://proxy.example.com:8080",
  );
});

test("settings tab shows TLS and redirect checkboxes", async ({ page }) => {
  await page.getByRole("tab", { name: "Settings" }).click();

  const settingsPanel = page.locator("#tabpanel-settings");

  await expect(
    settingsPanel.getByLabel("Verify TLS certificate"),
  ).toBeVisible();
  await expect(settingsPanel.getByLabel("Disable redirects")).toBeVisible();
});

// ── 観点I-9: リクエストヘッダーの追加 ────────────────────────────────────────

test("can add custom headers to HTTP request", async ({ page }) => {
  await page.getByRole("tab", { name: "Headers" }).click();

  const headersPanel = page.locator("#tabpanel-headers");

  // 行を追加
  await headersPanel.getByRole("button", { name: "Add" }).click();

  // キーと値を入力
  await headersPanel.getByPlaceholder("Header").fill("Content-Type");
  await headersPanel.getByPlaceholder("Value").fill("application/json");

  await expect(headersPanel.getByPlaceholder("Header")).toHaveValue(
    "Content-Type",
  );
  await expect(headersPanel.getByPlaceholder("Value")).toHaveValue(
    "application/json",
  );
});

test("can add multiple headers to HTTP request", async ({ page }) => {
  await page.getByRole("tab", { name: "Headers" }).click();

  const headersPanel = page.locator("#tabpanel-headers");
  const addButton = headersPanel.getByRole("button", { name: "Add" });

  // 2 行追加
  await addButton.click();
  await addButton.click();

  const headerInputs = headersPanel.getByPlaceholder("Header");
  await expect(headerInputs).toHaveCount(2);

  await headerInputs.nth(0).fill("Accept");
  await headerInputs.nth(1).fill("Authorization");

  await expect(headerInputs.nth(0)).toHaveValue("Accept");
  await expect(headerInputs.nth(1)).toHaveValue("Authorization");
});
