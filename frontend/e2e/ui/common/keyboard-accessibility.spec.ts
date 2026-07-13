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
