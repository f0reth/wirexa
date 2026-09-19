import { defineConfig } from "vitest/config";

const nodeMajorVersion = Number(process.versions.node.split(".")[0]);

export default defineConfig({
  test: {
    // 既定は node。DOM が要るファイルだけ先頭に `// @vitest-environment jsdom` を付ける
    environment: "node",
    // Node.js 25+ の組み込み Web Storage と jsdom の localStorage の衝突を避ける。
    execArgv: nodeMajorVersion >= 25 ? ["--no-webstorage"] : [],
    include: ["src/**/*.test.ts"],
    // Node の実験的グローバル localStorage/sessionStorage (Node 22+) が jsdom 環境の
    // localStorage/sessionStorage を覆い隠してしまうため、テストプロセスでは無効化する。
    // (vitest の jsdom 環境は、既に global に存在するキーは上書きしない実装のため)
    execArgv: ["--no-experimental-webstorage"],
  },
});
