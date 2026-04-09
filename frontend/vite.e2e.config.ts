import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

const mockWailsDir = resolve(
  fileURLToPath(new URL(".", import.meta.url)),
  "e2e/mocks/wailsjs",
);

export default defineConfig({
  plugins: [
    solid(),
    {
      name: "mock-wailsjs",
      enforce: "pre",
      resolveId(id: string) {
        const match = id.replace(/\\/g, "/").match(/.*wailsjs\/(.*)/);
        if (match) {
          return resolve(mockWailsDir, match[1]);
        }
        return null;
      },
    },
  ],
  server: {
    port: 5173,
  },
});
