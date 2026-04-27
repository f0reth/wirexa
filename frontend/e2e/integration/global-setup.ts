import { chromium } from "@playwright/test";

const wailsDevUrl = process.env.WAILS_DEV_URL ?? "http://localhost:34115";

// Playwright の webServer.url はHTTPサーバー起動を確認するが、
// Wailsアプリのレンダリング完了は保証しない。
// この globalSetup でアプリが完全に表示されるまで待機する。
async function globalSetup() {
  const browser = await chromium.launch();
  const context = await browser.newContext({ baseURL: wailsDevUrl });
  const page = await context.newPage();

  try {
    await page.goto("/", { waitUntil: "domcontentloaded" });
    await page.waitForFunction(() => /Wirexa/.test(document.title), {
      timeout: 30000,
    });
  } finally {
    await browser.close();
  }
}

export default globalSetup;
