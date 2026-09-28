import { test as base, expect, type Page } from "@playwright/test";
import { App } from "./app";

export { expect };
export { App } from "./app";

/**
 * fullstack e2e 用の test。実 Go バックエンド (wails dev) に対して動く。
 * 保存先は playwright.integration.config.ts が APPDATA を一時ディレクトリへ向けて隔離するので、
 * 前回実行の残骸を UI 越しに消して回る afterEach は不要。
 * ただし Go 側のメモリにだけある状態 (UDP リスナーなど) はページを読み直しても消えず、次のテストの
 * 読み込み時に復元される。これは spec の afterEach で下の掃除ヘルパーを呼んで止める。
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

/**
 * Wails が window.go へ注入するバインディング。生成された .d.ts から型を借りるので、
 * Go 側の rename に追従できていなければ tsc で落ちる。page.evaluate の中でキャストして使う。
 */
export interface WailsGo {
  adapters: {
    HTTPHandler: typeof import("../../wailsjs/go/adapters/HTTPHandler");
    MQTTHandler: typeof import("../../wailsjs/go/adapters/MQTTHandler");
    UDPHandler: typeof import("../../wailsjs/go/adapters/UDPHandler");
  };
}

/**
 * Go 側に残っている UDP リスナーをすべて止める。UI の Stop ボタンに頼らないので、
 * テストが途中で失敗しても次のテストに受信中のセッションが持ち越されない。
 */
export async function stopUdpListeners(page: Page): Promise<void> {
  await page.evaluate(async () => {
    const udp = (window as unknown as { go: WailsGo }).go.adapters.UDPHandler;
    const sessions = await udp.GetListeners();
    await Promise.all(sessions.map((s) => udp.StopListen(s.id)));
  });
}
