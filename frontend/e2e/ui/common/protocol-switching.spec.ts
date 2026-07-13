import { expect, test } from "../../fixtures/ui";

test("can switch between all protocols in order", async ({ app }) => {
  await expect(app.protocolButton("MQTT")).toHaveAttribute(
    "aria-pressed",
    "true",
  );

  await app.protocolButton("HTTP").click();
  await expect(app.protocolButton("HTTP")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(app.protocolButton("MQTT")).toHaveAttribute(
    "aria-pressed",
    "false",
  );

  await app.protocolButton("UDP").click();
  await expect(app.protocolButton("UDP")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(app.protocolButton("HTTP")).toHaveAttribute(
    "aria-pressed",
    "false",
  );

  await app.protocolButton("OpenAPI").click();
  await expect(app.protocolButton("OpenAPI")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(app.protocolButton("UDP")).toHaveAttribute(
    "aria-pressed",
    "false",
  );
});

test("sidebar content changes when switching protocols", async ({
  page,
  app,
}) => {
  await expect(page.getByText("Brokers", { exact: true })).toBeVisible();

  await app.switchTo("HTTP");
  await expect(page.getByText("Brokers", { exact: true })).toBeHidden();

  await app.switchTo("UDP");
  await expect(page.getByText("Collections", { exact: true })).toBeHidden();

  await app.switchTo("OpenAPI");
  await expect(page.getByText("Targets", { exact: true })).toBeHidden();
});

test("returning to mqtt after visiting http preserves mqtt panel in dom", async ({
  page,
  app,
}) => {
  await app.switchTo("HTTP");
  await expect(page.getByTestId("http-panel")).toHaveCSS("display", "flex");

  // MQTTパネルはDOMに残ったまま非表示になっている（keep-alive）
  const mqttPanel = page.getByTestId("mqtt-panel");
  await expect(mqttPanel).toHaveCSS("display", "none");

  await app.switchTo("MQTT");
  await expect(mqttPanel).toHaveCSS("display", "flex");
  await expect(page.getByTestId("http-panel")).toHaveCSS("display", "none");
});

test("previous protocol panel is hidden (display:none) after switch", async ({
  page,
  app,
}) => {
  await expect(page.getByTestId("mqtt-panel")).toHaveCSS("display", "flex");

  await app.switchTo("HTTP");
  await expect(page.getByTestId("mqtt-panel")).toHaveCSS("display", "none");

  await app.switchTo("UDP");
  await expect(page.getByTestId("http-panel")).toHaveCSS("display", "none");

  await app.switchTo("OpenAPI");
  await expect(page.getByTestId("udp-panel")).toHaveCSS("display", "none");
});
