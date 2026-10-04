import type { Locator, Page } from "@playwright/test";
import { expect, test, WailsEvents } from "../../fixtures/ui";

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
  app,
}) => {
  await page.getByRole("button", { name: "New Broker" }).click();
  const dialog = app.brokerDialog("New Profile");
  const save = dialog.getByRole("button", { name: "Save", exact: true });
  const saveAndConnect = dialog.getByRole("button", { name: "Save & Connect" });
  const host = app.brokerHostInput(dialog);
  const port = app.brokerPortInput(dialog);

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
    // 保存済みの broker URL をスキーム・ホスト・ポートに分けて読み込む。
    const dialog = await app.openBrokerEditDialog(ALPHA.name);
    const name = dialog.getByLabel("Name", { exact: true });
    await expect(name).toHaveValue(ALPHA.name);
    await expect(app.brokerSchemeSelect(dialog)).toHaveValue("tcp");
    await expect(app.brokerHostInput(dialog)).toHaveValue(
      "alpha.local",
    );
    await expect(app.brokerPortInput(dialog)).toHaveValue("1883");

    await name.fill("Broker Renamed");
    await app.brokerHostInput(dialog).fill("renamed.local");
    await app.brokerPortInput(dialog).fill("1884");
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
    app,
  }) => {
    await app.selectBroker(ALPHA.name);
    const host = app.brokerHostInput();
    await expect(host).toHaveValue("alpha.local");

    const dialog = await app.openBrokerEditDialog(ALPHA.name);
    await app.brokerHostInput(dialog).fill("renamed.local");
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog).toBeHidden();

    await expect(host).toHaveValue("renamed.local");
    await expect(app.brokerPortInput()).toHaveValue("1883");
  });

  test("cancelling the edit dialog keeps the profile", async ({
    app,
    fake,
  }) => {
    const dialog = await app.openBrokerEditDialog(ALPHA.name);
    await dialog.getByLabel("Name", { exact: true }).fill("Not Saved");
    await dialog.getByRole("button", { name: "Cancel" }).click();

    await expect(dialog).toBeHidden();
    await expect(app.broker(ALPHA.name)).toContainText(ALPHA.broker);
    expect(await fake.calls("SaveProfile")).toBe(0);
  });
});

// ── 観点H: スキーム・クライアント ID・認証情報・TLS ───────────────────────────

test.describe("broker url scheme", () => {
  test.use({ seed: { mqttProfiles: [ALPHA] } });

  test("changing the scheme resets the port to its default and saves the broker URL", async ({
    app,
    fake,
  }) => {
    await app.selectBroker(ALPHA.name);
    const lastSaved = async () =>
      (await fake.args("SaveProfile")).at(-1)?.[0];

    // 接続バーは入力のたびに保存する。
    await app.brokerSchemeSelect().selectOption("mqtts");
    await expect(app.brokerPortInput()).toHaveValue("8883");
    await expect(app.broker(ALPHA.name)).toContainText(
      "mqtts://alpha.local:8883",
    );
    expect(await lastSaved()).toMatchObject({
      id: ALPHA.id,
      broker: "mqtts://alpha.local:8883",
    });

    await app.brokerSchemeSelect().selectOption("ws");
    await expect(app.brokerPortInput()).toHaveValue("9001");
    await expect(app.broker(ALPHA.name)).toContainText("ws://alpha.local:9001");
    expect(await lastSaved()).toMatchObject({
      broker: "ws://alpha.local:9001",
    });
    const savesFromBar = await fake.calls("SaveProfile");

    // ダイアログは Save を押すまで保存しない。
    const dialog = await app.openBrokerEditDialog(ALPHA.name);
    await expect(app.brokerSchemeSelect(dialog)).toHaveValue("ws");
    await app.brokerSchemeSelect(dialog).selectOption("wss");
    await expect(app.brokerPortInput(dialog)).toHaveValue("8884");
    expect(await fake.calls("SaveProfile")).toBe(savesFromBar);
    await saveButton(dialog).click();
    await expect(dialog).toBeHidden();

    await expect(app.broker(ALPHA.name)).toContainText(
      "wss://alpha.local:8884",
    );
    await expect(app.brokerSchemeSelect()).toHaveValue("wss");
    await expect(app.brokerPortInput()).toHaveValue("8884");
    expect((await fake.snapshot()).mqttProfiles[0].broker).toBe(
      "wss://alpha.local:8884",
    );
  });
});

test.describe("client id, credentials and TLS", () => {
  const SECURE = {
    id: "profile-secure",
    name: "Secure Broker",
    broker: "mqtts://secure.local:8883",
    clientId: "wirexa-e2e",
    username: "alice",
    password: "s3cret",
    useTls: true,
  };

  test.describe("new broker", () => {
    test.use({ seed: { mqttConnect: "ok" } });

    test("client id, credentials and TLS from the dialog reach SaveProfile and Connect", async ({
      page,
      app,
      fake,
    }) => {
      await page.getByRole("button", { name: "New Broker" }).click();
      const dialog = app.brokerDialog("New Profile");
      await dialog.getByLabel("Name", { exact: true }).fill(SECURE.name);
      await app.brokerSchemeSelect(dialog).selectOption("mqtts");
      await app.brokerHostInput(dialog).fill("secure.local");
      await dialog.getByLabel("Client ID").fill(SECURE.clientId);
      await dialog.getByLabel("Username").fill(SECURE.username);
      await dialog.getByLabel("Password").fill(SECURE.password);
      await dialog.getByLabel("Use TLS").check();
      await dialog.getByRole("button", { name: "Save & Connect" }).click();
      await expect(dialog).toBeHidden();

      await expect(app.mqttStatus("Connected")).toBeVisible();
      const { id: _id, ...fields } = SECURE;
      expect(await fake.args("SaveProfile")).toEqual([[{ id: "", ...fields }]]);
      // 接続は保存後のプロファイル (採番された ID) で行う。
      const [saved] = (await fake.snapshot()).mqttProfiles;
      expect(saved).toEqual({ ...fields, id: saved.id });
      expect(saved.id).not.toBe("");
      const { name, broker, clientId, username, password, useTls } = SECURE;
      expect(await fake.args("Connect")).toEqual([
        [
          {
            name,
            broker,
            clientId,
            username,
            password,
            useTls,
            profileId: saved.id,
          },
        ],
      ]);
    });
  });

  test.describe("saved broker", () => {
    test.use({ seed: { mqttProfiles: [SECURE] } });

    test("edit dialog loads the saved client id, credentials and TLS", async ({
      app,
      fake,
    }) => {
      const dialog = await app.openBrokerEditDialog(SECURE.name);

      await expect(app.brokerSchemeSelect(dialog)).toHaveValue("mqtts");
      await expect(app.brokerHostInput(dialog)).toHaveValue("secure.local");
      await expect(app.brokerPortInput(dialog)).toHaveValue("8883");
      await expect(dialog.getByLabel("Client ID")).toHaveValue(SECURE.clientId);
      await expect(dialog.getByLabel("Username")).toHaveValue(SECURE.username);
      await expect(dialog.getByLabel("Password")).toHaveValue(SECURE.password);
      await expect(dialog.getByLabel("Use TLS")).toBeChecked();

      // 触っていない欄は、読み込んだ値のまま保存する。
      await dialog.getByLabel("Username").fill("bob");
      await saveButton(dialog).click();
      await expect(dialog).toBeHidden();
      expect(await fake.args("SaveProfile")).toEqual([
        [{ ...SECURE, username: "bob" }],
      ]);
    });
  });

  // 画面のスキームはどれも TLS で使えるので、この検証は画面からは通らない。バインディングを直接呼ぶ。
  test("Connect rejects a scheme that cannot be used with TLS", async ({
    app,
    fake,
  }) => {
    expect(
      await app.mqttConnectError({
        broker: "http://127.0.0.1:1883",
        useTls: true,
      }),
    ).toContain("invalid broker URL: scheme cannot be used with TLS");
    expect((await fake.snapshot()).mqttConnections).toEqual([]);

    // TLS を使わなければ、スキームは検証しない。
    expect(
      await app.mqttConnectError({
        broker: "http://127.0.0.1:1883",
        useTls: false,
      }),
    ).toBeNull();
  });
});

// ── 観点H: 接続が生きているブローカーの編集 ──────────────────────────────────
// バックエンドに接続が残っている間 (Connected・確立待ち・自動再接続中) は、ダイアログの Save と
// 接続バーの入力欄を使えない。編集前の宛先に繋がったまま、編集後の宛先を表示しないため。

const LIVE_NOTICE =
  "This broker has an active connection. Use Save & Connect to apply changes.";

function saveButton(dialog: Locator): Locator {
  return dialog.getByRole("button", { name: "Save", exact: true });
}

/** 確立待ち・自動再接続中に接続バーへ出るボタン。押すと接続を中止する。 */
function connectingButton(page: Page): Locator {
  return page.getByRole("button", { name: "Connecting…", exact: true });
}

test.describe("editing a broker whose connection is pending", () => {
  test.use({ seed: { mqttProfiles: [ALPHA], mqttConnect: "pending" } });

  test("Save and the connection bar are locked while a connection is pending", async ({
    page,
    app,
    fake,
  }) => {
    await app.selectBroker(ALPHA.name);
    await app.brokerConnectButton.click();

    const expectLocked = async () => {
      await expect(connectingButton(page)).toBeVisible();
      const dialog = await app.openBrokerEditDialog(ALPHA.name);
      await expect(saveButton(dialog)).toBeDisabled();
      await expect(dialog.getByText(LIVE_NOTICE)).toBeVisible();
      await expect(
        dialog.getByRole("button", { name: "Save & Connect" }),
      ).toBeEnabled();
      await dialog.getByRole("button", { name: "Cancel" }).click();
      await expect(dialog).toBeHidden();

      await expect(app.brokerHostInput()).toBeDisabled();
      await expect(app.brokerPortInput()).toBeDisabled();
      await expect(app.brokerConnectButton).toBeHidden();
      expect(await fake.calls("SaveProfile")).toBe(0);
    };
    await expectLocked();

    // 復元したタブも、バックエンドに接続が残っている。
    await page.reload();
    await expectLocked();
    expect((await fake.snapshot()).mqttConnections).toHaveLength(1);
  });

  test("clicking Connecting… cancels a pending connection", async ({
    page,
    app,
    fake,
  }) => {
    await app.selectBroker(ALPHA.name);
    await app.brokerConnectButton.click();

    await connectingButton(page).click();

    await expect(app.brokerConnectButton).toBeVisible();
    expect(await fake.calls("Disconnect")).toBe(1);
    expect((await fake.snapshot()).mqttConnections).toEqual([]);

    // 中止したあとは、接続バーでもダイアログでも編集できる。
    await app.brokerHostInput().fill("edited.local");
    await expect(app.broker(ALPHA.name)).toContainText(
      "tcp://edited.local:1883",
    );
    const dialog = await app.openBrokerEditDialog(ALPHA.name);
    await expect(dialog.getByText(LIVE_NOTICE)).toBeHidden();
    await saveButton(dialog).click();
    await expect(dialog).toBeHidden();
  });
});

test.describe("editing a broker after its connection failed", () => {
  test.use({ seed: { mqttProfiles: [ALPHA] } });

  test("Save becomes available after a failed connection", async ({
    page,
    app,
  }) => {
    await app.selectBroker(ALPHA.name);
    await app.brokerConnectButton.click();
    await expect(
      page.getByRole("alert").filter({ hasText: "MQTT connection failed" }),
    ).toBeVisible();

    await app.brokerHostInput().fill("edited.local");
    await expect(app.broker(ALPHA.name)).toContainText(
      "tcp://edited.local:1883",
    );

    const dialog = await app.openBrokerEditDialog(ALPHA.name);
    await expect(dialog.getByText(LIVE_NOTICE)).toBeHidden();
    await dialog.getByLabel("Name", { exact: true }).fill("Broker Renamed");
    await saveButton(dialog).click();
    await expect(dialog).toBeHidden();
    await expect(app.broker("Broker Renamed")).toContainText(
      "tcp://edited.local:1883",
    );
  });
});

test.describe("editing a connected broker", () => {
  test.use({ seed: { mqttProfiles: [ALPHA], mqttConnect: "ok" } });

  test("Save is disabled for a connected broker and Save & Connect reconnects with the edit", async ({
    app,
    fake,
  }) => {
    await app.connectBroker(ALPHA.name);
    await app.subscribeMqtt("sensors/#");
    const oldId = (await fake.snapshot()).mqttConnections[0].id;

    const dialog = await app.openBrokerEditDialog(ALPHA.name);
    await expect(saveButton(dialog)).toBeDisabled();
    await expect(dialog.getByText(LIVE_NOTICE)).toBeVisible();
    await app.brokerHostInput(dialog).fill("renamed.local");
    // 入力を変えても Save は押せないまま。
    await expect(saveButton(dialog)).toBeDisabled();
    await dialog.getByRole("button", { name: "Save & Connect" }).click();
    await expect(dialog).toBeHidden();

    // 編集後の URL で張り直し、購読を新しい接続へ引き継ぐ。
    await fake.waitForCalls("Subscribe", 2);
    expect(await fake.calls("Disconnect")).toBe(1);
    expect((await fake.args("Connect")).at(-1)?.[0]).toMatchObject({
      broker: "tcp://renamed.local:1883",
    });
    const { mqttConnections } = await fake.snapshot();
    expect(mqttConnections).toHaveLength(1);
    expect(mqttConnections[0].id).not.toBe(oldId);
    expect((await fake.args("Subscribe")).at(-1)).toEqual([
      mqttConnections[0].id,
      "sensors/#",
      0,
    ]);
    await expect(app.mqttStatus("Connected")).toBeVisible();
    await expect(app.mqttSubscription("sensors/#")).toBeVisible();
  });

  test("saving an edit of a disconnected broker keeps its subscription rows", async ({
    page,
    app,
    fake,
  }) => {
    await app.connectBroker(ALPHA.name);
    await app.subscribeMqtt("sensors/#");
    await app.brokerDisconnectButton.click();
    await expect(app.brokerConnectButton).toBeVisible();

    const dialog = await app.openBrokerEditDialog(ALPHA.name);
    await expect(dialog.getByText(LIVE_NOTICE)).toBeHidden();
    await app.brokerHostInput(dialog).fill("renamed.local");
    await saveButton(dialog).click();
    await expect(dialog).toBeHidden();

    // タブを作り直さないので、購読の行が残る。
    await expect(app.brokerHostInput()).toHaveValue("renamed.local");
    await expect(app.mqttSubscription("sensors/#")).toBeVisible();
    await expect(page.getByRole("alert")).toHaveCount(0);

    await app.brokerConnectButton.click();
    await expect(app.mqttStatus("Connected")).toBeVisible();
    expect((await fake.args("Connect")).at(-1)?.[0]).toMatchObject({
      broker: "tcp://renamed.local:1883",
    });
    await expect(app.mqttSubscription("sensors/#")).toBeVisible();
  });

  // 確立待ちでは購読を足せないので、購読の行が残ることは自動再接続中の中止で確かめる。
  test("clicking Connecting… during an automatic reconnect stops it and keeps the subscription rows", async ({
    page,
    app,
    fake,
  }) => {
    await app.connectBroker(ALPHA.name);
    await app.subscribeMqtt("sensors/#");
    const connectionId = (await fake.snapshot()).mqttConnections[0].id;

    await fake.emit(WailsEvents.mqttConnectionLost, {
      connectionId,
      error: "EOF",
    });
    await expect(app.brokerHostInput()).toBeDisabled();
    await connectingButton(page).click();

    await expect(app.brokerConnectButton).toBeVisible();
    expect(await fake.calls("Disconnect")).toBe(1);
    expect((await fake.snapshot()).mqttConnections).toEqual([]);
    await expect(app.mqttSubscription("sensors/#")).toBeVisible();
    await expect(app.brokerHostInput()).toBeEnabled();
  });
});

// ── 観点D: 接続バーの入力検証 ────────────────────────────────────────────────
// 接続バーは入力のたびに保存する。読み戻せないホスト・ポート (broker-url.ts が既定値として読む)
// は保存しない。

test.describe("connection bar validation", () => {
  test.use({ seed: { mqttProfiles: [ALPHA] } });

  test("the connection bar does not save a host or port that cannot be read back", async ({
    page,
    app,
    fake,
  }) => {
    await app.selectBroker(ALPHA.name);
    const host = app.brokerHostInput();
    const port = app.brokerPortInput();

    const expectRejected = async (saveCalls: number) => {
      await expect(host).toHaveAttribute("aria-invalid", "true");
      await expect(port).toHaveAttribute("aria-invalid", "true");
      await expect(app.brokerConnectButton).toBeDisabled();
      expect(await fake.calls("SaveProfile")).toBe(saveCalls);
      expect((await fake.snapshot()).mqttProfiles[0].broker).toBe(ALPHA.broker);
    };

    await host.fill("");
    await expectRejected(0);

    // 有効な値に戻すと保存する。
    await host.fill("alpha.local");
    await expect(host).toHaveAttribute("aria-invalid", "false");
    await expect(app.brokerConnectButton).toBeEnabled();
    await fake.waitForCalls("SaveProfile", 1);

    for (const invalid of ["0", "65536", ""]) {
      await port.fill(invalid);
      await expectRejected(1);
    }

    await port.fill("1884");
    await expect(port).toHaveAttribute("aria-invalid", "false");
    await expect(app.brokerConnectButton).toBeEnabled();
    await expect(app.broker(ALPHA.name)).toContainText(
      "tcp://alpha.local:1884",
    );
    expect(await fake.calls("SaveProfile")).toBe(2);
    expect((await fake.snapshot()).mqttProfiles[0].broker).toBe(
      "tcp://alpha.local:1884",
    );

    // 無効な入力は保存していないので、リロード後は最後に保存した URL を表示する。
    await port.fill("");
    await page.reload();
    await expect(app.brokerSchemeSelect()).toHaveValue("tcp");
    await expect(host).toHaveValue("alpha.local");
    await expect(port).toHaveValue("1884");
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
    const dialog = app.brokerDialog("New Profile");
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
    pageErrors,
  }) => {
    await app.selectBroker(ALPHA.name);

    const host = app.brokerHostInput();
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
    pageErrors,
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
    pageErrors,
  }) => {
    await app.connectBroker(ALPHA.name);

    await (await app.brokerRowAction(ALPHA.name, "Delete broker")).click();
    await app.confirmDelete();

    await expect(
      page.getByRole("alert").filter({ hasText: "Failed to delete broker" }),
    ).toContainText("access denied");
    // 削除に失敗したら、ブローカーもタブも接続も残す。
    await expect(app.broker(ALPHA.name)).toBeVisible();
    await expect(app.mqttStatus("Connected")).toBeVisible();
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
    await expect(app.mqttStatus("Disconnected")).toBeVisible();
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
    const dialog = app.brokerDialog("New Profile");
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
    await expect(app.mqttStatus("Disconnected")).toBeVisible();
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
    const host = app.brokerHostInput();
    const port = app.brokerPortInput();
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
    app,
    fake,
  }) => {
    await app.selectBroker(ALPHA.name);
    await app.selectBroker(BETA.name);

    await app.brokerPortInput().fill("9999");

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

// ── 観点H: 接続中のブローカーの削除 ──────────────────────────────────────────

test.describe("deleting a connected broker", () => {
  test.use({ seed: { mqttProfiles: [ALPHA, BETA], mqttConnect: "ok" } });

  test("deleting a connected broker disconnects it and closes its tab", async ({
    page,
    app,
    fake,
  }) => {
    await app.connectBroker(ALPHA.name);
    const connectionId = (await fake.snapshot()).mqttConnections[0].id;

    await (await app.brokerRowAction(ALPHA.name, "Delete broker")).click();
    await app.confirmDelete();

    await expect(app.broker(ALPHA.name)).toHaveCount(0);
    // バックエンドに接続を残さない。
    await fake.waitForCalls("Disconnect");
    expect(await fake.args("Disconnect")).toEqual([[connectionId]]);
    expect(await fake.args("DeleteProfile")).toEqual([[ALPHA.id]]);
    const { mqttConnections, mqttProfiles } = await fake.snapshot();
    expect(mqttConnections).toEqual([]);
    expect(mqttProfiles.map((p) => p.id)).toEqual([BETA.id]);

    // 残ったブローカーが選ばれ、接続バーにその URL が出る。
    await expect(app.brokerConnectButton).toBeVisible();
    await expect(app.mqttStatus("Disconnected")).toBeVisible();
    await expect(app.brokerSchemeSelect()).toHaveValue("mqtts");
    await expect(app.brokerHostInput()).toHaveValue("beta.local");
    await expect(app.brokerPortInput()).toHaveValue("8883");
    await expect(page.getByRole("alert")).toHaveCount(0);
  });
});
