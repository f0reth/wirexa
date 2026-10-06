import type { Locator, Page } from "@playwright/test";
import type { HttpRequest } from "../../../src/domain/http/types";
import { type App, type FakeControl, expect, test } from "../../fixtures/ui";

// HTTP ツリーのフォルダ／リクエスト操作 (D&D 以外)。コレクション単位の操作は
// sidebar-operations.spec.ts にある。フォルダは UI で作る (追加の操作も通すため)。
const COLLECTION = "Tree Collection";
const REQUEST = "Tree Request";

test.use({
  seed: { collections: [{ name: COLLECTION, items: [{ name: REQUEST }] }] },
});

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

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
  expect((await fake.collection(COLLECTION)).items).toEqual([]);
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
  const items = (await fake.collection(COLLECTION)).items;
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

// 開いているリクエストが消えたら、編集エリアは保存せずに未選択へ戻す。残すと、消えた
// リクエストへ自動保存しようとして "request not found" の Save failed が出続ける。
test.describe("deleting the open request", () => {
  const ACTIVE_REQUEST_KEY = "wirexa:http:activeRequest";
  const OPEN = { id: "req-open", name: "Open Request" };
  const HELD = { id: "req-held", name: "Held Request" };
  const SURVIVOR = { id: "req-survivor", name: "Survivor" };

  test.use({
    seed: {
      collections: [
        {
          name: COLLECTION,
          items: [OPEN, { folder: "Holder", items: [HELD] }, SURVIVOR],
        },
      ],
    },
  });

  const savedSelection = (page: Page) =>
    page.evaluate((key) => localStorage.getItem(key), ACTIVE_REQUEST_KEY);

  /** request を開いて編集し、保存されるのを待つ。 */
  async function openAndEdit(
    app: App,
    fake: FakeControl,
    request: Locator,
  ): Promise<void> {
    await request.click();
    await app.urlInput.fill("https://example.com/doomed");
    await fake.waitForCalls("UpdateRequest");
  }

  /**
   * 編集エリアが未選択に戻り、消えたリクエストへの保存が起きていないことを確かめる。
   * 「保存が起きない」は待っても確かめられないので、残ったリクエストを編集して保存させ、
   * savesBefore 以降の保存がそのリクエストのものだけであることを見る。
   */
  async function expectEditorCleared(
    page: Page,
    app: App,
    fake: FakeControl,
    savesBefore: number,
  ): Promise<void> {
    await expect(app.urlInput).toHaveValue("");
    await expect.poll(() => savedSelection(page)).toBeNull();

    await app.request(/Survivor/).click();
    await app.urlInput.fill("https://example.com/survivor");
    await expect
      .poll(async () => (await fake.args("UpdateRequest")).at(-1))
      .toMatchObject([
        expect.anything(),
        { id: SURVIVOR.id, url: "https://example.com/survivor" },
      ]);
    const later = (await fake.args("UpdateRequest")).slice(savesBefore);
    expect(later.map(([, req]) => (req as HttpRequest).id)).toEqual(
      later.map(() => SURVIVOR.id),
    );
    await expect(app.saveErrorBanner).toBeHidden();
    await expect(
      page.getByRole("alert").filter({ hasText: "Failed to" }),
    ).toHaveCount(0);
  }

  test("deleting the open request clears the editor", async ({
    page,
    app,
    fake,
  }) => {
    const request = app.request(/Open Request/);
    await openAndEdit(app, fake, request);
    expect(await savedSelection(page)).not.toBeNull();
    const saves = await fake.calls("UpdateRequest");

    await app.rowAction(request, "Delete request").click();
    await app.confirmDelete();
    await expect(request).toBeHidden();

    await expectEditorCleared(page, app, fake, saves);
  });

  test("deleting the folder that holds the open request clears the editor", async ({
    page,
    app,
    fake,
  }) => {
    await app.folder("Holder").click();
    await openAndEdit(app, fake, app.request(/Held Request/));
    const saves = await fake.calls("UpdateRequest");

    await app.rowAction(app.folder("Holder"), "Delete folder").click();
    await app.confirmDelete();
    await expect(app.folder("Holder")).toBeHidden();

    await expectEditorCleared(page, app, fake, saves);
  });
});

// ── リネーム ────────────────────────────────────────────────────────────────

test("can rename a request by double-click", async ({ app, fake }) => {
  await app.startRename(app.request(/Tree Request/), REQUEST);
  await app.confirmRename("Renamed Request");

  await expect(app.request(/Renamed Request/)).toBeVisible();
  await expect(app.request(/Tree Request/)).toBeHidden();
  const items = (await fake.collection(COLLECTION)).items;
  expect(items.map((i) => i.name)).toEqual(["Renamed Request"]);
});

test("can rename a folder by double-click", async ({ app, fake }) => {
  await app.addFolder(COLLECTION, "Old Folder");

  await app.startRename(app.folder("Old Folder"), "Old Folder");
  await app.confirmRename("New Folder Name");

  await expect(app.folder("New Folder Name")).toBeVisible();
  await expect(app.folder("Old Folder")).toBeHidden();
  const items = (await fake.collection(COLLECTION)).items;
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
  const items = (await fake.collection(COLLECTION)).items;
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
  const folder = (await fake.collection(COLLECTION)).items.find(
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
  const outer = (await fake.collection(COLLECTION)).items.find(
    (i) => i.name === "Outer Folder",
  );
  expect(outer?.children.map((c) => [c.type, c.name])).toEqual([
    ["folder", "Inner Folder"],
  ]);
});

test("deleting a root-level request removes it from the sidebar layout", async ({
  app,
  fake,
}) => {
  const request = await app.createRootRequest("Root Request");
  expect((await fake.snapshot()).sidebar).toHaveLength(2);

  await app.rowAction(request, "Delete request").click();
  await app.confirmDelete();

  await expect(request).toBeHidden();
  const { rootItems, sidebar } = await fake.snapshot();
  expect(rootItems).toEqual([]);
  expect(sidebar.map((e) => e.kind)).toEqual(["collection"]);
});

test("the add menu closes when clicking outside it", async ({ app }) => {
  await app.addMenuButton.click();
  await expect(app.addMenuItem("New Collection")).toBeVisible();

  await app.urlInput.click();
  await expect(app.addMenuItem("New Collection")).toBeHidden();
  await expect(app.addMenuItem("New Request")).toBeHidden();
});

// ── 操作の失敗 (観点E) ──────────────────────────────────────────────────────

test.describe("when a collection operation fails", () => {
  test.use({
    seed: {
      collections: [{ name: COLLECTION, items: [{ name: REQUEST }] }],
      httpRpcErrors: {
        DeleteItem: "disk is full",
        AddRequest: "disk is full",
        AddFolder: "disk is full",
        CreateCollection: "disk is full",
      },
    },
  });

  test("a failed delete shows an error toast and keeps the item", async ({
    page,
    app,
    fake,
  }) => {
    const request = app.request(/Tree Request/);
    await app.rowAction(request, "Delete request").click();
    await app.confirmDelete();

    await expect(
      page.getByRole("alert").filter({ hasText: "Failed to delete item" }),
    ).toContainText("disk is full");
    await expect(page.getByRole("dialog")).toBeHidden();
    await expect(request).toBeVisible();
    expect((await fake.collection(COLLECTION)).items.map((i) => i.name)).toEqual([
      REQUEST,
    ]);
  });

  // 追加に成功したときだけ、足したアイテムのリネーム入力を開く。
  for (const [action, label] of [
    ["Add request", "Failed to add request"],
    ["Add folder", "Failed to add folder"],
  ] as const) {
    test(`a failed "${action}" shows an error toast and does not start renaming`, async ({
      page,
      app,
      fake,
    }) => {
      await app.rowAction(app.collection(COLLECTION), action).click();

      await expect(
        page.getByRole("alert").filter({ hasText: label }),
      ).toContainText("disk is full");
      await expect(app.renameInput).toHaveCount(0);
      expect((await fake.collection(COLLECTION)).items.map((i) => i.name)).toEqual([
        REQUEST,
      ]);
    });
  }

  test("a failed collection creation shows an error toast and does not start renaming", async ({
    page,
    app,
    fake,
  }) => {
    await app.addMenuButton.click();
    await app.addMenuItem("New Collection").click();

    await expect(
      page
        .getByRole("alert")
        .filter({ hasText: "Failed to create collection" }),
    ).toContainText("disk is full");
    await expect(app.renameInput).toHaveCount(0);
    expect((await fake.snapshot()).collections.map((c) => c.name)).toEqual([
      COLLECTION,
    ]);
  });
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
          items: [
            { id: REQUEST_ID, name: REQUEST, url: "https://example.test/saved" },
          ],
        },
      ],
      httpRpcErrors: { GetSidebarLayout: "rpc down" },
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

  // 一覧は 3 つとも読めたときだけ反映するので、サイドバーは空のまま。作成そのものは成功して
  // いるが、そのあとの再読み込みがまた失敗するので、失敗として知らせる。
  test("creating a collection after a failed load reports the failure and keeps existing collections", async ({
    page,
    app,
    fake,
  }) => {
    await app.addMenuButton.click();
    await app.addMenuItem("New Collection").click();

    await expect(
      page
        .getByRole("alert")
        .filter({ hasText: "Failed to create collection" }),
    ).toContainText("rpc down");
    await expect(app.renameInput).toHaveCount(0);
    await expect(page.getByText("No collections yet")).toBeVisible();
    expect((await fake.snapshot()).collections.map((c) => c.name)).toEqual([
      COLLECTION,
      "New Collection",
    ]);
  });
});
