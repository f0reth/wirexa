import type { Collection } from "../../../src/domain/http/types";
import { type FakeControl, expect, test } from "../../fixtures/ui";

// HTTP ツリーの D&D (マウス方式。app.dragTreeNode を使う)。どのテストも、画面の並びに加えて
// 偽バックエンドに渡った引数と、その結果の状態を確かめる。
const ALPHA = { id: "col-alpha", name: "Alpha Collection" };
const BETA = { id: "col-beta", name: "Beta Collection" };
const REQUEST = { id: "req-moving", name: "Moving Request" };

test.use({
  seed: {
    collections: [{ ...ALPHA, requests: [REQUEST] }, BETA],
  },
});

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
  await expect(app.request(/Moving Request/)).toBeVisible();
});

async function storedCollection(
  fake: FakeControl,
  id: string,
): Promise<Collection> {
  const col = (await fake.snapshot()).collections.find((c) => c.id === id);
  if (!col) throw new Error(`${id} is missing in the fake backend`);
  return col;
}

test("dragging a request onto another collection moves it", async ({
  app,
  fake,
}) => {
  await app.dragTreeNode(
    app.request(/Moving Request/),
    app.collection(BETA.name),
  );

  // コレクションの見出しに落とすと、そのコレクションの末尾に入る (position -1)
  await fake.waitForCalls("MoveItem");
  expect(await fake.args("MoveItem")).toEqual([
    [ALPHA.id, REQUEST.id, BETA.id, "", -1],
  ]);
  expect((await storedCollection(fake, ALPHA.id)).items).toEqual([]);
  expect(
    (await storedCollection(fake, BETA.id)).items.map((i) => i.id),
  ).toEqual([REQUEST.id]);

  // 移動先を閉じると隠れ、移動元を閉じても見えたまま
  const request = app.request(/Moving Request/);
  await expect(request).toBeVisible();
  await app.collection(ALPHA.name).click();
  await expect(request).toBeVisible();
  await app.collection(BETA.name).click();
  await expect(request).toBeHidden();
});

test("dragging a collection reorders the sidebar", async ({
  page,
  app,
  fake,
}) => {
  const collections = page.getByRole("button", {
    name: /^(Alpha|Beta) Collection$/,
  });
  await expect(collections).toHaveText([ALPHA.name, BETA.name]);

  await app.dragTreeNode(app.collection(BETA.name), app.sidebarDropZone(0));

  await expect(collections).toHaveText([BETA.name, ALPHA.name]);
  expect(await fake.args("MoveSidebarEntry")).toEqual([
    ["collection", BETA.id, 0],
  ]);
  expect((await fake.snapshot()).sidebar).toEqual([
    { kind: "collection", id: BETA.id },
    { kind: "collection", id: ALPHA.id },
  ]);

  // 並びは GetSidebarLayout から読み直しても保たれる
  await page.reload();
  await app.switchTo("HTTP");
  await expect(collections).toHaveText([BETA.name, ALPHA.name]);
});

test("dragging a request out of its collection makes it a root item", async ({
  app,
  fake,
}) => {
  // 末尾 (Beta の後ろ) のサイドバーゾーンに落とす。
  // NOTE: コレクション内のアイテムをドラッグすると、サイドバーの挿入ゾーンのうち「そのアイテムの
  // 親の中での位置」と同じ番号の前後 2 つが no-op 扱いで隠れる (tree-item-node.tsx の isNoOp が
  // 親の中の位置とサイドバー上の位置を区別していない)。先頭の Moving Request では 0 と 1 が
  // 隠れるので、ここでは 2 を使う。
  await app.dragTreeNode(
    app.request(/Moving Request/),
    app.sidebarDropZone(2),
  );

  await fake.waitForCalls("MoveItemToSidebar");
  expect(await fake.args("MoveItemToSidebar")).toEqual([
    [ALPHA.id, REQUEST.id, 2],
  ]);
  const { rootItems, sidebar } = await fake.snapshot();
  expect(rootItems.map((i) => i.id)).toEqual([REQUEST.id]);
  expect(sidebar).toEqual([
    { kind: "collection", id: ALPHA.id },
    { kind: "collection", id: BETA.id },
    { kind: "item", id: REQUEST.id },
  ]);
  expect((await storedCollection(fake, ALPHA.id)).items).toEqual([]);

  // どのコレクションを閉じても見えたまま
  await app.collection(ALPHA.name).click();
  await app.collection(BETA.name).click();
  await expect(app.request(/Moving Request/)).toBeVisible();
});

test("dropping a request into a folder nests it", async ({ app, fake }) => {
  await app.addFolder(ALPHA.name, "Target Folder");
  const folder = app.folder("Target Folder");
  await folder.click();

  await app.dragTreeNode(
    app.request(/Moving Request/),
    app.childrenEndDropZone(folder),
  );

  await fake.waitForCalls("MoveItem");
  const [stored] = (await storedCollection(fake, ALPHA.id)).items;
  expect(stored).toMatchObject({ type: "folder", name: "Target Folder" });
  expect(await fake.args("MoveItem")).toEqual([
    [ALPHA.id, REQUEST.id, ALPHA.id, stored.id, 0],
  ]);
  expect(stored.children.map((c) => c.id)).toEqual([REQUEST.id]);

  // フォルダを閉じると隠れる
  const request = app.request(/Moving Request/);
  await expect(request).toBeVisible();
  await folder.click();
  await expect(request).toBeHidden();
});

// 偽バックエンドが Go (cmn.InsertAt) と同じく、負の position を末尾として扱うことを確かめる。
// UI は負の position をサイドバーへ送らないので、バインディングを直接呼ぶ。
test("a negative sidebar position appends to the end, like the Go backend", async ({
  page,
  fake,
}) => {
  type Handler = Record<string, (...args: unknown[]) => Promise<unknown>>;
  const call = (method: string, ...callArgs: unknown[]) =>
    page.evaluate(
      ([m, a]) =>
        (
          window as unknown as { go: { adapters: { HTTPHandler: Handler } } }
        ).go.adapters.HTTPHandler[m](...a),
      [method, callArgs] as const,
    );

  await call("MoveSidebarEntry", "collection", ALPHA.id, -1);
  expect((await fake.snapshot()).sidebar).toEqual([
    { kind: "collection", id: BETA.id },
    { kind: "collection", id: ALPHA.id },
  ]);

  await call("MoveItemToSidebar", ALPHA.id, REQUEST.id, -1);
  expect((await fake.snapshot()).sidebar).toEqual([
    { kind: "collection", id: BETA.id },
    { kind: "collection", id: ALPHA.id },
    { kind: "item", id: REQUEST.id },
  ]);
});
