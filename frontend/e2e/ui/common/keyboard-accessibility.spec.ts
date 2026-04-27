import { test, expect } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
});

// ── 観点G-3: aria-label が正しく設定されているか ──────────────────────────────

test("protocol switcher buttons have correct aria-labels", async ({ page }) => {
  const protocols = ["MQTT", "HTTP", "UDP", "OpenAPI"] as const;

  for (const label of protocols) {
    const btn = page.getByRole("button", { name: label, exact: true });
    await expect(btn).toBeVisible();
    await expect(btn).toHaveAttribute("aria-label", label);
  }
});

test("mqtt button has aria-pressed=true by default", async ({ page }) => {
  await expect(
    page.getByRole("button", { name: "MQTT", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");

  for (const label of ["HTTP", "UDP", "OpenAPI"]) {
    await expect(
      page.getByRole("button", { name: label, exact: true }),
    ).toHaveAttribute("aria-pressed", "false");
  }
});

test("aria-pressed updates when switching protocols", async ({ page }) => {
  await page.getByRole("button", { name: "HTTP", exact: true }).click();

  await expect(
    page.getByRole("button", { name: "HTTP", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("button", { name: "MQTT", exact: true }),
  ).toHaveAttribute("aria-pressed", "false");
});

// ── 観点G-4: Tab キーによるフォーカス移動 ───────────────────────────────────

test("tab key moves focus away from URL input field", async ({ page }) => {
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(
    page.getByPlaceholder("https://api.example.com/endpoint"),
  ).toBeVisible();

  const urlInput = page.getByPlaceholder("https://api.example.com/endpoint");
  await urlInput.focus();
  await expect(urlInput).toBeFocused();

  await page.keyboard.press("Tab");

  await expect(urlInput).not.toBeFocused();
});

test("tab key moves focus to send button from URL input", async ({ page }) => {
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(
    page.getByPlaceholder("https://api.example.com/endpoint"),
  ).toBeVisible();

  await page
    .getByPlaceholder("https://api.example.com/endpoint")
    .fill("https://api.example.com");

  const urlInput = page.getByPlaceholder("https://api.example.com/endpoint");
  await urlInput.focus();
  await page.keyboard.press("Tab");

  const sendButton = page.getByRole("button", { name: "Send", exact: true });
  await expect(sendButton).toBeFocused();
});
