import { type Page, expect, test } from "@playwright/test";

const VALID_YAML = [
  "openapi: 3.0.0",
  "info:",
  "  title: Test API",
  "  version: 1.0.0",
  "paths: {}",
].join("\n");

async function fillEditor(page: Page, content: string) {
  await page.locator(".cm-editor").click();
  await page.evaluate((text) => {
    const el = document.querySelector(".cm-content") as HTMLElement | null;
    if (!el) return;
    el.focus();
    document.execCommand("selectAll");
    document.execCommand("insertText", false, text);
  }, content);
}

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "OpenAPI", exact: true }).click();
  await expect(page.locator(".cm-editor")).toBeVisible();
});

// ── 観点K-2: エディタでの編集とプレビュー反映 ────────────────────────────────────

test("editing openapi yaml in editor updates the preview panel", async ({
  page,
}) => {
  // 初期状態: 空のエディタ → プレビューに "No valid OpenAPI spec" が表示
  await expect(page.getByText("No valid OpenAPI spec")).toBeVisible();

  // 有効な YAML を入力
  await fillEditor(page, VALID_YAML);

  // デバウンス後にパースが完了し、プレビューが更新される
  await expect(page.getByText("No valid OpenAPI spec")).not.toBeVisible({
    timeout: 3000,
  });
  await expect(page.locator("rapi-doc")).toBeAttached({ timeout: 3000 });
});

test("clearing editor content shows no valid spec message in preview", async ({
  page,
}) => {
  // 有効な YAML を入力してプレビューを表示
  await fillEditor(page, VALID_YAML);
  await expect(page.getByText("No valid OpenAPI spec")).not.toBeVisible({
    timeout: 3000,
  });

  // エディタをクリア → プレビューが "No valid OpenAPI spec" に戻る
  await page.locator(".cm-content").click();
  await page.keyboard.press("Control+a");
  await page.keyboard.press("Backspace");

  await expect(page.getByText("No valid OpenAPI spec")).toBeVisible({
    timeout: 3000,
  });
});

// ── 観点K-3: 無効なYAML/JSONを入力したときのエラー表示 ──────────────────────────

test("invalid yaml in editor shows parse error", async ({ page }) => {
  // 不正な YAML（閉じられていないブラケット）を入力
  await fillEditor(page, "key: [unclosed bracket");

  // パースエラーのためプレビューに "No valid OpenAPI spec" が表示される
  await expect(page.getByText("No valid OpenAPI spec")).toBeVisible({
    timeout: 3000,
  });

  // CodeMirror linter がエラー位置をマークする（点エラーまたは範囲エラー）
  await expect(
    page.locator(".cm-lintPoint-error, .cm-lintRange-error"),
  ).toBeAttached({
    timeout: 3000,
  });
});
