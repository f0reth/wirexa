import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig, devices } from "@playwright/test";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// wails dev のデフォルトアドレス。変更が必要な場合は WAILS_DEV_URL 環境変数で上書きできる。
const wailsDevUrl = process.env.WAILS_DEV_URL ?? "http://localhost:34115";

export default defineConfig({
  globalSetup: "./e2e/fullstack/global-setup.ts",
  testDir: "./e2e/fullstack",
  testMatch: "**/*.spec.ts",
  testIgnore: "**/global-setup.ts",
  fullyParallel: false, // Wails アプリは1インスタンスのため並列不可
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: "html",
  use: {
    baseURL: wailsDevUrl,
    trace: "on-first-retry",
    launchOptions: {
      cdpPort: 0,
      timeout: 360000,
      // biome-ignore lint/suspicious/noExplicitAny: cdpPort is a Chromium-specific property not in Playwright's LaunchOptions type
    } as any,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    // wails dev はプロジェクトルートから実行する必要がある
    cwd: path.resolve(__dirname, ".."),
    command: "wails dev",
    url: wailsDevUrl,
    // Go のコンパイルを含むため長めのタイムアウトを設定
    timeout: 120 * 1000,
    // wails dev が既に起動している場合はそれを再利用する（ローカル開発での快適な体験のため）
    reuseExistingServer: true,
  },
});
