import { test as base, expect } from "@playwright/test";
import { App } from "./app";

export { expect };
export { App } from "./app";

/**
 * fullstack e2e 用の test。実 Go バックエンド (wails dev) に対して動く。
 * 保存先は playwright.integration.config.ts が APPDATA を一時ディレクトリへ向けて隔離するので、
 * 前回実行の残骸を UI 越しに消して回る afterEach は不要。
 */
export const test = base.extend<{ app: App }>({
  page: async ({ page }, use) => {
    // goto → clear → reload の二重ロードを避ける。wails dev は全アセットを Vite へプロキシする
    // ため、ロード回数がそのまま TCP 接続数に効く (Windows のエフェメラルポート枯渇対策)。
    // 初回ロードでだけ消すので、page.reload() で復元を検証するテストの邪魔をしない。
    await page.addInitScript(() => {
      if (!sessionStorage.getItem("__wirexa_e2e_boot")) {
        sessionStorage.setItem("__wirexa_e2e_boot", "1");
        localStorage.clear();
      }
    });
    await page.goto("/");
    await use(page);
  },

  app: async ({ page }, use) => {
    await use(new App(page));
  },
});
