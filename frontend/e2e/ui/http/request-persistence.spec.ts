import type { HttpRequest } from "../../../src/domain/http/types";
import { expect, test } from "../../fixtures/ui";

// 選択中リクエストとフォーム内容の自動保存・復元。フィクスチャは seed で用意するので、
// テストごとに「コレクション作成 → リネーム → リクエスト追加 → リネーム」を UI で再演しない。
test.use({
  seed: {
    collections: [
      { name: "Persistence Collection", items: [{ name: "Saved Request" }] },
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

test.describe("a request inside a folder under the sidebar root", () => {
  test.use({
    seed: {
      rootItems: [{ folder: "Root Folder", items: [{ name: "Deep Request" }] }],
    },
  });

  test("a request inside a root folder is restored after reload", async ({
    page,
    app,
    fake,
  }) => {
    await app.folder("Root Folder").click();
    await app.request(/Deep Request/).click();
    await app.urlInput.fill(SAVED_URL);
    await fake.waitForCalls("UpdateRequest");

    await page.reload();
    await app.switchTo("HTTP");

    await expect(app.urlInput).toHaveValue(SAVED_URL);
  });
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

test("method, headers and body are restored after reload", async ({
  page,
  app,
  fake,
}) => {
  await app.request(/Saved Request/).click();
  await app.urlInput.fill(SAVED_URL);
  await app.selectMethod("POST");

  const headers = await app.openRequestTab("Headers");
  await app.addKeyValue(headers, "Header", "X-Saved", "yes");

  const body = await app.openRequestTab("Body");
  await app.chooseOption(body, "none", "Text");
  await body.getByPlaceholder("Enter body content...").fill("saved body");

  // 入力の途中で自動保存が走ることがあるので、回数ではなく最後の保存内容が揃うまで待つ。
  await expect
    .poll(async () => {
      const calls = await fake.args("UpdateRequest");
      const req = calls[calls.length - 1]?.[1] as HttpRequest | undefined;
      return req && { method: req.method, text: req.body.contents.text };
    })
    .toEqual({ method: "POST", text: "saved body" });

  await page.reload();
  await app.switchTo("HTTP");
  await expect(app.request(/Saved Request/)).toHaveAttribute(
    "aria-current",
    "true",
  );

  await expect(page.getByTestId("method-select")).toContainText("POST");
  const restoredHeaders = await app.openRequestTab("Headers");
  await expect(restoredHeaders.getByPlaceholder("Header")).toHaveValue(
    "X-Saved",
  );
  await expect(restoredHeaders.getByPlaceholder("Value")).toHaveValue("yes");
  const restoredBody = await app.openRequestTab("Body");
  await expect(
    restoredBody.getByRole("button", { name: "text", exact: true }),
  ).toBeVisible();
  await expect(
    restoredBody.getByPlaceholder("Enter body content..."),
  ).toHaveValue("saved body");
});
