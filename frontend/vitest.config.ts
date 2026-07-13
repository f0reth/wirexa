import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // 既定は node。DOM が要るファイルだけ先頭に `// @vitest-environment jsdom` を付ける
    // (現状は src/infrastructure/storage/local-storage.test.ts のみ)。
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
