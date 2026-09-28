import { writeFile } from "node:fs/promises";
import type { Page } from "@playwright/test";
import {
  echoRequest,
  startTestServer,
  type TestServer,
} from "../../fixtures/http-server";
import { expect, test, type WailsGo } from "../../fixtures/integration";

// 実 Go バックエンドから実サーバーへ HTTP を投げる。保存先は一時 APPDATA へ隔離済みなので
// コレクションの後始末は不要。

let server: TestServer;

test.beforeAll(async () => {
  server = await startTestServer((req, res) => {
    const pathname = new URL(req.url ?? "/", "http://localhost").pathname;
    if (pathname === "/json") {
      res.writeHead(200, {
        "Content-Type": "application/json",
        "X-Custom-Header": "test-value",
      });
      res.end(JSON.stringify({ message: "hello", status: "ok" }));
    } else if (pathname === "/echo") {
      // 受け取ったリクエスト行・ヘッダー・ボディをそのまま返す。ワイヤ形式を
      // レスポンスビューアで直接検証するため、解析はしない。
      echoRequest(req, res);
    } else if (pathname === "/multi-header") {
      res.writeHead(200, {
        "Content-Type": "text/plain",
        "Set-Cookie": ["a=1; Path=/", "b=2; Path=/"],
      });
      res.end("ok");
    } else {
      res.writeHead(404, { "Content-Type": "text/plain" });
      res.end("Not found");
    }
  });
});

test.afterAll(async () => {
  await server.close();
});

const jsonUrl = () => `http://127.0.0.1:${server.port}/json`;
const echoUrl = () => `http://127.0.0.1:${server.port}/echo`;

/** バックエンドに保存されているリクエストの URL を読む (自動保存の完了判定に使う)。 */
const savedUrl = (page: Page, requestName: RegExp) =>
  page.evaluate(async (name) => {
    const handler = (window as unknown as { go: WailsGo }).go.adapters
      .HTTPHandler;
    const collections = await handler.GetCollections();
    for (const c of collections) {
      for (const item of c.items ?? []) {
        if (new RegExp(name).test(item.name)) return item.request?.url ?? "";
      }
    }
    return undefined;
  }, requestName.source);

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

// ── 観点I-2: URL入力とリクエスト送信 ─────────────────────────────────────────

test("entering a URL and clicking send initiates a request", async ({
  page,
  app,
}) => {
  await app.urlInput.fill(jsonUrl());
  await app.sendButton.click();

  await expect(page.getByText("200", { exact: true })).toBeVisible();
  await expect(app.sendButton).toBeVisible();
});

// ── 観点I-5: レスポンス表示（ステータス・ボディ・ヘッダー） ───────────────────

test("response viewer shows status code and body", async ({ page, app }) => {
  await app.urlInput.fill(jsonUrl());
  await app.sendButton.click();

  await expect(page.getByText("200", { exact: true })).toBeVisible();
  await expect(page.getByText('"hello"', { exact: true })).toBeVisible();
});

test("response viewer headers tab shows response headers", async ({
  page,
  app,
}) => {
  await app.urlInput.fill(jsonUrl());
  await app.sendButton.click();

  await expect(page.getByText("200", { exact: true })).toBeVisible();

  await page.getByRole("tab", { name: "Headers" }).nth(1).click();

  await expect(page.getByText("content-type")).toBeVisible();
  await expect(page.getByText("x-custom-header")).toBeVisible();
});

test("response viewer headers tab shows every value of a multi-value header", async ({
  page,
  app,
}) => {
  await app.urlInput.fill(`http://127.0.0.1:${server.port}/multi-header`);
  await app.sendButton.click();

  await expect(page.getByText("200", { exact: true })).toBeVisible();

  await page.getByRole("tab", { name: "Headers" }).nth(1).click();

  // Set-Cookie は値ごとに 1 行ずつ描画される
  await expect(page.getByText("set-cookie")).toHaveCount(2);
  await expect(page.getByText("a=1; Path=/")).toBeVisible();
  await expect(page.getByText("b=2; Path=/")).toBeVisible();
});

// ── form-data の行種別（text / json / file）────────────────────────────────

// UI で組んだ行が実 Go バックエンドで multipart に組み立てられることを確認する。
// file 行はネイティブのファイルダイアログで確定したものだけが送られ、入力しただけの
// パスは許可にならない。ダイアログは E2E から操作できないため、実ファイルのバイトが
// ワイヤに載ることは Go の統合テスト (TestHTTP_SendRequest_FormDataKinds) で確認する。
test("form-data rows are sent as multipart parts and a typed file path is not uploaded", async ({
  page,
  app,
}, testInfo) => {
  const filePath = testInfo.outputPath("upload.json");
  await writeFile(filePath, '{"from":"file"}');

  await app.urlInput.fill(echoUrl());
  await page.getByRole("tab", { name: "Body" }).click();
  const bodyPanel = page.locator("#request-tabpanel-body");
  await bodyPanel.getByRole("button").first().click();
  await bodyPanel.getByRole("button", { name: "Form Data" }).click();

  // text 行
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("plain");
  await bodyPanel.getByPlaceholder("Value").fill("text value");

  // file 行: パスを入力しただけでは未確定のままで、送信は止まる
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").nth(1).fill("doc");
  const kindSelect = bodyPanel.getByTestId("form-kind-select").nth(1);
  await kindSelect.getByRole("button").first().click();
  await kindSelect.getByRole("button", { name: "File" }).click();
  await bodyPanel.getByPlaceholder("No file selected").fill(filePath);
  await expect(bodyPanel.getByText("Not confirmed")).toBeVisible();

  await app.sendButton.click();
  await expect(page.getByTestId("response-error")).toContainText("Browse");

  // 未確定の file 行を外せば、残りの行は multipart で送られる
  await bodyPanel.getByRole("button", { name: "Remove row" }).nth(1).click();
  await app.sendButton.click();
  await expect(page.getByText("200", { exact: true })).toBeVisible();

  // エコーされた multipart のワイヤ形式をそのまま確認する。
  // ボディは 1 要素にまとめて描画されるため、部分一致で見る。
  const responseBody = page.getByTestId("response-body");

  // text 行は従来どおりパートに Content-Type を付けない。
  await expect(responseBody).toContainText(
    'Content-Disposition: form-data; name="plain"',
  );
  await expect(responseBody).toContainText("text value");

  // 入力しただけのパスのファイルは送られていない。
  await expect(responseBody).not.toContainText('name="doc"');
  await expect(responseBody).not.toContainText('{"from":"file"}');
});

// ── 観点I: 入力値のワイヤ形式 ────────────────────────────────────────────────
// SendRequest の引数に載ることは UI モード (e2e/ui/http/request-send.spec.ts) で確かめている。
// ここでは Go の NetClient がそれをクエリ文字列・ヘッダーに組み立てた結果を /echo で見る。

test("headers, query params and bearer token are sent on the wire", async ({
  page,
  app,
}) => {
  await app.urlInput.fill(echoUrl());

  const params = await app.openRequestTab("Params");
  await app.addKeyValue(params, "Parameter", "page", "2");
  await app.addKeyValue(params, "Parameter", "q", "a b");

  const headers = await app.openRequestTab("Headers");
  await app.addKeyValue(headers, "Header", "X-Trace", "abc");
  await app.addKeyValue(headers, "Header", "X-Disabled", "off");
  // Add は末尾に行を足すので、最後のチェックボックスが今足した X-Disabled の行。
  await headers.getByRole("checkbox").last().uncheck();
  await app.addKeyValue(headers, "Header", "Authorization", "from-headers");

  const auth = await app.openRequestTab("Auth");
  await app.chooseOption(auth, "none", "Bearer Token");
  await auth.getByPlaceholder("Token").fill("secret-token");

  await app.sendButton.click();
  await expect(page.getByText("200", { exact: true })).toBeVisible();

  const echoed = page.getByTestId("response-body");
  // クエリは url.Values.Encode でキー順に並び、空白は + になる。
  await expect(echoed).toContainText("GET /echo?page=2&q=a+b");
  await expect(echoed).toContainText("x-trace: abc");
  // 無効にした行は送らない。
  await expect(echoed).not.toContainText("x-disabled");
  // 認証の設定は Headers タブの Authorization より優先する。
  await expect(echoed).toContainText("authorization: Bearer secret-token");
  await expect(echoed).not.toContainText("from-headers");
});

test("basic auth credentials are sent as an Authorization header", async ({
  page,
  app,
}) => {
  await app.urlInput.fill(echoUrl());

  const auth = await app.openRequestTab("Auth");
  await app.chooseOption(auth, "none", "Basic Auth");
  await auth.getByPlaceholder("Username").fill("alice");
  await auth.getByPlaceholder("Password").fill("p@ss");

  await app.sendButton.click();
  await expect(page.getByText("200", { exact: true })).toBeVisible();

  const credentials = Buffer.from("alice:p@ss").toString("base64");
  await expect(page.getByTestId("response-body")).toContainText(
    `authorization: Basic ${credentials}`,
  );
});

// ── 観点I-6: コレクションへの保存・読み込み ──────────────────────────────────

test("saved request URL is restored when request is re-opened", async ({
  page,
  app,
}) => {
  await app.createCollection("HTTP Test Collection");
  await app.addRequest("HTTP Test Collection", "First Request");

  const first = app.request(/First Request/);
  await first.click();
  await expect(first).toHaveAttribute("aria-current", "true");

  await app.urlInput.fill(jsonUrl());
  // デバウンスされた自動保存がバックエンドに届くまで待つ (固定 sleep の代わり)。
  await expect.poll(() => savedUrl(page, /First Request/)).toBe(jsonUrl());

  // 別のリクエストを選択して元のリクエストを非アクティブにする
  await app.addRequest("HTTP Test Collection", "Second Request");
  await app.request(/Second Request/).click();
  await expect(app.urlInput).toHaveValue("");

  // 元のリクエストを再度開くと URL が復元されている
  await first.click();
  await expect(first).toHaveAttribute("aria-current", "true");
  await expect(app.urlInput).toHaveValue(jsonUrl());
});

// ── 観点I-8: レスポンスのクリップボードコピー ────────────────────────────────

test("copy button writes response body to clipboard", async ({
  page,
  context,
  app,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);

  await app.urlInput.fill(jsonUrl());
  await app.sendButton.click();

  await expect(page.getByText("200", { exact: true })).toBeVisible();

  const copyBtn = page.getByRole("button", { name: "Copy body" });
  await expect(copyBtn).toBeVisible();
  await copyBtn.click();

  const clipboardText = await page.evaluate(() =>
    navigator.clipboard.readText(),
  );
  expect(clipboardText).toContain('"message"');
  expect(clipboardText).toContain('"hello"');
});
