import { expect, test } from "../../fixtures/ui";

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

test("can create a new collection from sidebar", async ({ page, app }) => {
  await app.createCollection("E2E Test Collection");
  await expect(page.getByText("E2E Test Collection")).toBeVisible();
});

test("can rename collection with double-click then Enter", async ({
  page,
  app,
}) => {
  await app.createCollection();

  await page
    .locator("span")
    .filter({ hasText: /^New Collection$/ })
    .first()
    .dblclick();
  await app.confirmRename("Renamed Collection");

  await expect(page.getByText("Renamed Collection")).toBeVisible();
});

test("rename is cancelled on Escape", async ({ page, app }) => {
  await app.createCollection();

  await page
    .locator("span")
    .filter({ hasText: /^New Collection$/ })
    .first()
    .dblclick();

  const input = app.renameInput;
  await expect(input).toBeVisible();
  await input.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.type("Will Be Cancelled");
  await page.keyboard.press("Escape");

  await expect(app.collection("New Collection")).toBeVisible();
  await expect(page.getByText("Will Be Cancelled")).toBeHidden();
});

test("delete collection shows confirm dialog and removes on confirm", async ({
  page,
  app,
}) => {
  await app.createCollection();

  await app.collection("New Collection").hover();
  await page.getByRole("button", { name: "Delete collection" }).first().click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("Delete collection");

  await dialog.getByRole("button", { name: "Delete" }).click();
  await expect(dialog).toBeHidden();
  await expect(app.collection("New Collection")).toBeHidden();
});

test("clicking a request in sidebar opens it in the editor panel", async ({
  app,
}) => {
  await app.createCollection();
  await app.addRequest("New Collection", "Test Request");

  const request = app.request(/Test Request/);
  await expect(request).toBeVisible();

  await request.click();
  await expect(request).toHaveAttribute("aria-current", "true");
});

test("can expand and collapse folders in sidebar", async ({ page, app }) => {
  await app.createCollection();
  await app.addFolder("New Collection"); // 既定名 "New Folder" のまま
  await expect(page.getByText("New Folder")).toBeVisible();

  // コレクションのトグルで折りたたむ
  await app.collection("New Collection").click();
  await expect(page.getByText("New Folder")).toBeHidden();

  // もう一度クリックで展開
  await app.collection("New Collection").click();
  await expect(page.getByText("New Folder")).toBeVisible();
});
