import type { Page } from "@playwright/test";
import { type App, expect, test } from "../../fixtures/ui";

// 未接続のブローカーでの Publish タブ。プリセットはバックエンドではなく localStorage
// (mqtt:presets) に保存される。初回起動時は空の "no name" プリセットが 1 件作られ、選択される。

const BROKER = { id: "profile-local", name: "Local Broker" };

test.use({ seed: { mqttProfiles: [BROKER] } });

test.beforeEach(async ({ page, app }) => {
  await app.selectBroker(BROKER.name);
  await page.getByRole("tab", { name: "Publish" }).click();
});

/** Publish フォームの入力欄と送信ボタン。 */
function publishForm(app: App) {
  const form = app.mqttSection("Publish");
  return {
    form,
    topic: form.getByPlaceholder("Topic", { exact: true }),
    payload: form.getByPlaceholder("Message payload"),
    retain: form.getByRole("checkbox", { name: "Retain" }),
    publish: form.getByRole("button", { name: "Publish", exact: true }),
  };
}

/** 選択中のプリセットの名前を、行内の入力欄で書き換えて確定する。 */
async function renameSelectedPreset(
  app: App,
  topic: string,
  name: string,
): Promise<void> {
  const input = app.mqttPreset(topic).getByRole("textbox");
  await input.fill(name);
  await input.press("Enter");
  await expect(input).not.toBeFocused();
}

/** 保存されているプリセット (localStorage の mqtt:presets)。 */
async function storedPresets(page: Page): Promise<unknown> {
  const raw = await page.evaluate(() => localStorage.getItem("mqtt:presets"));
  return JSON.parse(raw ?? "null");
}

// ── 観点H: 未接続時の Publish ────────────────────────────────────────────────

test("publish button is disabled while offline", async ({ app, fake }) => {
  const { topic, payload, publish } = publishForm(app);
  await topic.fill("devices/lamp");
  await payload.fill("on");

  await expect(publish).toBeDisabled();
  expect(await fake.calls("Publish")).toBe(0);
});

// ── 観点H: プリセットの追加・選択・改名・削除 ─────────────────────────────────

test("presets can be added, selected, renamed and deleted", async ({
  page,
  app,
}) => {
  const { form, topic, payload } = publishForm(app);

  // 初回起動時の既定プリセット。選択中なので名前は入力欄に出る。
  await expect(app.mqttPresets).toHaveCount(1);
  await expect(app.mqttPresets.getByRole("textbox")).toHaveValue("no name");

  // フォームの入力は選択中のプリセットに書き戻される。
  await topic.fill("sensors/a");
  await payload.fill("payload-a");
  await renameSelectedPreset(app, "sensors/a", "Preset A");

  // 追加したプリセットが選ばれ、フォームは空に戻る。
  await app.addMqttPresetButton.click();
  await expect(app.mqttPresets).toHaveCount(2);
  await expect(topic).toHaveValue("");
  await expect(payload).toHaveValue("");
  await topic.fill("sensors/b");
  await payload.fill("payload-b");
  await app.chooseOption(form, "0", "QoS 1");
  await renameSelectedPreset(app, "sensors/b", "Preset B");
  await expect(app.mqttPreset("sensors/b")).toContainText("QoS 1");
  // 選ばれていないプリセットは名前をテキストで出す。
  await expect(app.mqttPreset("sensors/a")).toContainText("Preset A");

  // 選び直すと、そのプリセットの内容がフォームに読み込まれる。
  await app.mqttPreset("sensors/a").getByText("sensors/a").click();
  await expect(topic).toHaveValue("sensors/a");
  await expect(payload).toHaveValue("payload-a");
  await expect(
    app.mqttPreset("sensors/a").getByRole("textbox"),
  ).toHaveValue("Preset A");
  await expect(app.mqttPreset("sensors/b")).toContainText("Preset B");

  // 削除は確認を挟まない。
  await app.deleteMqttPresetButton("sensors/b").click();
  await expect(app.mqttPreset("sensors/b")).toHaveCount(0);
  await expect(app.mqttPresets).toHaveCount(1);

  await app.deleteMqttPresetButton("sensors/a").click();
  await expect(app.mqttPresets).toHaveCount(0);
  await expect(page.getByText("No saved presets")).toBeVisible();
  expect(await storedPresets(page)).toEqual([]);
});

// ── 観点C・F: プリセットの並び替えと永続化 ─────────────────────────────────────

test("publish presets survive a reload", async ({ page, app }) => {
  const { topic, payload, retain } = publishForm(app);
  await topic.fill("sensors/a");
  await payload.fill("payload-a");
  await renameSelectedPreset(app, "sensors/a", "Preset A");

  await app.addMqttPresetButton.click();
  await topic.fill("sensors/b");
  await payload.fill("payload-b");
  await retain.check();
  await renameSelectedPreset(app, "sensors/b", "Preset B");

  await expect(app.mqttPresets).toHaveText([/sensors\/a/, /sensors\/b/]);
  await app.dragListRowBefore(
    app.mqttPreset("sensors/b"),
    app.mqttPreset("sensors/a"),
  );
  await expect(app.mqttPresets).toHaveText([/sensors\/b/, /sensors\/a/]);

  expect(await storedPresets(page)).toEqual([
    expect.objectContaining({
      name: "Preset B",
      topic: "sensors/b",
      payload: "payload-b",
      qos: 0,
      retain: true,
    }),
    expect.objectContaining({
      name: "Preset A",
      topic: "sensors/a",
      payload: "payload-a",
      qos: 0,
      retain: false,
    }),
  ]);

  // リロード後は最後に選んだブローカーが復元される。プリセットはどれも選ばれていない。
  await page.reload();
  await page.getByRole("tab", { name: "Publish" }).click();
  await expect(app.mqttPresets).toHaveText([
    /Preset B.*sensors\/b.*Retained/,
    /Preset A.*sensors\/a/,
  ]);
  await expect(topic).toHaveValue("");

  await app.mqttPreset("sensors/b").getByText("sensors/b").click();
  await expect(topic).toHaveValue("sensors/b");
  await expect(payload).toHaveValue("payload-b");
  await expect(retain).toBeChecked();
});

// ── 観点G: キーボードでのプリセット選択 ──────────────────────────────────────

test("Enter on a focused preset selects it", async ({ app }) => {
  const { topic } = publishForm(app);
  await topic.fill("sensors/a");
  await app.addMqttPresetButton.click();
  await expect(app.mqttPresets).toHaveCount(2);
  await topic.fill("sensors/b");

  const presetA = app.mqttPreset("sensors/a");
  await presetA.focus();
  await presetA.press("Enter");

  await expect(topic).toHaveValue("sensors/a");
  // 選択中のプリセットは名前を入力欄で出す。
  await expect(presetA.getByRole("textbox")).toBeVisible();
  await expect(app.mqttPreset("sensors/b").getByRole("textbox")).toHaveCount(0);
});

test("Escape while renaming a preset restores its name", async ({
  page,
  app,
}) => {
  const { topic } = publishForm(app);
  await topic.fill("sensors/a");
  await renameSelectedPreset(app, "sensors/a", "Preset A");
  const presetA = app.mqttPreset("sensors/a");
  const input = presetA.getByRole("textbox");
  const storedNames = async () =>
    ((await storedPresets(page)) as Array<{ name: string }>).map((p) => p.name);

  // Escape は入力を捨てる。
  await input.fill("Discarded");
  await input.press("Escape");
  await expect(input).not.toBeFocused();
  await expect(input).toHaveValue("Preset A");
  expect(await storedNames()).toEqual(["Preset A"]);

  // フォーカスした行は Space でも選べる。
  await app.addMqttPresetButton.click();
  await expect(app.mqttPresets).toHaveCount(2);
  await expect(topic).toHaveValue("");
  await presetA.focus();
  await presetA.press("Space");
  await expect(topic).toHaveValue("sensors/a");
  await expect(input).toHaveValue("Preset A");

  // 空の名前では確定せず、入力欄は元の名前に戻る。空白だけでも同じ。
  for (const blank of ["", "   "]) {
    await input.fill(blank);
    await input.press("Enter");
    await expect(input).not.toBeFocused();
    await expect(input).toHaveValue("Preset A");
  }
  expect(await storedNames()).toEqual(["Preset A", "no name"]);
  await app.mqttPresets.filter({ hasNotText: "sensors/a" }).focus();
  await page.keyboard.press("Enter");
  await expect(presetA).toContainText("Preset A");
});

// ── 観点H: 選択中のプリセットの削除 ──────────────────────────────────────────

test("form edits after deleting the selected preset are not written to another preset", async ({
  page,
  app,
}) => {
  const { topic, payload } = publishForm(app);
  await topic.fill("sensors/a");
  await payload.fill("payload-a");
  await renameSelectedPreset(app, "sensors/a", "Preset A");
  await app.addMqttPresetButton.click();
  await topic.fill("sensors/b");
  await expect(app.mqttPresets).toHaveCount(2);
  const kept = [
    expect.objectContaining({
      name: "Preset A",
      topic: "sensors/a",
      payload: "payload-a",
    }),
  ];

  // 選択中のプリセットを消す。
  await app.deleteMqttPresetButton("sensors/b").click();
  await expect(app.mqttPresets).toHaveCount(1);
  expect(await storedPresets(page)).toEqual(kept);

  // どのプリセットも選ばれていないので、フォームの入力はどこにも書き戻さない。
  await topic.fill("sensors/edited");
  await payload.fill("payload-edited");

  await expect(app.mqttPreset("sensors/a")).toContainText("Preset A");
  await expect(app.mqttPreset("sensors/edited")).toHaveCount(0);
  expect(await storedPresets(page)).toEqual(kept);
});

// ── 観点F: 壊れた保存値 ──────────────────────────────────────────────────────
// localStorage の値は形を確かめて読み、読めない値は既定値として扱う (local-storage.ts)。

test.describe("malformed localStorage", () => {
  const VALID = {
    id: "preset-valid",
    name: "Valid Preset",
    topic: "sensors/valid",
    payload: "ok",
    qos: 1,
    retain: true,
  };

  /** 保存値を raw (JSON にしない文字列) で書いて読み込み直す。 */
  async function reloadWithStorage(
    page: Page,
    values: Record<string, string>,
  ): Promise<void> {
    await page.evaluate((entries) => {
      for (const [key, value] of entries) localStorage.setItem(key, value);
    }, Object.entries(values));
    await page.reload();
  }

  test("malformed mqtt values in localStorage fall back to defaults", async ({
    page,
    app,
    pageErrors,
  }) => {
    // 配列の中の形の違う要素だけを捨て、読めるプリセットは残す。
    await reloadWithStorage(page, {
      "mqtt:presets": JSON.stringify([
        { ...VALID, id: "preset-bad-qos", qos: 3 },
        VALID,
        "not-a-preset",
        null,
        { id: "preset-no-topic", name: "No Topic", payload: "", qos: 0 },
      ]),
      "mqtt:profileOrder": JSON.stringify({ not: "an array" }),
      "mqtt:lastActiveProfileId": JSON.stringify({ not: "a string" }),
    });

    // 最後に選んだブローカーは読めないので、どれも選ばれない。一覧は出る。
    await expect(app.broker(BROKER.name)).toBeVisible();
    await expect(app.brokerConnectButton).toBeHidden();
    await app.selectBroker(BROKER.name);
    await page.getByRole("tab", { name: "Publish" }).click();
    await expect(app.mqttPresets).toHaveText([/Valid Preset.*sensors\/valid/]);
    await expect(app.mqttPreset("sensors/valid")).toContainText("QoS 1");
    await expect(app.mqttPreset("sensors/valid")).toContainText("Retained");

    // 配列でない値と、JSON として読めない値は空の一覧として扱う。
    for (const raw of [JSON.stringify({ not: "an array" }), "not-json{"]) {
      await reloadWithStorage(page, {
        "mqtt:presets": raw,
        "mqtt:profileOrder": "not-json{",
        "mqtt:lastActiveProfileId": "not-json{",
      });
      await expect(app.broker(BROKER.name)).toBeVisible();
      await app.selectBroker(BROKER.name);
      await page.getByRole("tab", { name: "Publish" }).click();
      // 空のときは、初回起動と同じく既定のプリセットを 1 件作る。
      await expect(app.mqttPresets).toHaveCount(1);
      await expect(app.mqttPresets.getByRole("textbox")).toHaveValue("no name");
    }

    await expect(page.getByRole("alert")).toHaveCount(0);
    expect(pageErrors).toEqual([]);
  });
});
