import { expect, test } from "../../fixtures/integration";

const VALID_YAML = [
  "openapi: 3.0.0",
  "info:",
  "  title: Test API",
  "  version: 1.0.0",
  "paths: {}",
].join("\n");

test.beforeEach(async ({ page }) => {
  await page.getByRole("button", { name: "OpenAPI", exact: true }).click();
});

// ── 観点K-1: 無題ドキュメントへのペーストでプレビュー表示 ─────────────────────────
// capability ベースのファイルアクセスへ移行後、ネイティブダイアログを介さずに
// 開ける経路（ペースト/無題文書）を実バックエンドで検証する。

test("pasting a spec into a new document renders the preview", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);

  // 新規（無題）ドキュメントを作成
  await page.getByRole("button", { name: "New (paste a spec)" }).click();

  // エディタにスペックをペースト
  await page.evaluate(
    (text) => navigator.clipboard.writeText(text),
    VALID_YAML,
  );
  await page.locator(".cm-content").click();
  await page.keyboard.press("ControlOrMeta+V");

  // エディタに YAML 内容が表示される
  await expect(page.locator(".cm-editor")).toContainText("Test API", {
    timeout: 5000,
  });

  // 有効な spec のため Swagger UI プレビューが表示される
  await expect(page.getByText("No valid OpenAPI spec")).not.toBeVisible({
    timeout: 5000,
  });
  await expect(page.getByTestId("openapi-preview")).toBeAttached({
    timeout: 5000,
  });
});

// ── 観点K-2: 未許可パスの読み込み拒否（capability の中核）─────────────────────────
// ダイアログを介さず JS から直接渡された任意パスは、許可リストに無いため拒否される。

test("ReadFile denies a path that was never granted via a dialog", async ({
  page,
}) => {
  const denied = await page.evaluate(async () => {
    try {
      // biome-ignore lint/suspicious/noExplicitAny: Wails runtime binding is injected at runtime
      await (window as any).go.adapters.OpenAPIHandler.ReadFile(
        "C:\\Windows\\win.ini",
      );
      return false;
    } catch {
      return true;
    }
  });
  expect(denied).toBe(true);
});
