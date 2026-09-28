import { expect, test } from "../../fixtures/ui";

const VALID_YAML = [
  "openapi: 3.0.0",
  "info:",
  "  title: Test API",
  "  version: 1.0.0",
  "paths: {}",
].join("\n");

test.beforeEach(async ({ page, app }) => {
  await page.getByRole("button", { name: "OpenAPI", exact: true }).click();
  await expect(app.editor().root).toBeVisible();
});

// ── 観点K-2: エディタでの編集とプレビュー反映 ────────────────────────────────────

test("editing openapi yaml in editor updates the preview panel", async ({
  page,
  app,
}) => {
  // 初期状態: 空のエディタ → プレビューに "No valid OpenAPI spec" が表示
  await expect(page.getByText("No valid OpenAPI spec")).toBeVisible();

  // 有効な YAML を入力
  await app.fillOpenApiEditor(VALID_YAML);

  // デバウンス後にパースが完了し、プレビューが更新される
  await expect(page.getByText("No valid OpenAPI spec")).not.toBeVisible();
  await expect(page.getByTestId("openapi-preview")).toBeAttached();
});

test("clearing editor content shows no valid spec message in preview", async ({
  page,
  app,
}) => {
  // 有効な YAML を入力してプレビューを表示
  await app.fillOpenApiEditor(VALID_YAML);
  await expect(page.getByText("No valid OpenAPI spec")).not.toBeVisible();

  // エディタをクリア → プレビューが "No valid OpenAPI spec" に戻る
  await app.editor().content.click();
  await page.keyboard.press("ControlOrMeta+A");
  await page.keyboard.press("Backspace");

  await expect(page.getByText("No valid OpenAPI spec")).toBeVisible();
});

// ── 観点K-3: 無効なYAML/JSONを入力したときのエラー表示 ──────────────────────────

test("invalid yaml in editor shows parse error", async ({ page, app }) => {
  // 不正な YAML（閉じられていないブラケット）を入力
  await app.fillOpenApiEditor("key: [unclosed bracket");

  // パースエラーのためプレビューに "No valid OpenAPI spec" が表示される
  await expect(page.getByText("No valid OpenAPI spec")).toBeVisible();

  // CodeMirror linter がエラー位置をマークする（点エラーまたは範囲エラー）
  await expect(app.editor().lintErrors).toBeAttached();
});
