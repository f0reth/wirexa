import type { HttpRequest } from "../../../src/domain/http/types";
import { type App, expect, type FakeControl, test } from "../../fixtures/ui";

// request file は入力欄のパスでは許可にならず、ファイルダイアログ (OpenFilePicker) で
// 確定したものだけが token 付きで送られる。偽バックエンドの OpenFilePicker は
// seed.pickedFile を「ダイアログで選ばれたファイル」として返す。

const PICKED = {
  token: "tok-picked",
  name: "upload.json",
  contentType: "application/json",
};

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

/** SendRequest に最後に渡ったリクエスト (第 2 引数)。呼ばれるまで待つ。 */
async function lastSent(fake: FakeControl): Promise<HttpRequest> {
  return (await fake.lastArgs("SendRequest"))[1] as HttpRequest;
}

test.describe("file body", () => {
  test.use({ seed: { pickedFile: PICKED } });

  test("a typed path stays unconfirmed and cannot be sent", async ({
    page,
    app,
    fake,
  }) => {
    await app.urlInput.fill("https://example.com/upload");
    const bodyPanel = await app.chooseBodyType("File");
    await bodyPanel.getByPlaceholder("No file selected").fill("/tmp/upload.json");

    await expect(bodyPanel.getByText("Not confirmed")).toBeVisible();
    await app.sendButton.click();
    await expect(page.getByTestId("response-error")).toContainText("Browse");
    expect(await fake.calls("SendRequest")).toBe(0);
  });

  test("Browse confirms the file and uses the typed path only as a hint", async ({
    app,
    fake,
  }) => {
    await app.urlInput.fill("https://example.com/upload");
    const bodyPanel = await app.chooseBodyType("File");
    const input = bodyPanel.getByPlaceholder("No file selected");
    await input.fill("/tmp/upload.json");
    await bodyPanel.getByRole("button", { name: "Browse..." }).click();

    await expect(bodyPanel.getByText("Selected", { exact: true })).toBeVisible();
    await expect(input).toHaveValue("upload.json");
    expect(await fake.args("OpenFilePicker")).toEqual([["/tmp/upload.json"]]);

    await app.sendButton.click();
    const sent = await lastSent(fake);
    expect(sent.body.file?.token).toBe(PICKED.token);
    expect(JSON.stringify(sent)).not.toContain("/tmp/upload.json");
  });

  test("editing a confirmed file drops its token", async ({
    page,
    app,
    fake,
  }) => {
    await app.urlInput.fill("https://example.com/upload");
    const bodyPanel = await app.chooseBodyType("File");
    await bodyPanel.getByRole("button", { name: "Browse..." }).click();
    await expect(bodyPanel.getByText("Selected", { exact: true })).toBeVisible();

    await bodyPanel.getByPlaceholder("No file selected").fill("/tmp/other.json");
    await expect(bodyPanel.getByText("Not confirmed")).toBeVisible();
    await app.sendButton.click();
    await expect(page.getByTestId("response-error")).toContainText("Browse");
    expect(await fake.calls("SendRequest")).toBe(0);
  });

  test("clear selection removes the confirmed file", async ({ app, fake }) => {
    await app.urlInput.fill("https://example.com/upload");
    const bodyPanel = await app.chooseBodyType("File");
    const input = bodyPanel.getByPlaceholder("No file selected");
    await bodyPanel.getByRole("button", { name: "Browse..." }).click();
    await expect(bodyPanel.getByText("Selected", { exact: true })).toBeVisible();

    const clear = bodyPanel.getByRole("button", { name: "Clear selection" });
    await clear.click();

    await expect(input).toHaveValue("");
    await expect(bodyPanel.getByText("Selected", { exact: true })).toHaveCount(
      0,
    );
    await expect(clear).toHaveCount(0);

    // 未選択に戻っているので、ファイルを付けずに送られる。
    await app.sendButton.click();
    expect((await lastSent(fake)).body.file).toBeUndefined();
  });

  test("pressing Enter in the path field opens the file picker", async ({
    app,
    fake,
  }) => {
    const bodyPanel = await app.chooseBodyType("File");
    const input = bodyPanel.getByPlaceholder("No file selected");
    await input.fill("/tmp/upload.json");
    await input.press("Enter");

    await fake.waitForCalls("OpenFilePicker");
    await expect(bodyPanel.getByText("Selected", { exact: true })).toBeVisible();
  });
});

// seed.pickedFile を省くと、OpenFilePicker はダイアログのキャンセルとして空の参照を返す。
test("cancelling the file dialog keeps the typed path unconfirmed", async ({
  page,
  app,
  fake,
}) => {
  await app.urlInput.fill("https://example.com/upload");
  const bodyPanel = await app.chooseBodyType("File");
  const input = bodyPanel.getByPlaceholder("No file selected");
  await input.fill("/tmp/upload.json");

  await bodyPanel.getByRole("button", { name: "Browse..." }).click();
  await fake.waitForCalls("OpenFilePicker");

  await expect(input).toHaveValue("/tmp/upload.json");
  await expect(bodyPanel.getByText("Not confirmed")).toBeVisible();
  await app.sendButton.click();
  await expect(page.getByTestId("response-error")).toContainText("Browse");
  expect(await fake.calls("SendRequest")).toBe(0);
});

test.describe("when the file dialog fails", () => {
  test.use({
    seed: { httpRpcErrors: { OpenFilePicker: "dialog unavailable" } },
  });

  test("a failed file dialog shows an error toast", async ({ page, app }) => {
    const bodyPanel = await app.chooseBodyType("File");
    const input = bodyPanel.getByPlaceholder("No file selected");
    await input.fill("/tmp/upload.json");

    await bodyPanel.getByRole("button", { name: "Browse..." }).click();

    await expect(
      page.getByRole("alert").filter({ hasText: "Failed to open file picker" }),
    ).toContainText("dialog unavailable");
    // 入力したヒントは残り、確定はしない。
    await expect(input).toHaveValue("/tmp/upload.json");
    await expect(bodyPanel.getByText("Not confirmed")).toBeVisible();
  });
});

test.describe("form-data file row", () => {
  test.use({
    seed: {
      pickedFile: { token: "tok-png", name: "a.png", contentType: "image/png" },
    },
  });

  async function addFileRow(app: App) {
    const bodyPanel = await app.chooseBodyType("Form Data");
    await bodyPanel.getByRole("button", { name: "Add" }).click();
    await bodyPanel.getByPlaceholder("Field").fill("doc");
    await app.chooseOption(
      bodyPanel.getByTestId("form-kind-select"),
      "Text",
      "File",
    );
    return bodyPanel;
  }

  test("a typed path in a file row cannot be sent", async ({
    page,
    app,
    fake,
  }) => {
    await app.urlInput.fill("https://example.com/upload");
    const bodyPanel = await addFileRow(app);
    await bodyPanel.getByPlaceholder("No file selected").fill("/tmp/a.png");

    await expect(bodyPanel.getByText("Not confirmed")).toBeVisible();
    await app.sendButton.click();
    await expect(page.getByTestId("response-error")).toContainText("Browse");
    expect(await fake.calls("SendRequest")).toBe(0);
  });

  // Content-Type 未指定のときは、実際に送信される選択時の判定値をヒントとして出す。
  test("expanding a confirmed file row shows the Content-Type from the selection", async ({
    app,
  }) => {
    const bodyPanel = await addFileRow(app);
    await bodyPanel.getByRole("button", { name: "Browse..." }).click();

    await bodyPanel.getByRole("button", { name: "Expand row" }).click();
    await expect(bodyPanel.getByText("auto: image/png")).toBeVisible();

    // 明示した Content-Type が自動判定に勝つので、ヒントは引っ込む。
    await bodyPanel.getByPlaceholder("auto").fill("application/custom");
    await expect(bodyPanel.getByText("auto: image/png")).toHaveCount(0);
  });

  test("a confirmed file row is sent with its token", async ({
    app,
    fake,
  }) => {
    await app.urlInput.fill("https://example.com/upload");
    const bodyPanel = await addFileRow(app);
    await bodyPanel.getByRole("button", { name: "Browse..." }).click();
    await expect(bodyPanel.getByText("Selected", { exact: true })).toBeVisible();

    await app.sendButton.click();
    const sent = await lastSent(fake);
    expect(sent.body.formData?.[0].file?.token).toBe("tok-png");
  });
});

test.describe("saved file request", () => {
  test.use({
    seed: {
      pickedFile: PICKED,
      collections: [
        {
          name: "Files",
          items: [
            {
              name: "Upload",
              url: "https://example.com/upload",
              body: {
                type: "file",
                contents: {},
                file: { name: "old.bin", needsReselect: true },
              },
            },
          ],
        },
      ],
    },
  });

  test("asks to reselect the file after a restart", async ({ app }) => {
    await app.request(/Upload/).click();
    const bodyPanel = await app.openRequestTab("Body");

    await expect(bodyPanel.getByText("Reselect file")).toBeVisible();
    await expect(bodyPanel.getByPlaceholder("No file selected")).toHaveValue(
      "old.bin",
    );
  });

  // 保存済みの参照は許可にならない。ダイアログで選び直すと、その token で送れる。
  test("a file that needs reselecting can be sent after Browse", async ({
    page,
    app,
    fake,
  }) => {
    await app.request(/Upload/).click();
    const bodyPanel = await app.openRequestTab("Body");
    await expect(bodyPanel.getByText("Reselect file")).toBeVisible();

    await app.sendButton.click();
    await expect(page.getByTestId("response-error")).toContainText("Browse");
    expect(await fake.calls("SendRequest")).toBe(0);

    await bodyPanel.getByRole("button", { name: "Browse..." }).click();
    await expect(bodyPanel.getByText("Selected", { exact: true })).toBeVisible();
    await expect(bodyPanel.getByPlaceholder("No file selected")).toHaveValue(
      PICKED.name,
    );

    await app.sendButton.click();
    const sent = await lastSent(fake);
    expect(sent.body.file).toMatchObject({
      token: PICKED.token,
      name: PICKED.name,
    });
  });
});
