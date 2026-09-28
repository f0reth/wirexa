import { expect, test } from "../../fixtures/ui";

// 観点K / C: 最近使ったファイルの一覧 (選択・削除・並び替え) と空状態。
// 一覧は偽バックエンドの seed.openApiFiles で用意する。並び替えはマウス方式の D&D
// (use-long-press-drag.ts → openapi-file-tree.tsx) なので App.dragTreeNode を使う。

const NO_FILE_LABEL = "No file opened — use the sidebar to open or paste a spec";

function spec(title: string): string {
  return [
    "openapi: 3.0.0",
    "info:",
    `  title: ${title}`,
    "  version: 1.0.0",
    "paths: {}",
  ].join("\n");
}

const A = { path: "C:\\specs\\a.yaml", content: spec("API A") };
const B = { path: "C:\\specs\\b.yaml", content: spec("API B") };
const C = { path: "C:\\specs\\c.yaml", content: spec("API C") };

test.beforeEach(async ({ app }) => {
  await app.switchTo("OpenAPI");
});

test("empty recents show No files opened yet", async ({ app }) => {
  await expect(app.page.getByText("No files opened yet")).toBeVisible();
  await expect(app.openApiFiles).toHaveCount(0);
  await expect(app.page.getByText(NO_FILE_LABEL)).toBeVisible();
});

test.describe("with recent files", () => {
  test.use({ seed: { openApiFiles: [A, B, C] } });

  test("selecting a recent file opens its content", async ({ app, fake }) => {
    await expect(app.openApiFiles).toHaveText(["a.yaml", "b.yaml", "c.yaml"]);
    await expect(app.page.getByText("No files opened yet")).toBeHidden();

    await app.openApiFile("b.yaml").click();

    await expect(app.page.getByText(B.path, { exact: true })).toBeVisible();
    await expect(app.openApiEditor).toContainText("title: API B");
    expect(await fake.args("ReadFile")).toEqual([[B.path]]);

    // 別のファイルへ切り替える (未編集なので確認は出ない)
    await app.openApiFile("a.yaml").click();

    await expect(app.page.getByText(A.path, { exact: true })).toBeVisible();
    await expect(app.openApiEditor).toContainText("title: API A");
    await expect(app.page.getByRole("dialog")).toBeHidden();
  });

  test("removing a recent file closes it when it was active", async ({
    app,
    fake,
  }) => {
    await app.openApiFile("a.yaml").click();
    await expect(app.page.getByText(A.path, { exact: true })).toBeVisible();

    await app.removeOpenApiFileButton("a.yaml").click();

    await expect(app.openApiFiles).toHaveText(["b.yaml", "c.yaml"]);
    expect(await fake.args("RemoveRecent")).toEqual([[A.path]]);
    // アクティブな文書が無くなり、保存先も無くなる
    await expect(app.page.getByText(NO_FILE_LABEL)).toBeVisible();
    await expect(
      app.page.getByRole("button", { name: "Save (Ctrl+S)" }),
    ).toBeHidden();
    // 一覧から外したことはリロードしても保たれる
    await app.page.reload();
    await app.switchTo("OpenAPI");
    await expect(app.openApiFiles).toHaveText(["b.yaml", "c.yaml"]);
  });

  test("removing an inactive recent file keeps the open document", async ({
    app,
  }) => {
    await app.openApiFile("a.yaml").click();
    await expect(app.page.getByText(A.path, { exact: true })).toBeVisible();

    await app.removeOpenApiFileButton("b.yaml").click();

    await expect(app.openApiFiles).toHaveText(["a.yaml", "c.yaml"]);
    await expect(app.page.getByText(A.path, { exact: true })).toBeVisible();
    await expect(app.openApiEditor).toContainText("title: API A");
  });

  test("recent files can be reordered by drag and drop", async ({
    app,
    fake,
  }) => {
    // 末尾の c を先頭の挿入ゾーンへ
    await app.dragTreeNode(app.openApiFile("c.yaml"), app.openApiDropZone(0));

    await expect(app.openApiFiles).toHaveText(["c.yaml", "a.yaml", "b.yaml"]);
    expect(await fake.args("MoveRecent")).toEqual([[C.path, 0]]);

    // 先頭の c を末尾の挿入ゾーンへ。index は移動前の一覧に対する位置 (= ファイル数)
    await app.dragTreeNode(app.openApiFile("c.yaml"), app.openApiDropZone(3));

    await expect(app.openApiFiles).toHaveText(["a.yaml", "b.yaml", "c.yaml"]);
    expect((await fake.args("MoveRecent"))[1]).toEqual([C.path, 3]);

    // ドラッグはクリックとして扱わない (ファイルは開かれない)
    await expect(app.page.getByText(NO_FILE_LABEL)).toBeVisible();
    expect(await fake.calls("ReadFile")).toBe(0);
  });

  test("reordered recent files survive a reload", async ({ app, fake }) => {
    await app.dragTreeNode(app.openApiFile("b.yaml"), app.openApiDropZone(0));
    await expect(app.openApiFiles).toHaveText(["b.yaml", "a.yaml", "c.yaml"]);

    await app.page.reload();
    await app.switchTo("OpenAPI");

    await expect(app.openApiFiles).toHaveText(["b.yaml", "a.yaml", "c.yaml"]);
    expect(
      (await fake.snapshot()).openApiRecents.map((r) => [r.name, r.order]),
    ).toEqual([
      ["b.yaml", 0],
      ["a.yaml", 1],
      ["c.yaml", 2],
    ]);
  });
});
