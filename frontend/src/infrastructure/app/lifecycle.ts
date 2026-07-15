import { ConfirmQuit } from "../../../wailsjs/go/main/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { WailsEvents } from "../../shared/wails-events";

/**
 * lifecycle はアプリのライフサイクル（ウィンドウを閉じる等）に関わる
 * Wails バインディング / ランタイムイベントを薄くラップする。
 */

/** confirmQuit はバックエンドに「終了してよい」と伝える。 */
export async function confirmQuit(): Promise<void> {
  return ConfirmQuit();
}

/**
 * onBeforeClose はウィンドウを閉じようとしたときの通知を購読する。
 * バックエンド (beforeClose) が閉じるのを一旦阻止したうえで発火する。
 * 戻り値の関数で購読を解除できる。
 */
export function onBeforeClose(cb: () => void): () => void {
  return EventsOn(WailsEvents.appBeforeClose, cb);
}
