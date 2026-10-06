import type { HttpRequest } from "../../../src/domain/http/types";
import { expect, type FakeControl, test } from "../../fixtures/ui";

// 画面で入力した値が SendRequest の引数 (RPC 境界) にそのまま載ることを確かめる。
// ワイヤ上の形 (クエリ文字列・Authorization ヘッダーなど) への変換は Go 側の責務で、
// フルスタックのスイートで検証する。

const URL = "https://example.com/items";

test.beforeEach(async ({ app }) => {
  await app.switchTo("HTTP");
});

/** SendRequest に最後に渡ったリクエスト (第 2 引数)。 */
async function lastSent(fake: FakeControl): Promise<HttpRequest> {
  await fake.waitForCalls("SendRequest");
  const calls = await fake.args("SendRequest");
  return calls[calls.length - 1][1] as HttpRequest;
}

test("headers, query params and bearer token are sent with the request", async ({
  app,
  fake,
}) => {
  await app.urlInput.fill(URL);

  const params = await app.openRequestTab("Params");
  await app.addKeyValue(params, "Parameter", "page", "2");

  const headers = await app.openRequestTab("Headers");
  await app.addKeyValue(headers, "Header", "X-Trace", "abc");
  await app.addKeyValue(headers, "Header", "X-Disabled", "off");
  // 無効にした行も enabled: false のまま送られ、除外は backend が行う。
  // Add は末尾に行を足すので、最後のチェックボックスが今足した X-Disabled の行。
  await headers.getByRole("checkbox").last().uncheck();

  const auth = await app.openRequestTab("Auth");
  await app.chooseOption(auth, "none", "Bearer Token");
  await auth.getByPlaceholder("Token").fill("secret-token");

  await app.sendButton.click();
  const sent = await lastSent(fake);

  expect(sent.url).toBe(URL);
  expect(sent.params).toEqual([{ key: "page", value: "2", enabled: true }]);
  expect(sent.headers).toEqual([
    { key: "X-Trace", value: "abc", enabled: true },
    { key: "X-Disabled", value: "off", enabled: false },
  ]);
  expect(sent.auth).toMatchObject({ type: "bearer", token: "secret-token" });
});

test("basic auth credentials are sent with the request", async ({
  app,
  fake,
}) => {
  await app.urlInput.fill(URL);

  const auth = await app.openRequestTab("Auth");
  await app.chooseOption(auth, "none", "Basic Auth");
  await auth.getByPlaceholder("Username").fill("alice");
  await auth.getByPlaceholder("Password").fill("p@ss");

  await app.sendButton.click();
  const sent = await lastSent(fake);

  expect(sent.auth).toMatchObject({
    type: "basic",
    username: "alice",
    password: "p@ss",
  });
});

test("method and text body are sent with the request", async ({
  app,
  fake,
}) => {
  await app.urlInput.fill(URL);
  await app.selectMethod("PUT");

  const body = await app.openRequestTab("Body");
  await app.chooseOption(body, "none", "Text");
  await body.getByPlaceholder("Enter body content...").fill("hello body");

  await app.sendButton.click();
  const sent = await lastSent(fake);

  expect(sent.method).toBe("PUT");
  expect(sent.body.type).toBe("text");
  expect(sent.body.contents.text).toBe("hello body");
});

test("request settings are sent with the request", async ({ app, fake }) => {
  await app.urlInput.fill(URL);

  const settings = await app.openRequestTab("Settings");
  await settings.getByLabel("Timeout (s)").fill("5");
  await settings.getByLabel("Max Response Body (MB)").fill("3");
  await app.chooseOption(settings, "none", "Custom");
  await settings.getByLabel("Proxy URL").fill("http://proxy.local:8080");
  // 既定は検証しない (insecureSkipVerify: true)。チェックを入れると検証する。
  await settings.getByLabel("Verify TLS certificate").check();
  await settings.getByLabel("Disable redirects").check();

  await app.sendButton.click();
  const sent = await lastSent(fake);

  expect(sent.settings).toEqual({
    timeoutSec: 5,
    maxResponseBodyMB: 3,
    proxyMode: "custom",
    proxyURL: "http://proxy.local:8080",
    insecureSkipVerify: false,
    disableRedirects: true,
  });
});

// タイムアウトの欄は 0 を空欄で表す。空にすると 0 が送られ、backend の既定値が使われる。
test("clearing the timeout sends 0 so the backend default applies", async ({
  app,
  fake,
}) => {
  await app.urlInput.fill(URL);

  const settings = await app.openRequestTab("Settings");
  const timeout = settings.getByLabel("Timeout (s)");
  await timeout.fill("5");
  await timeout.fill("");

  await app.sendButton.click();
  const sent = await lastSent(fake);

  expect(sent.settings.timeoutSec).toBe(0);
});

test("json body is sent with the request", async ({ app, fake }) => {
  await app.urlInput.fill(URL);
  await app.selectMethod("POST");

  const body = await app.chooseBodyType("JSON");
  await app.editor(body).fill('{"name": "wirexa"}');

  await app.sendButton.click();
  const sent = await lastSent(fake);

  expect(sent.method).toBe("POST");
  expect(sent.body.type).toBe("json");
  expect(sent.body.contents.json).toBe('{"name": "wirexa"}');
});

// JSON を選んだだけのときに出る雛形は表示だけで、本文としては送らない。
test("an untouched json body sends no content", async ({ app, fake }) => {
  await app.urlInput.fill(URL);

  const body = await app.chooseBodyType("JSON");
  await expect(app.editor(body).content).toContainText('"": ""');

  await app.sendButton.click();
  const sent = await lastSent(fake);

  expect(sent.body.type).toBe("json");
  expect(sent.body.contents).toEqual({});
});
