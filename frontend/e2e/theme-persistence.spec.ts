import { test, expect } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
});

// ── 観点F-1: ダーク/ライトテーマ切り替えの永続化 ─────────────────────────────

test("dark theme persists after page reload", async ({ page }) => {
  // 初期状態はライトモード
  await expect(
    page.getByRole("button", { name: "Switch to dark mode" }),
  ).toBeVisible();

  // テーマトグルをクリックしてダークモードへ
  await page.getByRole("button", { name: "Switch to dark mode" }).click();
  await expect(
    page.getByRole("button", { name: "Switch to light mode" }),
  ).toBeVisible();

  // ページをリロード
  await page.reload();

  // リロード後もダークモードが維持されている
  await expect(
    page.getByRole("button", { name: "Switch to light mode" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Switch to dark mode" }),
  ).not.toBeVisible();
});

test("light theme is restored after switching back and reloading", async ({
  page,
}) => {
  // ダークモードに切り替える
  await page.getByRole("button", { name: "Switch to dark mode" }).click();
  await expect(
    page.getByRole("button", { name: "Switch to light mode" }),
  ).toBeVisible();

  // ライトモードに戻す
  await page.getByRole("button", { name: "Switch to light mode" }).click();
  await expect(
    page.getByRole("button", { name: "Switch to dark mode" }),
  ).toBeVisible();

  // リロード後もライトモードが維持されている
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Switch to dark mode" }),
  ).toBeVisible();
});
