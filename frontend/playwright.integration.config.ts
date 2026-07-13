import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig, devices } from "@playwright/test";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

// wails dev のデフォルトアドレス。変更が必要な場合は WAILS_DEV_URL 環境変数で上書きできる。
const wailsDevUrl = process.env.WAILS_DEV_URL ?? "http://localhost:34115";

// ビルド済みバンドルを配信するプレビューサーバー。wails dev にはここへプロキシさせる。
//
// wails.json の frontend:dev:serverUrl は "auto" で、通常の `wails dev` は Vite の dev サーバーへ
// プロキシする。dev サーバーは ESM モジュールを 1 ファイル 1 リクエストで返すため、1 ロードあたり
// 数百の新規 TCP 接続が wails → Vite 間に張られ、Windows ではエフェメラルポートが枯渇して
// ページが真っ白になっていた (このスイートのフレークの正体)。
// プレビューサーバーはビルド済みの単一バンドルを返すので、1 ロードあたり数リクエストで済む。
const previewPort = 5174;
const previewUrl = `http://localhost:${previewPort}`;

// バックエンドの保存先を隔離する。app.go は os.UserConfigDir() を使い、Windows ではこれが
// %APPDATA% を読む。子プロセス (wails dev → アプリ) にだけ一時ディレクトリを渡すことで、
// 本番コードを変えずにテストデータを実ユーザーの AppData から切り離せる。
// これがあるので、前回実行の残骸を UI 越しに消して回る afterEach は要らない。
// 固定パスを使い、実行の「開始時」に消す。終了時に消そうとすると、まだ生きている WebView2 が
// ロックを握っていて EBUSY になる。開始時なら前回のプロセスは終わっているので確実に消せるし、
// 一時ディレクトリが溜まっていくこともない。
// この config はワーカープロセスでも読み込まれる。掃除はランナー (ワーカーでない側) でだけ行う。
// ワーカーが読み込む頃にはアプリが起動していて、消そうとすると EPERM になる。
const appDataDir = path.join(os.tmpdir(), "wirexa-e2e-appdata");
if (process.env.TEST_WORKER_INDEX === undefined) {
  fs.rmSync(appDataDir, { recursive: true, force: true });
  fs.mkdirSync(appDataDir, { recursive: true });
}

// 既存サーバーを使い回すとこの隔離が効かない (env が渡らない) ため、明示的な opt-in にする。
const reuseExistingServer = process.env.WIREXA_E2E_REUSE === "1";

export default defineConfig({
  globalSetup: "./e2e/integration/global-setup.ts",
  testDir: "./e2e/integration",
  testMatch: "**/*.spec.ts",
  testIgnore: "**/global-setup.ts",
  fullyParallel: false, // Wails アプリは1インスタンスのため並列不可
  forbidOnly: !!process.env.CI,
  // ローカルでも再試行しない。常時 retries を効かせるとフレークが失敗として表面化せず、
  // 実行時間だけが延びる。
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: "html",
  // アサーションの既定待ち時間。個々の expect に 10000〜20000 を撒くのをやめてここに集約する。
  expect: { timeout: 10_000 },
  use: {
    baseURL: wailsDevUrl,
    screenshot: "only-on-failure",
    trace: "on-first-retry",
    actionTimeout: 15_000,
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
  webServer: [
    {
      cwd: __dirname,
      command: `bun run build && bunx vite preview --port ${previewPort} --strictPort`,
      url: previewUrl,
      // Vite のプロダクションビルドを含むため長めに取る
      timeout: 180 * 1000,
      reuseExistingServer,
    },
    {
      // wails dev はプロジェクトルートから実行する必要がある
      cwd: repoRoot,
      // -s: フロントエンドのビルドは上のサーバー側で済ませてあるので skip する
      // -noreload / -nogorebuild: テスト中のファイル監視・再ビルドは不要
      command: `wails dev -s -noreload -nogorebuild -frontenddevserverurl ${previewUrl}`,
      url: wailsDevUrl,
      env: { APPDATA: appDataDir },
      // Go のコンパイルを含むため長めのタイムアウトを設定
      timeout: 120 * 1000,
      reuseExistingServer,
    },
  ],
});
