import { expect, test } from "../../fixtures/ui";

// UDP の送信フォーム。送信内容は偽バックエンドの UDPHandler.Send に渡った引数で確かめる
// (ワイヤ上のバイト列はフルスタックの integration/udp で見る)。

const TARGET = {
  id: "target-device",
  name: "Device",
  host: "192.168.0.10",
  port: 5000,
};

/** Send の引数のうち、エンコーディングに関わらず同じ値になる部分。 */
const BASE_REQUEST = {
  host: TARGET.host,
  port: TARGET.port,
  messageLength: 0,
  endianness: "big",
};

test.use({ seed: { udpTargets: [TARGET] } });

test.beforeEach(async ({ app }) => {
  await app.switchTo("UDP");
  await app.selectUdpTarget(TARGET.name);
});

// ── 観点J: エンコーディングごとの送信内容 ────────────────────────────────────

test("json encoding sends the json payload", async ({ page, app, fake }) => {
  await page.getByRole("radio", { name: "json", exact: true }).check();
  await page.getByPlaceholder('{"key": "value"}').fill('{"temp": 21.5}');
  await app.udpSendButton.click();

  await fake.waitForCalls("Send");
  expect(await fake.args("Send")).toEqual([
    [
      {
        ...BASE_REQUEST,
        encoding: "json",
        payload: '{"temp": 21.5}',
        fixedLengthPayload: { fields: [] },
      },
    ],
  ]);
});

test("text and json keep separate payloads and fixed fields are not sent with text", async ({
  page,
  app,
  fake,
}) => {
  // ペイロード欄はラベルが無く、placeholder がエンコーディングで変わるので両方で取る。
  const payload = page.getByPlaceholder(
    /^(Enter payload\.\.\.|\{"key": "value"\})$/,
  );
  await payload.fill("hello");

  // json に切り替えると、text の入力は引き継がず json 用の入力欄になる。
  await page.getByRole("radio", { name: "json", exact: true }).check();
  await expect(payload).toHaveValue("");
  await expect(payload).toHaveAttribute("placeholder", '{"key": "value"}');
  await payload.fill('{"a": 1}');

  // fixed ではペイロード欄が消え、フィールドの編集になる。
  await page.getByRole("radio", { name: "fixed", exact: true }).check();
  await expect(payload).toBeHidden();
  await page.getByRole("button", { name: "+ Add Field" }).click();
  await app.udpFixedField(0).getByLabel("Field value").fill("x");

  // text に戻すと text の入力が残っている。
  await page.getByRole("radio", { name: "text", exact: true }).check();
  await expect(payload).toHaveValue("hello");
  await app.udpSendButton.click();

  await fake.waitForCalls("Send");
  expect(await fake.args("Send")).toEqual([
    [
      {
        ...BASE_REQUEST,
        encoding: "text",
        payload: "hello",
        fixedLengthPayload: { fields: [] },
      },
    ],
  ]);
});

// ── 観点J: 固定長フィールド ──────────────────────────────────────────────────

test("fixed-length fields show byte counts and flag invalid hex", async ({
  page,
  app,
}) => {
  await page.getByRole("radio", { name: "fixed", exact: true }).check();
  const total = page.getByText(/^Total: \d+ bytes$/);
  await expect(total).toHaveText("Total: 0 bytes");

  // 追加直後は string 型・長さ 1。バッジは「入力バイト数/長さ」。
  await page.getByRole("button", { name: "+ Add Field" }).click();
  const field = app.udpFixedField(0);
  const value = field.getByLabel("Field value");
  await expect(field.getByLabel("Field type")).toHaveValue("string");
  await expect(field.getByLabel("Field length")).toHaveValue("1");
  await expect(field.getByText("0/1", { exact: true })).toBeVisible();
  await expect(total).toHaveText("Total: 1 bytes");

  await value.fill("hello");
  await expect(field.getByText("5/1", { exact: true })).toBeVisible();
  await field.getByLabel("Field length").fill("8");
  await expect(field.getByText("5/8", { exact: true })).toBeVisible();
  await expect(total).toHaveText("Total: 8 bytes");

  // 数値型は長さ欄が消え、型のサイズで数える。範囲外の値は赤枠になる。
  await field.getByLabel("Field type").selectOption("uint16");
  await expect(field.getByLabel("Field length")).toBeHidden();
  await value.fill("65535");
  await expect(field.getByText("2 bytes", { exact: true })).toBeVisible();
  await expect(value).not.toHaveAttribute("style", /--color-destructive/);
  await expect(total).toHaveText("Total: 2 bytes");
  await value.fill("65536");
  await expect(field.getByText("out of range", { exact: true })).toBeVisible();
  await expect(value).toHaveAttribute("style", /--color-destructive/);

  // bytes 型は hex で入力する。奇数桁や hex 以外の文字は invalid hex。
  await field.getByLabel("Field type").selectOption("bytes");
  await expect(field.getByText("Value (hex)", { exact: true })).toBeVisible();
  await value.fill("0a 1");
  await expect(field.getByText("invalid hex", { exact: true })).toBeVisible();
  await expect(value).toHaveAttribute("style", /--color-destructive/);
  await value.fill("0a zz");
  await expect(field.getByText("invalid hex", { exact: true })).toBeVisible();
  await value.fill("0a 1b");
  await expect(field.getByText("2/2", { exact: true })).toBeVisible();
  await expect(value).not.toHaveAttribute("style", /--color-destructive/);

  await field.getByRole("button", { name: "Delete field" }).click();
  await expect(page.getByLabel("Field name")).toHaveCount(0);
  await expect(total).toHaveText("Total: 0 bytes");
});

test("fixed-length fields are sent in order with the chosen endianness", async ({
  page,
  app,
  fake,
}) => {
  await page.getByRole("radio", { name: "fixed", exact: true }).check();
  const addField = page.getByRole("button", { name: "+ Add Field" });

  await addField.click();
  const id = app.udpFixedField(0);
  await id.getByLabel("Field name").fill("id");
  await id.getByLabel("Field type").selectOption("uint16");
  await id.getByLabel("Field value").fill("258");

  await addField.click();
  const label = app.udpFixedField(1);
  await label.getByLabel("Field name").fill("label");
  await label.getByLabel("Field length").fill("4");
  await label.getByLabel("Field value").fill("ab");

  await addField.click();
  const raw = app.udpFixedField(2);
  await raw.getByLabel("Field name").fill("raw");
  await raw.getByLabel("Field type").selectOption("bytes");
  await raw.getByLabel("Field length").fill("2");
  await raw.getByLabel("Field value").fill("0a0b");

  await page
    .getByRole("combobox")
    .filter({ has: page.getByRole("option", { name: "little-endian" }) })
    .selectOption("little");
  await expect(page.getByText("Total: 8 bytes", { exact: true })).toBeVisible();
  await app.udpSendButton.click();

  await fake.waitForCalls("Send");
  expect(await fake.args("Send")).toEqual([
    [
      {
        ...BASE_REQUEST,
        encoding: "fixed",
        payload: "",
        endianness: "little",
        fixedLengthPayload: {
          fields: [
            { name: "id", fieldType: "uint16", length: 2, value: "258" },
            { name: "label", fieldType: "string", length: 4, value: "ab" },
            { name: "raw", fieldType: "bytes", length: 2, value: "0a0b" },
          ],
        },
      },
    ],
  ]);
});

// ── 観点E: 送信中の無効化 ────────────────────────────────────────────────────

test.describe("with a slow send", () => {
  test.use({ seed: { udpTargets: [TARGET], udpSendDelayMs: 1000 } });

  test("send button shows Sending... and is disabled while sending", async ({
    page,
    app,
    fake,
  }) => {
    await page.getByPlaceholder("Enter payload...").fill("ping");
    await app.udpSendButton.click();

    const sending = page.getByRole("button", { name: "Sending..." });
    await expect(sending).toBeDisabled();
    await expect(app.udpSendButton).toBeEnabled();
    await expect(sending).toBeHidden();
    expect(await fake.calls("Send")).toBe(1);
  });
});

// ── 観点E: 送信エラー ────────────────────────────────────────────────────────
// Go の DecodePayload (偽バックエンドも同じ規則) が不正な JSON を拒否し、トーストで通知される。

test("sending invalid json shows an error toast", async ({ page, app }) => {
  await page.getByRole("radio", { name: "json", exact: true }).check();
  await page.getByPlaceholder('{"key": "value"}').fill("{not json");
  await app.udpSendButton.click();

  const toast = page
    .getByRole("alert")
    .filter({ hasText: "Failed to send packet" });
  await expect(toast).toContainText("invalid payload: invalid JSON");
  // 入力はそのまま残り、直して送り直せる。
  await expect(page.getByPlaceholder('{"key": "value"}')).toHaveValue(
    "{not json",
  );
  await expect(app.udpSendButton).toBeEnabled();
});
