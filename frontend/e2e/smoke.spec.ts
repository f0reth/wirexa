import { test, expect } from "@playwright/test";

test("app launches and renders", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/Wirexa/);
});
