import type { Collection } from "../../../src/domain/http/types";
import { type FakeControl, expect, test } from "../../fixtures/ui";

// HTTP ツリーのフォルダ／リクエスト操作 (D&D 以外)。コレクション単位の操作は
// sidebar-operations.spec.ts にある。フォルダは seed で仕込めないので UI で作る。
const COLLECTION = "Tree Collection";
const REQUEST = "Tree Request";

test.use({
  seed: { collections: [{ name: COLLECTION, requests: [{ name: REQUEST }] }] },
});

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

/** 偽バックエンドが持つ seed のコレクション。 */
async function storedCollection(fake: FakeControl): Promise<Collection> {
  const { collections } = await fake.snapshot();
  const col = collections.find((c) => c.name === COLLECTION);
  if (!col) throw new Error(`${COLLECTION} is missing in the fake backend`);
  return col;
}

// ── 削除 ────────────────────────────────────────────────────────────────────

test("can delete a request after confirming", async ({ page, app, fake }) => {
  const request = app.request(/Tree Request/);
  await app.rowAction(request, "Delete request").click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Delete request");
  await expect(dialog).toContainText(`"${REQUEST}"`);
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();

  await expect(dialog).toBeHidden();
  await expect(request).toBeHidden();
  expect((await storedCollection(fake)).items).toEqual([]);
});

test("can delete a folder together with its children", async ({
  page,
  app,
  fake,
}) => {
  await app.addFolder(COLLECTION, "Doomed Folder");
  await app.folder("Doomed Folder").click();
  await app.addRequest("Doomed Folder", "Nested Request");
  await expect(app.request(/Nested Request/)).toBeVisible();

  await app.rowAction(app.folder("Doomed Folder"), "Delete folder").click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Delete folder");
  await app.confirmDelete();

  await expect(dialog).toBeHidden();
  await expect(app.folder("Doomed Folder")).toBeHidden();
  await expect(app.request(/Nested Request/)).toBeHidden();
  const items = (await storedCollection(fake)).items;
  expect(items.map((i) => i.name)).toEqual([REQUEST]);
});

test("cancelling the delete dialog keeps the item", async ({
  page,
  app,
  fake,
}) => {
  const request = app.request(/Tree Request/);
  await app.rowAction(request, "Delete request").click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();

  await expect(dialog).toBeHidden();
  await expect(request).toBeVisible();
  expect(await fake.calls("DeleteItem")).toBe(0);
});

// ── リネーム ────────────────────────────────────────────────────────────────

test("can rename a request by double-click", async ({ app, fake }) => {
  await app.startRename(app.request(/Tree Request/), REQUEST);
  await app.confirmRename("Renamed Request");

  await expect(app.request(/Renamed Request/)).toBeVisible();
  await expect(app.request(/Tree Request/)).toBeHidden();
  const items = (await storedCollection(fake)).items;
  expect(items.map((i) => i.name)).toEqual(["Renamed Request"]);
});

test("can rename a folder by double-click", async ({ app, fake }) => {
  await app.addFolder(COLLECTION, "Old Folder");

  await app.startRename(app.folder("Old Folder"), "Old Folder");
  await app.confirmRename("New Folder Name");

  await expect(app.folder("New Folder Name")).toBeVisible();
  await expect(app.folder("Old Folder")).toBeHidden();
  const items = (await storedCollection(fake)).items;
  expect(items.map((i) => i.name)).toEqual([REQUEST, "New Folder Name"]);
});

test("rename is committed on blur", async ({ page, app, fake }) => {
  await app.startRename(app.request(/Tree Request/), REQUEST);
  await app.renameInput.fill("Blurred Name");

  // 入力の外 (サイドバーの見出し) をクリックしてフォーカスを外す
  await page.getByText("Collections", { exact: true }).click();

  await expect(app.renameInput).toBeHidden();
  await expect(app.request(/Blurred Name/)).toBeVisible();
  await fake.waitForCalls("RenameItem");
  const items = (await storedCollection(fake)).items;
  expect(items.map((i) => i.name)).toEqual(["Blurred Name"]);
});

// ── 開閉・追加 ──────────────────────────────────────────────────────────────

test("expanding a folder reveals its children", async ({ app }) => {
  await app.addFolder(COLLECTION, "Folder A");
  const folder = app.folder("Folder A");
  const child = app.request(/Child Request/);

  // フォルダは既定で閉じている。開いてから子を足す。
  await folder.click();
  await app.addRequest("Folder A", "Child Request");
  await expect(child).toBeVisible();

  await folder.click();
  await expect(child).toBeHidden();
  // 閉じるのはフォルダの中だけで、同じコレクションの他の項目は見えたまま
  await expect(app.request(/Tree Request/)).toBeVisible();

  await folder.click();
  await expect(child).toBeVisible();
});

test("New Request from the add menu creates a root-level request", async ({
  app,
  fake,
}) => {
  await app.createRootRequest("Root Request");

  const { rootItems, sidebar, collections } = await fake.snapshot();
  const root = rootItems.find((i) => i.name === "Root Request");
  expect(root?.type).toBe("request");
  expect(sidebar).toContainEqual({ kind: "item", id: root?.id });
  // どのコレクションにも入っていない
  const tree = collections.find((c) => c.name === COLLECTION);
  expect(tree?.items.map((i) => i.name)).toEqual([REQUEST]);
});

test("can add a request inside a folder", async ({ app, fake }) => {
  await app.addFolder(COLLECTION, "Parent Folder");
  await app.folder("Parent Folder").click();
  await app.addRequest("Parent Folder", "Inner Request");

  await expect(app.request(/Inner Request/)).toBeVisible();
  const folder = (await storedCollection(fake)).items.find(
    (i) => i.name === "Parent Folder",
  );
  expect(folder?.children.map((c) => [c.type, c.name])).toEqual([
    ["request", "Inner Request"],
  ]);
});

test("can add a folder inside a folder", async ({ app, fake }) => {
  await app.addFolder(COLLECTION, "Outer Folder");
  await app.folder("Outer Folder").click();
  await app.addFolder("Outer Folder", "Inner Folder");

  await expect(app.folder("Inner Folder")).toBeVisible();
  const outer = (await storedCollection(fake)).items.find(
    (i) => i.name === "Outer Folder",
  );
  expect(outer?.children.map((c) => [c.type, c.name])).toEqual([
    ["folder", "Inner Folder"],
  ]);
});

// ── 永続化 (観点F) ──────────────────────────────────────────────────────────

test("expanded folders are restored after reload", async ({ page, app }) => {
  await app.addFolder(COLLECTION, "Kept Open");
  await app.folder("Kept Open").click();
  await app.addRequest("Kept Open", "Visible Child");
  await expect(app.request(/Visible Child/)).toBeVisible();

  await page.reload();
  await app.switchTo("HTTP");
  await expect(app.request(/Visible Child/)).toBeVisible();
});

test("collapsed collections stay collapsed after reload", async ({
  page,
  app,
}) => {
  // コレクションは既定で開いているので、閉じた状態が保存されていることを確かめられる
  await app.collection(COLLECTION).click();
  await expect(app.request(/Tree Request/)).toBeHidden();

  await page.reload();
  await app.switchTo("HTTP");
  await expect(app.collection(COLLECTION)).toBeVisible();
  await expect(app.request(/Tree Request/)).toBeHidden();
});

// ── 起動時の読み込みの失敗 ──────────────────────────────────────────────────

test.describe("when loading collections fails on startup", () => {
  const COLLECTION_ID = "col-load-failure";
  const REQUEST_ID = "req-load-failure";
  const ACTIVE_REQUEST_KEY = "wirexa:http:activeRequest";

  // 失敗させるのは最後の GetSidebarLayout。collections は読めているので、誤って復元すれば
  // URL の入力欄に値が入る (url を省くと空文字になり、復元されても区別できない)。
  test.use({
    seed: {
      collections: [
        {
          id: COLLECTION_ID,
          name: COLLECTION,
          requests: [
            { id: REQUEST_ID, name: REQUEST, url: "https://example.test/saved" },
          ],
        },
      ],
      getSidebarLayoutError: "rpc down",
    },
  });

  test("shows an error toast and does not restore the active request", async ({
    page,
    app,
  }) => {
    // createActiveRequestStorage の保存形式
    const saved = JSON.stringify({
      requestId: REQUEST_ID,
      collectionId: COLLECTION_ID,
    });
    await page.evaluate(
      ([key, value]) => localStorage.setItem(key, value),
      [ACTIVE_REQUEST_KEY, saved] as const,
    );

    // fixture は goto を済ませているので、reload で起動時の失敗をもう一度起こす。
    const pageErrors: string[] = [];
    page.on("pageerror", (err) => pageErrors.push(err.message));
    await page.reload();

    // HttpProvider は起動時にマウントされるので、HTTP の画面に切り替える前に出る。
    const toast = page
      .getByRole("alert")
      .filter({ hasText: "Failed to load collections" });
    await expect(toast).toHaveCount(1);
    await expect(toast).toContainText("rpc down");

    await app.switchTo("HTTP");
    await expect(page.getByText("No collections yet")).toBeVisible();
    await expect(app.urlInput).toHaveValue("");
    expect(
      await page.evaluate((key) => localStorage.getItem(key), ACTIVE_REQUEST_KEY),
    ).toBe(saved);
    expect(pageErrors).toEqual([]);
  });
});
