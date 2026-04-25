import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { expect, test } from "@playwright/test";

const VALID_YAML = [
  "openapi: 3.0.0",
  "info:",
  "  title: Test API",
  "  version: 1.0.0",
  "paths: {}",
].join("\n");

const STORAGE_KEY = "wirexa:openapi-files";
const E2E_FILE_NAME = "wirexa-e2e-openapi.yaml";

const getFilePath = () => path.join(os.tmpdir(), E2E_FILE_NAME);

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
});

test.afterEach(() => {
  const filePath = getFilePath();
  if (fs.existsSync(filePath)) {
    fs.unlinkSync(filePath);
  }
});

// ── 観点K-1: OpenAPIファイルの読み込み ───────────────────────────────────────

test("loading an openapi file displays it in the editor", async ({ page }) => {
  const filePath = getFilePath();
  fs.writeFileSync(filePath, VALID_YAML, "utf-8");

  // localStorage にファイル参照を設定（ネイティブファイルピッカーをバイパス）
  await page.evaluate(
    ({ storageKey, fileData }) => {
      localStorage.setItem(storageKey, JSON.stringify([fileData]));
    },
    {
      storageKey: STORAGE_KEY,
      fileData: {
        id: "e2e-openapi-file",
        name: E2E_FILE_NAME,
        path: filePath,
        order: 0,
        lastOpenedAt: new Date().toISOString(),
      },
    },
  );

  await page.reload();
  await page.getByRole("button", { name: "OpenAPI", exact: true }).click();

  // サイドバーにファイルが表示される
  await expect(page.getByText(E2E_FILE_NAME)).toBeVisible({ timeout: 5000 });

  // ファイルをクリックして内容を読み込む（Wails ReadFile バインディングを呼ぶ）
  await page.getByText(E2E_FILE_NAME).first().click();

  // エディタに YAML 内容が表示される
  await expect(page.locator(".cm-editor")).toContainText("Test API", {
    timeout: 5000,
  });

  // プレビューパネルが更新される（有効な spec のため rapi-doc が表示される）
  await expect(page.getByText("No valid OpenAPI spec")).not.toBeVisible({
    timeout: 5000,
  });
  await expect(page.locator("rapi-doc")).toBeAttached({ timeout: 5000 });
});
