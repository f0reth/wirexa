import { test as base, expect } from "@playwright/test";
import type { FakeBackend, FakeSeed } from "../fake-backend/types";
import { App } from "./app";

export { expect };
export { App } from "./app";

interface Fixtures {
  /** test.use({ seed: {...} }) で仕込む初期状態。ページ読み込み前に注入される。 */
  seed: FakeSeed;
  app: App;
  fake: FakeControl;
}

/** ブラウザ側の偽バックエンドをテストから覗く口。 */
export class FakeControl {
  constructor(private readonly app: App) {}

  /** バインディングの呼び出し回数。 */
  calls(name: string): Promise<number> {
    return this.app.page.evaluate(
      (n) => (window as unknown as { __wirexaFake: FakeBackend }).__wirexaFake
        .calls[n] ?? 0,
      name,
    );
  }

  /**
   * バインディングが指定回数以上呼ばれるまで待つ。
   * デバウンスされた自動保存を「800ms 寝て待つ」代わりに使う。
   */
  async waitForCalls(name: string, atLeast = 1): Promise<void> {
    await expect
      .poll(() => this.calls(name), { message: `${name} calls >= ${atLeast}` })
      .toBeGreaterThanOrEqual(atLeast);
  }

  /** 偽バックエンドが持っている状態のスナップショット。 */
  snapshot(): Promise<ReturnType<FakeBackend["snapshot"]>> {
    return this.app.page.evaluate(() =>
      (window as unknown as { __wirexaFake: FakeBackend }).__wirexaFake.snapshot(),
    );
  }
}

/**
 * UI e2e 用の test。ページは 1 回だけ読み込む (goto のみ。localStorage クリアのための
 * reload は addInitScript に置き換え済み)。状態はページのメモリ内にしか無いので、
 * テスト間のクリーンアップは不要。
 */
export const test = base.extend<Fixtures>({
  seed: [{}, { option: true }],

  page: async ({ page, seed }, use) => {
    await page.addInitScript((s: FakeSeed) => {
      // このコンテキストの初回ロードでだけ localStorage を消す。テストが page.reload() で
      // 復元を検証する場合に、アプリが保存した状態まで巻き込んで消さないため。
      if (!sessionStorage.getItem("__wirexa_e2e_boot")) {
        sessionStorage.setItem("__wirexa_e2e_boot", "1");
        localStorage.clear();
      }
      (window as unknown as { __wirexaSeed: FakeSeed }).__wirexaSeed = s;
    }, seed);
    await page.goto("/");
    await use(page);
  },

  app: async ({ page }, use) => {
    await use(new App(page));
  },

  fake: async ({ app }, use) => {
    await use(new FakeControl(app));
  },
});
