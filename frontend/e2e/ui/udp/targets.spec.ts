import { expect, test } from "../../fixtures/ui";

// UDP ターゲットの管理 (サイドバーの一覧と編集ダイアログ)。保存・削除は偽バックエンドの
// UDPHandler に届き、並び順だけは localStorage (udp:targetOrder) に保存される。

const ALPHA = {
  id: "target-alpha",
  name: "Target Alpha",
  host: "127.0.0.1",
  port: 9001,
};
const BETA = {
  id: "target-beta",
  name: "Target Beta",
  host: "127.0.0.1",
  port: 9002,
};

test.beforeEach(async ({ app }) => {
  await app.switchTo("UDP");
});

// ── 観点J: 作成・編集・削除 ──────────────────────────────────────────────────

test("created udp target appears in the sidebar and is saved", async ({
  app,
  fake,
}) => {
  await app.createUdpTarget("Device A", "192.168.0.10", 5000);

  await expect(app.udpTarget("Device A")).toContainText("192.168.0.10:5000");
  const { udpTargets } = await fake.snapshot();
  expect(udpTargets).toEqual([
    {
      id: expect.any(String),
      name: "Device A",
      host: "192.168.0.10",
      port: 5000,
    },
  ]);
});

test.describe("with saved targets", () => {
  test.use({ seed: { udpTargets: [ALPHA, BETA] } });

  test("editing a udp target updates its host:port in the sidebar", async ({
    page,
    app,
    fake,
  }) => {
    await app
      .udpTarget(ALPHA.name)
      .getByRole("button", { name: "Edit target" })
      .click();

    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("Edit Target")).toBeVisible();
    await expect(dialog.getByLabel("Name")).toHaveValue(ALPHA.name);
    await expect(dialog.getByLabel("Host")).toHaveValue(ALPHA.host);
    await expect(dialog.getByLabel("Port")).toHaveValue(String(ALPHA.port));

    await dialog.getByLabel("Host").fill("10.0.0.5");
    await dialog.getByLabel("Port").fill("7000");
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(dialog).toBeHidden();

    await expect(app.udpTarget(ALPHA.name)).toContainText("10.0.0.5:7000");
    // 同じ ID のまま上書きされ、新しいターゲットは増えない。
    const { udpTargets } = await fake.snapshot();
    expect(udpTargets).toEqual([
      { ...ALPHA, host: "10.0.0.5", port: 7000 },
      BETA,
    ]);
  });

  test("deleting a udp target after confirming removes it", async ({
    app,
    fake,
  }) => {
    await app
      .udpTarget(ALPHA.name)
      .getByRole("button", { name: "Delete target" })
      .click();
    await app.confirmDelete();

    await expect(app.udpTarget(ALPHA.name)).toBeHidden();
    await expect(app.udpTarget(BETA.name)).toBeVisible();
    const { udpTargets } = await fake.snapshot();
    expect(udpTargets).toEqual([BETA]);
  });

  test("cancelling the delete dialog keeps the udp target", async ({
    page,
    app,
    fake,
  }) => {
    await app
      .udpTarget(ALPHA.name)
      .getByRole("button", { name: "Delete target" })
      .click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "Cancel" }).click();

    await expect(dialog).toBeHidden();
    await expect(app.udpTarget(ALPHA.name)).toBeVisible();
    expect(await fake.calls("DeleteTarget")).toBe(0);
  });

  // ── 観点C / F: 並び替えと順序の永続化 ──────────────────────────────────────
  // 偽バックエンドは seed の順 (Alpha, Beta) で返すので、リロード後に Beta が先なら
  // localStorage に保存した並びが使われている。

  test("udp targets can be reordered by drag and drop", async ({
    page,
    app,
    fake,
  }) => {
    const rows = page.getByRole("button").filter({ hasText: /^Target / });
    await expect(rows).toHaveText([/Target Alpha/, /Target Beta/]);

    await app.dragListRowBefore(
      app.udpTarget(BETA.name),
      app.udpTarget(ALPHA.name),
    );

    await expect(rows).toHaveText([/Target Beta/, /Target Alpha/]);
    const stored = await page.evaluate(() =>
      localStorage.getItem("udp:targetOrder"),
    );
    expect(JSON.parse(stored ?? "null")).toEqual([BETA.id, ALPHA.id]);
    // 並びはバックエンドに保存しない。
    expect(await fake.calls("SaveTarget")).toBe(0);

    await page.reload();
    await app.switchTo("UDP");
    await expect(rows).toHaveText([/Target Beta/, /Target Alpha/]);
  });
});

// ── 観点D: バックエンドの検証エラー ──────────────────────────────────────────
// ダイアログ側では検証せず、Go の UDPTarget.Validate (偽バックエンドも同じ規則) が拒否する。
// 失敗はトーストで通知され、入力をやり直せるようダイアログは開いたままになる。

test("saving a udp target with port 0 shows an error and keeps the dialog open", async ({
  page,
  app,
  fake,
}) => {
  await page.getByRole("button", { name: "New Target" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill("Bad Port");
  await dialog.getByLabel("Host").fill("127.0.0.1");
  await dialog.getByLabel("Port").fill("0");
  await dialog.getByRole("button", { name: "Save" }).click();

  const toast = page
    .getByRole("alert")
    .filter({ hasText: "Failed to save target" });
  await expect(toast).toContainText("invalid port: must be 1-65535");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByLabel("Port")).toHaveValue("0");
  await expect(app.udpTarget("Bad Port")).toBeHidden();
  expect((await fake.snapshot()).udpTargets).toEqual([]);
});

test("saving a udp target without a host shows an error and keeps the dialog open", async ({
  page,
  fake,
}) => {
  await page.getByRole("button", { name: "New Target" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill("No Host");
  await dialog.getByLabel("Port").fill("9000");
  await dialog.getByRole("button", { name: "Save" }).click();

  const toast = page
    .getByRole("alert")
    .filter({ hasText: "Failed to save target" });
  await expect(toast).toContainText("invalid host: is required");
  await expect(dialog).toBeVisible();
  expect((await fake.snapshot()).udpTargets).toEqual([]);
});

// form-validation.spec.ts から移した。ブラウザの制約属性は付いているが、Save はそれを見ない
// (上のテストのとおりバックエンドが拒否する)。

test("udp target port field has min=1 max=65535 constraints", async ({
  page,
}) => {
  await page.getByRole("button", { name: "New Target" }).click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  const portInput = dialog.getByLabel("Port");
  await expect(portInput).toHaveAttribute("type", "number");
  await expect(portInput).toHaveAttribute("min", "1");
  await expect(portInput).toHaveAttribute("max", "65535");

  const valid = () =>
    portInput.evaluate((el: HTMLInputElement) => el.validity.valid);

  await portInput.fill("0");
  expect(await valid()).toBe(false);

  await portInput.fill("65536");
  expect(await valid()).toBe(false);

  await portInput.fill("8080");
  expect(await valid()).toBe(true);
});
