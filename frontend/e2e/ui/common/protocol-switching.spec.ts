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

// ── 観点B: keep-alive で入力状態が残る ───────────────────────────────────────
// 一度訪れたパネルは display だけを切り替えて残す (App.tsx)。アンマウントする実装に
// 変わると、URL やタブの選択のようにコンポーネント内にしか無い状態が消える。

test("http form input survives a round trip through another protocol", async ({
  app,
}) => {
  await app.switchTo("HTTP");
  await app.urlInput.fill("https://api.example.com/keep-alive");
  await app.openRequestTab("Headers");

  await app.switchTo("MQTT");
  await app.switchTo("HTTP");

  await expect(app.urlInput).toHaveValue("https://api.example.com/keep-alive");
  await expect(
    app.requestEditor.getByRole("tab", { name: "Headers", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
});

// ── 観点B: 選択中のプロトコルを押し直すとサイドバーが開閉する ─────────────────
// 閉じたサイドバーは幅 0 のパネルに overflow: hidden で切り取られるだけで、中身は DOM に
// 残り大きさも持つ。toBeHidden では見分けられないので、切り取りを考慮する toBeInViewport で見る。

test("clicking the active protocol toggles the sidebar", async ({
  page,
  app,
}) => {
  await app.switchTo("HTTP");
  const heading = page.getByText("Collections", { exact: true });

  await app.protocolButton("HTTP").click();
  await expect(heading).not.toBeInViewport();
  await expect(app.protocolButton("HTTP")).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  // サイドバーを閉じてもパネルは HTTP のまま。
  await expect(page.getByTestId("http-panel")).toHaveCSS("display", "flex");

  await app.protocolButton("HTTP").click();
  await expect(heading).toBeInViewport();
  await expect(app.protocolButton("HTTP")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
});

test("selecting another protocol reopens a closed sidebar", async ({
  page,
  app,
}) => {
  await app.switchTo("HTTP");
  await app.protocolButton("HTTP").click();
  await expect(
    page.getByText("Collections", { exact: true }),
  ).not.toBeInViewport();

  await app.switchTo("UDP");
  await expect(page.getByText("Targets", { exact: true })).toBeInViewport();
  await expect(app.protocolButton("UDP")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
});
