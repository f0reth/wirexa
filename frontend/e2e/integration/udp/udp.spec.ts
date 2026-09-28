import * as dgram from "node:dgram";
import type { Page } from "@playwright/test";
import {
  expect,
  readStoredEntities,
  stopUdpListeners,
  test,
  type WailsGo,
} from "../../fixtures/integration";
import {
  occupyUdpPort,
  reserveUdpPort,
  sendUdpPacket,
  startUdpServer,
} from "../../fixtures/udp-server";

// 実 Go バックエンド越しに実 UDP パケットを送受信する。ターゲット名はテストごとに変えてあるので、
// 1 回の実行の中で名前が衝突しない。保存先も一時 APPDATA へ隔離済みなのでターゲットの後始末は不要。
// リスナーだけは Go 側に残り、次のテストの読み込み時に復元されて Start を無効にするので、
// afterEach でバックエンドから直接止める。

async function startListen(page: Page, listenPort: number) {
  await page.getByRole("tab", { name: "Listen", exact: true }).click();
  await page.getByPlaceholder("12345").fill(String(listenPort));
  await page.getByRole("button", { name: "Start", exact: true }).click();
  await expect(page.getByText(`Listening :${listenPort} (text)`)).toBeVisible();
}

test.beforeEach(async ({ app }) => {
  await app.switchTo("UDP");
});

test.afterEach(async ({ page }) => {
  await stopUdpListeners(page);
});

// ── 観点J-1: 送信フォームへの入力と送信 ──────────────────────────────────────

test("can fill udp send form and click send", async ({ page, app }) => {
  const name = "E2E UDP Send Target";
  const server = await startUdpServer();

  try {
    await app.createUdpTarget(name, "127.0.0.1", server.port);
    await app.selectUdpTarget(name);

    await page.getByPlaceholder("Enter payload...").fill("E2E-UDP-HELLO");
    const sendButton = app.udpSendButton;
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
  await app.selectUdpTarget(name);
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
  await app.selectUdpTarget(name);
  await startListen(page, listenPort);

  const payload = "E2E-UDP-RECEIVED-TEST";
  await sendUdpPacket(payload, listenPort);

  await expect(page.getByText(payload)).toBeVisible();
  await expect(page.getByText("Received (1)")).toBeVisible();
});

// ── 観点J-4: メッセージ上限（500件）到達時の挙動 ────────────────────────────
// 上限と並び順は application/udp/receive.ts が決める。実ソケットから届いた順に積まれ、
// 501 件目で最古の msg-0 が捨てられることを画面で確かめる。

test("udp log shows newest first and drops msg-0 at the cap", async ({
  page,
  app,
}) => {
  const name = "E2E UDP Cap Target";
  const listenPort = await reserveUdpPort();

  await app.createUdpTarget(name, "127.0.0.1", listenPort);
  await app.selectUdpTarget(name);
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

  // 最古の 1 件だけが捨てられ、最新の msg-500 が先頭 (最古の msg-1 より上) に並ぶ。
  await expect(page.getByText("msg-0", { exact: true })).toBeHidden();
  const newest = page.getByText("msg-500", { exact: true });
  const oldest = page.getByText("msg-1", { exact: true });
  await expect(newest).toBeAttached();
  await expect(oldest).toBeAttached();
  const newestBox = await newest.boundingBox();
  const oldestBox = await oldest.boundingBox();
  expect(newestBox?.y).toBeLessThan(oldestBox?.y ?? 0);
});

// ── 観点M: バックエンドのエラー表示 ──────────────────────────────────────────

test("backend validation error for udp target is shown as a toast", async ({
  page,
  app,
}) => {
  const name = "E2E UDP Invalid Port";
  await page.getByRole("button", { name: "New Target" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill(name);
  await dialog.getByLabel("Host").fill("127.0.0.1");
  await dialog.getByLabel("Port").fill("0");
  await dialog.getByRole("button", { name: "Save" }).click();

  // ダイアログは検証せず、Go の UDPTarget.Validate が拒否した文言がそのままトーストに出る。
  const toast = page
    .getByRole("alert")
    .filter({ hasText: "Failed to save target" });
  await expect(toast).toContainText("invalid port: must be 1-65535");
  await expect(dialog).toBeVisible();
  await expect(app.udpTarget(name)).toBeHidden();
  expect(
    readStoredEntities<{ name: string }>("udp-targets").map((e) => e.data.name),
  ).not.toContain(name);
});

// ── 観点J: リスン失敗 ────────────────────────────────────────────────────────

test("listening on a port already in use shows an error", async ({
  page,
  app,
}) => {
  const name = "E2E UDP Port In Use Target";
  const occupied = await occupyUdpPort();

  try {
    await app.createUdpTarget(name, "127.0.0.1", occupied.port);
    await app.selectUdpTarget(name);
    await page.getByRole("tab", { name: "Listen", exact: true }).click();
    await page.getByPlaceholder("12345").fill(String(occupied.port));
    const start = page.getByRole("button", { name: "Start", exact: true });
    await start.click();

    // bind の失敗は OS ごとに文言が違うので、Go 側で付ける前置きまでを見る。
    const message = `failed to listen on port ${occupied.port}`;
    const toast = page
      .getByRole("alert")
      .filter({ hasText: "Failed to start listening" });
    await expect(toast).toContainText(message);
    // 同じ文言がフォーム内にも残る。
    await toast.getByRole("button", { name: "Dismiss" }).click();
    await expect(toast).toBeHidden();
    await expect(page.getByText(message)).toBeVisible();

    await expect(page.getByText(/^Listening :/)).toBeHidden();
    await expect(start).toBeEnabled();
    const sessions = await page.evaluate(() =>
      (window as unknown as { go: WailsGo }).go.adapters.UDPHandler.GetListeners(),
    );
    expect(sessions).toEqual([]);
  } finally {
    await occupied.close();
  }
});

// ── 観点J-5: 新着メッセージへのオートスクロール ──────────────────────────────

test("udp message log newest message is visible after receiving multiple messages", async ({
  page,
  app,
}) => {
  const name = "E2E UDP Scroll Target";
  const listenPort = await reserveUdpPort();

  await app.createUdpTarget(name, "127.0.0.1", listenPort);
  await app.selectUdpTarget(name);
  await startListen(page, listenPort);

  const newest = "E2E-UDP-NEWEST-MESSAGE";
  for (let i = 0; i < 10; i++) {
    await sendUdpPacket(i === 9 ? newest : `older-msg-${i}`, listenPort);
  }

  // 最新メッセージ（リストの先頭）がビューポートに表示される
  await expect(page.getByText(newest)).toBeVisible();
  await expect(page.getByText("Received (10)")).toBeVisible();
});
