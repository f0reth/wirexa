import { test, expect } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
});

test("app launches and renders", async ({ page }) => {
  await expect(page).toHaveTitle(/Wirexa/);
});
