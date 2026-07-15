import { errorMessage } from "../../shared/error";
import { notify } from "./notifications";

/**
 * 非同期処理を実行し、失敗時に notify.error(label, ...) を出すだけの
 * try/catch ボイラープレートをまとめるヘルパー。
 *
 * fn の返り値は無視するため（`Promise<unknown>` を受ける）、
 * `() => api.addRequest(...)` のように値を返す Promise もそのまま渡せる。
 */
export async function runGuarded(
  label: string,
  fn: () => Promise<unknown>,
): Promise<void> {
  try {
    await fn();
  } catch (err) {
    notify.error(label, errorMessage(err));
  }
}
