import { expect, test } from "./fixtures/app";

async function goToUdp(page: import("@playwright/test").Page) {
  await page.getByTitle("UDP").click();
}

test.describe("UDP Client UI", () => {
  test("ターゲット一覧表示 - startup shows targets in sidebar", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setUdpTargets([
      { id: "t1", name: "Local Server", host: "127.0.0.1", port: 9000 },
      { id: "t2", name: "Remote Server", host: "10.0.0.1", port: 8080 },
    ]);
    await goToUdp(page);

    await expect(page.getByText("Local Server")).toBeVisible();
    await expect(page.getByText("Remote Server")).toBeVisible();
  });

  test("ターゲット作成 - host/port input → save → target appears in list", async ({
    page,
  }) => {
    await goToUdp(page);
    await page.getByTitle("New Target").click();

    await page.getByLabel("Name").fill("My UDP Target");
    await page.getByLabel("Host").fill("192.168.1.10");
    await page.getByLabel("Port").fill("5000");
    await page.getByRole("button", { name: "Save" }).click();

    await expect(page.getByText("My UDP Target")).toBeVisible();
  });

  test("ターゲット削除 - delete → removed from list", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setUdpTargets([
      { id: "t1", name: "Delete Me", host: "127.0.0.1", port: 9000 },
    ]);
    await goToUdp(page);

    await expect(page.getByText("Delete Me")).toBeVisible();

    await page.getByText("Delete Me").hover();
    await page.getByTitle("Delete").click();

    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Delete" })
      .click();

    await expect(page.getByText("Delete Me")).not.toBeVisible();
  });

  test("フレーム送信 (text) - payload input → send", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setUdpTargets([
      { id: "t1", name: "Send Target", host: "127.0.0.1", port: 9000 },
    ]);
    await goToUdp(page);
    await page.getByText("Send Target").click();

    // "text" encoding is the default; payload textarea uses "Enter payload..." placeholder
    await page.getByPlaceholder("Enter payload...").fill("Hello UDP");
    await page
      .getByRole("button", { name: /^Send$/ })
      .last()
      .click();

    await expect(
      page.getByRole("button", { name: /^Send$/ }).last(),
    ).toBeEnabled();
  });

  test("フレーム送信 (json) - json encoding send", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setUdpTargets([
      { id: "t1", name: "Send Target", host: "127.0.0.1", port: 9000 },
    ]);
    await goToUdp(page);
    await page.getByText("Send Target").click();

    await page.getByRole("radio", { name: "json" }).click();
    await page.getByPlaceholder('{"key": "value"}').fill('{"value":42}');
    await page
      .getByRole("button", { name: /^Send$/ })
      .last()
      .click();

    await expect(
      page.getByRole("button", { name: /^Send$/ }).last(),
    ).toBeEnabled();
  });

  test("リスナー起動 - port input → start → session shown", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setUdpTargets([
      { id: "t1", name: "Listen Target", host: "127.0.0.1", port: 9000 },
    ]);
    await goToUdp(page);
    await page.getByText("Listen Target").click();

    await page.getByRole("button", { name: "Listen" }).click();

    await page.getByPlaceholder("12345").fill("7777");
    await page.getByRole("button", { name: "Start" }).click();

    await expect(page.getByText(/Listening :7777/)).toBeVisible();
  });

  test("リスナー停止 - stop button → session removed", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setUdpTargets([
      { id: "t1", name: "Listen Target", host: "127.0.0.1", port: 9000 },
    ]);
    await goToUdp(page);
    await page.getByText("Listen Target").click();

    await page.getByRole("button", { name: "Listen" }).click();

    await page.getByPlaceholder("12345").fill("7777");
    await page.getByRole("button", { name: "Start" }).click();
    await expect(page.getByText(/Listening :7777/)).toBeVisible();

    await page.getByRole("button", { name: "Stop" }).click();

    await expect(page.getByText(/Listening :7777/)).not.toBeVisible();
  });

  test("受信データ表示 - message event → displayed in received list", async ({
    page,
    wailsMock,
  }) => {
    await wailsMock.setUdpTargets([
      { id: "t1", name: "Listen Target", host: "127.0.0.1", port: 9000 },
    ]);
    await goToUdp(page);
    await page.getByText("Listen Target").click();

    await page.getByRole("button", { name: "Listen" }).click();

    await page.getByPlaceholder("12345").fill("7777");
    await page.getByRole("button", { name: "Start" }).click();
    await expect(page.getByText(/Listening :7777/)).toBeVisible();

    await wailsMock.emit("udp:message", {
      sessionId: "s1",
      port: 7777,
      remoteAddr: "192.168.1.5:54321",
      payload: "hello world",
      encoding: "text",
      timestamp: Date.now(),
    });

    await expect(page.getByText("hello world")).toBeVisible();
  });
});
