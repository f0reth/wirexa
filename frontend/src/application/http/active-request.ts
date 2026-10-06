import { createEffect, untrack } from "solid-js";
import { type CollectionsState, findRequestLocation } from "./collections";
import type { RequestState } from "./request";

/**
 * 開いているリクエストを、読み込み済みのツリーに追従させる。
 *
 * 移動も削除も「ツリーが変わった結果、開いているリクエストの所属が変わる・無くなる」という
 * 同じ事象なので、操作ごとに後始末を書かずにここで追従する。
 * - 別のコレクション（またはルート）へ移っていたら、保存先を付け替える。
 * - どこにも無ければ（本人か、それを含むフォルダ・コレクションが削除された）、
 *   保存せずに未選択へ戻す。
 *
 * activeCollectionId をユーザー操作以外で書くのはこの effect だけにする。
 */
export function createActiveRequestTracking(
  request: Pick<
    RequestState,
    | "activeRequestId"
    | "activeCollectionId"
    | "closeRequest"
    | "relocateActiveRequest"
  >,
  collections: Pick<
    CollectionsState,
    "collections" | "rootItems" | "collectionsLoaded"
  >,
): void {
  createEffect(() => {
    // 読み込む前の空のツリーを「削除された」と解釈しない。
    if (!collections.collectionsLoaded()) return;
    const id = request.activeRequestId();
    if (!id) return;
    const location = findRequestLocation(
      collections.collections,
      collections.rootItems,
      id,
    );
    const current = request.activeCollectionId();
    // 付け替え・クローズが読む編集中の値には追従しない（入力のたびに走らせない）。
    untrack(() => {
      if (location === null) request.closeRequest();
      else if (location !== current) request.relocateActiveRequest(location);
    });
  });
}
