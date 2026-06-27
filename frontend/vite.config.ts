import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  resolve: {
    dedupe: ["@codemirror/state", "@codemirror/view"],
  },
  build: {
    target: "esnext",
    cssMinify: "lightningcss",
    sourcemap: false,
    rolldownOptions: {
      treeshake: true,
    },
  },
});
