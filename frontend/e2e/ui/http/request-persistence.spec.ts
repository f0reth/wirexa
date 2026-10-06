import type { HttpRequest } from "../../../src/domain/http/types";
import { expect, test } from "../../fixtures/ui";

// 選択中リクエストとフォーム内容の自動保存・復元。フィクスチャは seed で用意するので、
// テストごとに「コレクション作成 → リネーム → リクエスト追加 → リネーム」を UI で再演しない。
test.use({
  seed: {
    collections: [
      {
        name: "Persistence Collection",
        items: [{ name: "Saved Request" }, { name: "Other Request" }],
      },
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

// Go から読み直した値は infrastructure/http/client.ts の変換を通る。項目ごとの変換の
// 取りこぼしは、保存したものを読み直して画面に出すここでしか見つからない。
test("params, auth, settings and form rows are restored after reload", async ({
  page,
  app,
  fake,
}) => {
  await app.request(/Saved Request/).click();
  await app.urlInput.fill(SAVED_URL);

  const params = await app.openRequestTab("Params");
  await app.addKeyValue(params, "Parameter", "page", "2");

  const auth = await app.openRequestTab("Auth");
  await app.chooseOption(auth, "none", "Bearer Token");
  await auth.getByPlaceholder("Token").fill("saved-token");

  const settings = await app.openRequestTab("Settings");
  await settings.getByLabel("Timeout (s)").fill("5");
  await settings.getByLabel("Disable redirects").check();

  const body = await app.chooseBodyType("Form Data");
  await body.getByRole("button", { name: "Add" }).click();
  await body.getByPlaceholder("Field").fill("field");
  await body.getByPlaceholder("Value").fill("form value");

  // 入力の途中で自動保存が走ることがあるので、最後に入力した値が保存されるまで待つ。
  await expect
    .poll(async () => {
      const req = (await fake.args("UpdateRequest")).at(-1)?.[1] as
        | HttpRequest
        | undefined;
      return req?.body.formData?.[0]?.value;
    })
    .toBe("form value");

  await page.reload();
  await app.switchTo("HTTP");
  await expect(app.request(/Saved Request/)).toHaveAttribute(
    "aria-current",
    "true",
  );

  const restoredParams = await app.openRequestTab("Params");
  await expect(restoredParams.getByPlaceholder("Parameter")).toHaveValue(
    "page",
  );
  await expect(restoredParams.getByPlaceholder("Value")).toHaveValue("2");

  const restoredAuth = await app.openRequestTab("Auth");
  await expect(
    restoredAuth.getByRole("button", { name: "bearer", exact: true }),
  ).toBeVisible();
  await expect(restoredAuth.getByPlaceholder("Token")).toHaveValue(
    "saved-token",
  );

  const restoredSettings = await app.openRequestTab("Settings");
  await expect(restoredSettings.getByLabel("Timeout (s)")).toHaveValue("5");
  await expect(restoredSettings.getByLabel("Disable redirects")).toBeChecked();
  // 触っていない設定は既定のまま。
  await expect(
    restoredSettings.getByLabel("Verify TLS certificate"),
  ).not.toBeChecked();

  const restoredBody = await app.openRequestTab("Body");
  await expect(
    restoredBody.getByRole("button", { name: "form-data", exact: true }),
  ).toBeVisible();
  await expect(restoredBody.getByPlaceholder("Field")).toHaveValue("field");
  await expect(restoredBody.getByPlaceholder("Value")).toHaveValue(
    "form value",
  );
});

// 保存した値はツリーにも反映する (サイドバーのメソッド表示はツリーの値)。
test("the method shown in the sidebar follows the auto-saved request", async ({
  app,
  fake,
}) => {
  await app.request(/GET Saved Request/).click();
  await app.selectMethod("DELETE");
  await fake.waitForCalls("UpdateRequest");

  await expect(app.request(/DELETE Saved Request/)).toBeVisible();
  await expect(app.request(/GET Saved Request/)).toBeHidden();
});

// 別のリクエストへ切り替えるときに保存し、その値がツリーに入るので、リロードしなくても
// 戻ったときに編集した内容が出る。
test("an edited request keeps its values when coming back from another request", async ({
  app,
  fake,
}) => {
  await app.request(/Saved Request/).click();
  await app.urlInput.fill(SAVED_URL);

  await app.request(/Other Request/).click();
  await expect(app.urlInput).toHaveValue("");
  await expect
    .poll(async () => (await fake.args("UpdateRequest")).at(-1))
    .toMatchObject([expect.anything(), { url: SAVED_URL }]);

  await app.request(/Saved Request/).click();
  await expect(app.urlInput).toHaveValue(SAVED_URL);
});

// ── 観点F: 壊れた・古い保存値 ───────────────────────────────────────────────

test("a stale or malformed saved selection is ignored on startup", async ({
  page,
  app,
  pageErrors,
}) => {
  const ACTIVE_REQUEST_KEY = "wirexa:http:activeRequest";
  const EXPANDED_FOLDERS_KEY = "wirexa:http:expandedFolders";

  for (const saved of [
    // 消えたリクエストを指している。
    JSON.stringify({ requestId: "gone", collectionId: "gone" }),
    // 形が違う。
    JSON.stringify({ requestId: 1 }),
    // JSON として読めない。
    "{not json",
  ]) {
    await page.evaluate(
      ([activeKey, active, expandedKey]) => {
        localStorage.setItem(activeKey, active);
        localStorage.setItem(expandedKey, "[1,");
      },
      [ACTIVE_REQUEST_KEY, saved, EXPANDED_FOLDERS_KEY] as const,
    );

    await page.reload();
    await app.switchTo("HTTP");

    // 既定の開閉 (コレクションは開く) で描かれ、何も選ばれていない。
    await expect(app.request(/Saved Request/)).toBeVisible();
    await expect(app.urlInput).toHaveValue("");
    await expect(page.locator('[aria-current="true"]')).toHaveCount(0);
  }

  // そのあとも選べる。
  const request = app.request(/Saved Request/);
  await request.click();
  await expect(request).toHaveAttribute("aria-current", "true");
  expect(pageErrors).toEqual([]);
});
