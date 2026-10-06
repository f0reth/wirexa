import type { Page } from "@playwright/test";
import type { HttpRequest } from "../../../src/domain/http/types";
import { type App, expect, type FakeControl, test } from "../../fixtures/ui";

test.beforeEach(async ({ page }) => {
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(
    page.getByPlaceholder("https://api.example.com/endpoint"),
  ).toBeVisible();
});

// ── 観点I-1: HTTPメソッドの選択 ──────────────────────────────────────────────

test("can select different HTTP methods from dropdown", async ({ page }) => {
  const methodSelect = page.getByTestId("method-select");
  const trigger = methodSelect.getByRole("button").first();

  // 初期状態: GET
  await expect(trigger).toContainText("GET");

  // POST を選択
  await trigger.click();
  await methodSelect.getByRole("button", { name: "POST" }).click();
  await expect(trigger).toContainText("POST");

  // DELETE を選択
  await trigger.click();
  await methodSelect.getByRole("button", { name: "DELETE" }).click();
  await expect(trigger).toContainText("DELETE");

  // PUT を選択
  await trigger.click();
  await methodSelect.getByRole("button", { name: "PUT" }).click();
  await expect(trigger).toContainText("PUT");

  // GET に戻す
  await trigger.click();
  await methodSelect.getByRole("button", { name: "GET" }).click();
  await expect(trigger).toContainText("GET");
});

// ── 観点I-3: ボディタイプ切り替え ────────────────────────────────────────────

test("switching body type to JSON shows CodeMirror editor", async ({ app }) => {
  const bodyPanel = await app.openRequestTab("Body");
  const bodyTypeTrigger = bodyPanel.getByRole("button").first();

  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "JSON" }).click();

  await expect(app.editor(bodyPanel).root).toBeVisible();
});

test("switching body type to Text shows textarea", async ({ app }) => {
  const bodyPanel = await app.openRequestTab("Body");
  const bodyTypeTrigger = bodyPanel.getByRole("button").first();

  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "Text" }).click();

  await expect(bodyPanel.getByRole("textbox")).toBeVisible();
});

test("switching body type to Form Data shows key-value editor", async ({
  app,
}) => {
  const bodyPanel = await app.openRequestTab("Body");
  const bodyTypeTrigger = bodyPanel.getByRole("button").first();

  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "Form Data" }).click();

  await expect(bodyPanel.getByRole("button", { name: "Add" })).toBeVisible();
});

// ボディタイプを選択して Body タブの key-value エディタを開く。
async function openFormBody(app: App, bodyTypeLabel: string) {
  const bodyPanel = await app.openRequestTab("Body");
  await bodyPanel.getByRole("button").first().click();
  await bodyPanel.getByRole("button", { name: bodyTypeLabel }).click();
  return bodyPanel;
}

// 行は文字列へ直列化して導出し直していたため、空キー行が直列化で捨てられ
// Add が無反応になっていた。行が実体の state であることを担保する回帰テスト。
for (const bodyTypeLabel of ["Form Data", "Form URL Encoded"]) {
  test(`can add rows to ${bodyTypeLabel} body`, async ({ app }) => {
    const bodyPanel = await openFormBody(app, bodyTypeLabel);
    const addButton = bodyPanel.getByRole("button", { name: "Add" });

    await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(0);

    await addButton.click();
    await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(1);

    await addButton.click();
    await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(2);

    // 行は名前を持たないので、足した順 (上から) の位置で取る。
    const fields = bodyPanel.getByPlaceholder("Field");
    await fields.nth(0).fill("username");
    await fields.nth(1).fill("password");
    await expect(fields.nth(0)).toHaveValue("username");
    await expect(fields.nth(1)).toHaveValue("password");
  });
}

// チェックボックスは行の有効/無効を切り替えるものであり、行を削除してはいけない。
test("unchecking a Form Data row disables it without removing it", async ({
  app,
}) => {
  const bodyPanel = await openFormBody(app, "Form Data");

  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("token");

  const checkbox = bodyPanel.getByRole("checkbox");
  await expect(checkbox).toBeChecked();
  await checkbox.uncheck();

  await expect(checkbox).not.toBeChecked();
  await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(1);
  await expect(bodyPanel.getByPlaceholder("Field")).toHaveValue("token");
});

// キーを空にしても値ごと行が消えてはいけない。
test("clearing the key of a Form Data row keeps the row", async ({ app }) => {
  const bodyPanel = await openFormBody(app, "Form Data");

  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("k");
  await bodyPanel.getByPlaceholder("Value").fill("kept");

  await bodyPanel.getByPlaceholder("Field").fill("");

  await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(1);
  await expect(bodyPanel.getByPlaceholder("Value")).toHaveValue("kept");
});

// form-data と form-urlencoded は独立した行を持ち、切り替えで互いを壊さない。
test("Form Data and Form URL Encoded keep independent rows", async ({
  app,
}) => {
  const bodyPanel = await openFormBody(app, "Form Data");
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("from-form-data");

  await openFormBody(app, "Form URL Encoded");
  await expect(bodyPanel.getByPlaceholder("Field")).toHaveCount(0);
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("from-urlencoded");

  await openFormBody(app, "Form Data");
  await expect(bodyPanel.getByPlaceholder("Field")).toHaveValue(
    "from-form-data",
  );
});

// ── form-data の行種別と行ごとの Content-Type ────────────────────────────────

// 行を1つ追加して、その行の kind セレクタを返す。
async function addFormRow(app: App, bodyTypeLabel: string) {
  const bodyPanel = await openFormBody(app, bodyTypeLabel);
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  return { bodyPanel, kindSelect: bodyPanel.getByTestId("form-kind-select") };
}

test("a new Form Data row starts as a Text row", async ({ app }) => {
  const { bodyPanel, kindSelect } = await addFormRow(app, "Form Data");

  await expect(kindSelect.getByRole("button").first()).toContainText("Text");
  await expect(bodyPanel.getByPlaceholder("Value")).toBeVisible();
});

test("selecting the File kind swaps the value field for a file path picker", async ({
  app,
}) => {
  const { bodyPanel, kindSelect } = await addFormRow(app, "Form Data");

  await kindSelect.getByRole("button").first().click();
  await kindSelect.getByRole("button", { name: "File" }).click();

  await expect(bodyPanel.getByPlaceholder("No file selected")).toBeVisible();
  await expect(bodyPanel.getByPlaceholder("Value")).toHaveCount(0);
  await expect(bodyPanel.getByRole("button", { name: "Browse..." })).toBeVisible();
});

// value と file 参照は別フィールドなので、kind を往復しても入力が消えてはいけない。
test("switching kind back and forth keeps both the value and the file path", async ({
  app,
}) => {
  const { bodyPanel, kindSelect } = await addFormRow(app, "Form Data");
  const trigger = kindSelect.getByRole("button").first();

  await bodyPanel.getByPlaceholder("Value").fill("text kept");

  await trigger.click();
  await kindSelect.getByRole("button", { name: "File" }).click();
  await bodyPanel.getByPlaceholder("No file selected").fill("C:\\tmp\\a.png");

  await trigger.click();
  await kindSelect.getByRole("button", { name: "Text" }).click();
  await expect(bodyPanel.getByPlaceholder("Value")).toHaveValue("text kept");

  await trigger.click();
  await kindSelect.getByRole("button", { name: "File" }).click();
  await expect(bodyPanel.getByPlaceholder("No file selected")).toHaveValue(
    "C:\\tmp\\a.png",
  );
});

test("expanding a JSON row shows a JSON editor and an application/json hint", async ({
  app,
}) => {
  const { bodyPanel, kindSelect } = await addFormRow(app, "Form Data");

  await kindSelect.getByRole("button").first().click();
  await kindSelect.getByRole("button", { name: "JSON" }).click();
  await bodyPanel.getByRole("button", { name: "Expand row" }).click();

  await expect(app.editor(bodyPanel).root).toBeVisible();
  await expect(bodyPanel.getByText("auto: application/json")).toBeVisible();
});

// urlencoded はワイヤ形式にパートが無いので file も行ごとの Content-Type も表現できない。
test("Form URL Encoded rows offer no File kind and no Content-Type field", async ({
  app,
}) => {
  const { bodyPanel, kindSelect } = await addFormRow(app, "Form URL Encoded");

  // Text はトリガー側にも出ている（現在の kind）ので、選択肢側は JSON/File で見る。
  await kindSelect.getByRole("button").first().click();
  await expect(kindSelect.getByRole("button", { name: "JSON" })).toBeVisible();
  await expect(kindSelect.getByRole("button", { name: "File" })).toHaveCount(0);

  // text 行は展開しても出すものが無いのでトグルごと出さない。
  await expect(
    bodyPanel.getByRole("button", { name: "Expand row" }),
  ).toHaveCount(0);

  await kindSelect.getByRole("button", { name: "JSON" }).click();
  await bodyPanel.getByRole("button", { name: "Expand row" }).click();
  await expect(app.editor(bodyPanel).root).toBeVisible();
  await expect(bodyPanel.getByPlaceholder("auto")).toHaveCount(0);
});

test("switching body type back to none hides the editor", async ({ app }) => {
  const bodyPanel = await app.openRequestTab("Body");
  const bodyTypeTrigger = bodyPanel.getByRole("button").first();

  // JSON を選択してエディタを表示
  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "JSON" }).click();
  await expect(app.editor(bodyPanel).root).toBeVisible();

  // none に戻す
  await bodyTypeTrigger.click();
  await bodyPanel.getByRole("button", { name: "None" }).click();
  await expect(app.editor(bodyPanel).root).not.toBeVisible();
});

// ── 観点I-4: 認証設定の切り替え ──────────────────────────────────────────────

test("selecting Basic auth shows username and password fields", async ({
  app,
}) => {
  const authPanel = await app.openRequestTab("Auth");
  const authTypeTrigger = authPanel.getByRole("button").first();

  await authTypeTrigger.click();
  await authPanel.getByRole("button", { name: "Basic Auth" }).click();

  await expect(authPanel.getByPlaceholder("Username")).toBeVisible();
  await expect(authPanel.getByPlaceholder("Password")).toBeVisible();
});

test("selecting Bearer auth shows token field", async ({ app }) => {
  const authPanel = await app.openRequestTab("Auth");
  const authTypeTrigger = authPanel.getByRole("button").first();

  await authTypeTrigger.click();
  await authPanel.getByRole("button", { name: "Bearer Token" }).click();

  await expect(authPanel.getByPlaceholder("Token")).toBeVisible();
});

test("switching back to no auth hides credential fields", async ({ app }) => {
  const authPanel = await app.openRequestTab("Auth");
  const authTypeTrigger = authPanel.getByRole("button").first();

  // Basic Auth を選択
  await authTypeTrigger.click();
  await authPanel.getByRole("button", { name: "Basic Auth" }).click();
  await expect(authPanel.getByPlaceholder("Username")).toBeVisible();

  // None に戻す
  await authTypeTrigger.click();
  await authPanel.getByRole("button", { name: "None" }).click();
  await expect(authPanel.getByPlaceholder("Username")).not.toBeVisible();
  await expect(authPanel.getByPlaceholder("Password")).not.toBeVisible();
});

// ── 観点I-7: プロキシ・タイムアウト設定の入力UI ──────────────────────────────

test("can configure timeout in settings tab", async ({ app }) => {
  const settingsPanel = await app.openRequestTab("Settings");
  const timeoutInput = settingsPanel.getByLabel("Timeout (s)");

  await expect(timeoutInput).toBeVisible();
  await timeoutInput.fill("60");
  await expect(timeoutInput).toHaveValue("60");
});

test("selecting custom proxy mode shows proxy URL field", async ({ app }) => {
  const settingsPanel = await app.openRequestTab("Settings");

  // proxy ドロップダウンを開いて Custom を選択
  const proxyTrigger = settingsPanel.getByRole("button").first();
  await proxyTrigger.click();
  await settingsPanel.getByRole("button", { name: "Custom" }).click();

  await expect(settingsPanel.getByLabel("Proxy URL")).toBeVisible();
  await settingsPanel
    .getByLabel("Proxy URL")
    .fill("http://proxy.example.com:8080");
  await expect(settingsPanel.getByLabel("Proxy URL")).toHaveValue(
    "http://proxy.example.com:8080",
  );
});

test("settings tab shows TLS and redirect checkboxes", async ({ app }) => {
  const settingsPanel = await app.openRequestTab("Settings");

  await expect(
    settingsPanel.getByLabel("Verify TLS certificate"),
  ).toBeVisible();
  await expect(settingsPanel.getByLabel("Disable redirects")).toBeVisible();
});

// ── 観点I-9: リクエストヘッダーの追加 ────────────────────────────────────────

test("can add custom headers to HTTP request", async ({ app }) => {
  const headersPanel = await app.openRequestTab("Headers");

  // 行を追加
  await headersPanel.getByRole("button", { name: "Add" }).click();

  // キーと値を入力
  await headersPanel.getByPlaceholder("Header").fill("Content-Type");
  await headersPanel.getByPlaceholder("Value").fill("application/json");

  await expect(headersPanel.getByPlaceholder("Header")).toHaveValue(
    "Content-Type",
  );
  await expect(headersPanel.getByPlaceholder("Value")).toHaveValue(
    "application/json",
  );
});

test("can add multiple headers to HTTP request", async ({ app }) => {
  const headersPanel = await app.openRequestTab("Headers");
  const addButton = headersPanel.getByRole("button", { name: "Add" });

  // 2 行追加
  await addButton.click();
  await addButton.click();

  const headerInputs = headersPanel.getByPlaceholder("Header");
  await expect(headerInputs).toHaveCount(2);

  // 行は名前を持たないので、足した順 (上から) の位置で取る。
  await headerInputs.nth(0).fill("Accept");
  await headerInputs.nth(1).fill("Authorization");

  await expect(headerInputs.nth(0)).toHaveValue("Accept");
  await expect(headerInputs.nth(1)).toHaveValue("Authorization");
});

// ── 観点I: Doc タブ ──────────────────────────────────────────────────────────

test.describe("with a saved request", () => {
  test.use({
    seed: {
      collections: [
        {
          name: "Doc Collection",
          items: [{ name: "Documented", url: "https://example.com/doc" }],
        },
      ],
    },
  });

  test("doc tab text is saved with the request and restored after reload", async ({
    page,
    app,
    fake,
  }) => {
    await app.request(/Documented/).click();

    const docPanel = await app.openRequestTab("Doc");
    await app.editor(docPanel).content.click();
    await page.keyboard.type("Returns the documented item.");

    // デバウンスされた自動保存に Doc の本文が載るまで待つ。
    await expect
      .poll(async () => {
        const calls = await fake.args("UpdateRequest");
        return (calls.at(-1)?.[1] as HttpRequest | undefined)?.doc;
      })
      .toBe("Returns the documented item.");

    await page.reload();
    await app.switchTo("HTTP");
    await app.request(/Documented/).click();

    const restored = await app.openRequestTab("Doc");
    await expect(app.editor(restored).content).toHaveText(
      "Returns the documented item.",
    );
  });
});

// ── 観点I: レスポンスパネルの表示切替 ────────────────────────────────────────

test("response panel can be hidden and shown again", async ({ page, app }) => {
  const placeholder = page.getByText("Send a request to see the response");
  await expect(placeholder).toBeVisible();

  await page.getByRole("button", { name: "Hide response panel" }).click();
  await expect(placeholder).toBeHidden();
  // 隠している間もリクエスト編集エリアは使える。
  await expect(app.requestEditor).toBeVisible();

  await page.getByRole("button", { name: "Show response panel" }).click();
  await expect(placeholder).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Hide response panel" }),
  ).toBeVisible();
});

test("sending a request shows the hidden response panel", async ({
  page,
  app,
}) => {
  await page.getByRole("button", { name: "Hide response panel" }).click();
  await expect(app.responseViewer).toBeHidden();

  await app.urlInput.fill("https://example.com/items");
  await app.sendButton.click();

  await expect(
    app.responseViewer.getByText("200", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Hide response panel" }),
  ).toBeVisible();
});

// ── 観点E: 自動保存の失敗 ────────────────────────────────────────────────────

test.describe("when saving the request fails", () => {
  test.use({
    seed: {
      collections: [
        { name: "Failing Collection", items: [{ name: "Unsaved" }] },
      ],
      httpRpcErrors: { UpdateRequest: "disk is full" },
    },
  });

  test("auto-save failure shows a dismissible banner", async ({
    page,
    app,
    fake,
  }) => {
    await app.request(/Unsaved/).click();
    await app.urlInput.fill("https://example.com/unsaved");
    await fake.waitForCalls("UpdateRequest");

    const banner = page.getByText("Save failed: disk is full");
    await expect(banner).toBeVisible();
    // 失敗はトーストでも知らせる。
    await expect(
      page.getByRole("alert").filter({ hasText: "Failed to auto-save request" }),
    ).toBeVisible();
    // 入力は失われない。
    await expect(app.urlInput).toHaveValue("https://example.com/unsaved");

    await app.saveErrorBanner.getByRole("button", { name: "Dismiss" }).click();
    await expect(banner).toBeHidden();
  });
});

// ── 観点E: 送信中にリクエストを切り替える ────────────────────────────────────

// 切り替えより先に応答が届くと、切り替え時のクリアで「表示されない」が成り立ってしまうので、
// 切り替えたあとも送信中であることを確かめてから、応答が届くのを待つ。
const SWITCH_DELAY_MS = 3_000;

const switchingSeed = (bodyTruncated: boolean) => ({
  collections: [
    {
      name: "Switch Collection",
      items: [
        { name: "Sent First", url: "https://example.com/first" },
        { name: "Opened Later", url: "https://example.com/later" },
      ],
    },
  ],
  httpResponse: { bodyTruncated },
  httpResponseDelayMs: SWITCH_DELAY_MS,
});

/** Sent First を送信し、応答が届く前に Opened Later へ切り替えて、応答が届くのを待つ。 */
async function sendThenSwitch(
  page: Page,
  app: App,
  fake: FakeControl,
): Promise<void> {
  await app.request(/Sent First/).click();
  await expect(app.urlInput).toHaveValue("https://example.com/first");
  await app.sendButton.click();
  await expect(page.getByText("Sending request...")).toBeVisible();

  await app.request(/Opened Later/).click();
  await expect(app.urlInput).toHaveValue("https://example.com/later");
  await expect(page.getByText("Sending request...")).toBeVisible();

  // 送信中の表示が消えたら応答が届いている。
  await expect(app.sendButton).toBeVisible();
  expect(await fake.calls("SendRequest")).toBe(1);
}

test.describe("switching requests while sending", () => {
  test.use({ seed: switchingSeed(false) });

  test("response of a request sent before switching is not shown", async ({
    page,
    app,
    fake,
  }) => {
    await sendThenSwitch(page, app, fake);

    await expect(
      page.getByText("Send a request to see the response"),
    ).toBeVisible();
    await expect(
      app.responseViewer.getByText("200", { exact: true }),
    ).toHaveCount(0);
    // 切り詰められていない応答は backend に何も残らないので破棄の通知も無い。
    expect(await fake.calls("DiscardResponseBody")).toBe(0);
  });
});

test.describe("switching requests while sending a large response", () => {
  test.use({ seed: switchingSeed(true) });

  test("truncated response of a request sent before switching is discarded", async ({
    page,
    app,
    fake,
  }) => {
    await sendThenSwitch(page, app, fake);

    await expect(
      page.getByText("Send a request to see the response"),
    ).toBeVisible();
    // 全文の一時ファイルは送信時の execution ID で破棄する。
    await fake.waitForCalls("DiscardResponseBody");
    const [[executionId]] = await fake.args("SendRequest");
    expect(await fake.args("DiscardResponseBody")).toEqual([[executionId]]);
  });
});
