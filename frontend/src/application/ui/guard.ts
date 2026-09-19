import type { Notifier } from "../../domain/ui/ports";
import { errorMessage } from "../../shared/error";

/**
 * 非同期処理を実行し、失敗時に notifier.error(label, ...) を出して正常終了する。
 *
 * 戻り値も後続処理も持たない操作専用。呼び出し側が成功と失敗を区別する必要がある
 * 操作では notifyOnError を使い、例外を伝播させる。
 *
 * fn の返り値は無視するため（`Promise<unknown>` を受ける）、
 * `() => api.addRequest(...)` のように値を返す Promise もそのまま渡せる。
 */
export async function runGuarded(
  notifier: Notifier,
  label: string,
  fn: () => Promise<unknown>,
): Promise<void> {
  try {
    await fn();
  } catch (err) {
    notifier.error(label, errorMessage(err));
  }
}

/**
 * 非同期処理を実行し、失敗時に notifier.error(label, ...) を出したうえで例外を再送出する。
 *
 * 戻り値を持つ操作（作成系など）で使う。呼び出し側は通知の存在を知らずに、
 * 失敗を例外として分岐できる。
 */
export async function notifyOnError<T>(
  notifier: Notifier,
  label: string,
  fn: () => Promise<T>,
): Promise<T> {
  try {
    return await fn();
  } catch (err) {
    notifier.error(label, errorMessage(err));
    throw err;
  }
}
