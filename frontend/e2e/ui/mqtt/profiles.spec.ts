import { expect, test } from "../../fixtures/ui";

// ブローカープロファイルの編集・選択・接続失敗。偽バックエンドの Connect は seed.mqttConnect を
// "ok" にしない限り失敗するので、未接続のまま書ける範囲を扱う。

const ALPHA = {
  id: "profile-alpha",
  name: "Broker Alpha",
  broker: "tcp://alpha.local:1883",
};
const BETA = {
  id: "profile-beta",
  name: "Broker Beta",
  broker: "mqtts://beta.local:8883",
};

// ── 観点A: 空状態 ────────────────────────────────────────────────────────────

test("empty state shows no brokers and no active connection", async ({
  page,
}) => {
  await expect(page.getByText("No brokers yet")).toBeVisible();
  await expect(
    page.getByText(
      "No active connection. Select a broker from the sidebar to connect.",
    ),
  ).toBeVisible();
  await expect(page.getByRole("tab", { name: "Subscribe" })).toBeHidden();
});

// ── 観点D: ダイアログの入力検証 ──────────────────────────────────────────────
// 範囲外のポートや空のホストは Save / Save & Connect とも押せない (profile-validation.ts)。

test("broker dialog rejects port 0 and 65536 and a blank host", async ({
  page,
}) => {
  await page.getByRole("button", { name: "New Broker" }).click();
  const dialog = page.getByRole("dialog", { name: "New Profile" });
  const save = dialog.getByRole("button", { name: "Save", exact: true });
  const saveAndConnect = dialog.getByRole("button", { name: "Save & Connect" });
  const host = dialog.getByPlaceholder("localhost");
  const port = dialog.getByPlaceholder("1883");

  await dialog.getByLabel("Name", { exact: true }).fill("Range Broker");
  await expect(host).toHaveValue("localhost");
  await expect(port).toHaveValue("1883");
  await expect(save).toBeEnabled();
  await expect(saveAndConnect).toBeEnabled();

  for (const invalid of ["0", "65536"]) {
    await port.fill(invalid);
    await expect(save).toBeDisabled();
    await expect(saveAndConnect).toBeDisabled();
  }
  await port.fill("65535");
  await expect(save).toBeEnabled();

  for (const blank of ["", "   "]) {
    await host.fill(blank);
    await expect(save).toBeDisabled();
    await expect(saveAndConnect).toBeDisabled();
  }
  await host.fill("broker.local");
  await expect(save).toBeEnabled();
});

// ── 観点H: プロファイルの編集 ────────────────────────────────────────────────

test.describe("editing a broker", () => {
  test.use({ seed: { mqttProfiles: [ALPHA] } });

  test("edit broker loads the saved values and updates the profile", async ({
    page,
    app,
    fake,
  }) => {
    const row = app.broker(ALPHA.name);
    await row.hover();
    await row.getByRole("button", { name: "Edit broker" }).click();

    // 保存済みの broker URL をスキーム・ホスト・ポートに分けて読み込む。
    const dialog = page.getByRole("dialog", { name: "Edit Profile" });
    const name = dialog.getByLabel("Name", { exact: true });
    await expect(name).toHaveValue(ALPHA.name);
    // スキームはダイアログで唯一のネイティブ select。
    await expect(dialog.getByRole("combobox")).toHaveValue("tcp");
    await expect(dialog.getByPlaceholder("localhost")).toHaveValue(
      "alpha.local",
    );
    await expect(dialog.getByPlaceholder("1883")).toHaveValue("1883");

    await name.fill("Broker Renamed");
    await dialog.getByPlaceholder("localhost").fill("renamed.local");
    await dialog.getByPlaceholder("1883").fill("1884");
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog).toBeHidden();

    const renamed = app.broker("Broker Renamed");
    await expect(renamed).toContainText("tcp://renamed.local:1884");
    await expect(app.broker(ALPHA.name)).toHaveCount(0);
    // 同じ ID のまま上書きし、新しいプロファイルは作らない。
    expect((await fake.snapshot()).mqttProfiles).toEqual([
      expect.objectContaining({
        id: ALPHA.id,
        name: "Broker Renamed",
        broker: "tcp://renamed.local:1884",
      }),
    ]);

    await page.reload();
    await expect(app.broker("Broker Renamed")).toContainText(
      "tcp://renamed.local:1884",
    );
  });

  test("editing the selected broker in the dialog refreshes the connection bar", async ({
    page,
    app,
  }) => {
    await app.selectBroker(ALPHA.name);
    const host = page.getByPlaceholder("localhost");
    await expect(host).toHaveValue("alpha.local");

    const row = app.broker(ALPHA.name);
    await row.hover();
    await row.getByRole("button", { name: "Edit broker" }).click();
    const dialog = page.getByRole("dialog", { name: "Edit Profile" });
    await dialog.getByPlaceholder("localhost").fill("renamed.local");
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog).toBeHidden();

    await expect(host).toHaveValue("renamed.local");
    await expect(page.getByPlaceholder("1883")).toHaveValue("1883");
  });

  test("cancelling the edit dialog keeps the profile", async ({
    page,
    app,
    fake,
  }) => {
    const row = app.broker(ALPHA.name);
    await row.hover();
    await row.getByRole("button", { name: "Edit broker" }).click();
    const dialog = page.getByRole("dialog", { name: "Edit Profile" });
    await dialog.getByLabel("Name", { exact: true }).fill("Not Saved");
    await dialog.getByRole("button", { name: "Cancel" }).click();

    await expect(dialog).toBeHidden();
    await expect(app.broker(ALPHA.name)).toContainText(ALPHA.broker);
    expect(await fake.calls("SaveProfile")).toBe(0);
  });
});

// ── 観点E: 保存・削除の失敗 ──────────────────────────────────────────────────

test.describe("save failure", () => {
  test.use({
    seed: { mqttProfiles: [ALPHA], saveProfileError: "disk full" },
  });

  test("failed save shows an error toast and keeps the dialog open", async ({
    page,
    app,
  }) => {
    await page.getByRole("button", { name: "New Broker" }).click();
    const dialog = page.getByRole("dialog", { name: "New Profile" });
    await dialog.getByLabel("Name", { exact: true }).fill("Not Saved");
    await dialog.getByRole("button", { name: "Save", exact: true }).click();

    await expect(
      page.getByRole("alert").filter({ hasText: "Failed to save broker" }),
    ).toContainText("disk full");
    // 入力をやり直せるよう、ダイアログは開いたままにする。一覧にも増えない。
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel("Name", { exact: true })).toHaveValue(
      "Not Saved",
    );
    await expect(app.broker("Not Saved")).toHaveCount(0);
  });

  // 接続バーは入力のたびに保存するので、失敗が続いても通知は 1 つにまとめる。
  test("failed saves from the connection bar show a single toast", async ({
    page,
    app,
    fake,
  }) => {
    const pageErrors: string[] = [];
    page.on("pageerror", (err) => pageErrors.push(err.message));
    await app.selectBroker(ALPHA.name);

    const host = page.getByPlaceholder("localhost");
    await host.press("End");
    await host.pressSequentially("xyz");
    await fake.waitForCalls("SaveProfile", 3);

    const toast = page
      .getByRole("alert")
      .filter({ hasText: "Failed to save broker" });
    await expect(toast).toHaveCount(1);
    await expect(toast).toContainText("disk full");
    // 入力した値は巻き戻さない。
    await expect(host).toHaveValue("alpha.localxyz");
    expect(pageErrors).toEqual([]);
  });
});

// ── 観点E: 起動時の読み込みの失敗 ────────────────────────────────────────────

test.describe("when loading brokers fails on startup", () => {
  const LAST_PROFILE_KEY = "mqtt:lastActiveProfileId";
  const PRESETS_KEY = "mqtt:presets";

  test.use({ seed: { mqttProfiles: [ALPHA], getProfilesError: "rpc down" } });

  test("shows an error toast, keeps the last broker and still creates a preset", async ({
    page,
    fake,
  }) => {
    // saveToStorage の保存形式 (JSON)
    const saved = JSON.stringify(ALPHA.id);
    // 最初の goto でも読み込みに失敗して初期プリセットができている。消しておかないと、
    // reload のあとの 1 件が保存済みを読んだだけなのか、この起動で作ったのか区別できない。
    await page.evaluate(
      ([lastKey, value, presetsKey]) => {
        localStorage.setItem(lastKey, value);
        localStorage.removeItem(presetsKey);
      },
      [LAST_PROFILE_KEY, saved, PRESETS_KEY] as const,
    );

    // fixture は goto を済ませているので、reload で起動時の失敗をもう一度起こす。
    const pageErrors: string[] = [];
    page.on("pageerror", (err) => pageErrors.push(err.message));
    await page.reload();

    const toast = page
      .getByRole("alert")
      .filter({ hasText: "Failed to load brokers" });
    await expect(toast).toHaveCount(1);
    await expect(toast).toContainText("rpc down");
    await expect(page.getByText("No brokers yet")).toBeVisible();

    // プリセットの初期作成は RPC に依存しないので、読み込みに失敗しても実行する
    // (onMount の続きで作るので、トーストの表示より後になることがある)。
    await expect
      .poll(() =>
        page.evaluate(
          (key) => JSON.parse(localStorage.getItem(key) ?? "[]").length,
          PRESETS_KEY,
        ),
      )
      .toBe(1);

    // restore() を呼ばないので、最後に選んだブローカーの保存値は消えない。
    expect(await fake.calls("GetConnections")).toBe(0);
    expect(
      await page.evaluate((key) => localStorage.getItem(key), LAST_PROFILE_KEY),
    ).toBe(saved);
    expect(pageErrors).toEqual([]);
  });
});

test.describe("delete failure", () => {
  test.use({
    seed: {
      mqttProfiles: [ALPHA],
      mqttConnect: "ok",
      deleteProfileError: "access denied",
    },
  });

  test("failed delete shows an error toast and keeps the broker connected", async ({
    page,
    app,
    fake,
  }) => {
    const pageErrors: string[] = [];
    page.on("pageerror", (err) => pageErrors.push(err.message));
    await app.connectBroker(ALPHA.name);

    const row = app.broker(ALPHA.name);
    await row.hover();
    await row.getByRole("button", { name: "Delete broker" }).click();
    await app.confirmDelete();

    await expect(
      page.getByRole("alert").filter({ hasText: "Failed to delete broker" }),
    ).toContainText("access denied");
    // 削除に失敗したら、ブローカーもタブも接続も残す。
    await expect(app.broker(ALPHA.name)).toBeVisible();
    await expect(page.getByText("Connected", { exact: true })).toBeVisible();
    expect(await fake.calls("Disconnect")).toBe(0);
    expect((await fake.snapshot()).mqttConnections).toHaveLength(1);
    expect(pageErrors).toEqual([]);
  });
});

// ── 観点E: 接続失敗のトースト ────────────────────────────────────────────────
// 既定の偽バックエンドは実バックエンドと同じく、Connect で接続 ID を返してから
// mqtt:connection-failed で失敗を知らせる。

test.describe("connect failure", () => {
  test.use({ seed: { mqttProfiles: [ALPHA] } });

  test("failed mqtt connect shows an error toast", async ({
    page,
    app,
    fake,
  }) => {
    await app.selectBroker(ALPHA.name);
    await app.brokerConnectButton.click();

    await expect(
      page.getByRole("alert").filter({ hasText: "MQTT connection failed" }),
    ).toContainText("connection refused");
    // 未接続のまま、もう一度押せる。失敗した接続はバックエンドに残らない。
    await expect(page.getByText("Disconnected", { exact: true })).toBeVisible();
    await expect(app.brokerConnectButton).toBeEnabled();
    expect(await fake.calls("Connect")).toBe(1);
    expect((await fake.snapshot()).mqttConnections).toEqual([]);

    await app.brokerConnectButton.click();
    await expect.poll(() => fake.calls("Connect")).toBe(2);
    await expect(app.brokerConnectButton).toBeEnabled();
    // 偽バックエンドは失敗した接続を少し遅れて取り消すので、消えるまで待つ。
    await expect
      .poll(async () => (await fake.snapshot()).mqttConnections)
      .toEqual([]);
  });

  test("Save & Connect keeps the new profile when the connection fails", async ({
    page,
    app,
    fake,
  }) => {
    await page.getByRole("button", { name: "New Broker" }).click();
    const dialog = page.getByRole("dialog", { name: "New Profile" });
    await dialog.getByLabel("Name", { exact: true }).fill("Unreachable");
    await dialog.getByRole("button", { name: "Save & Connect" }).click();

    await expect(dialog).toBeHidden();
    await expect(
      page.getByRole("alert").filter({ hasText: "MQTT connection failed" }),
    ).toContainText("connection refused");
    // 保存は接続より先に済んでいる。
    await expect(app.broker("Unreachable")).toContainText(
      "mqtt://localhost:1883",
    );
    const { mqttProfiles } = await fake.snapshot();
    expect(mqttProfiles.map((p) => p.name)).toEqual([
      ALPHA.name,
      "Unreachable",
    ]);
  });

  test("the error toast can be dismissed", async ({ page, app }) => {
    await app.selectBroker(ALPHA.name);
    await app.brokerConnectButton.click();

    const toast = page
      .getByRole("alert")
      .filter({ hasText: "MQTT connection failed" });
    await expect(toast).toBeVisible();
    await toast.getByRole("button", { name: "Dismiss" }).click();
    await expect(toast).toBeHidden();
  });
});

// Connect の RPC 自体が失敗する場合 (実バックエンドでは終了処理中や入力の検証エラー)。
test.describe("connect rejected", () => {
  test.use({ seed: { mqttProfiles: [ALPHA], mqttConnect: "reject" } });

  test("rejected mqtt connect shows an error toast that can be dismissed", async ({
    page,
    app,
    fake,
  }) => {
    await app.selectBroker(ALPHA.name);
    await app.brokerConnectButton.click();

    const toast = page
      .getByRole("alert")
      .filter({ hasText: "Failed to reconnect" });
    await expect(toast).toContainText("connection refused");
    // 未接続のまま、もう一度押せる。
    await expect(page.getByText("Disconnected", { exact: true })).toBeVisible();
    await expect(app.brokerConnectButton).toBeEnabled();
    expect(await fake.calls("Connect")).toBe(1);
    expect((await fake.snapshot()).mqttConnections).toEqual([]);

    await toast.getByRole("button", { name: "Dismiss" }).click();
    await expect(toast).toBeHidden();
  });
});

// ── 観点H・F: 複数ブローカーの切り替えと、最後に選んだブローカーの復元 ─────────
// 接続バーは未接続のブローカーの URL を、スキーム・ホスト・ポートの入力欄で表示する。

test.describe("multiple brokers", () => {
  test.use({ seed: { mqttProfiles: [ALPHA, BETA] } });

  test("last active broker is selected again after reload", async ({
    page,
    app,
  }) => {
    // 接続バーの入力欄。ダイアログは閉じているので、この placeholder は接続バーにしか無い。
    const host = page.getByPlaceholder("localhost");
    const port = page.getByPlaceholder("1883");
    const emptyState = page.getByText(
      "No active connection. Select a broker from the sidebar to connect.",
    );
    await expect(emptyState).toBeVisible();

    await app.selectBroker(ALPHA.name);
    await expect(emptyState).toBeHidden();
    await expect(host).toHaveValue("alpha.local");
    await expect(port).toHaveValue("1883");

    await app.selectBroker(BETA.name);
    await expect(host).toHaveValue("beta.local");
    await expect(port).toHaveValue("8883");
    const stored = await page.evaluate(() =>
      localStorage.getItem("mqtt:lastActiveProfileId"),
    );
    expect(stored).toBe(JSON.stringify(BETA.id));

    await page.reload();
    await expect(app.brokerConnectButton).toBeVisible();
    await expect(emptyState).toBeHidden();
    await expect(host).toHaveValue("beta.local");
    await expect(port).toHaveValue("8883");
  });

  // 接続バーで編集した URL は、選んでいるブローカーのプロファイルに保存される。
  test("editing the connection bar after switching saves only the selected broker", async ({
    page,
    app,
    fake,
  }) => {
    await app.selectBroker(ALPHA.name);
    await app.selectBroker(BETA.name);

    await page.getByPlaceholder("1883").fill("9999");

    await expect(app.broker(BETA.name)).toContainText(
      "mqtts://beta.local:9999",
    );
    await expect(app.broker(ALPHA.name)).toContainText(ALPHA.broker);
    expect(
      (await fake.snapshot()).mqttProfiles.map((p) => [p.id, p.broker]),
    ).toEqual([
      [ALPHA.id, ALPHA.broker],
      [BETA.id, "mqtts://beta.local:9999"],
    ]);
  });
});
