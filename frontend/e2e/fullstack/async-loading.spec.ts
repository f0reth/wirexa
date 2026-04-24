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
    if (req.url === "/fast") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ message: "ok" }));
    } else if (req.url === "/slow") {
      // 3秒後に応答する（ローディング状態テスト用）
      setTimeout(() => {
        if (!res.writableEnded) {
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ message: "slow response" }));
        }
      }, 3000);
    } else {
      res.writeHead(404);
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

const deleteCollection = async (page: Page, name: string) => {
  const btn = page.getByRole("button", { name, exact: true }).first();
  await btn.hover();
  await page.getByRole("button", { name: "Delete collection" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
};

test.afterEach(async ({ page }) => {
  try {
    const visible = await page
      .getByRole("button", { name: "New Collection", exact: true })
      .first()
      .isVisible({ timeout: 500 });
    if (visible) {
      await deleteCollection(page, "New Collection");
    }
  } catch {
    // コレクションが存在しない場合は無視
  }
});

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections")).toBeVisible();
});

const getUrlInput = (page: Page) =>
  page.getByPlaceholder("https://api.example.com/endpoint");
const getSendButton = (page: Page) =>
  page.getByRole("button", { name: "Send", exact: true });
const getCancelButton = (page: Page) =>
  page.getByRole("button", { name: "Cancel", exact: true });

// ── 観点E-1: HTTPリクエスト送信中にキャンセルボタンが表示される ──────────────

test("cancel button appears while request is in progress", async ({ page }) => {
  await getUrlInput(page).fill(`http://127.0.0.1:${testServerPort}/slow`);
  await getSendButton(page).click();

  await expect(getCancelButton(page)).toBeVisible({ timeout: 5000 });

  // slow レスポンスが返るまで待機してクリーンアップ
  await expect(getSendButton(page)).toBeVisible({ timeout: 8000 });
});

// ── 観点E-2: レスポンス受信後にレスポンスビューワーに内容が表示される ─────────

test("response viewer shows status and body after successful request", async ({
  page,
}) => {
  await getUrlInput(page).fill(`http://127.0.0.1:${testServerPort}/fast`);
  await getSendButton(page).click();

  // ローディング中
  await expect(page.getByText("Sending request...")).toBeVisible({
    timeout: 5000,
  });

  // 200 ステータスコードが表示される
  await expect(page.getByText("200")).toBeVisible({ timeout: 10000 });

  // レスポンスボディに "ok" が含まれる
  await expect(page.getByText(/\"ok\"|ok/)).toBeVisible({ timeout: 5000 });
});

// ── 観点E-3: 送信中は送信ボタンがキャンセルボタンに置き換えられる ───────────

test("send button is replaced by cancel during request", async ({ page }) => {
  await getUrlInput(page).fill(`http://127.0.0.1:${testServerPort}/slow`);

  await expect(getSendButton(page)).toBeVisible();
  await expect(getCancelButton(page)).not.toBeVisible();

  await getSendButton(page).click();

  await expect(getCancelButton(page)).toBeVisible({ timeout: 3000 });
  await expect(getSendButton(page)).not.toBeVisible();

  // slow レスポンスが返るまで待機してクリーンアップ
  await expect(getSendButton(page)).toBeVisible({ timeout: 8000 });
});

// ── 観点E-4: キャンセル操作でリクエストが中断される ────────────────────────

test("clicking cancel aborts the in-progress request", async ({ page }) => {
  // コレクションとリクエストを作成して activeRequestId を設定する
  await page.locator('[aria-label="Add"]').click();
  await page.getByRole("button", { name: "New Collection" }).first().click();
  const collInput = page.getByTestId("rename-input");
  await expect(collInput).toBeVisible();
  await collInput.press("Escape");
  await expect(
    page.getByRole("button", { name: "New Collection", exact: true }).first(),
  ).toBeVisible();

  await page
    .getByRole("button", { name: "New Collection", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Add request" }).first().click();
  const reqInput = page.getByTestId("rename-input");
  await expect(reqInput).toBeVisible();
  await reqInput.press("Enter");

  const reqBtn = page.getByRole("button", { name: /New Request/ }).first();
  await expect(reqBtn).toBeVisible({ timeout: 10000 });
  await reqBtn.click();
  await expect(reqBtn).toHaveAttribute("aria-current", "true");

  // slow エンドポイントへリクエスト送信
  await getUrlInput(page).fill(`http://127.0.0.1:${testServerPort}/slow`);
  await getSendButton(page).click();

  // ローディング状態を確認
  await expect(getCancelButton(page)).toBeVisible({ timeout: 3000 });
  await expect(page.getByText("Sending request...")).toBeVisible();

  // キャンセルをクリック
  await getCancelButton(page).click();

  // 送信ボタンが即座に戻る（リクエストが中断された）
  await expect(getSendButton(page)).toBeVisible({ timeout: 5000 });
  await expect(page.getByText("Sending request...")).not.toBeVisible();
});

// ── 観点E-5: エラー時にエラーメッセージが表示される ─────────────────────────

test("connection error displays error message in response viewer", async ({
  page,
}) => {
  // 到達不能なポートへリクエスト送信
  await getUrlInput(page).fill("http://127.0.0.1:1/test");
  await getSendButton(page).click();

  // 送信ボタンが戻る（リクエスト完了）
  await expect(getSendButton(page)).toBeVisible({ timeout: 15000 });

  // プレースホルダーが非表示であること（何らかの結果が表示されている）
  await expect(
    page.getByText("Send a request to see the response"),
  ).not.toBeVisible();

  // エラーメッセージが表示されていること
  await expect(
    page.getByText(/refused|connection|connect|failed|error/i),
  ).toBeVisible({ timeout: 5000 });
});
