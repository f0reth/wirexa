import { expect, test } from "../../fixtures/ui";

// 偽バックエンドの応答遅延で「送信中」を作る。実サーバーの /slow を 3 秒待っていた頃と違い、
// テストは必要な状態を観測した時点で終われる (完了待ちのクリーンアップが要らない)。
const SLOW = 30_000;

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

// ── 観点E-1 / E-3: 送信中はキャンセルボタンが送信ボタンを置き換える ────────────

test.describe("while a request is in progress", () => {
  test.use({ seed: { httpResponseDelayMs: SLOW } });

  test("send button is replaced by cancel", async ({ app }) => {
    await app.urlInput.fill("http://127.0.0.1:9999/slow");

    await expect(app.sendButton).toBeVisible();
    await expect(app.cancelButton).toBeHidden();

    await app.sendButton.click();

    await expect(app.cancelButton).toBeVisible();
    await expect(app.sendButton).toBeHidden();
  });

  // ── 観点E-4: キャンセル操作でリクエストが中断される ──────────────────────────

  test("clicking cancel aborts the in-progress request", async ({
    page,
    app,
  }) => {
    await app.urlInput.fill("http://127.0.0.1:9999/slow");
    await app.sendButton.click();

    await expect(app.cancelButton).toBeVisible();
    await expect(page.getByText("Sending request...")).toBeVisible();

    await app.cancelButton.click();

    await expect(app.sendButton).toBeVisible();
    await expect(page.getByText("Sending request...")).toBeHidden();
  });
});

// ── 観点E-2: レスポンス受信後にレスポンスビューワーに内容が表示される ─────────

test("response viewer shows status and body after successful request", async ({
  page,
  app,
}) => {
  await app.urlInput.fill("http://127.0.0.1:9999/fast");
  await app.sendButton.click();

  await expect(page.getByText("200", { exact: true })).toBeVisible();
  await expect(page.getByText('"ok"', { exact: true })).toBeVisible();
});

// ── 観点E-5: エラー時にエラーメッセージが表示される ─────────────────────────

test.describe("when the request fails", () => {
  test.use({ seed: { httpError: "dial tcp 127.0.0.1:1: connection refused" } });

  test("connection error displays error message in response viewer", async ({
    page,
    app,
  }) => {
    await app.urlInput.fill("http://127.0.0.1:1/test");
    await app.sendButton.click();

    await expect(app.sendButton).toBeVisible();
    await expect(
      page.getByText("Send a request to see the response"),
    ).toBeHidden();
    await expect(page.getByTestId("response-error")).toBeVisible();
  });
});
