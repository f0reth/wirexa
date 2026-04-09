import { expect, test } from "./fixtures/app";

async function goToHttp(page: import("@playwright/test").Page) {
  await page.getByTitle("HTTP").click();
}

test.describe("HTTP Client UI", () => {
  test("コレクション一覧の表示 - startup shows collections in sidebar", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setCollections([
      { id: "col-1", name: "My API", items: [] },
      { id: "col-2", name: "Auth Service", items: [] },
    ]);
    await goToHttp(page);

    await expect(page.getByText("My API")).toBeVisible();
    await expect(page.getByText("Auth Service")).toBeVisible();
  });

  test("コレクション作成 - create dialog → name input → confirmed → added to list", async ({
    page,
    wailsMock: _wailsMock,
  }) => {
    await goToHttp(page);
    await page.getByTitle("New Collection").click();

    // Rename input appears focused after creation
    const renameInput = page.locator('input[class*="renameInput"]').last();
    await renameInput.fill("New API Collection");
    await renameInput.press("Enter");

    await expect(page.getByText("New API Collection")).toBeVisible();
  });

  test("コレクション名変更 - double-click rename → reflected in sidebar", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setCollections([
      { id: "col-1", name: "Original Name", items: [] },
    ]);
    await goToHttp(page);

    await page.getByText("Original Name").dblclick();
    const renameInput = page.locator('input[class*="renameInput"]');
    await renameInput.fill("Renamed Collection");
    await renameInput.press("Enter");

    await expect(page.getByText("Renamed Collection")).toBeVisible();
    await expect(page.getByText("Original Name")).not.toBeVisible();
  });

  test("コレクション削除 - confirm dialog → delete → removed from list", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setCollections([
      { id: "col-1", name: "To Delete", items: [] },
    ]);
    await goToHttp(page);

    await expect(page.getByText("To Delete")).toBeVisible();

    // Hover to reveal hidden action buttons, then click delete
    await page.getByText("To Delete").hover();
    await page.getByTitle("Delete collection").click();

    // Confirm dialog
    await expect(page.getByRole("dialog")).toBeVisible();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Delete" })
      .click();

    await expect(page.getByText("To Delete")).not.toBeVisible();
  });

  test("リクエスト追加 - folder and request add flow", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setCollections([
      { id: "col-1", name: "My API", items: [] },
    ]);
    await goToHttp(page);

    await expect(page.getByText("My API")).toBeVisible();

    // Add a folder to the collection
    await page.getByText("My API").hover();
    await page.getByTitle("Add folder").click();
    const folderInput = page.locator('input[class*="renameInput"]').last();
    await folderInput.fill("Auth Endpoints");
    await folderInput.press("Enter");
    await expect(page.getByText("Auth Endpoints")).toBeVisible();

    // Add a request directly to the collection
    await page.getByText("My API").hover();
    await page.getByTitle("Add request").first().click();
    const requestInput = page.locator('input[class*="renameInput"]').last();
    await requestInput.fill("Get Users");
    await requestInput.press("Enter");
    await expect(page.getByText("Get Users")).toBeVisible();
  });

  test("HTTP リクエスト実行 - URL input → send → response displayed", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setHttpResponse({
      statusCode: 200,
      statusText: "OK",
      headers: { "content-type": "application/json" },
      body: '{"hello":"world"}',
      contentType: "application/json",
      size: 17,
      timingMs: 42,
      error: "",
    });
    await goToHttp(page);

    await page
      .getByPlaceholder("https://api.example.com/endpoint")
      .fill("https://example.com/api");
    await page.getByRole("button", { name: /^Send$/ }).click();

    await expect(page.getByText("200")).toBeVisible();
    await expect(page.getByText("OK")).toBeVisible();
    await expect(page.getByText("42 ms")).toBeVisible();
  });

  test("ヘッダー追加 - add header row via KeyValueEditor", async ({
    page,
    wailsMock: _wailsMock,
  }) => {
    await goToHttp(page);

    // Switch to Headers tab in request editor
    await page.getByRole("tab", { name: "Headers" }).click();

    // Add a new header row
    await page.getByRole("button", { name: "Add" }).click();

    // Fill in the key field
    const keyInput = page.getByPlaceholder("Header").last();
    await keyInput.fill("Authorization");
    await expect(keyInput).toHaveValue("Authorization");

    // Fill in the value field
    const valueInput = page.getByPlaceholder("Value").last();
    await valueInput.fill("Bearer token123");
    await expect(valueInput).toHaveValue("Bearer token123");
  });

  test("エラーレスポンス表示 - 4xx status shown appropriately", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setHttpResponse({
      statusCode: 404,
      statusText: "Not Found",
      headers: {},
      body: "Resource not found",
      contentType: "text/plain",
      size: 18,
      timingMs: 15,
      error: "",
    });
    await goToHttp(page);

    await page
      .getByPlaceholder("https://api.example.com/endpoint")
      .fill("https://example.com/missing");
    await page.getByRole("button", { name: /^Send$/ }).click();

    await expect(page.getByText("404")).toBeVisible();
    await expect(page.getByText("Not Found")).toBeVisible();
  });
});
