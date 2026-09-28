import type { Page } from "@playwright/test";
import type { UdpReceivedMessage } from "../../../src/domain/udp/types";
import { expect, test, WailsEvents } from "../../fixtures/ui";

// UDP のリスン開始・停止と受信ログ。セッションは偽バックエンドが持ち、Go と同じくページの
// リロードを跨いで残る (UI は起動時に GetListeners で復元する)。受信メッセージは Go の
// リスナーが発火する udp:message を偽バックエンドから流して模す。

const TARGET = {
  id: "target-device",
  name: "Device",
  host: "127.0.0.1",
  port: 9000,
};

const LISTEN_PORT = 9100;

test.use({ seed: { udpTargets: [TARGET] } });

test.beforeEach(async ({ app }) => {
  await app.switchTo("UDP");
  await app.selectUdpTarget(TARGET.name);
  await app.openUdpTab("Listen");
});

// ── 観点J / E: 開始・停止とセッション中の無効化 ──────────────────────────────

test("udp Start is disabled while a session is listening", async ({
  page,
  fake,
}) => {
  const start = page.getByRole("button", { name: "Start", exact: true });
  await page.getByPlaceholder("12345").fill(String(LISTEN_PORT));
  // エンコーディングの Select はトリガーに現在の値が出る。
  await page.getByRole("button", { name: "text", exact: true }).click();
  await page.getByRole("button", { name: "json", exact: true }).click();
  await start.click();

  const session = page.getByText(`Listening :${LISTEN_PORT} (json)`);
  await expect(session).toBeVisible();
  await expect(start).toBeDisabled();
  expect(await fake.args("StartListen")).toEqual([[LISTEN_PORT, "json"]]);

  await page.getByRole("button", { name: "Stop", exact: true }).click();
  await expect(session).toBeHidden();
  await expect(start).toBeEnabled();
  expect(await fake.args("StopListen")).toEqual([[expect.any(String)]]);
});

test("listening session is restored after reload", async ({ page, app }) => {
  await page.getByPlaceholder("12345").fill(String(LISTEN_PORT));
  await page.getByRole("button", { name: "Start", exact: true }).click();
  const session = page.getByText(`Listening :${LISTEN_PORT} (text)`);
  await expect(session).toBeVisible();

  await page.reload();
  await app.switchTo("UDP");
  await app.selectUdpTarget(TARGET.name);
  await app.openUdpTab("Listen");

  await expect(session).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Start", exact: true }),
  ).toBeDisabled();
});

test.describe("with a slow listener", () => {
  test.use({ seed: { udpTargets: [TARGET], startListenDelayMs: 1000 } });

  test("Start shows Starting... and is disabled while the listener opens", async ({
    page,
  }) => {
    await page.getByPlaceholder("12345").fill(String(LISTEN_PORT));
    await page.getByRole("button", { name: "Start", exact: true }).click();

    const starting = page.getByRole("button", { name: "Starting..." });
    await expect(starting).toBeDisabled();
    await expect(
      page.getByText(`Listening :${LISTEN_PORT} (text)`),
    ).toBeVisible();
    await expect(starting).toBeHidden();
  });
});

// ── 観点E: リスン失敗のエラー表示 ────────────────────────────────────────────
// 失敗はトーストで通知され、同じ文言がフォーム内にも残る。トーストを閉じてからフォームの
// 表示を確かめる (閉じる前は同じ文言が 2 か所にある)。

async function expectListenError(
  page: Page,
  message: string,
): Promise<void> {
  const toast = page
    .getByRole("alert")
    .filter({ hasText: "Failed to start listening" });
  await expect(toast).toContainText(message);
  await toast.getByRole("button", { name: "Dismiss" }).click();
  await expect(toast).toBeHidden();
  await expect(page.getByText(message, { exact: true })).toBeVisible();
}

test.describe("when the socket cannot be opened", () => {
  const error = `listen udp :${LISTEN_PORT}: bind: address already in use`;
  test.use({ seed: { udpTargets: [TARGET], startListenError: error } });

  test("a failed StartListen shows the error in the form", async ({ page }) => {
    await page.getByPlaceholder("12345").fill(String(LISTEN_PORT));
    const start = page.getByRole("button", { name: "Start", exact: true });
    await start.click();

    await expectListenError(page, error);
    await expect(page.getByText(/^Listening :/)).toBeHidden();
    await expect(start).toBeEnabled();
  });
});

test("starting without a port shows the backend validation error", async ({
  page,
  fake,
}) => {
  // ポート欄が空だと 0 のまま送られ、Go の listener_service.go の範囲検証が拒否する。
  await page.getByRole("button", { name: "Start", exact: true }).click();

  await expectListenError(page, "invalid port: must be 1-65535");
  expect(await fake.args("StartListen")).toEqual([[0, "text"]]);
});

// ── 観点J: 受信ログの表示・Clear・Copy payload ───────────────────────────────

test.describe("received messages", () => {
  let sessionId: string;

  // Go と同じく、リスン中のセッションからメッセージが届く形にする。
  test.beforeEach(async ({ page }) => {
    await page.getByPlaceholder("12345").fill(String(LISTEN_PORT));
    await page.getByRole("button", { name: "Start", exact: true }).click();
    await expect(
      page.getByText(`Listening :${LISTEN_PORT} (text)`),
    ).toBeVisible();
    sessionId = await page.evaluate(async () => {
      // biome-ignore lint/suspicious/noExplicitAny: 偽バックエンドのバインディングを直接読む
      const [session] = await (window as any).go.adapters.UDPHandler.GetListeners();
      return session.id as string;
    });
  });

  function message(
    payload: string,
    timestamp: number,
  ): UdpReceivedMessage {
    return {
      sessionId,
      port: LISTEN_PORT,
      remoteAddr: "127.0.0.1:50000",
      payload,
      encoding: "text",
      timestamp,
    };
  }

  test("received udp messages are listed newest first", async ({
    page,
    fake,
  }) => {
    await expect(page.getByText("No messages received yet")).toBeVisible();

    await fake.emit(WailsEvents.udpMessage, message("first-payload", 1000));
    await fake.emit(WailsEvents.udpMessage, message("second-payload", 2000));

    await expect(page.getByText("Received (2)")).toBeVisible();
    await expect(page.getByText("No messages received yet")).toBeHidden();
    const first = page.getByText("first-payload", { exact: true });
    const second = page.getByText("second-payload", { exact: true });
    await expect(first).toBeVisible();
    await expect(second).toBeVisible();
    await expect(page.getByText("127.0.0.1:50000")).toHaveCount(2);

    // 新しいメッセージが上に積まれる
    const firstBox = await first.boundingBox();
    const secondBox = await second.boundingBox();
    expect(secondBox?.y).toBeLessThan(firstBox?.y ?? 0);
  });

  test("Clear empties the received log", async ({ page, fake }) => {
    await fake.emit(WailsEvents.udpMessage, message("to-be-cleared", 1000));
    await expect(page.getByText("to-be-cleared", { exact: true })).toBeVisible();

    await page.getByRole("button", { name: "Clear", exact: true }).click();

    await expect(page.getByText("to-be-cleared", { exact: true })).toBeHidden();
    await expect(page.getByText("Received (0)")).toBeVisible();
    await expect(page.getByText("No messages received yet")).toBeVisible();
    // ログを消すだけでリスンは続く
    await expect(
      page.getByText(`Listening :${LISTEN_PORT} (text)`),
    ).toBeVisible();

    await fake.emit(WailsEvents.udpMessage, message("after-clear", 2000));
    await expect(page.getByText("after-clear", { exact: true })).toBeVisible();
    await expect(page.getByText("Received (1)")).toBeVisible();
  });

  test("Copy payload writes the payload to the clipboard", async ({
    page,
    context,
    fake,
  }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    const payload = '{"temp": 21.5}';
    await fake.emit(WailsEvents.udpMessage, message(payload, 1000));
    await expect(page.getByText(payload, { exact: true })).toBeVisible();

    await page.getByRole("button", { name: "Copy payload" }).click();

    await expect
      .poll(() => page.evaluate(() => navigator.clipboard.readText()))
      .toBe(payload);
  });
});
