import type { Plugin } from "vite";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

// 偽バックエンド (e2e/fake-backend/install.ts) をアプリ本体より先に実行させる。
// module script は文書順に実行されるため、head の先頭に挿せば main.tsx より前に window.go が生える。
function fakeBackendPlugin(): Plugin {
  return {
    name: "wirexa-fake-backend",
    transformIndexHtml() {
      return [
        {
          tag: "script",
          injectTo: "head-prepend",
          attrs: { type: "module", src: "/e2e/fake-backend/install.ts" },
        },
      ];
    },
  };
}

export default defineConfig({
  plugins: [fakeBackendPlugin(), solid()],
  resolve: {
    dedupe: ["@codemirror/state", "@codemirror/view"],
  },
  server: {
    port: 5173,
  },
});
