import { expect, test } from "../../fixtures/ui";

// 選択中リクエストとフォーム内容の自動保存・復元。フィクスチャは seed で用意するので、
// テストごとに「コレクション作成 → リネーム → リクエスト追加 → リネーム」を UI で再演しない。
test.use({
  seed: {
    collections: [
      { name: "Persistence Collection", requests: [{ name: "Saved Request" }] },
    ],
  },
});

const SAVED_URL = "http://localhost:9999/persistence-test";

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

// ── 観点F-2: 選択中のリクエストがページリロード後に自動復元される ─────────────

test("selected request is restored after reload", async ({
  page,
  app,
  fake,
}) => {
  const request = app.request(/Saved Request/);
  await request.click();
  await expect(request).toHaveAttribute("aria-current", "true");

  await app.urlInput.fill(SAVED_URL);
  // デバウンスされた自動保存がバックエンドに届くまで待つ (固定 sleep の代わり)。
  await fake.waitForCalls("UpdateRequest");

  await page.reload();
  await app.switchTo("HTTP");

  // コレクションが展開され、リクエストが選択済みで復元されている
  await expect(app.request(/Saved Request/)).toHaveAttribute(
    "aria-current",
    "true",
  );
  await expect(app.urlInput).toHaveValue(SAVED_URL);
});

// ── 観点F-3: フォームの入力内容が自動保存されリロード後に復元される ──────────

test("http request form values are auto-saved and restored after reload", async ({
  page,
  app,
  fake,
}) => {
  await app.request(/Saved Request/).click();
  await app.urlInput.fill(SAVED_URL);
  await fake.waitForCalls("UpdateRequest");

  await page.reload();
  await app.switchTo("HTTP");

  // 同じリクエストを明示的に開き直しても、保存された URL が入っている
  await app.request(/Saved Request/).click();
  await expect(app.urlInput).toHaveValue(SAVED_URL);
});
