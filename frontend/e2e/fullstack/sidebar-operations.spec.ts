import { type Page, test, expect } from "@playwright/test";

// リネーム入力に data-testid="rename-input" を付与しているためこれで一意に特定できる
const getRenameInput = (page: Page) => page.getByTestId("rename-input");

// リネーム入力に名前を入力して Enter で確定する
// fill() はイベント発火の順序によっては onKeyDown の value が古い値になるため
// click() → Ctrl+A → type() のキーボード操作で確実に入力する
const typeAndConfirmRename = async (page: Page, name: string) => {
  const input = getRenameInput(page);
  await expect(input).toBeVisible();
  await input.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type(name);
  await page.keyboard.press("Enter");
};

// ドロップダウンメニュー内の "New Collection" を選択する
// CSS Module の class 名は hash されるため DOM 順 (.first()) で区別する
const clickNewCollectionInDropdown = async (page: Page) => {
  await page.locator('[aria-label="Add"]').click();
  await page.getByRole("button", { name: "New Collection" }).first().click();
};

// コレクションのアクションボタンを表示させてから削除する（hover → confirm）
const deleteCollection = async (page: Page, name: string) => {
  const toggleBtn = page.getByRole("button", { name, exact: true }).first();
  await toggleBtn.hover();
  await page.getByRole("button", { name: "Delete collection" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
};

// テスト終了後に残留データを削除する
test.afterEach(async ({ page }) => {
  const collectionNames = [
    "E2E Test Collection",
    "Renamed Collection",
    "New Collection",
  ];
  for (const name of collectionNames) {
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

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  // Switch to HTTP to access the collection sidebar
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections", { exact: true })).toBeVisible();
});

test("can create a new collection from sidebar", async ({ page }) => {
  // Open add menu and select New Collection
  await clickNewCollectionInDropdown(page);

  // Rename input appears — type a name and confirm via keyboard
  await typeAndConfirmRename(page, "E2E Test Collection");

  // Collection should appear in the sidebar
  await expect(page.getByText("E2E Test Collection")).toBeVisible({
    timeout: 10000,
  });
});

test("can rename collection with double-click then Enter", async ({ page }) => {
  // Create a collection, dismiss the initial rename
  await clickNewCollectionInDropdown(page);
  const input = getRenameInput(page);
  await expect(input).toBeVisible();
  await input.press("Escape");

  // Wait for collection to be visible
  await expect(
    page.getByRole("button", { name: "New Collection", exact: true }).first(),
  ).toBeVisible();

  // Double-click the collection name span to enter rename mode
  await page
    .locator("span")
    .filter({ hasText: /^New Collection$/ })
    .first()
    .dblclick();

  await typeAndConfirmRename(page, "Renamed Collection");

  await expect(page.getByText("Renamed Collection")).toBeVisible({
    timeout: 10000,
  });
});

test("rename is cancelled on Escape", async ({ page }) => {
  // Create a collection, dismiss the initial rename
  await clickNewCollectionInDropdown(page);
  const input = getRenameInput(page);
  await expect(input).toBeVisible();
  await input.press("Escape");

  // Double-click to rename, then press Escape to cancel
  await page
    .locator("span")
    .filter({ hasText: /^New Collection$/ })
    .first()
    .dblclick();

  const renameInput = getRenameInput(page);
  await expect(renameInput).toBeVisible();
  await renameInput.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type("Will Be Cancelled");
  await page.keyboard.press("Escape");

  // Original name should remain unchanged
  await expect(
    page.getByRole("button", { name: "New Collection", exact: true }).first(),
  ).toBeVisible();
  await expect(page.getByText("Will Be Cancelled")).not.toBeVisible();
});

test("delete collection shows confirm dialog and removes on confirm", async ({
  page,
}) => {
  // Create a collection, dismiss the initial rename
  await clickNewCollectionInDropdown(page);
  const input = getRenameInput(page);
  await expect(input).toBeVisible();
  await input.press("Escape");

  // Hover to reveal action buttons, then click Delete
  await page
    .getByRole("button", { name: "New Collection", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Delete collection" }).first().click();

  // Confirm dialog should appear with correct title
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("Delete collection");

  // Confirm deletion
  await dialog.getByRole("button", { name: "Delete" }).click();
  await expect(dialog).not.toBeVisible();
});

test("clicking a request in sidebar opens it in the editor panel", async ({
  page,
}) => {
  // Create a collection and keep the default name
  await clickNewCollectionInDropdown(page);
  const input = getRenameInput(page);
  await expect(input).toBeVisible();
  await input.press("Enter");

  // Add a request (hover to reveal collection action buttons)
  await page
    .getByRole("button", { name: "New Collection", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Add request" }).first().click();

  // Rename the request via keyboard
  await typeAndConfirmRename(page, "Test Request");

  // Wait for the request to appear in the sidebar
  const requestBtn = page
    .getByRole("button", { name: /GET.*Test Request|GETTest Request/ })
    .first();
  await expect(requestBtn).toBeVisible({ timeout: 10000 });

  // Click the request to select it
  await requestBtn.click();

  // The request should now be active (aria-current="true")
  await expect(requestBtn).toHaveAttribute("aria-current", "true");

  // Cleanup: hover to reveal the delete button, then scope to the active request's row
  await requestBtn.hover();
  await requestBtn.locator("xpath=..").getByRole("button", { name: "Delete request" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
});

test("can expand and collapse folders in sidebar", async ({ page }) => {
  // Create a collection and keep the default name
  await clickNewCollectionInDropdown(page);
  const input = getRenameInput(page);
  await expect(input).toBeVisible();
  await input.press("Enter");

  // Add a folder (hover to reveal collection action buttons)
  await page
    .getByRole("button", { name: "New Collection", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Add folder" }).first().click();

  // Dismiss folder rename (keep "New Folder")
  const folderInput = getRenameInput(page);
  await expect(folderInput).toBeVisible();
  await folderInput.press("Escape");
  await expect(page.getByText("New Folder")).toBeVisible({ timeout: 10000 });

  // Collapse the collection by clicking the toggle button
  await page
    .getByRole("button", { name: "New Collection", exact: true })
    .first()
    .click();
  await expect(page.getByText("New Folder")).not.toBeVisible();

  // Expand again
  await page
    .getByRole("button", { name: "New Collection", exact: true })
    .first()
    .click();
  await expect(page.getByText("New Folder")).toBeVisible();

  // Cleanup: delete folder first
  await page
    .getByRole("button", { name: "New Folder", exact: true })
    .first()
    .hover();
  await page.getByRole("button", { name: "Delete folder" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
});
