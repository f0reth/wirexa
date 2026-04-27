import * as http from "node:http";
import * as net from "node:net";
import { test, expect } from "@playwright/test";

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
  testServer = http.createServer((_req, res) => {
    setTimeout(() => {
      if (!res.writableEnded) {
        res.writeHead(200, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ message: "ok" }));
      }
    }, 3000);
  });
  await new Promise<void>((resolve) =>
    testServer.listen(testServerPort, "127.0.0.1", resolve),
  );
});

test.afterAll(async () => {
  await new Promise<void>((resolve) => testServer.close(() => resolve()));
});

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(
    page.getByPlaceholder("https://api.example.com/endpoint"),
  ).toBeVisible();
});

// ── 観点G-1: Enter キーで HTTP リクエスト送信 ────────────────────────────────

test("pressing Enter in URL field sends the request", async ({ page }) => {
  const urlInput = page.getByPlaceholder("https://api.example.com/endpoint");
  await urlInput.fill(`http://127.0.0.1:${testServerPort}/`);
  await urlInput.press("Enter");

  // リクエストが開始されキャンセルボタンが表示される
  await expect(
    page.getByRole("button", { name: "Cancel", exact: true }),
  ).toBeVisible({ timeout: 5000 });

  // クリーンアップ: slow レスポンスが返るまで待機
  await expect(
    page.getByRole("button", { name: "Send", exact: true }),
  ).toBeVisible({ timeout: 8000 });
});

