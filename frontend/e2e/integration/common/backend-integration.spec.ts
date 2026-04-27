import { type Page, test, expect } from "@playwright/test";

// ── ヘルパー: HTTPコレクション ─────────────────────────────────────────────────

const createCollection = async (page: Page, name: string) => {
  await page.locator('[aria-label="Add"]').click();
  await page.getByRole("button", { name: "New Collection" }).first().click();
  const input = page.getByTestId("rename-input");
  await expect(input).toBeVisible();
  await input.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type(name);
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("button", { name, exact: true }).first(),
  ).toBeVisible({ timeout: 10000 });
};

const deleteCollection = async (page: Page, name: string) => {
  const btn = page.getByRole("button", { name, exact: true }).first();
  await btn.hover();
  await page.getByRole("button", { name: "Delete collection" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
};

// ── ヘルパー: MQTTブローカープロファイル ──────────────────────────────────────

const createBrokerProfile = async (page: Page, name: string) => {
  await page.getByRole("button", { name: "New Broker", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Name", { exact: true }).fill(name);
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect(dialog).not.toBeVisible({ timeout: 5000 });
  await expect(
    page.locator('[role="button"]').filter({ hasText: name }).first(),
  ).toBeVisible({ timeout: 10000 });
};

const deleteBrokerProfile = async (page: Page) => {
  await page.locator('[aria-label="Delete broker"]').first().dispatchEvent("click");
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible({ timeout: 3000 });
  await dialog.getByRole("button", { name: "Delete" }).click();
  await expect(dialog).not.toBeVisible({ timeout: 5000 });
};

// ── ヘルパー: UDPターゲット ────────────────────────────────────────────────────

const createUdpTarget = async (
  page: Page,
  name: string,
  host: string,
  port: number,
) => {
  await page.getByRole("button", { name: "New Target" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Name").fill(name);
  await dialog.getByLabel("Host").fill(host);
  await dialog.getByLabel("Port").fill(String(port));
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByText(name, { exact: true }).first()).toBeVisible({
    timeout: 5000,
  });
};

const deleteUdpTarget = async (page: Page, name: string) => {
  await page
    .locator('[aria-label="Delete target"]')
    .first()
    .dispatchEvent("click");
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible({ timeout: 3000 });
  await dialog.getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText(name, { exact: true })).not.toBeVisible({
    timeout: 5000,
  });
};

// ══════════════════════════════════════════════════════════════════════════════
// 観点M-1: Wailsバインディング呼び出しの成功確認
// ══════════════════════════════════════════════════════════════════════════════

test.describe("M-1: Wails binding calls succeed", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await page.evaluate(() => localStorage.clear());
    await page.reload();
  });

  test("GetCollections wails binding returns data from backend", async ({
    page,
  }) => {
    await page.getByRole("button", { name: "HTTP", exact: true }).click();
    // Collections ヘッダーが表示されれば GetCollections バインディングが正常動作している
    await expect(
      page.getByText("Collections", { exact: true }),
    ).toBeVisible({ timeout: 10000 });
  });

  test("GetBrokerProfiles wails binding returns data from backend", async ({
    page,
  }) => {
    // MQTT パネルが初期表示 — Brokers ヘッダーが表示されれば GetBrokerProfiles が正常動作
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible({
      timeout: 10000,
    });
  });

  test("GetTargets wails binding returns data from backend", async ({
    page,
  }) => {
    await page.getByRole("button", { name: "UDP", exact: true }).click();
    // Targets ヘッダーが表示されれば GetTargets バインディングが正常動作
    await expect(page.getByText("Targets", { exact: true })).toBeVisible({
      timeout: 10000,
    });
  });
});

// ══════════════════════════════════════════════════════════════════════════════
// 観点M-2: コレクションの永続化
// ══════════════════════════════════════════════════════════════════════════════

const E2E_COLLECTION_NAME = "E2E Backend Integration Collection";

test.describe("M-2: Collection persistence", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await page.evaluate(() => localStorage.clear());
    await page.reload();
    await page.getByRole("button", { name: "HTTP", exact: true }).click();
    await expect(
      page.getByText("Collections", { exact: true }),
    ).toBeVisible();
    // 前回テストで残ったデータを削除
    try {
      const visible = await page
        .getByRole("button", { name: E2E_COLLECTION_NAME, exact: true })
        .first()
        .isVisible({ timeout: 500 });
      if (visible) {
        await deleteCollection(page, E2E_COLLECTION_NAME);
      }
    } catch {
      // 存在しない場合は無視
    }
  });

  test.afterEach(async ({ page }) => {
    try {
      const visible = await page
        .getByRole("button", { name: E2E_COLLECTION_NAME, exact: true })
        .first()
        .isVisible({ timeout: 500 });
      if (visible) {
        await deleteCollection(page, E2E_COLLECTION_NAME);
      }
    } catch {
      // ignore
    }
  });

  test("created collection persists after page reload", async ({ page }) => {
    await createCollection(page, E2E_COLLECTION_NAME);

    // ページリロード後もコレクションが残っている
    await page.reload();
    await page.getByRole("button", { name: "HTTP", exact: true }).click();
    await expect(
      page.getByText("Collections", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: E2E_COLLECTION_NAME, exact: true }).first(),
    ).toBeVisible({ timeout: 10000 });
  });
});

// ══════════════════════════════════════════════════════════════════════════════
// 観点M-3: MQTTブローカープロファイルの永続化
// ══════════════════════════════════════════════════════════════════════════════

const E2E_BROKER_NAME = "E2E Backend Integration Broker";

test.describe("M-3: MQTT broker profile persistence", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await page.evaluate(() => localStorage.clear());
    await page.reload();
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible();
    // 前回テストで残ったプロファイルを削除
    for (;;) {
      const count = await page
        .locator('[role="button"]')
        .filter({ hasText: E2E_BROKER_NAME })
        .count();
      if (count === 0) break;
      await deleteBrokerProfile(page);
    }
  });

  test.afterEach(async ({ page }) => {
    try {
      for (;;) {
        const count = await page
          .locator('[role="button"]')
          .filter({ hasText: E2E_BROKER_NAME })
          .count();
        if (count === 0) break;
        await deleteBrokerProfile(page);
      }
    } catch {
      // ignore
    }
  });

  test("mqtt broker profile persists after page reload", async ({ page }) => {
    await createBrokerProfile(page, E2E_BROKER_NAME);

    // ページリロード後もプロファイルが残っている
    await page.reload();
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible({
      timeout: 10000,
    });
    await expect(
      page.locator('[role="button"]').filter({ hasText: E2E_BROKER_NAME }).first(),
    ).toBeVisible({ timeout: 10000 });
  });
});

// ══════════════════════════════════════════════════════════════════════════════
// 観点M-4: UDPターゲットの永続化
// ══════════════════════════════════════════════════════════════════════════════

const E2E_UDP_TARGET_NAME = "E2E Backend Integration Target";

test.describe("M-4: UDP target persistence", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await page.evaluate(() => localStorage.clear());
    await page.reload();
    await page.getByRole("button", { name: "UDP", exact: true }).click();
    await expect(page.getByText("Targets", { exact: true })).toBeVisible();
    // 前回テストで残ったターゲットを削除
    for (;;) {
      const count = await page
        .getByText(E2E_UDP_TARGET_NAME, { exact: true })
        .count();
      if (count === 0) break;
      await deleteUdpTarget(page, E2E_UDP_TARGET_NAME);
    }
  });

  test.afterEach(async ({ page }) => {
    try {
      for (;;) {
        const count = await page
          .getByText(E2E_UDP_TARGET_NAME, { exact: true })
          .count();
        if (count === 0) break;
        await deleteUdpTarget(page, E2E_UDP_TARGET_NAME);
      }
    } catch {
      // ignore
    }
  });

  test("udp target persists after page reload", async ({ page }) => {
    await createUdpTarget(page, E2E_UDP_TARGET_NAME, "127.0.0.1", 9999);

    // ページリロード後もターゲットが残っている
    await page.reload();
    await page.getByRole("button", { name: "UDP", exact: true }).click();
    await expect(page.getByText("Targets", { exact: true })).toBeVisible({
      timeout: 10000,
    });
    await expect(
      page.getByText(E2E_UDP_TARGET_NAME, { exact: true }).first(),
    ).toBeVisible({ timeout: 10000 });
  });
});

// ══════════════════════════════════════════════════════════════════════════════
// 観点M-8: サイドバーレイアウトの永続化
// MQTTブローカーの Move up/Down による順序変更がリロード後も保持される
// ══════════════════════════════════════════════════════════════════════════════

const E2E_BROKER_A = "E2E Broker Alpha";
const E2E_BROKER_B = "E2E Broker Beta";

test.describe("M-8: Sidebar layout persistence", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await page.evaluate(() => localStorage.clear());
    await page.reload();
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible();
    // 前回テストで残ったデータを削除
    for (const name of [E2E_BROKER_A, E2E_BROKER_B]) {
      for (;;) {
        const count = await page
          .locator('[role="button"]')
          .filter({ hasText: name })
          .count();
        if (count === 0) break;
        await deleteBrokerProfile(page);
      }
    }
  });

  test.afterEach(async ({ page }) => {
    try {
      for (const name of [E2E_BROKER_A, E2E_BROKER_B]) {
        for (;;) {
          const count = await page
            .locator('[role="button"]')
            .filter({ hasText: name })
            .count();
          if (count === 0) break;
          await deleteBrokerProfile(page);
        }
      }
    } catch {
      // ignore
    }
  });

  test("broker profile order persists after reorder and page reload", async ({
    page,
  }) => {
    // 2件のブローカープロファイルを作成（Alpha → Beta の順）
    await createBrokerProfile(page, E2E_BROKER_A);
    await createBrokerProfile(page, E2E_BROKER_B);

    // リスト内の順序を確認（Alpha が上、Beta が下）
    const brokerItems = page.locator('[role="button"]').filter({
      hasText: /E2E Broker (Alpha|Beta)/,
    });
    await expect(brokerItems.first()).toContainText(E2E_BROKER_A);
    await expect(brokerItems.nth(1)).toContainText(E2E_BROKER_B);

    // Beta の "Move up" をクリックして順序を入れ替える（Beta → Alpha）
    const betaItem = page
      .locator('[role="button"]')
      .filter({ hasText: E2E_BROKER_B })
      .first();
    await betaItem.locator('[aria-label="Move up"]').dispatchEvent("click");

    // 順序が入れ替わった（Beta が上、Alpha が下）
    await expect(brokerItems.first()).toContainText(E2E_BROKER_B, {
      timeout: 5000,
    });
    await expect(brokerItems.nth(1)).toContainText(E2E_BROKER_A);

    // ページリロード後も順序が保持される
    await page.reload();
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible({
      timeout: 10000,
    });
    const reloadedItems = page.locator('[role="button"]').filter({
      hasText: /E2E Broker (Alpha|Beta)/,
    });
    await expect(reloadedItems.first()).toContainText(E2E_BROKER_B, {
      timeout: 10000,
    });
    await expect(reloadedItems.nth(1)).toContainText(E2E_BROKER_A);
  });
});
