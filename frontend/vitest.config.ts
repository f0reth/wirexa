import { defineConfig } from "vitest/config";

const nodeMajorVersion = Number(process.versions.node.split(".")[0]);

export default defineConfig({
  test: {
    // 既定は node。DOM が要るファイルだけ先頭に `// @vitest-environment jsdom` を付ける
    environment: "node",
    // Node のグローバル localStorage/sessionStorage (Node 22+) が jsdom 環境の
    // localStorage/sessionStorage を覆い隠してしまうため、テストプロセスでは無効化する。
    // (vitest の jsdom 環境は、既に global に存在するキーは上書きしない実装のため)
    // Node 25 で Web Storage が安定化した際にフラグ名が変わったので版で振り分ける。
    execArgv:
      nodeMajorVersion >= 25
        ? ["--no-webstorage"]
        : ["--no-experimental-webstorage"],
    include: ["src/**/*.test.ts"],
  },
});
