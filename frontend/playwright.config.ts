import { defineConfig, devices } from "@playwright/test";

// 既定ではビルド済みバンドル (vite build + vite preview) を配信する。dev サーバーはモジュールを
// 1 ファイル 1 リクエストで返し、テストごとに新しくなるブラウザコンテキストではキャッシュも
// 効かないので、ページの読み込みだけで数秒かかる。
// WIREXA_E2E_DEV=1 のときだけ従来の dev サーバーを使う (playwright test --ui でアプリの
// ソースを編集しながら調べるとき用)。
const useDevServer = process.env.WIREXA_E2E_DEV === "1";
const port = useDevServer ? 5173 : 5175;

// UI e2e。バックエンドは e2e/fake-backend が window.go を差し替えて再現するので、Go も
// wails dev も要らない。状態はページのメモリ (sessionStorage) にしか無いため完全に並列化できる。
export default defineConfig({
  testDir: "./e2e/ui",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  // CI では list も出し、末尾の集計 (N flaky) をログで読めるようにする。retries があるので、
  // ジョブの成否だけでは不安定なテストを見逃す。
  reporter: process.env.CI ? [["list"], ["html"]] : "html",
  // アサーションの既定待ち時間。個々の expect に timeout を撒かずここに集約する。
  expect: { timeout: 10_000 },
  use: {
    baseURL: `http://localhost:${port}`,
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
  webServer: useDevServer
    ? {
        command: "bun run dev:e2e",
        port,
        reuseExistingServer: true,
      }
    : {
        command: "bun run build:e2e && bun run preview:e2e",
        port,
        // プレビューサーバーを使い回すとビルドが走らず、古いバンドルでテストが通ってしまう。
        // ポートが埋まっていれば vite.e2e.config.ts の strictPort で起動に失敗する。
        reuseExistingServer: false,
      },
});
