import { expect, test } from "../../fixtures/ui";

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

test("can create a new collection from sidebar", async ({
  page,
  app,
  fake,
}) => {
  await app.createCollection("E2E Test Collection");
  await expect(page.getByText("E2E Test Collection")).toBeVisible();

  // 作成 (既定名) とリネームの 2 つの RPC を経て、バックエンドにもその名前で残る。
  const { collections, sidebar } = await fake.snapshot();
  expect(collections.map((c) => c.name)).toEqual(["E2E Test Collection"]);
  expect(sidebar).toEqual([{ kind: "collection", id: collections[0].id }]);
});

test("can rename collection with double-click then Enter", async ({
  page,
  app,
}) => {
  await app.createCollection();

  await app.startRename(app.collection("New Collection"), "New Collection");
  await app.confirmRename("Renamed Collection");

  await expect(page.getByText("Renamed Collection")).toBeVisible();
});

test("rename is cancelled on Escape", async ({ page, app }) => {
  await app.createCollection();

  await app.startRename(app.collection("New Collection"), "New Collection");

  const input = app.renameInput;
  await expect(input).toBeVisible();
  await input.fill("Will Be Cancelled");
  await input.press("Escape");

  await expect(app.collection("New Collection")).toBeVisible();
  await expect(page.getByText("Will Be Cancelled")).toBeHidden();
});

test("delete collection shows confirm dialog and removes on confirm", async ({
  page,
  app,
  fake,
}) => {
  await app.createCollection();

  await app
    .rowAction(app.collection("New Collection"), "Delete collection")
    .click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("Delete collection");

  await dialog.getByRole("button", { name: "Delete" }).click();
  await expect(dialog).toBeHidden();
  await expect(app.collection("New Collection")).toBeHidden();

  const { collections, sidebar } = await fake.snapshot();
  expect(collections).toEqual([]);
  expect(sidebar).toEqual([]);
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

// フォルダ自体の開閉は http/tree.spec.ts の "expanding a folder reveals its children" で確かめる。
test("clicking a collection collapses and expands its children", async ({
  page,
  app,
}) => {
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
