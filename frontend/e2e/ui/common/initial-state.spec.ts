import { expect, test } from "../../fixtures/ui";

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

// ── 観点A: 各プロトコルの空状態とパネルの表示 ────────────────────────────────

test("each protocol shows its empty state on first launch", async ({
  page,
  app,
}) => {
  await expect(page.getByText("No brokers yet")).toBeVisible();
  await expect(
    page.getByText(
      "No active connection. Select a broker from the sidebar to connect.",
    ),
  ).toBeVisible();

  await app.switchTo("HTTP");
  await expect(page.getByText("No collections yet")).toBeVisible();
  await expect(
    page.getByText("Send a request to see the response"),
  ).toBeVisible();

  await app.switchTo("UDP");
  await expect(page.getByText("No targets yet")).toBeVisible();
  await expect(page.getByText("ターゲットを選択してください")).toBeVisible();

  await app.switchTo("OpenAPI");
  await expect(page.getByText("No files opened yet")).toBeVisible();
  await expect(page.getByText(/^No file opened/)).toBeVisible();
});

test("udp and openapi panels become visible when selected", async ({
  page,
  app,
}) => {
  await app.switchTo("UDP");
  await expect(page.getByTestId("udp-panel")).toHaveCSS("display", "flex");

  await app.switchTo("OpenAPI");
  await expect(page.getByTestId("openapi-panel")).toHaveCSS("display", "flex");
  await expect(page.getByTestId("udp-panel")).toHaveCSS("display", "none");
});
