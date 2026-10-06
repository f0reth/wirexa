import type { Collection } from "../../../src/domain/http/types";
import { type FakeControl, expect, test } from "../../fixtures/ui";

// HTTP ツリーの D&D (マウス方式。app.dragTreeNode を使う)。どのテストも、画面の並びに加えて
// 偽バックエンドに渡った引数と、その結果の状態を確かめる。
const ALPHA = { id: "col-alpha", name: "Alpha Collection" };
const BETA = { id: "col-beta", name: "Beta Collection" };
const REQUEST = { id: "req-moving", name: "Moving Request" };

test.use({
  seed: {
    collections: [{ ...ALPHA, items: [REQUEST] }, BETA],
  },
});

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
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

// ── 同じ親の中の並び替え ────────────────────────────────────────────────────

test.describe("reordering within the same parent", () => {
  const ORDERED = { id: "col-ordered", name: "Ordered Collection" };
  const FIRST = { id: "req-first", name: "First Request" };
  const SECOND = { id: "req-second", name: "Second Request" };
  const THIRD = { id: "req-third", name: "Third Request" };

  test.use({
    seed: { collections: [{ ...ORDERED, items: [FIRST, SECOND, THIRD] }] },
  });

  const storedOrder = async (fake: FakeControl) =>
    (await fake.collection(ORDERED.id)).items.map((i) => i.id);

  // position は移動前の並びに対する挿入位置。後ろへ動かすときは Go (偽バックエンド) が、
  // 取り除いたぶんを 1 つ詰める。
  test("dropping a request on the lower half of a sibling moves it after that sibling", async ({
    page,
    app,
    fake,
  }) => {
    await app.dragTreeNode(
      app.request(/First Request/),
      app.request(/Second Request/),
      "lower",
    );

    await fake.waitForCalls("MoveItem");
    expect(await fake.args("MoveItem")).toEqual([
      [ORDERED.id, FIRST.id, ORDERED.id, "", 2],
    ]);
    expect(await storedOrder(fake)).toEqual([SECOND.id, FIRST.id, THIRD.id]);
    await expect(page.getByRole("button", { name: /Request$/ })).toHaveText([
      /Second Request/,
      /First Request/,
      /Third Request/,
    ]);
  });

  test("dropping a request on the upper half of a sibling moves it before that sibling", async ({
    page,
    app,
    fake,
  }) => {
    await app.dragTreeNode(
      app.request(/Third Request/),
      app.request(/First Request/),
      "upper",
    );

    await fake.waitForCalls("MoveItem");
    expect(await fake.args("MoveItem")).toEqual([
      [ORDERED.id, THIRD.id, ORDERED.id, "", 0],
    ]);
    expect(await storedOrder(fake)).toEqual([THIRD.id, FIRST.id, SECOND.id]);
    await expect(page.getByRole("button", { name: /Request$/ })).toHaveText([
      /Third Request/,
      /First Request/,
      /Second Request/,
    ]);
  });

  // ドラッグを始めた行は、直後の click を 1 回無視する (ドラッグを終える mouseup が click に
  // ならないように)。別の行の上で離すと click が届かず、並び替えでは行も作り直されないので、
  // 無視する状態が残って次のクリックが効かなかった。
  test("a request can be selected with one click right after it was reordered", async ({
    app,
    fake,
  }) => {
    const first = app.request(/First Request/);
    await app.dragTreeNode(first, app.request(/Second Request/), "lower");
    await fake.waitForCalls("MoveItem");
    await expect
      .poll(() => storedOrder(fake))
      .toEqual([SECOND.id, FIRST.id, THIRD.id]);

    await first.click();
    await expect(first).toHaveAttribute("aria-current", "true");
  });
});

test("a collection can be collapsed with one click right after it was reordered", async ({
  page,
  app,
  fake,
}) => {
  const collections = page.getByRole("button", {
    name: /^(Alpha|Beta) Collection$/,
  });
  // 先頭のコレクションなのでゾーン 0・1 は隠れる。末尾 (2) へ動かす。
  await app.dragTreeNode(app.collection(ALPHA.name), app.sidebarDropZone(2));
  await fake.waitForCalls("MoveSidebarEntry");
  await expect(collections).toHaveText([BETA.name, ALPHA.name]);

  await app.collection(ALPHA.name).click();
  await expect(app.request(/Moving Request/)).toBeHidden();
});

// ── 開いているリクエストの移動 ──────────────────────────────────────────────

// 保存先のコレクションは選択したときの値なので、移動に追従しないと、Go は元のコレクションで
// リクエストを探して "request not found" を返し、以後の編集が保存されない。
test("edits made after moving the open request to another collection are saved", async ({
  page,
  app,
  fake,
}) => {
  const request = app.request(/Moving Request/);
  await request.click();
  await app.dragTreeNode(request, app.collection(BETA.name));
  await fake.waitForCalls("MoveItem");

  await app.urlInput.fill("https://example.com/moved");
  // 回数ではなく最後の保存の内容で待つ (移動の直後にも、新しい保存先へ保存し直す)。
  await expect
    .poll(async () => (await fake.args("UpdateRequest")).at(-1))
    .toMatchObject([
      BETA.id,
      { id: REQUEST.id, url: "https://example.com/moved" },
    ]);
  await expect(app.saveErrorBanner).toBeHidden();
  expect((await fake.collection(BETA.id)).items[0].request?.url).toBe(
    "https://example.com/moved",
  );

  // 選択も新しいコレクションで覚えているので、リロード後に復元される。
  await page.reload();
  await app.switchTo("HTTP");
  await expect(app.request(/Moving Request/)).toHaveAttribute(
    "aria-current",
    "true",
  );
  await expect(app.urlInput).toHaveValue("https://example.com/moved");
});

test("edits made after moving the open request to the sidebar root are saved", async ({
  page,
  app,
  fake,
}) => {
  const request = app.request(/Moving Request/);
  await request.click();
  // 先頭のアイテムなのでゾーン 0・1 は隠れる (上の NOTE を参照)。
  await app.dragTreeNode(request, app.sidebarDropZone(2));
  await fake.waitForCalls("MoveItemToSidebar");

  await app.urlInput.fill("https://example.com/rooted");
  await expect
    .poll(async () => (await fake.args("UpdateRequest")).at(-1))
    .toMatchObject([
      "__root__",
      { id: REQUEST.id, url: "https://example.com/rooted" },
    ]);
  await expect(app.saveErrorBanner).toBeHidden();
  expect((await fake.snapshot()).rootItems[0].request?.url).toBe(
    "https://example.com/rooted",
  );

  await page.reload();
  await app.switchTo("HTTP");
  await expect(app.request(/Moving Request/)).toHaveAttribute(
    "aria-current",
    "true",
  );
  await expect(app.urlInput).toHaveValue("https://example.com/rooted");
});

// 開閉の記録は再読み込みのたびに、もう無い ID を掃除する。ルートのアイテムを有効な ID に数えないと、
// ルートのフォルダは開いても次の再読み込み (追加・リネーム・移動のたびに走る) で閉じる。
test("a folder moved to the sidebar root stays expanded after adding a request to it", async ({
  app,
  fake,
}) => {
  await app.addFolder(ALPHA.name, "Rooted Folder");
  const folder = app.folder("Rooted Folder");
  await app.dragTreeNode(folder, app.sidebarDropZone(0));
  await fake.waitForCalls("MoveItemToSidebar");
  await expect
    .poll(async () => (await fake.snapshot()).rootItems.map((i) => i.name))
    .toEqual(["Rooted Folder"]);

  await folder.click();
  // 閉じたフォルダに足すとリネーム入力が出ないので、addRequest が通れば開いたままである。
  await app.addRequest("Rooted Folder", "Inside Root Folder");
  await expect(app.request(/Inside Root Folder/)).toBeVisible();
});

// ルートのフォルダの中のリクエストは collectionId が __root__ で、ルート直下には無い。保存した
// 内容がツリーに反映されないと、別のリクエストから戻ったときに古い値が編集エリアに入り、
// 自動保存がそれで上書きする。
test("edits to a request inside a folder moved to the sidebar root survive switching requests", async ({
  app,
  fake,
}) => {
  await app.addFolder(ALPHA.name, "Carrier");
  const folder = app.folder("Carrier");
  await folder.click();
  await app.addRequest("Carrier", "Carried Request");
  await app.addRequest(BETA.name, "Other Request");
  await folder.click();

  // フォルダごとルートへ出す (Alpha の中で 2 番目なので、サイドバーゾーン 0 は隠れない)。
  await app.dragTreeNode(folder, app.sidebarDropZone(0));
  await fake.waitForCalls("MoveItemToSidebar");

  await folder.click();
  await app.request(/Carried Request/).click();
  await app.urlInput.fill("https://example.com/carried");
  // 切り替えのたびに前のリクエストも保存されるので、回数ではなく最後の保存の内容で待つ。
  await expect
    .poll(async () => (await fake.args("UpdateRequest")).at(-1))
    .toMatchObject(["__root__", { url: "https://example.com/carried" }]);

  await app.request(/Other Request/).click();
  await expect(app.urlInput).toHaveValue("");
  await app.request(/Carried Request/).click();
  await expect(app.urlInput).toHaveValue("https://example.com/carried");
});

// ── フォルダの移動 ──────────────────────────────────────────────────────────

test.describe("moving folders", () => {
  const OUTER = { id: "folder-outer", name: "Outer Folder" };
  const SUB = { id: "folder-sub", name: "Sub Folder" };
  const INNER = { id: "req-inner", name: "Inner Request" };

  test.use({
    seed: {
      collections: [
        {
          ...ALPHA,
          items: [
            {
              id: OUTER.id,
              folder: OUTER.name,
              items: [INNER, { id: SUB.id, folder: SUB.name }],
            },
          ],
        },
        BETA,
      ],
    },
  });

  test("dragging a folder to another collection moves its children with it", async ({
    app,
    fake,
  }) => {
    await app.dragTreeNode(
      app.folder(OUTER.name),
      app.collection(BETA.name),
    );

    await fake.waitForCalls("MoveItem");
    expect(await fake.args("MoveItem")).toEqual([
      [ALPHA.id, OUTER.id, BETA.id, "", -1],
    ]);
    expect((await fake.collection(ALPHA.id)).items).toEqual([]);
    const [moved] = (await fake.collection(BETA.id)).items;
    expect(moved.id).toBe(OUTER.id);
    expect(moved.children.map((c) => c.id)).toEqual([INNER.id, SUB.id]);

    // 子は移動先のコレクションの中にある。
    await app.folder(OUTER.name).click();
    await expect(app.request(/Inner Request/)).toBeVisible();
    await app.collection(ALPHA.name).click();
    await expect(app.request(/Inner Request/)).toBeVisible();
    await app.collection(BETA.name).click();
    await expect(app.request(/Inner Request/)).toBeHidden();
  });

  test("dropping a folder into its own subfolder is rejected and keeps the tree", async ({
    page,
    app,
    fake,
  }) => {
    await app.folder(OUTER.name).click();
    const sub = app.folder(SUB.name);
    await sub.click();
    const before = await fake.collection(ALPHA.id);

    await app.dragTreeNode(
      app.folder(OUTER.name),
      app.childrenEndDropZone(sub),
    );

    const toast = page
      .getByRole("alert")
      .filter({ hasText: "Failed to move item" });
    await expect(toast).toContainText(
      "cannot move an item into its own subtree",
    );
    expect(await fake.args("MoveItem")).toEqual([
      [ALPHA.id, OUTER.id, ALPHA.id, SUB.id, 0],
    ]);
    expect(await fake.collection(ALPHA.id)).toEqual(before);
    await expect(sub).toBeVisible();
    await expect(app.request(/Inner Request/)).toBeVisible();
  });
});

// ── ルートのアイテム ────────────────────────────────────────────────────────

test.describe("root items", () => {
  const ROOT = { id: "req-root", name: "Root Request" };

  // サイドバーの並びは Alpha, Beta, Root Request (ルートのアイテムは末尾に並ぶ)。
  test.use({ seed: { collections: [ALPHA, BETA], rootItems: [ROOT] } });

  test("dragging a root request reorders it among collections", async ({
    page,
    app,
    fake,
  }) => {
    const entries = page.getByRole("button", {
      name: /^(Alpha Collection|Beta Collection|GET Root Request)$/,
    });
    await expect(entries).toHaveText([ALPHA.name, BETA.name, /Root Request/]);

    await app.dragTreeNode(app.request(/Root Request/), app.sidebarDropZone(0));

    await expect(entries).toHaveText([/Root Request/, ALPHA.name, BETA.name]);
    expect(await fake.args("MoveSidebarEntry")).toEqual([
      ["item", ROOT.id, 0],
    ]);
    expect((await fake.snapshot()).sidebar).toEqual([
      { kind: "item", id: ROOT.id },
      { kind: "collection", id: ALPHA.id },
      { kind: "collection", id: BETA.id },
    ]);

    await page.reload();
    await app.switchTo("HTTP");
    await expect(entries).toHaveText([/Root Request/, ALPHA.name, BETA.name]);
  });

  test("dragging a root request onto a collection moves it out of the sidebar root", async ({
    page,
    app,
    fake,
  }) => {
    await app.dragTreeNode(
      app.request(/Root Request/),
      app.collection(BETA.name),
    );

    await fake.waitForCalls("MoveItem");
    expect(await fake.args("MoveItem")).toEqual([
      ["__root__", ROOT.id, BETA.id, "", -1],
    ]);
    const { rootItems, sidebar } = await fake.snapshot();
    expect(rootItems).toEqual([]);
    expect(sidebar).toEqual([
      { kind: "collection", id: ALPHA.id },
      { kind: "collection", id: BETA.id },
    ]);
    expect((await fake.collection(BETA.id)).items.map((i) => i.id)).toEqual([
      ROOT.id,
    ]);

    // コレクションの中に入ったので、閉じると隠れる。
    const request = app.request(/Root Request/);
    await expect(request).toBeVisible();
    await app.collection(BETA.name).click();
    await expect(request).toBeHidden();

    // 出ていったアイテムはサイドバーの位置に数えないので、位置 0 は Alpha の前。
    await app.dragTreeNode(app.collection(BETA.name), app.sidebarDropZone(0));
    await expect(
      page.getByRole("button", { name: /^(Alpha|Beta) Collection$/ }),
    ).toHaveText([BETA.name, ALPHA.name]);
    expect(await fake.args("MoveSidebarEntry")).toEqual([
      ["collection", BETA.id, 0],
    ]);
  });
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

// 偽バックエンドが Go の MoveItem と同じく、自分の子孫への移動を取り外す前に拒否することを確かめる
// (拒否しないと、取り外したフォルダごと移動先が消える)。
test("moving a folder into its own subtree is rejected, like the Go backend", async ({
  fake,
}) => {
  expect(await fake.callHttp("AddFolder", ALPHA.id, "", "Outer")).toBeNull();
  const outer = (await fake.collection(ALPHA.id)).items.find(
    (i) => i.name === "Outer",
  );
  if (!outer) throw new Error("Outer is missing in the fake backend");
  expect(
    await fake.callHttp("AddFolder", ALPHA.id, outer.id, "Inner"),
  ).toBeNull();
  const before = await fake.collection(ALPHA.id);
  const inner = before.items.find((i) => i.id === outer.id)?.children[0];
  if (!inner) throw new Error("Inner is missing in the fake backend");

  const rejected = "invalid parent: cannot move an item into its own subtree";
  expect(
    await fake.callHttp("MoveItem", ALPHA.id, outer.id, ALPHA.id, inner.id, 0),
  ).toBe(rejected);
  expect(
    await fake.callHttp("MoveItem", ALPHA.id, outer.id, ALPHA.id, outer.id, 0),
  ).toBe(rejected);
  expect(await fake.collection(ALPHA.id)).toEqual(before);
});

// 偽バックエンドが Go の GetSidebarLayout と同じく、読み出すたびにコレクションと突き合わせることを
// 確かめる。ルートから出したアイテムのエントリが残ると、そのあとの並び替えの position が Go とずれる。
test("an item moved out of the sidebar root leaves the layout, like the Go backend", async ({
  fake,
}) => {
  expect(
    await fake.callHttp("MoveItemToSidebar", ALPHA.id, REQUEST.id, 0),
  ).toBeNull();
  expect((await fake.snapshot()).sidebar).toEqual([
    { kind: "item", id: REQUEST.id },
    { kind: "collection", id: ALPHA.id },
    { kind: "collection", id: BETA.id },
  ]);

  expect(
    await fake.callHttp("MoveItem", "__root__", REQUEST.id, BETA.id, "", -1),
  ).toBeNull();
  expect((await fake.snapshot()).sidebar).toEqual([
    { kind: "collection", id: ALPHA.id },
    { kind: "collection", id: BETA.id },
  ]);

  // 残ったエントリを数えないので、位置 0 は Alpha の前。
  expect(
    await fake.callHttp("MoveSidebarEntry", "collection", BETA.id, 0),
  ).toBeNull();
  expect((await fake.snapshot()).sidebar).toEqual([
    { kind: "collection", id: BETA.id },
    { kind: "collection", id: ALPHA.id },
  ]);
});
