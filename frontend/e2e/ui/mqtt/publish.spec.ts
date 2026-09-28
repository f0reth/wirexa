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

/**
 * プリセット一覧の追加ボタン。アクセシブル名が無いので、見出し "Messages" の行にある
 * 唯一のボタンとして取る。
 */
function addPresetButton(page: Page) {
  return page
    .getByRole("heading", { name: "Messages", exact: true })
    .locator("xpath=..")
    .getByRole("button");
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
  await addPresetButton(page).click();
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

  // 削除ボタンにはアクセシブル名が無い。行内のボタンはこれだけ。確認は挟まない。
  await app.mqttPreset("sensors/b").getByRole("button").click();
  await expect(app.mqttPreset("sensors/b")).toHaveCount(0);
  await expect(app.mqttPresets).toHaveCount(1);

  await app.mqttPreset("sensors/a").getByRole("button").click();
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

  await addPresetButton(page).click();
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

test("Enter on a focused preset selects it", async ({ page, app }) => {
  const { topic } = publishForm(app);
  await topic.fill("sensors/a");
  await addPresetButton(page).click();
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
