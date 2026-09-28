import type { Locator } from "@playwright/test";
import { type App, expect, test } from "../../fixtures/ui";

// 観点K: 未保存の文書から別の文書へ切り替えるときの確認ダイアログ。
// 切り替えの操作には、偽バックエンドの seed が要らない New（無題文書の作成）を使う。
// Save & continue で実際に保存して切り替える流れは SaveFileAs が保存先を返す seed が要るので
// Step 15 で扱う。ここでは保存ダイアログをキャンセルした場合だけを確かめる。

const VALID_YAML = [
  "openapi: 3.0.0",
  "info:",
  "  title: Test API",
  "  version: 1.0.0",
  "paths: {}",
].join("\n");

/** 無題文書を作って内容を入力し、未保存の状態にする。 */
async function createUnsavedUntitled(app: App): Promise<void> {
  await app.newOpenApiDocument();
  await expect(app.page.getByText("untitled.yaml (untitled)")).toBeVisible();
  await app.fillOpenApiEditor(VALID_YAML);
  await expect(
    app.page.getByText("untitled.yaml (untitled) *", { exact: true }),
  ).toBeVisible();
}

/** New をもう一度押して、未保存確認ダイアログが出るまで待つ。 */
async function requestNewDocument(app: App): Promise<Locator> {
  await app.newOpenApiDocument();
  const dialog = app.page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  return dialog;
}

test.beforeEach(async ({ app }) => {
  await app.switchTo("OpenAPI");
});

test("creating a new document with unsaved text asks to save", async ({
  app,
}) => {
  await createUnsavedUntitled(app);

  const dialog = await requestNewDocument(app);

  await expect(
    dialog.getByRole("heading", { name: "Unsaved changes" }),
  ).toBeVisible();
  await expect(dialog).toContainText(
    'You have unsaved changes in "untitled.yaml".',
  );
  await expect(
    dialog.getByRole("button", { name: "Save & continue" }),
  ).toBeVisible();
  await expect(
    dialog.getByRole("button", { name: "Discard changes" }),
  ).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Cancel" })).toBeVisible();
});

test("Cancel keeps the current document", async ({ app, fake }) => {
  await createUnsavedUntitled(app);

  const dialog = await requestNewDocument(app);
  await dialog.getByRole("button", { name: "Cancel" }).click();

  await expect(dialog).toBeHidden();
  await expect(app.openApiEditor).toContainText("title: Test API");
  // 未保存のまま残る
  await expect(
    app.page.getByText("untitled.yaml (untitled) *", { exact: true }),
  ).toBeVisible();
  expect(await fake.calls("SaveFileAs")).toBe(0);
});

test("Discard replaces the document", async ({ app, fake }) => {
  await createUnsavedUntitled(app);

  const dialog = await requestNewDocument(app);
  await dialog.getByRole("button", { name: "Discard changes" }).click();

  await expect(dialog).toBeHidden();
  // 新しい空の無題文書に置き換わり、入力していた内容は捨てられる
  await expect(app.openApiEditor).toHaveText("");
  await expect(
    app.page.getByText("untitled.yaml (untitled)", { exact: true }),
  ).toBeVisible();
  await expect(app.page.getByText("No valid OpenAPI spec")).toBeVisible();
  expect(await fake.calls("SaveFileAs")).toBe(0);
});

test("Save & continue keeps the document when the save dialog is cancelled", async ({
  app,
  fake,
}) => {
  await createUnsavedUntitled(app);

  const dialog = await requestNewDocument(app);
  // 偽バックエンドの SaveFileAs は空文字（保存ダイアログのキャンセル）を返す
  await dialog.getByRole("button", { name: "Save & continue" }).click();

  await expect(dialog).toBeHidden();
  // 無題文書なので保存先を選ばせる。既定名と入力内容がそのまま渡る
  await fake.waitForCalls("SaveFileAs");
  expect(await fake.args("SaveFileAs")).toEqual([
    ["untitled.yaml", VALID_YAML],
  ]);
  // 保存できなかったので切り替えは中止され、未保存のまま残る
  await expect(app.openApiEditor).toContainText("title: Test API");
  await expect(
    app.page.getByText("untitled.yaml (untitled) *", { exact: true }),
  ).toBeVisible();
});
