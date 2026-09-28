import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect, type Page } from "@playwright/test";
import { App } from "./app";
import { appDataDir } from "./app-data-dir";

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

// ── ディスク上の保存データ ──────────────────────────────────────────────────
// 保存先は実行中ずっと同じなので、前のテストが作ったデータも残っている。読んだ結果は
// 名前などで絞り込んで使う。書き込みは RPC の完了と画面の更新より後になることがあるので、
// 読むときは expect.poll で待つ。

/** アプリの設定ディレクトリ (os.UserConfigDir()/Wirexa)。 */
const configDir = path.join(appDataDir, "Wirexa");

const repoRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../..",
);

function readJson<T>(file: string): T {
  return JSON.parse(fs.readFileSync(file, "utf8")) as T;
}

/** 設定ディレクトリ直下の JSON ファイル (sidebar_layout.json など) を読む。無ければ undefined。 */
export function readStoredFile<T>(name: string): T | undefined {
  const file = path.join(configDir, name);
  return fs.existsSync(file) ? readJson<T>(file) : undefined;
}

/**
 * エンティティごとに 1 ファイルで保存するディレクトリ (collections / mqtt-profiles /
 * udp-targets) の中身をすべて読む。file はファイル名 ("<id>.json")。
 */
export function readStoredEntities<T>(
  dir: string,
): Array<{ file: string; data: T }> {
  const full = path.join(configDir, dir);
  if (!fs.existsSync(full)) return [];
  return fs
    .readdirSync(full)
    .filter((file) => file.endsWith(".json"))
    .map((file) => ({ file, data: readJson<T>(path.join(full, file)) }));
}

/** 保存形式の基準 (internal/infrastructure/<pkg>/testdata/<name>.golden.json) を読む。 */
export function readGolden<T>(pkg: string, name: string): T {
  return readJson<T>(
    path.join(
      repoRoot,
      "internal/infrastructure",
      pkg,
      "testdata",
      `${name}.golden.json`,
    ),
  );
}

/**
 * JSON の形。オブジェクトはキーの集合と各値の形、配列は先頭要素の形、それ以外は型名にする。
 * 値の違いを無視して、保存したファイルと golden のキー構成を比べるのに使う。
 */
export function shapeOf(value: unknown): unknown {
  if (Array.isArray(value)) return value.length ? [shapeOf(value[0])] : [];
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([k, v]) => [k, shapeOf(v)]),
    );
  }
  return value === null ? "null" : typeof value;
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
