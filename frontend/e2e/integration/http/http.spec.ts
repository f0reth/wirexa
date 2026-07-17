import { writeFile } from "node:fs/promises";
import type { Page } from "@playwright/test";
import { startTestServer, type TestServer } from "../../fixtures/http-server";
import { expect, test } from "../../fixtures/integration";

// 実 Go バックエンドから実サーバーへ HTTP を投げる。保存先は一時 APPDATA へ隔離済みなので
// コレクションの後始末は不要。

let server: TestServer;

test.beforeAll(async () => {
  server = await startTestServer((req, res) => {
    if (req.url === "/json") {
      res.writeHead(200, {
        "Content-Type": "application/json",
        "X-Custom-Header": "test-value",
      });
      res.end(JSON.stringify({ message: "hello", status: "ok" }));
    } else if (req.url === "/echo") {
      // 受け取ったボディをそのまま返す。multipart のワイヤ形式を
      // レスポンスビューアで直接検証するため、解析はしない。
      const chunks: Buffer[] = [];
      req.on("data", (c: Buffer) => chunks.push(c));
      req.on("end", () => {
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end(Buffer.concat(chunks));
      });
    } else if (req.url === "/multi-header") {
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
    // Wails が window.go へ注入するバインディング。生成された .d.ts から型を借りることで、
    // Go 側の rename に追従できていない場合は tsc で落ちる。
    const handler = (
      window as unknown as {
        go: {
          adapters: {
            HTTPHandler: typeof import("../../../wailsjs/go/adapters/HTTPHandler");
          };
        };
      }
    ).go.adapters.HTTPHandler;
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

// UI で組んだ行が実 Go バックエンドで multipart に組み立てられ、file 行では
// 実ファイルのバイトがワイヤに載ることを確認する。UI e2e の fake backend は
// ファイルを読まないため、この経路はここでしか確かめられない。
test("form-data rows are sent as multipart parts with real file bytes", async ({
  page,
  app,
}, testInfo) => {
  const filePath = testInfo.outputPath("upload.json");
  await writeFile(filePath, '{"from":"file"}');

  await app.urlInput.fill(echoUrl());
  await page.getByRole("tab", { name: "Body" }).click();
  const bodyPanel = page.locator("#tabpanel-body");
  await bodyPanel.getByRole("button").first().click();
  await bodyPanel.getByRole("button", { name: "Form Data" }).click();

  // text 行
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").fill("plain");
  await bodyPanel.getByPlaceholder("Value").fill("text value");

  // file 行（Browse はネイティブダイアログなのでパスを直接入力する）
  await bodyPanel.getByRole("button", { name: "Add" }).click();
  await bodyPanel.getByPlaceholder("Field").nth(1).fill("doc");
  const kindSelect = bodyPanel.getByTestId("form-kind-select").nth(1);
  await kindSelect.getByRole("button").first().click();
  await kindSelect.getByRole("button", { name: "File" }).click();
  await bodyPanel.getByPlaceholder("No file selected").fill(filePath);

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

  // file 行はファイル名と、拡張子から自動判定した Content-Type を載せる。
  await expect(responseBody).toContainText(
    'Content-Disposition: form-data; name="doc"; filename="upload.json"',
  );
  await expect(responseBody).toContainText("Content-Type: application/json");

  // ディスク上のファイルの中身がボディに載っている（fake backend では確かめられない部分）。
  await expect(responseBody).toContainText('{"from":"file"}');
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
