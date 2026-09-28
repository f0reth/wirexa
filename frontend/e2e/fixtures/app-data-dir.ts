import os from "node:os";
import path from "node:path";

/**
 * fullstack e2e でアプリの保存先を隔離するための APPDATA。playwright.integration.config.ts が
 * wails dev に渡し、spec は e2e/fixtures/integration.ts のヘルパー越しに中の JSON を読む。
 * config とヘルパーで同じ場所を指すよう、パスはここにだけ書く。
 */
export const appDataDir = path.join(os.tmpdir(), "wirexa-e2e-appdata");
