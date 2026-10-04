import type { Page } from "@playwright/test";
import type { MqttRawMessage } from "../../../src/domain/mqtt/types";
import {
  type App,
  test as base,
  expect,
  WailsEvents,
} from "../../fixtures/ui";

// 接続済みブローカーでの購読・受信・表示と Publish。偽バックエンドは seed.mqttConnect: "ok" で
// Connect を成功させ、Go と同じく接続 ID を返したあとで mqtt:connected を発火する。
// ブローカーは無いので、受信メッセージは Go の messageHandler が発火する mqtt:message を
// 偽バックエンドから流して模す。

const BROKER = { id: "profile-local", name: "Local Broker" };

/** 画面が 1 つの接続で保持するメッセージの上限 (src/config/limits.ts の MQTT_MAX_MESSAGES)。 */
const MAX_MESSAGES = 5000;

/**
 * connectionId は BROKER に接続して、その接続 ID を返す。自動では動かないので、接続が要るテストが
 * 引数に取る。
 */
const test = base.extend<{ connectionId: string }>({
  connectionId: async ({ app, fake }, use) => {
    await app.connectBroker(BROKER.name);
    const [conn] = (await fake.snapshot()).mqttConnections;
    await use(conn.id);
  },
});

test.use({ seed: { mqttProfiles: [BROKER], mqttConnect: "ok" } });

/**
 * Go の domain.MQTTMessage と同じ形の mqtt:message のペイロード。フロントエンドの domain 型は
 * retained を持たないが、Go は送る。
 */
function message(
  connectionId: string,
  topic: string,
  payload: string,
  qos: 0 | 1 | 2 = 0,
): MqttRawMessage & { retained: boolean } {
  return {
    connectionId,
    topic,
    payload,
    payloadBase64: false,
    qos,
    retained: false,
    timestamp: Date.now(),
  };
}

/** "msg-<from>" から "msg-<to - 1>" までのペイロードを持つメッセージ。 */
function numbered(connectionId: string, from: number, to: number) {
  return Array.from({ length: to - from }, (_, i) =>
    message(connectionId, "bulk/data", `msg-${from + i}`),
  );
}

/** 受信の反映 (requestAnimationFrame) とその後の描画が終わるまで待つ。 */
async function nextFrames(page: Page): Promise<void> {
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
}

// ── 観点H: 購読の追加・削除 ──────────────────────────────────────────────────

test("subscribing and unsubscribing reach the backend", async ({
  app,
  fake,
  connectionId,
}) => {
  const panel = app.mqttSection("Subscriptions");
  await expect(panel.getByText("No subscriptions")).toBeVisible();

  await app.subscribeMqtt("sensors/#", 1);

  const row = app.mqttSubscription("sensors/#");
  await expect(row).toContainText("QoS 1");
  expect(await fake.args("Subscribe")).toEqual([[connectionId, "sensors/#", 1]]);
  expect((await fake.snapshot()).mqttConnections[0].subscriptions).toEqual([
    { topic: "sensors/#", qos: 1 },
  ]);

  await app.removeMqttSubscriptionButton("sensors/#").click();

  await expect(row).toBeHidden();
  await expect(panel.getByText("No subscriptions")).toBeVisible();
  expect(await fake.args("Unsubscribe")).toEqual([[connectionId, "sensors/#"]]);
});

test("subscribing to an invalid filter shows the backend error", async ({
  page,
  app,
  // 接続だけが要る。
  connectionId: _connectionId,
}) => {
  const panel = app.mqttSection("Subscriptions");
  await app.mqttTopicInput.fill("a/#/b");
  await app.mqttSubscribeButton.click();

  // Go の ValidateTopicFilter と同じ文言がトーストに出て、購読は増えない。
  await expect(
    page.getByRole("alert").filter({ hasText: "Failed to subscribe to a/#/b" }),
  ).toContainText("invalid topic: # must occupy the last level entirely");
  await expect(panel.getByText("No subscriptions")).toBeVisible();
});

// ── 観点H: 受信 → 一覧 → 詳細 ────────────────────────────────────────────────

test("received message is listed and its details can be copied", async ({
  page,
  context,
  app,
  fake,
  connectionId,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await app.subscribeMqtt("sensors/temp");
  const messages = app.mqttSection("Messages");
  await expect(messages.getByText("No messages yet")).toBeVisible();

  await fake.emit(
    WailsEvents.mqttMessage,
    message(connectionId, "sensors/temp", '{"temp":21.5}', 1),
  );

  const item = app.mqttMessage('{"temp":21.5}');
  await expect(item).toContainText("sensors/temp");
  await expect(messages.getByText("No messages yet")).toBeHidden();

  // Auto が OFF なので、届いただけでは選ばれない。
  const placeholder = page.getByText("Select a message to view details");
  await expect(placeholder).toBeVisible();

  await item.click();

  await expect(placeholder).toBeHidden();
  await expect(page.getByText("QoS 1", { exact: true })).toBeVisible();
  await expect(page.getByText("incoming", { exact: true })).toBeVisible();
  // 詳細は JSON を整形して表示し、コピーも整形後の文字列になる。
  const formatted = '{\n  "temp": 21.5\n}';
  await expect(page.getByText(formatted)).toBeVisible();

  await page.getByRole("button", { name: "Copy payload" }).click();

  // Windows のクリップボードは改行を CRLF にして返すので LF に揃えて比べる。
  await expect
    .poll(async () =>
      (await page.evaluate(() => navigator.clipboard.readText())).replace(
        /\r\n/g,
        "\n",
      ),
    )
    .toBe(formatted);
});

test("muted subscription hides its messages", async ({ app, fake, connectionId }) => {
  await app.subscribeMqtt("sensors/temp");
  await app.subscribeMqtt("alerts/fire");

  const muteButton = app
    .mqttSubscription("sensors/temp")
    .getByRole("button", { name: "Mute", exact: true });
  await muteButton.click();
  const unmuteButton = app
    .mqttSubscription("sensors/temp")
    .getByRole("button", { name: "Unmute", exact: true });
  await expect(unmuteButton).toBeVisible();

  // 同じフレームに届けるので、alarm が見えた時点で残りも処理済み。
  await fake.emitAll(WailsEvents.mqttMessage, [
    message(connectionId, "sensors/temp", "muted-reading"),
    message(connectionId, "other/topic", "not-subscribed"),
    message(connectionId, "alerts/fire", "alarm"),
  ]);

  await expect(app.mqttMessage("alarm")).toBeVisible();
  await expect(app.mqttMessage("muted-reading")).toHaveCount(0);
  await expect(app.mqttMessage("not-subscribed")).toHaveCount(0);
  // ミュートは表示だけの設定で、ブローカーの購読は外さない。
  expect(await fake.calls("Unsubscribe")).toBe(0);

  await unmuteButton.click();
  await fake.emit(
    WailsEvents.mqttMessage,
    message(connectionId, "sensors/temp", "after-unmute"),
  );
  await expect(app.mqttMessage("after-unmute")).toBeVisible();
});

// 共有購読のメッセージは、接頭辞の無いトピックで届く (バックエンドは接頭辞を外して振り分ける)。
test("shared subscription shows its messages", async ({ app, fake, connectionId }) => {
  await app.subscribeMqtt("$share/group/sensors/#");

  await fake.emitAll(WailsEvents.mqttMessage, [
    message(connectionId, "other/topic", "not-subscribed"),
    message(connectionId, "sensors/temp", "shared-reading"),
  ]);

  await expect(app.mqttMessage("shared-reading")).toContainText(
    "sensors/temp",
  );
  await expect(app.mqttMessage("not-subscribed")).toHaveCount(0);
});

// 張り直しでバックエンドが外した購読 (mqtt:subscription-dropped) は、行が消えて通知が出る。
// 張り直しの拒否は偽バックエンドでは起こせないので、Go の resubscribe が発火するイベントを流して模す。
test("dropped subscription is removed and notified", async ({
  page,
  app,
  fake,
  connectionId,
}) => {
  await app.subscribeMqtt("sensors/#");
  await app.subscribeMqtt("alerts/fire");

  await fake.emit(WailsEvents.mqttSubscriptionDropped, {
    connectionId,
    topic: "sensors/#",
    error: "subscription rejected by broker",
  });

  await expect(app.mqttSubscription("sensors/#")).toBeHidden();
  await expect(app.mqttSubscription("alerts/fire")).toBeVisible();
  await expect(
    page
      .getByRole("alert")
      .filter({ hasText: "Subscription to sensors/# was dropped" }),
  ).toContainText("subscription rejected by broker");
});

test("topic filter can shorten a list that grew after it was first drawn", async ({
  page,
  app,
  fake,
  connectionId,
  pageErrors,
}) => {
  await app.subscribeMqtt("sensors/#");
  await fake.emitAll(WailsEvents.mqttMessage, [
    message(connectionId, "sensors/temp", "t-1"),
    message(connectionId, "sensors/humidity", "h-1"),
    message(connectionId, "sensors/temp", "t-2"),
    message(connectionId, "sensors/humidity", "h-2"),
  ]);
  await expect(app.mqttMessages).toHaveCount(4);
  // 描画済みの一覧に 1 件足してから絞り込む。
  await fake.emit(WailsEvents.mqttMessage, message(connectionId, "sensors/temp", "t-3"));
  await expect(app.mqttMessages).toHaveCount(5);

  await page
    .getByRole("combobox", { name: "Filter by topic" })
    .selectOption("sensors/temp");

  await expect(app.mqttMessages).toHaveText([/t-1$/, /t-2$/, /t-3$/]);
  expect(pageErrors).toEqual([]);
});

test("topic filter narrows the list and Clear empties it", async ({
  page,
  app,
  fake,
  connectionId,
}) => {
  await app.subscribeMqtt("sensors/#");
  await fake.emitAll(WailsEvents.mqttMessage, [
    message(connectionId, "sensors/temp", "t-1"),
    message(connectionId, "sensors/humidity", "h-1"),
  ]);
  const temp = app.mqttMessage("t-1");
  const humidity = app.mqttMessage("h-1");
  await expect(temp).toBeVisible();
  await expect(humidity).toBeVisible();

  // 選択肢は購読トピックと、ワイルドカード購読に一致した実トピック。
  const filter = page.getByRole("combobox", { name: "Filter by topic" });
  await expect(filter.getByRole("option")).toHaveText([
    "All topics",
    "sensors/#",
    "sensors/humidity",
    "sensors/temp",
  ]);

  await filter.selectOption("sensors/temp");
  await expect(temp).toBeVisible();
  await expect(humidity).toBeHidden();

  await filter.selectOption("");
  await expect(humidity).toBeVisible();

  await app.mqttMessagesAction("Clear").click();

  await expect(app.mqttMessages).toHaveCount(0);
  await expect(
    app.mqttSection("Messages").getByText("No messages yet"),
  ).toBeVisible();
  // 一覧を消すだけで購読は残る
  await expect(app.mqttSubscription("sensors/#")).toBeVisible();
});

// 共有購読の選択肢は接頭辞の付いた購読の文字列で、照合は接頭辞を外して行う。
test("topic filter narrows by a shared subscription and its concrete topics", async ({
  page,
  app,
  fake,
  connectionId,
}) => {
  await app.subscribeMqtt("$share/group/sensors/#");
  await fake.emitAll(WailsEvents.mqttMessage, [
    message(connectionId, "sensors/temp", "t-1"),
    message(connectionId, "sensors/humidity", "h-1"),
  ]);
  const temp = app.mqttMessage("t-1");
  const humidity = app.mqttMessage("h-1");
  await expect(temp).toBeVisible();
  await expect(humidity).toBeVisible();

  const filter = page.getByRole("combobox", { name: "Filter by topic" });
  await expect(filter.getByRole("option")).toHaveText([
    "All topics",
    "$share/group/sensors/#",
    "sensors/humidity",
    "sensors/temp",
  ]);

  await filter.selectOption("$share/group/sensors/#");
  await expect(filter).toHaveValue("$share/group/sensors/#");
  await expect(app.mqttMessages).toHaveCount(2);

  await filter.selectOption("sensors/temp");
  await expect(temp).toBeVisible();
  await expect(humidity).toBeHidden();
});

// ── 観点H: 一覧のレイアウト ──────────────────────────────────────────────────

/**
 * 一覧に描かれている行を index 順に並べ、隣り合う行の「上端 − 前の行の下端」を返す。
 * 行が隙間なく並んでいればすべて 0 になる。
 */
function rowGaps(app: App): Promise<number[]> {
  return app.mqttMessageRows.evaluateAll((rows) => {
    const rects = rows
      .sort((a, b) => Number(a.dataset.index) - Number(b.dataset.index))
      .map((el) => el.getBoundingClientRect());
    return rects.slice(1).map((rect, i) => rect.top - rects[i].bottom);
  });
}

/** 一覧の rows 件の行が、隙間も重なりもなく (±1px) 並ぶまで待つ。 */
async function expectRowsContiguous(app: App, rows: number): Promise<void> {
  await expect
    .poll(async () => (await rowGaps(app)).map((gap) => Math.abs(gap) <= 1))
    .toEqual(Array.from({ length: rows - 1 }, () => true));
}

test("message rows stay contiguous whatever the payload length", async ({
  page,
  app,
  fake,
  connectionId,
  pageErrors,
}) => {
  await app.subscribeMqtt("sensors/#");
  // プレビューが 1 行で収まるペイロードと、2 行に折り返して切り詰められるペイロード。
  const long = "x".repeat(400);
  await fake.emitAll(WailsEvents.mqttMessage, [
    message(connectionId, "sensors/temp", "short-1"),
    message(connectionId, "sensors/humidity", `${long}-1`),
    message(connectionId, "sensors/temp", "short-2"),
    message(connectionId, "sensors/humidity", `${long}-2`),
  ]);
  await expect(app.mqttMessages).toHaveCount(4);
  await expectRowsContiguous(app, 4);

  // 受信で件数が変わっても並びは崩れない。
  await fake.emit(WailsEvents.mqttMessage, message(connectionId, "sensors/temp", "short-3"));
  await expect(app.mqttMessages).toHaveCount(5);
  await expectRowsContiguous(app, 5);

  const filter = page.getByRole("combobox", { name: "Filter by topic" });
  await filter.selectOption("sensors/temp");
  await expect(app.mqttMessages).toHaveCount(3);
  await expectRowsContiguous(app, 3);

  await filter.selectOption("");
  await expect(app.mqttMessages).toHaveCount(5);
  await expectRowsContiguous(app, 5);
  expect(pageErrors).toEqual([]);
});

test("selecting a long single-line payload does not widen the window", async ({
  page,
  app,
  fake,
  connectionId,
  pageErrors,
}) => {
  await app.subscribeMqtt("sensors/temp");
  // 改行も空白も無く、詳細の <pre> で折り返されないペイロード。
  const payload = "x".repeat(400);
  await fake.emit(WailsEvents.mqttMessage, message(connectionId, "sensors/temp", payload));

  await app.mqttMessage(payload).click();
  await expect(page.getByText(payload, { exact: true })).toHaveCount(2);

  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth - window.innerWidth,
    ),
  ).toBeLessThanOrEqual(0);
  await expect(app.mqttMessagesAction("Clear")).toBeInViewport({ ratio: 1 });
  expect(pageErrors).toEqual([]);
});

// ── 観点H: Auto (最新メッセージへの追従) ─────────────────────────────────────

test("with Auto enabled the newest message is scrolled into view", async ({
  page,
  app,
  fake,
  connectionId,
}) => {
  await app.subscribeMqtt("bulk/#");
  // 一覧の表示領域を十分に超える件数
  await fake.emitAll(WailsEvents.mqttMessage, numbered(connectionId, 0, 50));

  // Auto は既定で OFF: 先頭に留まり、どのメッセージも選ばない。
  await expect(app.mqttMessage("msg-0")).toBeInViewport();
  await expect(app.mqttMessage("msg-49")).not.toBeInViewport();
  const placeholder = page.getByText("Select a message to view details");
  await expect(placeholder).toBeVisible();

  await app.mqttMessagesAction("Auto").click();

  // 末尾までスクロールし、最新のメッセージを選んで詳細に出す。
  await expect(app.mqttMessage("msg-49")).toBeInViewport();
  await expect(placeholder).toBeHidden();
  await expect(page.getByText("msg-49", { exact: true })).toHaveCount(2);

  // 以後に届いたメッセージにも追従する。
  await fake.emit(WailsEvents.mqttMessage, message(connectionId, "bulk/data", "msg-50"));
  await expect(app.mqttMessage("msg-50")).toBeInViewport();
  await expect(page.getByText("msg-50", { exact: true })).toHaveCount(2);
});

test("with Auto and a topic filter, a message outside the filter leaves the selection alone", async ({
  page,
  app,
  fake,
  connectionId,
  pageErrors,
}) => {
  await app.subscribeMqtt("sensors/#");
  await fake.emitAll(WailsEvents.mqttMessage, [
    message(connectionId, "sensors/temp", "t-1"),
    message(connectionId, "sensors/humidity", "h-1"),
  ]);
  await expect(app.mqttMessage("h-1")).toBeVisible();

  await app.mqttMessagesAction("Auto").click();
  await page
    .getByRole("combobox", { name: "Filter by topic" })
    .selectOption("sensors/temp");

  // フィルター後の末尾を選び、一覧と詳細の両方に出す。
  await expect(app.mqttMessage("h-1")).toBeHidden();
  await expect(page.getByText("t-1", { exact: true })).toHaveCount(2);

  // フィルター外の受信は一覧にも詳細にも出ない。
  await fake.emit(WailsEvents.mqttMessage, message(connectionId, "sensors/humidity", "h-2"));
  await nextFrames(page);
  await expect(page.getByText("h-2", { exact: true })).toHaveCount(0);
  await expect(page.getByText("t-1", { exact: true })).toHaveCount(2);

  // フィルターに一致する受信には追従し続ける。
  await fake.emit(WailsEvents.mqttMessage, message(connectionId, "sensors/temp", "t-2"));
  await expect(page.getByText("t-2", { exact: true })).toHaveCount(2);
  await expect(page.getByText("h-2", { exact: true })).toHaveCount(0);
  expect(pageErrors).toEqual([]);
});

// ── 観点D: 5000 件の上限 ─────────────────────────────────────────────────────
// 受信は 1 フレームにまとめて一覧へ反映する (message-buffer.ts の flushMessages)。一覧は
// 上限を超えた分の古いものを捨て、1 フレーム分のバッファは上限を超えた新しいものを捨てる。
// 一覧は仮想スクロールで表示中の行しか描かないので、先頭と末尾で見えている行を確かめる。

test.describe("message cap", () => {
  // 5000 件の描画を待つので既定より長くとる
  test.slow();

  test("mqtt message list drops the oldest message when the 5001st arrives", async ({
    app,
    fake,
    connectionId,
  }) => {
    await app.subscribeMqtt("bulk/#");
    await fake.emitAll(WailsEvents.mqttMessage, numbered(connectionId, 0, MAX_MESSAGES));
    // 表示されたら 1 フレーム目の反映は終わっている。次の 1 件は別のフレームに届く。
    await expect(app.mqttMessage("msg-0")).toBeVisible();

    await fake.emit(
      WailsEvents.mqttMessage,
      message(connectionId, "bulk/data", `msg-${MAX_MESSAGES}`),
    );

    // 先頭 (スクロール位置は先頭のまま) から最古の msg-0 が消え、msg-1 が繰り上がる。
    await expect(app.mqttMessage("msg-0")).toHaveCount(0);
    await expect(app.mqttMessages.first()).toHaveText(/msg-1$/);

    await app.mqttMessagesAction("Auto").click();
    await expect(app.mqttMessage(`msg-${MAX_MESSAGES}`)).toBeInViewport();
  });

  test("mqtt messages beyond 5000 within one frame are dropped from the buffer", async ({
    app,
    fake,
    connectionId,
  }) => {
    await app.subscribeMqtt("bulk/#");
    await fake.emitAll(WailsEvents.mqttMessage, numbered(connectionId, 0, MAX_MESSAGES + 1));

    // バッファに入った最初の 5000 件が一覧に載り、あふれた最新の 1 件は捨てられる。
    await expect(app.mqttMessages.first()).toHaveText(/msg-0$/);

    await app.mqttMessagesAction("Auto").click();
    const last = app.mqttMessage(`msg-${MAX_MESSAGES - 1}`);
    await expect(last).toBeInViewport();
    // 一覧の並び順そのものを確かめるので、末尾の項目を位置で取る。
    await expect(app.mqttMessages.last()).toHaveText(
      new RegExp(`msg-${MAX_MESSAGES - 1}$`),
    );
    await expect(app.mqttMessage(`msg-${MAX_MESSAGES}`)).toHaveCount(0);
  });
});

// ── 観点H: Publish ───────────────────────────────────────────────────────────

test("publish sends topic, QoS, retain and payload", async ({
  page,
  app,
  fake,
  connectionId,
}) => {
  await page.getByRole("tab", { name: "Publish" }).click();
  const form = app.mqttSection("Publish");
  const topic = form.getByPlaceholder("Topic", { exact: true });
  const payload = form.getByPlaceholder("Message payload");
  const publish = form.getByRole("button", { name: "Publish", exact: true });
  await expect(publish).toBeEnabled();

  // トピックが空なら送らない
  await payload.fill("on");
  await publish.click();
  // retain なしで空白だけのペイロードも送らない
  await topic.fill("devices/lamp");
  await payload.fill("   ");
  await publish.click();

  await payload.fill("on");
  await app.chooseOption(form, "0", "QoS 2");
  await publish.click();
  await fake.waitForCalls("Publish");

  // retain 付きの空ペイロードは retained メッセージの削除なので送る
  await form.getByRole("checkbox", { name: "Retain" }).check();
  await payload.fill("");
  await publish.click();
  await fake.waitForCalls("Publish", 2);

  expect(await fake.args("Publish")).toEqual([
    [connectionId, "devices/lamp", "on", 2, false],
    [connectionId, "devices/lamp", "", 2, true],
  ]);
});
