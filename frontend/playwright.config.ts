import { defineConfig, devices } from "@playwright/test";

// UI e2e。バックエンドは e2e/fake-backend が window.go を差し替えて再現するので、Go も
// wails dev も要らない。状態はページのメモリ (sessionStorage) にしか無いため完全に並列化できる。
export default defineConfig({
  testDir: "./e2e/ui",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: "html",
  // アサーションの既定待ち時間。個々の expect に timeout を撒かずここに集約する。
  expect: { timeout: 10_000 },
  use: {
    baseURL: "http://localhost:5173",
    trace: "on-first-retry",
    actionTimeout: 15_000,
    launchOptions: {
      // cdpPort forces WebSocket transport instead of pipe (works better with Bun on Windows)
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
    command: "bun run dev:e2e",
    port: 5173,
    reuseExistingServer: !process.env.CI,
  },
});
