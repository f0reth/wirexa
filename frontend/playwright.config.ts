import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: "html",
  use: {
    baseURL: "http://localhost:5173",
    trace: "on-first-retry",
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
