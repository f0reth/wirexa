import { type Page, test, expect } from "@playwright/test";

const getRenameInput = (page: Page) => page.getByTestId("rename-input");

const clickNewCollectionInDropdown = async (page: Page) => {
  await page.locator('[aria-label="Add"]').click();
  await page.getByRole("button", { name: "New Collection" }).first().click();
};

const deleteCollection = async (page: Page, name: string) => {
  const toggleBtn = page.getByRole("button", { name, exact: true }).first();
  await toggleBtn.hover();
  await page.getByRole("button", { name: "Delete collection" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
};

test.afterEach(async ({ page }) => {
  const names = ["New Collection", "API テスト コレクション"];
  for (const name of names) {
    try {
      let visible = await page
        .getByRole("button", { name, exact: true })
        .first()
        .isVisible({ timeout: 500 });
      while (visible) {
        await deleteCollection(page, name);
        await page.waitForTimeout(300);
        visible = await page
          .getByRole("button", { name, exact: true })
          .first()
          .isVisible({ timeout: 500 });
      }
    } catch {
      // 対象が存在しない場合は無視
    }
  }
});

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections")).toBeVisible();
});

// ── 観点D-8: 空白のみの入力でリネームがキャンセルされる ───────────────────────

test("rename input with whitespace-only value keeps original name", async ({
  page,
}) => {
  // コレクションを作成して初回リネームを Escape でキャンセル
  await clickNewCollectionInDropdown(page);
  const input = getRenameInput(page);
  await expect(input).toBeVisible();
  await input.press("Escape");

  await expect(
    page.getByRole("button", { name: "New Collection", exact: true }).first(),
  ).toBeVisible();

  // ダブルクリックでリネームモードへ
  await page
    .locator("span")
    .filter({ hasText: /^New Collection$/ })
    .first()
    .dblclick();

  const renameInput = getRenameInput(page);
  await expect(renameInput).toBeVisible();
  await renameInput.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type("   "); // 空白のみ
  await page.keyboard.press("Enter");

  // 元の名前 "New Collection" のままであること
  await expect(
    page.getByRole("button", { name: "New Collection", exact: true }).first(),
  ).toBeVisible();
  await expect(page.getByTestId("rename-input")).not.toBeVisible();
});

// ── 観点D-9: 特殊文字・Unicode のコレクション名 ──────────────────────────────

test("collection name supports unicode and special characters", async ({
  page,
}) => {
  await clickNewCollectionInDropdown(page);

  const input = getRenameInput(page);
  await expect(input).toBeVisible();
  await input.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type("API テスト コレクション");
  await page.keyboard.press("Enter");

  await expect(page.getByText("API テスト コレクション")).toBeVisible({
    timeout: 10000,
  });
});
