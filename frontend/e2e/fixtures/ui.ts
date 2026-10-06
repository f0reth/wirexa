import { test as base, expect, type Page } from "@playwright/test";
import type { Collection } from "../../src/domain/http/types";
import { WailsEvents } from "../../src/shared/wails-events";
import type { FakeBackend, FakeSeed } from "../fake-backend/types";
import { App } from "./app";

export { expect, WailsEvents };
export { App } from "./app";

type WailsEventName = (typeof WailsEvents)[keyof typeof WailsEvents];

// ページごとの未捕捉の例外。page の fixture が goto の前から集め、pageErrors の fixture が返す。
const errorsByPage = new WeakMap<Page, string[]>();

interface Fixtures {
  /** test.use({ seed: {...} }) で仕込む初期状態。ページ読み込み前に注入される。 */
  seed: FakeSeed;
  app: App;
  fake: FakeControl;
  /**
   * 最初の読み込みから起きた未捕捉の例外のメッセージ (起動時の例外も含む)。自動では検査しないので、
   * 使うテストが引数に取り、最後に expect(pageErrors).toEqual([]) で確かめる。
   */
  pageErrors: string[];
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

  /** バインディングの呼び出しごとの引数。 */
  args(name: string): Promise<unknown[][]> {
    return this.app.page.evaluate(
      (n) => (window as unknown as { __wirexaFake: FakeBackend }).__wirexaFake
        .args[n] ?? [],
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

  /**
   * バックエンドがイベントを発火したことにする。name には WailsEvents の定数を使う。
   * 購読者の呼び出しは evaluate の中で同期的に終わるので、戻ったあとは UI の反映を待てばよい。
   */
  async emit(name: WailsEventName, ...data: unknown[]): Promise<void> {
    await this.app.page.evaluate(
      ([n, d]) =>
        (window as unknown as { __wirexaFake: FakeBackend }).__wirexaFake.emit(
          n,
          ...d,
        ),
      [name, data] as const,
    );
  }

  /**
   * payloads の各要素を 1 回ずつ name で発火する。1 回の evaluate の中で同期的に流すので、
   * 間に描画 (requestAnimationFrame) を挟まず、すべてが同じフレームに届く。
   */
  async emitAll(name: WailsEventName, payloads: unknown[]): Promise<void> {
    await this.app.page.evaluate(
      ([n, ps]) => {
        const fake = (window as unknown as { __wirexaFake: FakeBackend })
          .__wirexaFake;
        for (const p of ps) fake.emit(n, p);
      },
      [name, payloads] as const,
    );
  }

  /** バインディングが呼ばれるのを待ち、最後の呼び出しの引数を返す。 */
  async lastArgs(name: string): Promise<unknown[]> {
    await this.waitForCalls(name);
    const all = await this.args(name);
    return all[all.length - 1];
  }

  /**
   * HTTPHandler のバインディングを画面を通さずに直接呼ぶ。失敗したらそのメッセージを、成功したら
   * null を返す。画面から送れない値 (無い ID、予約済みの __root__ など) の扱いを確かめるのに使う。
   */
  callHttp(method: string, ...callArgs: unknown[]): Promise<string | null> {
    return this.app.page.evaluate(
      async ([m, a]) => {
        type Handler = Record<string, (...args: unknown[]) => Promise<unknown>>;
        const handler = (
          window as unknown as { go: { adapters: { HTTPHandler: Handler } } }
        ).go.adapters.HTTPHandler;
        try {
          await handler[m](...a);
          return null;
        } catch (err) {
          return err instanceof Error ? err.message : String(err);
        }
      },
      [method, callArgs] as const,
    );
  }

  /** 偽バックエンドが持つコレクション (__root__ 以外)。ID か名前で引く。 */
  async collection(idOrName: string): Promise<Collection> {
    const { collections } = await this.snapshot();
    const col = collections.find(
      (c) => c.id === idOrName || c.name === idOrName,
    );
    if (!col) throw new Error(`${idOrName} is missing in the fake backend`);
    return col;
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
    // 起動時の例外も拾えるよう、読み込む前から集める。
    const errors: string[] = [];
    errorsByPage.set(page, errors);
    page.on("pageerror", (err) => errors.push(err.message));
    await page.goto("/");
    await use(page);
  },

  app: async ({ page }, use) => {
    await use(new App(page));
  },

  fake: async ({ app }, use) => {
    await use(new FakeControl(app));
  },

  pageErrors: async ({ page }, use) => {
    await use(errorsByPage.get(page) ?? []);
  },
});
