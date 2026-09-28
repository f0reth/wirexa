import { expect, test } from "../../fixtures/ui";

// ── 観点G-3: aria-label が正しく設定されているか ──────────────────────────────
// aria-pressed の既定値と切り替えは protocol-switching.spec.ts が見ている。

test("protocol switcher buttons have correct aria-labels", async ({ page }) => {
  const protocols = ["MQTT", "HTTP", "UDP", "OpenAPI"] as const;

  for (const label of protocols) {
    const btn = page.getByRole("button", { name: label, exact: true });
    await expect(btn).toBeVisible();
    await expect(btn).toHaveAttribute("aria-label", label);
  }
});

// ── 観点G-4: Tab キーによるフォーカス移動 ───────────────────────────────────

test("tab key moves focus away from URL input field", async ({ app }) => {
  await app.switchTo("HTTP");

  await app.urlInput.focus();
  await expect(app.urlInput).toBeFocused();

  await app.page.keyboard.press("Tab");

  await expect(app.urlInput).not.toBeFocused();
});

test("tab key moves focus to send button from URL input", async ({ app }) => {
  await app.switchTo("HTTP");

  await app.urlInput.fill("https://api.example.com");
  await app.urlInput.focus();
  await app.page.keyboard.press("Tab");

  await expect(app.sendButton).toBeFocused();
});

// ── 観点G-1: Enter キーで HTTP リクエスト送信 ────────────────────────────────

test.describe("sending with the keyboard", () => {
  // 応答を遅らせて「送信中」を観測できるようにする
  test.use({ seed: { httpResponseDelayMs: 30_000 } });

  test("pressing Enter in URL field sends the request", async ({ app }) => {
    await app.switchTo("HTTP");

    await app.urlInput.fill("http://127.0.0.1:9999/");
    await app.urlInput.press("Enter");

    await expect(app.cancelButton).toBeVisible();
  });
});

// ── 観点G-5: ダイアログの Escape と Tab 循環 (focus-trap.ts) ────────────────

test("Escape closes the confirm dialog without deleting", async ({
  page,
  app,
  fake,
}) => {
  await app.switchTo("HTTP");
  const collection = await app.createCollection("Escape Collection");

  await app.rowAction(collection, "Delete collection").click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");

  await expect(dialog).toBeHidden();
  await expect(collection).toBeVisible();
  expect(await fake.calls("DeleteCollection")).toBe(0);
});

test("Tab cycles focus inside the broker dialog", async ({ page }) => {
  await page.getByRole("button", { name: "New Broker" }).click();
  const dialog = page.getByRole("dialog");
  const name = dialog.getByLabel("Name", { exact: true });

  // 開いた時点で先頭の入力欄にフォーカスがある。
  await expect(name).toBeFocused();

  // 先頭から Shift+Tab で末尾へ回り込み、そこから Tab で先頭へ戻る。
  await page.keyboard.press("Shift+Tab");
  await expect(name).not.toBeFocused();
  await expect(dialog.locator(":focus")).toHaveCount(1);
  await page.keyboard.press("Tab");
  await expect(name).toBeFocused();

  // 一周しても、フォーカスはダイアログの外へ出ない。
  const focusable = await dialog
    .locator("input:not([disabled]), select:not([disabled]), button:not([disabled])")
    .count();
  for (let i = 0; i < focusable; i++) {
    await page.keyboard.press("Tab");
    await expect(dialog.locator(":focus")).toHaveCount(1);
  }
  await expect(name).toBeFocused();
});

// ── 観点G-6: キーボードでのツリー／リスト選択 ────────────────────────────────
// ブローカー・ターゲット・プリセットの行は role="button" の div で、Enter / Space を
// 自前で処理する (profile-list.tsx, publish-tab.tsx)。プリセットは mqtt/publish.spec.ts で見る。

test.describe("selecting with the keyboard", () => {
  const TARGET_A = { id: "target-a", name: "Target A", host: "10.0.0.1", port: 5001 };
  const TARGET_B = { id: "target-b", name: "Target B", host: "10.0.0.2", port: 5002 };

  test.use({
    seed: {
      collections: [
        {
          name: "Keyboard Collection",
          requests: [
            { name: "Keyboard Request", url: "https://api.example.com/kb" },
          ],
        },
      ],
      mqttProfiles: [
        { id: "profile-kb", name: "Keyboard Broker", broker: "tcp://kb.local:1884" },
      ],
      udpTargets: [TARGET_A, TARGET_B],
    },
  });

  test("Enter on a focused request selects it", async ({ app }) => {
    await app.switchTo("HTTP");
    const request = app.request(/Keyboard Request/);

    await request.focus();
    await request.press("Enter");

    await expect(request).toHaveAttribute("aria-current", "true");
    await expect(app.urlInput).toHaveValue("https://api.example.com/kb");
  });

  test("Enter on a focused broker selects it", async ({ page, app }) => {
    const row = app.broker("Keyboard Broker");

    await row.focus();
    await row.press("Enter");

    await expect(app.brokerConnectButton).toBeVisible();
    await expect(page.getByPlaceholder("localhost")).toHaveValue("kb.local");
    await expect(page.getByPlaceholder("1883")).toHaveValue("1884");
  });

  test("Space on a focused udp target selects it", async ({ page, app }) => {
    await app.switchTo("UDP");
    const row = app.udpTarget(TARGET_B.name);

    await row.focus();
    await row.press("Space");

    // 選んだターゲットの宛先が送信フォームに読み込まれる。
    await expect(page.getByPlaceholder("127.0.0.1")).toHaveValue(TARGET_B.host);
    await expect(page.getByPlaceholder("12345")).toHaveValue(
      String(TARGET_B.port),
    );
  });
});
