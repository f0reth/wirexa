import * as http from "node:http";
import * as net from "node:net";
import { type Page, test, expect } from "@playwright/test";

let testServer: http.Server;
let testServerPort: number;

async function findFreePort(): Promise<number> {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.listen(0, () => {
      const port = (server.address() as net.AddressInfo).port;
      server.close(() => resolve(port));
    });
  });
}

test.beforeAll(async () => {
  testServerPort = await findFreePort();
  testServer = http.createServer((req, res) => {
    if (req.url === "/json") {
      res.writeHead(200, {
        "Content-Type": "application/json",
        "X-Custom-Header": "test-value",
      });
      res.end(JSON.stringify({ message: "hello", status: "ok" }));
    } else {
      res.writeHead(404, { "Content-Type": "text/plain" });
      res.end("Not found");
    }
  });
  await new Promise<void>((resolve) =>
    testServer.listen(testServerPort, "127.0.0.1", resolve),
  );
});

test.afterAll(async () => {
  await new Promise<void>((resolve) => testServer.close(() => resolve()));
});

const getUrlInput = (page: Page) =>
  page.getByPlaceholder("https://api.example.com/endpoint");
const getSendButton = (page: Page) =>
  page.getByRole("button", { name: "Send", exact: true });

const deleteCollection = async (page: Page, name: string) => {
  const btn = page.getByRole("button", { name, exact: true }).first();
  await btn.hover();
  await page.getByRole("button", { name: "Delete collection" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
};

test.afterEach(async ({ page }) => {
  const names = ["HTTP Test Collection"];
  for (const name of names) {
    try {
      const visible = await page
        .getByRole("button", { name, exact: true })
        .first()
        .isVisible({ timeout: 500 });
      if (visible) {
        await deleteCollection(page, name);
      }
    } catch {
      // 削除対象が存在しない場合は無視
    }
  }
});

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections", { exact: true })).toBeVisible();
});

// ── 観点I-2: URL入力とリクエスト送信 ─────────────────────────────────────────

test("entering a URL and clicking send initiates a request", async ({
  page,
}) => {
  await getUrlInput(page).fill(`http://127.0.0.1:${testServerPort}/json`);
  await getSendButton(page).click();

  // レスポンスが表示される
  await expect(page.getByText("200", { exact: true })).toBeVisible({
    timeout: 10000,
  });
  // 送信ボタンが戻る（リクエスト完了）
  await expect(getSendButton(page)).toBeVisible({ timeout: 5000 });
});

// ── 観点I-5: レスポンス表示（ステータス・ボディ・ヘッダー） ───────────────────

test("response viewer shows status code and body", async ({ page }) => {
  await getUrlInput(page).fill(`http://127.0.0.1:${testServerPort}/json`);
  await getSendButton(page).click();

  await expect(page.getByText("200", { exact: true })).toBeVisible({
    timeout: 10000,
  });

  // ボディタブにJSONが表示される
  await expect(page.getByText('"hello"', { exact: true })).toBeVisible({
    timeout: 5000,
  });
});

test("response viewer headers tab shows response headers", async ({ page }) => {
  await getUrlInput(page).fill(`http://127.0.0.1:${testServerPort}/json`);
  await getSendButton(page).click();

  await expect(page.getByText("200", { exact: true })).toBeVisible({
    timeout: 10000,
  });

  // Headers タブに切り替え
  await page.getByRole("tab", { name: "Headers" }).nth(1).click();

  // レスポンスヘッダーが表示される
  await expect(page.getByText("content-type")).toBeVisible({ timeout: 5000 });
  await expect(page.getByText("x-custom-header")).toBeVisible({ timeout: 5000 });
});

// ── 観点I-6: コレクションへの保存・読み込み ──────────────────────────────────

test("saved request URL is restored when request is re-opened", async ({
  page,
}) => {
  // コレクションを作成する
  await page.locator('[aria-label="Add"]').click();
  await page.getByRole("button", { name: "New Collection" }).first().click();
  const collInput = page.getByTestId("rename-input");
  await expect(collInput).toBeVisible();
  await collInput.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type("HTTP Test Collection");
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("button", { name: "HTTP Test Collection", exact: true }).first(),
  ).toBeVisible({ timeout: 10000 });

  // リクエストを追加する
  await page
    .getByRole("button", { name: "HTTP Test Collection", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Add request" }).first().click();
  const reqInput = page.getByTestId("rename-input");
  await expect(reqInput).toBeVisible();
  await reqInput.press("Enter");

  const reqBtn = page
    .getByRole("button", { name: /New Request/ })
    .first();
  await expect(reqBtn).toBeVisible({ timeout: 10000 });
  await reqBtn.click();
  await expect(reqBtn).toHaveAttribute("aria-current", "true");

  // URL を設定する
  const testUrl = `http://127.0.0.1:${testServerPort}/json`;
  await getUrlInput(page).fill(testUrl);
  await expect(getUrlInput(page)).toHaveValue(testUrl);

  // 自動保存を待つ（500ms + バッファ）
  await page.waitForTimeout(1500);

  // 別のリクエストを作成して選択し、元のリクエストを非アクティブにする
  await page
    .getByRole("button", { name: "HTTP Test Collection", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Add request" }).first().click();
  const req2Input = page.getByTestId("rename-input");
  await expect(req2Input).toBeVisible();
  await req2Input.press("Enter");

  const req2Btn = page.getByRole("button", { name: /New Request/ }).nth(1);
  await expect(req2Btn).toBeVisible({ timeout: 10000 });
  await req2Btn.click();

  // URL 入力が空になることを確認（新しいリクエスト）
  await expect(getUrlInput(page)).toHaveValue("");

  // 元のリクエストを再度クリック
  await reqBtn.click();
  await expect(reqBtn).toHaveAttribute("aria-current", "true");

  // URL が保持されていることを確認
  await expect(getUrlInput(page)).toHaveValue(testUrl, { timeout: 5000 });
});

// ── 観点I-8: レスポンスのクリップボードコピー ────────────────────────────────

test("copy button writes response body to clipboard", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);

  await getUrlInput(page).fill(`http://127.0.0.1:${testServerPort}/json`);
  await getSendButton(page).click();

  await expect(page.getByText("200", { exact: true })).toBeVisible({
    timeout: 10000,
  });

  // Copy body ボタンが表示される（エラーなし・切り捨てなし）
  const copyBtn = page.getByRole("button", { name: "Copy body" });
  await expect(copyBtn).toBeVisible({ timeout: 5000 });
  await copyBtn.click();

  // クリップボードに JSON ボディが書き込まれる
  const clipboardText = await page.evaluate(() =>
    navigator.clipboard.readText(),
  );
  expect(clipboardText).toContain('"message"');
  expect(clipboardText).toContain('"hello"');
});
