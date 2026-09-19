import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // 既定は node。DOM が要るファイルだけ先頭に `// @vitest-environment jsdom` を付ける
    // (現状は src/infrastructure/storage/local-storage.test.ts のみ)。
    environment: "node",
    include: ["src/**/*.test.ts"],
    // Node の実験的グローバル localStorage/sessionStorage (Node 22+) が jsdom 環境の
    // localStorage/sessionStorage を覆い隠してしまうため、テストプロセスでは無効化する。
    // (vitest の jsdom 環境は、既に global に存在するキーは上書きしない実装のため)
    execArgv: ["--no-experimental-webstorage"],
  },
});
