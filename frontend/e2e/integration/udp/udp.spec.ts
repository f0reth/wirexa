import * as dgram from "node:dgram";
import * as net from "node:net";
import { type Page, expect, test } from "@playwright/test";

const E2E_TARGET_NAME = "E2E UDP Target";

async function findFreePort(): Promise<number> {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.listen(0, () => {
      const port = (server.address() as net.AddressInfo).port;
      server.close(() => resolve(port));
    });
  });
}

async function sendUdpPacket(
  payload: string,
  port: number,
  host = "127.0.0.1",
): Promise<void> {
  const client = dgram.createSocket("udp4");
  await new Promise<void>((resolve, reject) => {
    client.send(payload, port, host, (err) => {
      client.close();
      if (err) reject(err);
      else resolve();
    });
  });
}

async function navigateToUdp(page: Page) {
  await page.getByRole("button", { name: "UDP", exact: true }).click();
  await expect(page.getByText("Targets", { exact: true })).toBeVisible();
}

// 同名ターゲットを前のテストランから蓄積した場合も含め全件削除する。
// aria-label="Delete target" ボタンは CSS :hover で表示されるため、
// force: true で非表示状態のままクリックし、削除完了後にカウント減少を確認する。
async function cleanupAllTargets(page: Page, name: string) {
  for (;;) {
    const count = await page.getByText(name, { exact: true }).count();
    if (count === 0) break;
    await page
      .locator('[aria-label="Delete target"]')
      .first()
      .dispatchEvent("click");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible({ timeout: 3000 });
    await dialog.getByRole("button", { name: "Delete" }).click();
    // カウントが減るまで待ってから次のループへ（バックエンド refresh 完了を確認）
    await expect(page.getByText(name, { exact: true })).toHaveCount(count - 1, {
      timeout: 5000,
    });
  }
}

async function createTarget(
  page: Page,
  name: string,
  host: string,
  port: number,
) {
  await page.getByRole("button", { name: "New Target" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Name").fill(name);
  await dialog.getByLabel("Host").fill(host);
  await dialog.getByLabel("Port").fill(String(port));
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog).not.toBeVisible();
  // .first() で strict mode 違反を避けつつサイドバーへの追加を待つ
  await expect(page.getByText(name, { exact: true }).first()).toBeVisible({
    timeout: 5000,
  });
}

// ターゲット選択: role="button" 属性を持つ外側 div をフォーカス+Enter で選択する。
// span への click() を使うと CSS :hover でアクションボタンが出現し
// stopPropagation が発火して loadTarget に届かないケースがある。
async function selectTarget(page: Page, name: string) {
  const targetDiv = page
    .locator('[role="button"]')
    .filter({ hasText: name })
    .first();
  await expect(targetDiv).toBeVisible({ timeout: 5000 });
  await targetDiv.focus();
  await page.keyboard.press("Enter");
  // Listen ボタンが表示されれば選択完了（UdpClient がタブバーを表示している）
  await expect(
    page.getByRole("button", { name: "Listen", exact: true }),
  ).toBeVisible({ timeout: 5000 });
}

async function startListen(page: Page, listenPort: number) {
  await page.getByRole("button", { name: "Listen", exact: true }).click();
  await page.getByPlaceholder("12345").fill(String(listenPort));
  await page.getByRole("button", { name: "Start", exact: true }).click();
  await expect(
    page.getByText(`Listening :${listenPort} (text)`),
  ).toBeVisible({ timeout: 5000 });
}

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await navigateToUdp(page);
  // 前回テストランで残った同名ターゲットを全て削除する
  await cleanupAllTargets(page, E2E_TARGET_NAME);
});

test.afterEach(async ({ page }) => {
  try {
    await cleanupAllTargets(page, E2E_TARGET_NAME);
  } catch {
    // ignore cleanup failures
  }
});

// ── 観点J-1: 送信フォームへの入力と送信 ──────────────────────────────────────────

test("can fill udp send form and click send", async ({ page }) => {
  const targetPort = await findFreePort();

  // UDP 受信サーバーを起動
  const udpServer = dgram.createSocket("udp4");
  const receivedPayload = new Promise<string>((resolve) => {
    udpServer.on("message", (msg) => resolve(msg.toString()));
  });
  await new Promise<void>((resolve) =>
    udpServer.bind(targetPort, "127.0.0.1", resolve),
  );

  try {
    await createTarget(page, E2E_TARGET_NAME, "127.0.0.1", targetPort);
    await selectTarget(page, E2E_TARGET_NAME);

    // ペイロードを入力して送信
    await page.getByPlaceholder("Enter payload...").fill("E2E-UDP-HELLO");
    const sendButton = page
      .getByRole("button", { name: "Send", exact: true })
      .last();
    await sendButton.click();

    // 送信完了後ボタンが有効に戻る
    await expect(sendButton).toBeEnabled({ timeout: 5000 });

    // UDP サーバーがメッセージを受信した
    const received = await Promise.race([
      receivedPayload,
      new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error("UDP receive timeout")), 5000),
      ),
    ]);
    expect(received).toBe("E2E-UDP-HELLO");
  } finally {
    await new Promise<void>((resolve) => udpServer.close(() => resolve()));
  }
});

// ── 観点J-2: 受信リッスンの開始・停止 ──────────────────────────────────────────

test("can start and stop UDP listening", async ({ page }) => {
  const listenPort = await findFreePort();

  await createTarget(page, E2E_TARGET_NAME, "127.0.0.1", listenPort);
  await selectTarget(page, E2E_TARGET_NAME);

  // ポートを入力してリッスン開始
  await page.getByRole("button", { name: "Listen", exact: true }).click();
  await page.getByPlaceholder("12345").fill(String(listenPort));
  await page.getByRole("button", { name: "Start", exact: true }).click();

  // リッスン中バッジが表示される
  await expect(
    page.getByText(`Listening :${listenPort} (text)`),
  ).toBeVisible({ timeout: 5000 });
  await expect(
    page.getByRole("button", { name: "Stop", exact: true }),
  ).toBeVisible();

  // リッスン停止
  await page.getByRole("button", { name: "Stop", exact: true }).click();

  // バッジが消え Start ボタンが再表示される
  await expect(
    page.getByText(`Listening :${listenPort} (text)`),
  ).not.toBeVisible({ timeout: 5000 });
  await expect(
    page.getByRole("button", { name: "Start", exact: true }),
  ).toBeVisible();
});

// ── 観点J-3: 受信メッセージのログ表示 ──────────────────────────────────────────

test("received udp messages appear in the message log", async ({ page }) => {
  const listenPort = await findFreePort();

  await createTarget(page, E2E_TARGET_NAME, "127.0.0.1", listenPort);
  await selectTarget(page, E2E_TARGET_NAME);
  await startListen(page, listenPort);

  // Node.js から UDP パケットを送信
  const testPayload = "E2E-UDP-RECEIVED-TEST";
  await sendUdpPacket(testPayload, listenPort);

  // メッセージログに表示される
  await expect(page.getByText(testPayload)).toBeVisible({ timeout: 5000 });
  await expect(page.getByText("Received (1)")).toBeVisible({ timeout: 5000 });

  // リッスン停止（クリーンアップ）
  await page.getByRole("button", { name: "Stop", exact: true }).click();
});

// ── 観点J-4: メッセージ上限（500件）到達時の挙動 ────────────────────────────

test("udp message log discards oldest when 501st message arrives", async ({
  page,
}) => {
  const listenPort = await findFreePort();

  await createTarget(page, E2E_TARGET_NAME, "127.0.0.1", listenPort);
  await selectTarget(page, E2E_TARGET_NAME);
  await startListen(page, listenPort);

  // 501 件のメッセージを送信
  const client = dgram.createSocket("udp4");
  for (let i = 0; i < 501; i++) {
    await new Promise<void>((resolve, reject) => {
      client.send(`msg-${i}`, listenPort, "127.0.0.1", (err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  }
  client.close();

  // メッセージ数が 500 件に制限される（501 にならない）
  await expect(page.getByText("Received (500)")).toBeVisible({
    timeout: 20000,
  });
  await expect(page.getByText("Received (501)")).not.toBeVisible();

  // リッスン停止
  await page.getByRole("button", { name: "Stop", exact: true }).click();
});

// ── 観点J-5: 新着メッセージへのオートスクロール ────────────────────────────────

test("udp message log newest message is visible after receiving multiple messages", async ({
  page,
}) => {
  const listenPort = await findFreePort();

  await createTarget(page, E2E_TARGET_NAME, "127.0.0.1", listenPort);
  await selectTarget(page, E2E_TARGET_NAME);
  await startListen(page, listenPort);

  // 複数メッセージを送信（最新メッセージが先頭に表示される）
  const lastPayload = "E2E-UDP-NEWEST-MESSAGE";
  for (let i = 0; i < 10; i++) {
    await sendUdpPacket(
      i === 9 ? lastPayload : `older-msg-${i}`,
      listenPort,
    );
  }

  // 最新メッセージ（リストの先頭）がビューポートに表示される
  await expect(page.getByText(lastPayload)).toBeVisible({ timeout: 5000 });
  await expect(page.getByText("Received (10)")).toBeVisible({ timeout: 5000 });

  // リッスン停止
  await page.getByRole("button", { name: "Stop", exact: true }).click();
});
