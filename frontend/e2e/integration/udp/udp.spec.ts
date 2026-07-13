import * as dgram from "node:dgram";
import type { Page } from "@playwright/test";
import { expect, test } from "../../fixtures/integration";
import {
  reserveUdpPort,
  sendUdpPacket,
  startUdpServer,
} from "../../fixtures/udp-server";

// 実 Go バックエンド越しに実 UDP パケットを送受信する。ターゲット名はテストごとに変えてあるので、
// 1 回の実行の中で名前が衝突しない。保存先も一時 APPDATA へ隔離済みなので後始末は不要。

// ターゲット選択: role="button" 属性を持つ外側 div をフォーカス+Enter で選択する。
// span への click() を使うと CSS :hover でアクションボタンが出現し
// stopPropagation が発火して loadTarget に届かないケースがある。
async function selectTarget(page: Page, name: string) {
  const row = page.locator('[role="button"]').filter({ hasText: name }).first();
  await expect(row).toBeVisible();
  await row.focus();
  await page.keyboard.press("Enter");
  // Listen ボタンが表示されれば選択完了（UdpClient がタブバーを表示している）
  await expect(
    page.getByRole("button", { name: "Listen", exact: true }),
  ).toBeVisible();
}

async function startListen(page: Page, listenPort: number) {
  await page.getByRole("button", { name: "Listen", exact: true }).click();
  await page.getByPlaceholder("12345").fill(String(listenPort));
  await page.getByRole("button", { name: "Start", exact: true }).click();
  await expect(page.getByText(`Listening :${listenPort} (text)`)).toBeVisible();
}

test.beforeEach(async ({ app }) => {
  await app.switchTo("UDP");
});

// ── 観点J-1: 送信フォームへの入力と送信 ──────────────────────────────────────

test("can fill udp send form and click send", async ({ page, app }) => {
  const name = "E2E UDP Send Target";
  const server = await startUdpServer();

  try {
    await app.createUdpTarget(name, "127.0.0.1", server.port);
    await selectTarget(page, name);

    await page.getByPlaceholder("Enter payload...").fill("E2E-UDP-HELLO");
    const sendButton = page
      .getByRole("button", { name: "Send", exact: true })
      .last();
    await sendButton.click();
    await expect(sendButton).toBeEnabled();

    expect(await server.firstMessage).toBe("E2E-UDP-HELLO");
  } finally {
    await server.close();
  }
});

// ── 観点J-2: 受信リッスンの開始・停止 ────────────────────────────────────────

test("can start and stop UDP listening", async ({ page, app }) => {
  const name = "E2E UDP Listen Target";
  const listenPort = await reserveUdpPort();

  await app.createUdpTarget(name, "127.0.0.1", listenPort);
  await selectTarget(page, name);
  await startListen(page, listenPort);

  await expect(
    page.getByRole("button", { name: "Stop", exact: true }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Stop", exact: true }).click();

  await expect(page.getByText(`Listening :${listenPort} (text)`)).toBeHidden();
  await expect(
    page.getByRole("button", { name: "Start", exact: true }),
  ).toBeVisible();
});

// ── 観点J-3: 受信メッセージのログ表示 ────────────────────────────────────────

test("received udp messages appear in the message log", async ({ page, app }) => {
  const name = "E2E UDP Receive Target";
  const listenPort = await reserveUdpPort();

  await app.createUdpTarget(name, "127.0.0.1", listenPort);
  await selectTarget(page, name);
  await startListen(page, listenPort);

  const payload = "E2E-UDP-RECEIVED-TEST";
  await sendUdpPacket(payload, listenPort);

  await expect(page.getByText(payload)).toBeVisible();
  await expect(page.getByText("Received (1)")).toBeVisible();

  await page.getByRole("button", { name: "Stop", exact: true }).click();
});

// ── 観点J-4: メッセージ上限（500件）到達時の挙動 ────────────────────────────

test("udp message log discards oldest when 501st message arrives", async ({
  page,
  app,
}) => {
  const name = "E2E UDP Cap Target";
  const listenPort = await reserveUdpPort();

  await app.createUdpTarget(name, "127.0.0.1", listenPort);
  await selectTarget(page, name);
  await startListen(page, listenPort);

  const client = dgram.createSocket("udp4");
  for (let i = 0; i < 501; i++) {
    await new Promise<void>((resolve, reject) => {
      client.send(`msg-${i}`, listenPort, "127.0.0.1", (err) =>
        err ? reject(err) : resolve(),
      );
    });
  }
  client.close();

  // 501 件送っても 500 件で頭打ちになる (501 件を数えることは無い)
  await expect(page.getByText("Received (500)")).toBeVisible({
    timeout: 20_000,
  });
  await expect(page.getByText("Received (501)")).toBeHidden();

  await page.getByRole("button", { name: "Stop", exact: true }).click();
});

// ── 観点J-5: 新着メッセージへのオートスクロール ──────────────────────────────

test("udp message log newest message is visible after receiving multiple messages", async ({
  page,
  app,
}) => {
  const name = "E2E UDP Scroll Target";
  const listenPort = await reserveUdpPort();

  await app.createUdpTarget(name, "127.0.0.1", listenPort);
  await selectTarget(page, name);
  await startListen(page, listenPort);

  const newest = "E2E-UDP-NEWEST-MESSAGE";
  for (let i = 0; i < 10; i++) {
    await sendUdpPacket(i === 9 ? newest : `older-msg-${i}`, listenPort);
  }

  // 最新メッセージ（リストの先頭）がビューポートに表示される
  await expect(page.getByText(newest)).toBeVisible();
  await expect(page.getByText("Received (10)")).toBeVisible();

  await page.getByRole("button", { name: "Stop", exact: true }).click();
});
