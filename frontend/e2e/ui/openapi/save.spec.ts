import { expect, test } from "../../fixtures/ui";

// 観点K: 保存 (Ctrl+S・Save ボタン)、dirty 表示、プレビューの表示切り替え、ファイルの D&D。
// 保存ダイアログは偽バックエンドの seed.saveFileAsPath が選ばれたものとして扱う。

const SPEC_PATH = "C:\\specs\\petstore.yaml";

const VALID_YAML = [
  "openapi: 3.0.0",
  "info:",
  "  title: Test API",
  "  version: 1.0.0",
  "paths: {}",
].join("\n");

const EDITED_YAML = VALID_YAML.replace("Test API", "Edited API");

const NO_FILE_LABEL = "No file opened — use the sidebar to open or paste a spec";

test.beforeEach(async ({ app }) => {
  await app.switchTo("OpenAPI");
});

test.describe("with a save destination", () => {
  test.use({ seed: { saveFileAsPath: SPEC_PATH } });

  test("Ctrl+S on an untitled doc opens save-as and clears the dirty marker", async ({
    app,
    fake,
  }) => {
    await app.newOpenApiDocument();
    await app.fillOpenApiEditor(VALID_YAML);
    await expect(
      app.page.getByText("untitled.yaml (untitled) *", { exact: true }),
    ).toBeVisible();

    await app.openApiEditor.press("ControlOrMeta+s");

    // 無題文書なので保存先を選ばせる。既定名と入力内容がそのまま渡る
    await fake.waitForCalls("SaveFileAs");
    expect(await fake.args("SaveFileAs")).toEqual([
      ["untitled.yaml", VALID_YAML],
    ]);
    // 保存先のファイルとして開き直され、dirty 表示が消える
    await expect(app.page.getByText(SPEC_PATH, { exact: true })).toBeVisible();
    await expect(app.page.getByText(`${SPEC_PATH} *`)).toBeHidden();
    // 最近使ったファイルに載る
    await expect(app.openApiFiles).toHaveText(["petstore.yaml"]);

    // 2 回目以降は保存ダイアログを出さず、同じパスへ書き込む
    await app.fillOpenApiEditor(EDITED_YAML);
    await expect(
      app.page.getByText(`${SPEC_PATH} *`, { exact: true }),
    ).toBeVisible();
    await app.openApiEditor.press("ControlOrMeta+s");

    await fake.waitForCalls("WriteFile");
    expect(await fake.args("WriteFile")).toEqual([[SPEC_PATH, EDITED_YAML]]);
    expect(await fake.calls("SaveFileAs")).toBe(1);
    await expect(app.page.getByText(SPEC_PATH, { exact: true })).toBeVisible();
  });
});

test.describe("with a recent file", () => {
  test.use({
    seed: { openApiFiles: [{ path: SPEC_PATH, content: VALID_YAML }] },
  });

  test("Save button writes the edited file and clears the dirty marker", async ({
    app,
    fake,
  }) => {
    await app.openApiFile("petstore.yaml").click();
    await expect(app.openApiEditor).toContainText("title: Test API");

    await app.fillOpenApiEditor(EDITED_YAML);
    await expect(
      app.page.getByText(`${SPEC_PATH} *`, { exact: true }),
    ).toBeVisible();

    await app.page.getByRole("button", { name: "Save (Ctrl+S)" }).click();

    await fake.waitForCalls("WriteFile");
    expect(await fake.args("WriteFile")).toEqual([[SPEC_PATH, EDITED_YAML]]);
    expect(await fake.calls("SaveFileAs")).toBe(0);
    await expect(app.page.getByText(SPEC_PATH, { exact: true })).toBeVisible();
  });

  test("reverting the edit clears the dirty marker without saving", async ({
    app,
    fake,
  }) => {
    await app.openApiFile("petstore.yaml").click();
    await expect(app.openApiEditor).toContainText("title: Test API");

    await app.fillOpenApiEditor(EDITED_YAML);
    await expect(
      app.page.getByText(`${SPEC_PATH} *`, { exact: true }),
    ).toBeVisible();

    // dirty は保存済みの内容との比較なので、元に戻せば消える
    await app.fillOpenApiEditor(VALID_YAML);
    await expect(app.page.getByText(SPEC_PATH, { exact: true })).toBeVisible();
    expect(await fake.calls("WriteFile")).toBe(0);
  });
});

test("Save button is shown only while a document is open", async ({ app }) => {
  const save = app.page.getByRole("button", { name: "Save (Ctrl+S)" });
  await expect(app.page.getByText(NO_FILE_LABEL)).toBeVisible();
  await expect(save).toBeHidden();

  await app.newOpenApiDocument();

  await expect(save).toBeVisible();
});

test("preview panel can be hidden and shown again", async ({ app }) => {
  await app.newOpenApiDocument();
  await app.fillOpenApiEditor(VALID_YAML);
  const preview = app.page.getByTestId("openapi-preview");
  await expect(preview).toBeAttached();

  await app.page.getByRole("button", { name: "Hide preview" }).click();

  await expect(preview).not.toBeAttached();
  // エディタは全幅で残り、内容も保たれる
  await expect(app.openApiEditor).toContainText("title: Test API");

  await app.page.getByRole("button", { name: "Show preview" }).click();

  await expect(preview).toBeAttached();
  await expect(app.openApiEditor).toContainText("title: Test API");
});

// ── ファイルの D&D ─────────────────────────────────────────────────────────────
// OS からのドロップは HTML5 の drop で内容を読み、無題文書として開く (ディスクには触れない)。

test("dropping a yaml file opens it as an untitled document", async ({
  app,
  fake,
}) => {
  const accepted = await app.dropOpenApiFile(
    app.page.getByText(NO_FILE_LABEL),
    "petstore.yaml",
    VALID_YAML,
  );

  expect(accepted).toBe(true);
  await expect(
    app.page.getByText("petstore.yaml (untitled)", { exact: true }),
  ).toBeVisible();
  await expect(app.openApiEditor).toContainText("title: Test API");
  // 読み込んだ内容が基準になるので dirty ではなく、プレビューもすぐ出る
  await expect(app.page.getByTestId("openapi-preview")).toBeAttached();
  // 無題文書なので最近使ったファイルには載らず、バックエンドにも読み書きしない
  await expect(app.page.getByText("No files opened yet")).toBeVisible();
  expect(await fake.calls("ReadFile")).toBe(0);
  expect(await fake.calls("SaveFileAs")).toBe(0);
});

test("dropping a file over unsaved text asks before replacing it", async ({
  app,
}) => {
  await app.newOpenApiDocument();
  await app.fillOpenApiEditor(VALID_YAML);
  const label = app.page.getByText("untitled.yaml (untitled) *", {
    exact: true,
  });
  await expect(label).toBeVisible();

  await app.dropOpenApiFile(label, "other.yaml", EDITED_YAML);

  const dialog = app.page.getByRole("dialog");
  await expect(dialog).toContainText(
    'You have unsaved changes in "untitled.yaml".',
  );
  await dialog.getByRole("button", { name: "Discard changes" }).click();

  await expect(
    app.page.getByText("other.yaml (untitled)", { exact: true }),
  ).toBeVisible();
  await expect(app.openApiEditor).toContainText("title: Edited API");
});
