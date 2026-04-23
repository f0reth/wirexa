import { test, expect } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
});

test("mqtt button is active by default on app launch", async ({ page }) => {
  const mqttButton = page.getByRole("button", { name: "MQTT" });
  await expect(mqttButton).toHaveAttribute("aria-pressed", "true");

  const httpButton = page.getByRole("button", { name: "HTTP" });
  await expect(httpButton).toHaveAttribute("aria-pressed", "false");
});

test("switching to http shows http panel", async ({ page }) => {
  await page.getByRole("button", { name: "HTTP" }).click();

  const httpPanel = page.getByTestId("http-panel");
  await expect(httpPanel).toHaveCSS("display", "flex");

  const mqttPanel = page.getByTestId("mqtt-panel");
  await expect(mqttPanel).toHaveCSS("display", "none");
});

test("dark theme is restored from localStorage on reload", async ({ page }) => {
  await page.evaluate(() =>
    localStorage.setItem("app:theme", JSON.stringify("dark")),
  );
  await page.reload();

  const themeButton = page.getByRole("button", { name: "Switch to light mode" });
  await expect(themeButton).toBeVisible();
});
