import { type Page, test, expect } from "@playwright/test";

const getRenameInput = (page: Page) => page.getByTestId("rename-input");
const getUrlInput = (page: Page) =>
  page.getByPlaceholder("https://api.example.com/endpoint");

const clickNewCollectionInDropdown = async (page: Page) => {
  await page.locator('[aria-label="Add"]').click();
  await page.getByRole("button", { name: "New Collection" }).first().click();
};

const deleteCollection = async (page: Page, name: string) => {
  const btn = page.getByRole("button", { name, exact: true }).first();
  await btn.hover();
  await page.getByRole("button", { name: "Delete collection" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
};

test.afterEach(async ({ page }) => {
  const names = ["Persistence Test Collection"];
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
      // 削除対象が存在しない場合は無視
    }
  }
});

// ── 観点F-2: 選択中のリクエストがページリロード後に自動復元される ─────────────

test("selected request is restored after reload", async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections", { exact: true })).toBeVisible();

  // コレクションを作成する
  await clickNewCollectionInDropdown(page);
  const collInput = getRenameInput(page);
  await expect(collInput).toBeVisible();
  await collInput.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type("Persistence Test Collection");
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("button", {
      name: "Persistence Test Collection",
      exact: true,
    }),
  ).toBeVisible({ timeout: 10000 });

  // コレクションにリクエストを追加する
  await page
    .getByRole("button", { name: "Persistence Test Collection", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Add request" }).first().click();
  const reqInput = getRenameInput(page);
  await expect(reqInput).toBeVisible();
  await reqInput.press("Enter");

  const reqBtn = page.getByRole("button", { name: /New Request/ }).first();
  await expect(reqBtn).toBeVisible({ timeout: 10000 });

  // リクエストを選択してURLを入力する
  await reqBtn.click();
  await expect(reqBtn).toHaveAttribute("aria-current", "true");

  const testUrl = "http://localhost:9999/persistence-test";
  await getUrlInput(page).fill(testUrl);

  // 自動保存が完了するまで待機（500ms + バッファ）
  await page.waitForTimeout(800);

  // ページをリロードする
  await page.reload();
  await page.evaluate(() => {
    // localStorage のうちアクティブリクエスト保存キー以外をクリア（テーマ汚染防止）
    localStorage.removeItem("app:theme");
  });

  // HTTPパネルに戻る
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections", { exact: true })).toBeVisible();

  // コレクション展開後にリクエストが自動選択されている
  await expect(
    page.getByRole("button", { name: /New Request/ }).first(),
  ).toHaveAttribute("aria-current", "true", { timeout: 10000 });

  // URLフィールドに保存されたURLが表示されている（自動復元確認）
  await expect(getUrlInput(page)).toHaveValue(testUrl, { timeout: 5000 });
});

// ── 観点F-3: フォームの入力内容が自動保存されリロード後に復元される ──────────

test("http request form values are auto-saved and restored after reload", async ({
  page,
}) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections", { exact: true })).toBeVisible();

  // コレクションを作成する
  await clickNewCollectionInDropdown(page);
  const collInput = getRenameInput(page);
  await expect(collInput).toBeVisible();
  await collInput.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type("Persistence Test Collection");
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("button", {
      name: "Persistence Test Collection",
      exact: true,
    }),
  ).toBeVisible({ timeout: 10000 });

  // コレクションにリクエストを追加する
  await page
    .getByRole("button", { name: "Persistence Test Collection", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Add request" }).first().click();
  const reqInput = getRenameInput(page);
  await expect(reqInput).toBeVisible();
  await reqInput.press("Enter");

  const reqBtn = page.getByRole("button", { name: /New Request/ }).first();
  await expect(reqBtn).toBeVisible({ timeout: 10000 });

  // リクエストを選択してURLを入力する
  await reqBtn.click();
  await expect(reqBtn).toHaveAttribute("aria-current", "true");

  const savedUrl = "http://localhost:9999/auto-save-test";
  await getUrlInput(page).fill(savedUrl);

  // 自動保存が完了するまで待機（500ms + バッファ）
  await page.waitForTimeout(800);

  // ページをリロードしてHTTPパネルに戻る
  await page.reload();
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections", { exact: true })).toBeVisible();

  // 同じリクエストを明示的にクリックしてフォームを開く
  await expect(
    page.getByRole("button", { name: /New Request/ }).first(),
  ).toBeVisible({ timeout: 10000 });
  await page.getByRole("button", { name: /New Request/ }).first().click();

  // 自動保存されたURLが復元されている
  await expect(getUrlInput(page)).toHaveValue(savedUrl, { timeout: 5000 });
});
