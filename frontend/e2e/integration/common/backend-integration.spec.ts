import type { Locator, Page } from "@playwright/test";
import { expect, test } from "../../fixtures/integration";

// 実 Go バックエンド (wails dev) に対する疎通と永続化の確認。保存先は playwright.integration.
// config.ts が APPDATA を一時ディレクトリへ向けて隔離しているので、実行のたびにまっさらな状態から
// 始まる。前回の残骸を UI 越しに消して回る beforeEach/afterEach はもう要らない。

// HTML5 ネイティブ drag-and-drop で並び替えをトリガーする。
// Playwright の dragTo はネイティブ DnD イベントを発火しないため、
// 共有 DataTransfer オブジェクトで dragstart → dragover → drop を手動 dispatch する。
// dispatchEvent では clientY が 0 になるため、ドロップ先の行の上半分に落ちる扱いになり、
// source は target の直前（＝上）に挿入される。
const dragRowOnto = async (page: Page, source: Locator, target: Locator) => {
  const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
  await source.dispatchEvent("dragstart", { dataTransfer });
  await target.dispatchEvent("dragover", { dataTransfer });
  await target.dispatchEvent("drop", { dataTransfer });
  await source.dispatchEvent("dragend", { dataTransfer });
};

const brokerRow = (page: Page, name: string | RegExp) =>
  page.locator('[role="button"]').filter({ hasText: name });

const createBrokerProfile = async (page: Page, name: string) => {
  await page.getByRole("button", { name: "New Broker", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Name", { exact: true }).fill(name);
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(brokerRow(page, name).first()).toBeVisible();
};

// ── 観点M-1: Wails バインディング呼び出しの成功確認 ───────────────────────────

test.describe("M-1: Wails binding calls succeed", () => {
  test("GetCollections wails binding returns data from backend", async ({
    app,
  }) => {
    // Collections ヘッダーが表示されれば GetCollections バインディングが正常動作している
    await app.switchTo("HTTP");
  });

  test("GetBrokerProfiles wails binding returns data from backend", async ({
    page,
  }) => {
    // MQTT パネルが初期表示 — Brokers ヘッダーが表示されれば GetBrokerProfiles が正常動作
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible();
  });

  test("GetTargets wails binding returns data from backend", async ({ app }) => {
    await app.switchTo("UDP");
  });
});

// ── 観点M-2: コレクションの永続化 ────────────────────────────────────────────

test.describe("M-2: Collection persistence", () => {
  const NAME = "E2E Backend Integration Collection";

  test("created collection persists after page reload", async ({ page, app }) => {
    await app.switchTo("HTTP");
    await app.createCollection(NAME);

    await page.reload();
    await app.switchTo("HTTP");
    await expect(app.collection(NAME)).toBeVisible();
  });
});

// ── 観点M-3: MQTT ブローカープロファイルの永続化 ─────────────────────────────

test.describe("M-3: MQTT broker profile persistence", () => {
  const NAME = "E2E Backend Integration Broker";

  test("mqtt broker profile persists after page reload", async ({ page }) => {
    await createBrokerProfile(page, NAME);

    await page.reload();
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible();
    await expect(brokerRow(page, NAME).first()).toBeVisible();
  });
});

// ── 観点M-4: UDP ターゲットの永続化 ──────────────────────────────────────────

test.describe("M-4: UDP target persistence", () => {
  const NAME = "E2E Backend Integration Target";

  test("udp target persists after page reload", async ({ page, app }) => {
    await app.switchTo("UDP");
    await app.createUdpTarget(NAME, "127.0.0.1", 9999);

    await page.reload();
    await app.switchTo("UDP");
    await expect(page.getByText(NAME, { exact: true }).first()).toBeVisible();
  });
});

// ── 観点M-8: サイドバーレイアウトの永続化 ────────────────────────────────────
// ブローカーの drag-and-drop による順序変更がリロード後も保持される

test.describe("M-8: Sidebar layout persistence", () => {
  const ALPHA = "E2E Broker Alpha";
  const BETA = "E2E Broker Beta";

  test("broker profile order persists after reorder and page reload", async ({
    page,
  }) => {
    await createBrokerProfile(page, ALPHA);
    await createBrokerProfile(page, BETA);

    const rows = brokerRow(page, /E2E Broker (Alpha|Beta)/);
    await expect(rows.first()).toContainText(ALPHA);
    await expect(rows.nth(1)).toContainText(BETA);

    // Beta を Alpha の上にドラッグして順序を入れ替える
    await dragRowOnto(
      page,
      brokerRow(page, BETA).first(),
      brokerRow(page, ALPHA).first(),
    );

    await expect(rows.first()).toContainText(BETA);
    await expect(rows.nth(1)).toContainText(ALPHA);

    // ページリロード後も順序が保持される
    await page.reload();
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible();
    const reloaded = brokerRow(page, /E2E Broker (Alpha|Beta)/);
    await expect(reloaded.first()).toContainText(BETA);
    await expect(reloaded.nth(1)).toContainText(ALPHA);
  });
});
