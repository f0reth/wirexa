import { chromium } from "@playwright/test";

const wailsDevUrl = process.env.WAILS_DEV_URL ?? "http://localhost:34115";

// Playwright の webServer.url はHTTPサーバー起動を確認するが、
// Wailsアプリのレンダリング完了は保証しない。
// この globalSetup でアプリが完全に表示されるまで待機する。
// document.title は index.html の静的な値で HTML が返った時点で成立してしまうので使わない。
// Wails がバインディング (window.go) を注入し、SolidJS がプロトコル切り替えボタンを
// 描画したことをもって起動完了とみなす。
async function globalSetup() {
  const browser = await chromium.launch();
  const context = await browser.newContext({ baseURL: wailsDevUrl });
  const page = await context.newPage();

  try {
    await page.goto("/", { waitUntil: "domcontentloaded" });
    await page.waitForFunction(
      () =>
        (window as unknown as { go?: { adapters?: unknown } }).go?.adapters !==
        undefined,
      undefined,
      { timeout: 30000 },
    );
    await page
      .getByRole("button", { name: "MQTT", exact: true })
      .waitFor({ state: "visible", timeout: 30000 });
  } finally {
    await browser.close();
  }
}

export default globalSetup;
