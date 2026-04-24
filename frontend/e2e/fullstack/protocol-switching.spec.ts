import { test, expect } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.clear());
  await page.reload();
});

test("can switch between all protocols in order", async ({ page }) => {
  const mqttButton = page.getByRole("button", { name: "MQTT", exact: true });
  const httpButton = page.getByRole("button", { name: "HTTP", exact: true });
  const udpButton = page.getByRole("button", { name: "UDP", exact: true });
  const openApiButton = page.getByRole("button", {
    name: "OpenAPI",
    exact: true,
  });

  await expect(mqttButton).toHaveAttribute("aria-pressed", "true");

  await httpButton.click();
  await expect(httpButton).toHaveAttribute("aria-pressed", "true");
  await expect(mqttButton).toHaveAttribute("aria-pressed", "false");

  await udpButton.click();
  await expect(udpButton).toHaveAttribute("aria-pressed", "true");
  await expect(httpButton).toHaveAttribute("aria-pressed", "false");

  await openApiButton.click();
  await expect(openApiButton).toHaveAttribute("aria-pressed", "true");
  await expect(udpButton).toHaveAttribute("aria-pressed", "false");
});

test("sidebar content changes when switching protocols", async ({ page }) => {
  await expect(page.getByText("Brokers")).toBeVisible();

  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByText("Collections")).toBeVisible();
  await expect(page.getByText("Brokers")).not.toBeVisible();

  await page.getByRole("button", { name: "UDP", exact: true }).click();
  await expect(page.getByText("Targets")).toBeVisible();
  await expect(page.getByText("Collections")).not.toBeVisible();

  await page.getByRole("button", { name: "OpenAPI", exact: true }).click();
  await expect(page.getByText("OpenAPI Files")).toBeVisible();
  await expect(page.getByText("Targets")).not.toBeVisible();
});

test("returning to mqtt after visiting http preserves mqtt panel in dom", async ({
  page,
}) => {
  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(page.getByTestId("http-panel")).toHaveCSS("display", "flex");

  // MQTTパネルはDOMに残ったまま非表示になっている（keep-alive）
  const mqttPanel = page.getByTestId("mqtt-panel");
  await expect(mqttPanel).toHaveCSS("display", "none");

  await page.getByRole("button", { name: "MQTT", exact: true }).click();
  await expect(mqttPanel).toHaveCSS("display", "flex");
  await expect(page.getByTestId("http-panel")).toHaveCSS("display", "none");
});

test("previous protocol panel is hidden (display:none) after switch", async ({
  page,
}) => {
  const mqttPanel = page.getByTestId("mqtt-panel");
  await expect(mqttPanel).toHaveCSS("display", "flex");

  await page.getByRole("button", { name: "HTTP", exact: true }).click();
  await expect(mqttPanel).toHaveCSS("display", "none");

  await page.getByRole("button", { name: "UDP", exact: true }).click();
  await expect(page.getByTestId("http-panel")).toHaveCSS("display", "none");

  await page.getByRole("button", { name: "OpenAPI", exact: true }).click();
  await expect(page.getByTestId("udp-panel")).toHaveCSS("display", "none");
});
