import type { Plugin } from "vite";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

// 偽バックエンド (e2e/fake-backend/install.ts) をアプリ本体より先に実行させる。
// module script は文書順に実行されるため、head の先頭に挿せば src/index.tsx より前に window.go が生える。
function fakeBackendPlugin(): Plugin {
  return {
    name: "wirexa-fake-backend",
    transformIndexHtml: {
      // ビルドでは Vite が HTML を処理する前に挿さないと、install.ts がバンドルに含まれない。
      order: "pre",
      handler() {
        return [
          {
            tag: "script",
            injectTo: "head-prepend",
            attrs: { type: "module", src: "/e2e/fake-backend/install.ts" },
          },
        ];
      },
    },
  };
}

export default defineConfig({
  plugins: [fakeBackendPlugin(), solid()],
  resolve: {
    dedupe: ["@codemirror/state", "@codemirror/view"],
  },
  build: {
    // dist は main.go の go:embed と wails build が使う。偽バックエンド入りのバンドルを
    // 本番バイナリに混ぜないよう、必ず別のディレクトリへ出す。
    outDir: "dist-e2e",
  },
  server: {
    port: 5173,
  },
  // 5173 (wails dev / bun run dev) とも 5174 (フルスタック e2e のプレビュー) とも別のポートにする。
  preview: {
    port: 5175,
    strictPort: true,
  },
});
