import { type App, expect, test } from "../../fixtures/ui";

// レスポンス表示エリアの描き分け。レスポンスの中身は seed の httpResponse で作る
// (偽バックエンドの SendRequest が既定値に上書きして返す)。

const URL = "https://example.com/items";

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

/** URL を入れて送信する。表示の待ち合わせは各テストのアサーションに任せる。 */
async function send(app: App): Promise<void> {
  await app.urlInput.fill(URL);
  await app.sendButton.click();
}

// ── 観点I: ステータスの描き分け ──────────────────────────────────────────────

for (const [statusCode, statusText] of [
  [404, "Not Found"],
  [503, "Service Unavailable"],
] as const) {
  test.describe(`${statusCode} response`, () => {
    test.use({
      seed: {
        httpResponse: { statusCode, statusText, body: '{"error":"x"}' },
      },
    });

    test(`${statusCode} response is shown with destructive badge`, async ({
      app,
    }) => {
      await send(app);

      const viewer = app.responseViewer;
      const badge = viewer.getByText(String(statusCode), { exact: true });
      await expect(badge).toBeVisible();
      await expect(badge).toHaveClass(/variantDestructive/);
      await expect(viewer.getByText(statusText, { exact: true })).toBeVisible();
      // 4xx/5xx でもボディは通常どおり表示する (接続エラーの表示にはしない)。
      await expect(viewer.getByTestId("response-body")).toContainText(
        '"error": "x"',
      );
      await expect(viewer.getByTestId("response-error")).toHaveCount(0);
    });
  });
}

test("2xx response is not shown with destructive badge", async ({ app }) => {
  await send(app);

  const badge = app.responseViewer.getByText("200", { exact: true });
  await expect(badge).toBeVisible();
  await expect(badge).not.toHaveClass(/variantDestructive/);
});

test.describe("empty body", () => {
  test.use({
    seed: {
      httpResponse: {
        statusCode: 204,
        statusText: "No Content",
        body: "",
        contentType: "",
        size: 0,
      },
    },
  });

  test("empty body shows a placeholder", async ({ app }) => {
    await send(app);

    const viewer = app.responseViewer;
    await expect(viewer.getByText("204", { exact: true })).toBeVisible();
    await expect(viewer.getByText("No response body")).toBeVisible();
    await expect(viewer.getByText("0 B", { exact: true })).toBeVisible();
  });
});

// ── 観点I: Headers / Timing タブ ─────────────────────────────────────────────

test.describe("response metadata", () => {
  test.use({
    seed: {
      httpResponse: {
        headers: {
          "Content-Type": ["application/json"],
          "Set-Cookie": ["a=1", "b=2"],
        },
        size: 2048,
        timingMs: 1234,
      },
    },
  });

  test("headers and timing tabs show the response metadata", async ({
    app,
  }) => {
    await send(app);

    const headers = await app.openResponseTab("Headers");
    await expect(headers).toContainText("Content-Type");
    await expect(headers).toContainText("application/json");
    // 同名ヘッダーの値はそれぞれ 1 行で出す。
    await expect(headers.getByText("Set-Cookie", { exact: true })).toHaveCount(
      2,
    );
    await expect(headers.getByText("a=1", { exact: true })).toBeVisible();
    await expect(headers.getByText("b=2", { exact: true })).toBeVisible();

    const timing = await app.openResponseTab("Timing");
    await expect(timing).toContainText("Total time");
    await expect(timing).toContainText("1234 ms");
    await expect(timing).toContainText("Response size");
    await expect(timing).toContainText("2.0 KB");

    // Body タブに戻ると本文が出る。
    const body = await app.openResponseTab("Body");
    await expect(body).toContainText('"message": "ok"');
  });
});

// ── 観点I: 画像・バイナリ ────────────────────────────────────────────────────

// 1x1 の PNG。Go は UTF-8 でないボディを base64 で返す (EncodeMaybeBase64)。
const PNG_1X1 =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==";

test.describe("image response", () => {
  test.use({
    seed: {
      httpResponse: {
        body: PNG_1X1,
        bodyBase64: true,
        contentType: "image/png",
        size: 70,
      },
    },
  });

  test("image response is rendered as an image", async ({ app }) => {
    await send(app);

    const image = app.responseViewer.getByRole("img", { name: "Response" });
    await expect(image).toBeVisible();
    await expect(image).toHaveAttribute(
      "src",
      `data:image/png;base64,${PNG_1X1}`,
    );
    // デコードできた (壊れた画像ではない) ことを確かめる。
    await expect
      .poll(() => image.evaluate((img: HTMLImageElement) => img.naturalWidth))
      .toBe(1);
  });
});

// 00 01 02 ff 41 ("A")。
const BINARY = "AAEC/0E=";

test.describe("binary response", () => {
  test.use({
    seed: {
      httpResponse: {
        body: BINARY,
        bodyBase64: true,
        contentType: "application/octet-stream",
        size: 5,
      },
    },
  });

  test("binary response shows hex view and save button", async ({
    app,
    fake,
  }) => {
    await send(app);

    const viewer = app.responseViewer;
    const body = viewer.getByTestId("response-body");
    await expect(body).toContainText("Binary content (5 bytes)");
    await expect(body).toContainText("00 01 02 ff 41");
    // 印字できないバイトは "." にする。
    await expect(body).toContainText("....A");
    // 文字列としてのコピーは意味を持たないので出さない。
    await expect(viewer.getByRole("button", { name: "Copy body" })).toHaveCount(
      0,
    );

    await viewer.getByRole("button", { name: "Save body to file" }).click();

    // 切り詰められていないバイナリは、メモリ上の base64 をそのまま渡して保存する。
    await fake.waitForCalls("SaveResponseBase64");
    expect(await fake.args("SaveResponseBase64")).toEqual([
      [BINARY, "application/octet-stream"],
    ]);
    expect(await fake.calls("SaveResponseBody")).toBe(0);
  });
});

// ── 観点I: 切り詰められたボディ ──────────────────────────────────────────────

test.describe("truncated response body", () => {
  test.use({
    seed: {
      httpResponse: { body: '{"partial": tru', bodyTruncated: true },
    },
  });

  test("show truncated body reveals the partial body with a warning", async ({
    app,
  }) => {
    await send(app);

    const viewer = app.responseViewer;
    const body = viewer.getByTestId("response-body");
    await expect(
      body.getByText(/Response body exceeds the size limit/),
    ).toBeVisible();
    // 選ぶまでは本文を描かない。
    await expect(body).not.toContainText('{"partial": tru');
    // 切り詰められた本文をコピーさせない。
    await expect(viewer.getByRole("button", { name: "Copy body" })).toHaveCount(
      0,
    );

    await viewer.getByRole("button", { name: "Show truncated body" }).click();

    await expect(body.getByText(/Showing truncated body/)).toBeVisible();
    await expect(body).toContainText('{"partial": tru');
    await expect(
      body.getByText(/Response body exceeds the size limit/),
    ).toBeHidden();
  });
});

test.describe("capped response body", () => {
  test.use({
    seed: {
      httpResponse: {
        body: "partial",
        contentType: "text/plain",
        bodyTruncated: true,
        bodyCapped: true,
      },
    },
  });

  test("a body cut at the hard limit says the saved file is incomplete", async ({
    app,
  }) => {
    await send(app);

    const body = app.responseViewer.getByTestId("response-body");
    await expect(body).toContainText("even the saved file will be incomplete");

    await app.responseViewer
      .getByRole("button", { name: "Show truncated body" })
      .click();
    await expect(body).toContainText("the full body is unavailable");
    await expect(body).toContainText("partial");
  });
});

// ── 観点D: JSON レスポンス内の HTML ──────────────────────────────────────────

// JSON は構文ハイライトのため innerHTML で描く。キー・値に含まれる HTML がマークアップとして
// 解釈されないこと (スクリプトが走らず、要素が作られないこと) を確かめる。
const HTML_IN_JSON = JSON.stringify({
  "<b>key</b>": '<img src=x onerror="window.__wirexaXss=1"><b>bold</b>',
});

test.describe("json response containing html", () => {
  test.use({
    seed: {
      httpResponse: { body: HTML_IN_JSON, size: HTML_IN_JSON.length },
    },
  });

  test("json response containing html is rendered as text", async ({
    page,
    app,
  }) => {
    await send(app);

    const body = app.responseViewer.getByTestId("response-body");
    await expect(body).toContainText('"<b>key</b>":');
    await expect(body).toContainText(
      '"<img src=x onerror=\\"window.__wirexaXss=1\\"><b>bold</b>"',
    );
    await expect(body.locator("img, b")).toHaveCount(0);
    expect(
      await page.evaluate(
        () => (window as unknown as { __wirexaXss?: number }).__wirexaXss,
      ),
    ).toBeUndefined();
  });
});

// ── 観点I: Copy body ────────────────────────────────────────────────────────

test("copy button writes response body to clipboard", async ({
  page,
  context,
  app,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await send(app);

  await app.responseViewer.getByRole("button", { name: "Copy body" }).click();

  // JSON は整形後の文字列をコピーする。Windows のクリップボードは改行を CRLF にして
  // 返すので LF に揃えて比べる。
  await expect
    .poll(async () =>
      (await page.evaluate(() => navigator.clipboard.readText())).replace(
        /\r\n/g,
        "\n",
      ),
    )
    .toBe('{\n  "message": "ok"\n}');
});
