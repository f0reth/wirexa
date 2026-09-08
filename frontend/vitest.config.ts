import { defineConfig } from "vitest/config";

const nodeMajorVersion = Number(process.versions.node.split(".")[0]);

export default defineConfig({
  test: {
    // 既定は node。DOM が要るファイルだけ先頭に `// @vitest-environment jsdom` を付ける
    environment: "node",
    // Node.js 25+ の組み込み Web Storage と jsdom の localStorage の衝突を避ける。
    execArgv: nodeMajorVersion >= 25 ? ["--no-webstorage"] : [],
    include: ["src/**/*.test.ts"],
  },
});
