import type { Page } from "@playwright/test";
import { expect, test } from "../../fixtures/ui";

// UDP のリスン開始・停止。セッションは偽バックエンドが持ち、Go と同じくページの
// リロードを跨いで残る (UI は起動時に GetListeners で復元する)。受信メッセージの表示は
// イベントの注入口が要るので扱わない。

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
