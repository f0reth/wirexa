# 変更計画書: HTTP の e2e テストレビューへの対応

## 概要

`docs/e2e-test-review-http.md`（2026-10-05）の推奨アクション 1〜6 をすべて実施する。内容は次の 4 つ。

1. 偽バックエンドの HTTP コレクション操作を Go に合わせる（観点 N）。
2. レビューで疑いが出た本体の不具合を直し、回帰テストを足す。
3. 不足していた UI e2e・フルスタック e2e を足す（観点 A〜I・M）。
4. テスト構造を整理する（観点 L）。

### コードを読んで確認できた本体の不具合

レビューは「コードを読む限り」としていたが、該当箇所を読み直して、どれも不具合として成立することを確かめた（テストは実行していない）。

| # | 観点 | 不具合 | 原因 |
| --- | --- | --- | --- |
| B1 | I-1 | 開いているリクエストを別のコレクション・ルートへ移すと、以後の編集が保存されない。リロード後の選択も復元されない | `application/http/request.ts:317` の `activeCollectionId` は選択時に入れたきりで、`collections.ts:223` の `moveItem` は一覧を読み直すだけ。Go は `collection_service.go:351` で `request not found` を返す |
| B2 | C-6 | 開いているリクエスト（またはその親）を削除しても編集エリアが残り、入力すると `Save failed` になる | `collection-tree.tsx:118` が一覧から消すだけ |
| B3 | C-5 | ルートへ出したフォルダを開いても、次の再読み込みで閉じる | `collections.ts:115` の `pruneExpandedIds` が通常のコレクションの中しか歩かず、ルートのアイテムの ID を「無効」として消す |
| B4 | C-2 | 同じ親の中で並び替えた行・並び替えたコレクションが、直後のクリックを 1 回無視する | `use-long-press-drag.ts:25` が立てた `suppress` を降ろすのは同じ行の `onClick` だけ。別の要素の上でマウスを離すと行に click が届かず、行も作り直されないのでフラグが残る。OpenAPI のファイル一覧（`openapi-file-node.tsx:56`）も同じ |
| B5 | D-1・E-1 | URL が空・不正でも、送信中でも、URL 欄の Enter で送信される | `request-bar.tsx:82` が `urlValid()` も `loading()` も見ない |
| B6 | I-3 | `Show truncated body` を一度押すと、次の打ち切りレスポンスで選択の画面（`Save body to file`）が出ない | `response-viewer.tsx:42` の `showTruncatedBody` を戻す箇所が無い |
| B7 | （この計画のレビューで追加） | ルートのフォルダの中のリクエストは、リロード後に選択が復元されない。保存してもツリーの値が古いままで、別のリクエストから戻ると古い値が編集エリアに入り、自動保存で編集内容を上書きする | `collections.ts:24` の `findRequestById` と `collections.ts:265` の `patchRequest` が、`__root__` のときルート直下しか見ない。フォルダはルートへ出せ（`MoveItemToSidebar`）、その子は `collectionId = __root__` で選択される（`tree-item-node.tsx:238`）。Go の `UpdateRequest` は `FindNode` で再帰的に探すので保存自体は成功する |

### 決定済みの挙動（ユーザー確認済み）

- 開いているリクエストが削除されたら、編集エリアは**保存せずに未選択の状態へ戻す**。
- 送信中の Enter は**送らない**（Send ボタンと同じ条件にそろえる）。URL が空・不正のときの Enter も送らない。

対象プロトコルは HTTP。Go のコードは変更しない（変更はフロントエンドの application / presentation / components/ui と `frontend/e2e/` だけ）。

## 変更対象ファイル

パスは `frontend/` からの相対。

### 本体

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| src/application/http/collections.ts | Application | 変更 | `refreshCollections` を「3 つを取得し終えてから `batch` でまとめて反映」に変える。`pruneExpandedIds` にルートのアイテムを渡す（B3）。リクエストの現在の所属を返す `findRequestLocation(collections, rootItems, requestId): string \| null` を足す。`findRequestById` と `patchRequest` はルートのアイテムも再帰的に歩く（B7）。構造を変える操作と再読み込みを 1 本のキューで直列にする（下記「再読み込みの直列化」） |
| src/application/http/request.ts | Application | 変更 | `relocateActiveRequest(collectionId)`（保存先を付け替えて保存し直す）と `closeRequest()`（保存せずに未選択へ戻す。`view++`、レスポンス破棄、`saveError` クリア）を足す。`newRequest` は「保存 → `closeRequest` と同じリセット」に組み直す。保存先の世代を持ち、付け替え・クローズより前に始めた保存の結果を捨てる（下記「古い保存の結果を捨てる」） |
| src/application/http/active-request.ts | Application | 新規 | `createActiveRequestTracking(request, collections)`。読み込み済みのツリーに追従する effect を 1 つだけ持つ（B1・B2） |
| src/application/http/collections.test.ts | Application | 変更 | 下記「テスト方針」 |
| src/application/http/request.test.ts | Application | 変更 | 同上 |
| src/application/http/active-request.test.ts | Application | 新規 | 同上 |
| src/presentation/providers/http-provider.tsx | Presentation (合成ルート) | 変更 | `createAutoSaveEffect` の次で `createActiveRequestTracking(requestState, collectionsState)` を呼ぶ。`RequestContextValue` に足した関数を反映 |
| src/presentation/components/sidebar/use-long-press-drag.ts | Presentation | 変更 | ドラッグ開始時に document の `mouseup` を 1 回だけ待ち、click の配送が済んだあと（`setTimeout(0)`）で `suppress` を降ろす（B4）。降ろすのはここだけにする |
| src/presentation/components/sidebar/use-long-press-drag.test.ts | Presentation | 変更 | mouseup のあとフラグが降りること |
| src/presentation/components/sidebar/collection-node.tsx | Presentation | 変更 | `onClick` はフラグを読むだけにする（降ろさない） |
| src/presentation/components/sidebar/tree-item-node.tsx | Presentation | 変更 | 同上（フォルダ・リクエストの 2 か所） |
| src/presentation/components/sidebar/openapi-file-node.tsx | Presentation | 変更 | 同上 |
| src/presentation/components/http/request-bar.tsx | Presentation | 変更 | `canSend = urlValid() && !loading()` を 1 つ作り、ボタンの `disabled` と `handleSend`（Enter もここを通る）の両方で使う（B5） |
| src/presentation/components/http/response-viewer.tsx | Presentation | 変更 | `showTruncatedBody` を「表示を選んだレスポンス」の signal に替え、`shown() === response()` で導出する（B6）。effect は足さない |
| src/presentation/components/http/index.tsx | Presentation | 変更 | 保存失敗バナーの閉じるボタンに `aria-label="Dismiss"`（観点 G） |
| src/components/ui/badge.tsx | 共通 UI | 変更 | `data-variant` 属性を出す（観点 L-1。CSS Modules のクラス名に依存した検証をやめるため） |

### e2e（偽バックエンド・fixture）

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| e2e/fake-backend/install.ts | e2e | 変更 | 下記「偽バックエンド」 |
| e2e/fake-backend/types.ts | e2e | 変更 | `FakeSeed` の `collections[].requests` を `items`（ツリー）に替え、`rootItems` を足す。`httpRpcErrors`・`saveResponseBodyCancelled` を足し、`httpError`・`updateRequestError`・`saveResponseError`・`getSidebarLayoutError` を消す |
| e2e/fixtures/ui.ts | e2e | 変更 | `FakeControl` に `lastArgs(name)`（呼び出しを待って最後の引数を返す）、`collection(idOrName)`、`callHttp(method, ...args)`（バインディングを直接呼び、失敗時はメッセージを返す）を足す |
| e2e/fixtures/app.ts | e2e | 変更 | `dragTreeNode` に落とす位置（`"center" \| "upper" \| "lower"`）を足す。`chooseBodyType(label)`、`saveErrorBanner`、`httpSaveResponseError(executionId)`（フルスタック用。`mqttConnectError` と同じ形）を足す |
| e2e/fixtures/http-server.ts | e2e | 変更なし | 遅い応答・大きい応答は `integration/http/http.spec.ts` のハンドラに足す |

### e2e（spec）

| ファイル | 変更種別 | 変更内容 |
| --- | --- | --- |
| e2e/ui/common/{async-loading,form-validation,request-persistence,sidebar-operations}.spec.ts → e2e/ui/http/ | 移動 | 全件が HTTP のテストなので `git mv` する（観点 L-4）。以下の「追加先」は移動後のパス |
| e2e/ui/http/tree-drag-drop.spec.ts | 変更 | C-1〜C-5、I-1、偽バックエンドの拒否（N-1・N-3）を追加 |
| e2e/ui/http/tree.spec.ts | 変更 | A、C-6、C-7、E-3 を追加 |
| e2e/ui/http/root-collection.spec.ts | 変更 | 拒否の文言を Go と同じ値で検証する（N-2） |
| e2e/ui/http/request-send.spec.ts | 変更 | I-2（JSON ボディ）、D-3 を追加 |
| e2e/ui/http/request-file.spec.ts | 変更 | I-4 と `OpenFilePicker` の失敗を追加 |
| e2e/ui/http/response-viewer.spec.ts | 変更 | I-3、1MB 超の JSON を追加。バッジの検証を `data-variant` に替える |
| e2e/ui/http/response-save.spec.ts | 変更 | E-4、保存ダイアログのキャンセル、`SaveResponseBase64` の失敗を追加 |
| e2e/ui/http/http.spec.ts | 変更 | E-2、次の保存でバナーが消えること、G を追加。L-1・L-3 の整理 |
| e2e/ui/http/request-persistence.spec.ts | 変更 | I-5、F、サイドバーのメソッド表示、`integration/http/http.spec.ts:238` に当たる UI e2e を追加 |
| e2e/ui/http/form-validation.spec.ts | 変更 | D-1、D-2 を追加。L-1 の整理 |
| e2e/ui/http/async-loading.spec.ts | 変更 | E-1 を追加。L-3 の整理 |
| e2e/ui/http/sidebar-operations.spec.ts | 変更 | 作成・削除で `fake.snapshot()` も見る（L-3） |
| e2e/ui/common/keyboard-accessibility.spec.ts | 変更 | `:18` を `:29` に統合（L-3） |
| e2e/integration/http/http.spec.ts | 変更 | M-5、M-10、M-6、I-1 を追加。`:68` と `:266` を削除（L-4）。Select の操作を `chooseOption` に替える |
| e2e/integration/common/backend-integration.spec.ts | 変更 | M-2（リネーム・削除のファイルへの反映、自動保存のリロード後の保持）を追加 |

### 文書

| ファイル | 変更種別 | 変更内容 |
| --- | --- | --- |
| .claude/skills/refactor-assistant/SKILL.md | 変更 | `:216` の例 `common/sidebar-operations.spec.ts` を移動後のパスへ |
| .claude/skills/e2e-test-review/SKILL.md | 変更 | `:50` の「HTTP のフォーム検証、リクエストの復元など」を、`common/` に残る例（`keyboard-accessibility` など）へ |
| docs/e2e-test-review-http.md | 削除 | 対応が済んだら消す（MQTT のレビュー文書と同じ扱い） |

## 実装方針

### B1・B2: 選択中のリクエストをツリーに追従させる

移動も削除も「ツリーが変わった結果、選択中のリクエストの所属が変わる・無くなる」という同じ事象なので、操作ごとに後始末を書かず、**ツリーに追従する effect を application 層に 1 つだけ**置く。

```
createActiveRequestTracking(request, collections):
  createEffect:
    collectionsLoaded() でなければ何もしない
    id = request.activeRequestId()  （無ければ何もしない）
    location = findRequestLocation(collections.collections, collections.rootItems, id)
    location が null                         → request.closeRequest()
    location !== request.activeCollectionId() → request.relocateActiveRequest(location)
```

- **置き場所**: `application/http/active-request.ts`。AGENTS.md の「派生状態とそれに追従する effect は application 層に置き、状態ごとに書く effect は 1 か所」に合わせる。`activeCollectionId` をユーザー操作以外で書くのはこの effect だけになる。
- **注入**: `http-provider.tsx`（合成ルート）で `createAutoSaveEffect` と並べて呼ぶ。新しいポートは要らない（`request` と `collections` の state を渡すだけで、`infrastructure` には触れない）。依存方向は presentation → application のまま。
- **`refreshCollections` をまとめて反映する**: 今は `getCollections` → 反映 → `getRootItems` → 反映 → `getSidebarLayout` → 反映、と途中の状態が見える。コレクションからルートへ移した直後は「どこにも無い」瞬間ができ、追従の effect が誤って編集エリアを閉じる。3 つを取得し終えてから `batch` で一度に反映する。これで「読み込みに失敗したら途中まで読めた一覧を使わない」（`http-provider.tsx:185`）も state の側で保証される。`pruneExpandedIds` も同じ場所で `cols` と `rootItems` の両方を受け取る（B3）。
- **再読み込みの直列化**: `batch` が隠すのは 1 回の反映の中の途中状態だけで、次の 2 つは残る。(a) `refreshCollections` は操作ごとに呼ばれ、並行の制御が無いので、先に始めた再読み込みの結果があとから新しいツリーを上書きできる。(b) 3 つの RPC は同じ時点のスナップショットではない。先の操作の再読み込みが `getCollections` を終えたあとに「ルート → コレクション」の移動が入ると、`getRootItems` には移動後が返り、リクエストが「どこにも無い」ツリーが組み上がる。追従の effect はこれを削除と解釈して編集エリアを閉じ、入力中の内容が消える。そこで `createCollectionsState` に Promise のキューを 1 本持ち、**構造を変える操作（RPC とそのあとの再読み込み）と、外から呼ぶ `refreshCollections` をすべてこのキューに通す**。前の操作の再読み込みが反映されるまで次の操作の RPC を始めないので、取得の途中で構造が変わらず、反映の順も入れ替わらない（構造を変える RPC を呼ぶのはこの state だけ。`UpdateRequest` は所属を変えない）。キューは失敗しても止めない（前の結果にかかわらず次を実行する）。キューの中から呼ぶ再読み込みは、キューに積まない内側の関数（`loadTree`）にする。`moveSidebarEntry` もレイアウトを書くので同じキューに通す。
- **`relocateActiveRequest` は付け替えたあと保存し直す**: 移動の RPC が終わってから一覧の再読み込みが終わるまでの間にデバウンスの自動保存が走ると、古いコレクション ID で `request not found` になる。付け替え時に新しい保存先へ 1 回保存する。失敗は `loadRequest` と同じく `Failed to save request` で通知する。
- **古い保存の結果を捨てる**: 上の保存し直しだけでは、付け替え・クローズより前に始まっていた保存の完了を止められない。`saveCurrentRequest`（`request.ts:354`）は完了時に無条件で `saveError` とツリー（`afterSave`）を書き、自動保存の catch（`request.ts:427`）はトーストを出すので、古い保存があとから失敗すると、移動先への保存が成功したあとや編集エリアを閉じたあとにバナーとトーストが出る。`request.ts` に保存先の世代（`saveTarget`）を持ち、`relocateActiveRequest` と `closeRequest` で進める。`saveCurrentRequest` は開始時の世代を覚え、完了時に変わっていたら結果を捨てる（`saveError` を書かない・`afterSave` を呼ばない・例外も投げない）。捨ててよいのは、付け替えなら新しい保存先へ保存し直しており、クローズならリクエストがもう無いため。`view` は使い回さない（付け替えではレスポンスを残すので `view` を進められない）。`loadRequest`・`newRequest` による切り替えでは世代を進めない（切り替え前のリクエストの保存の失敗は今までどおり通知する）ので、`newRequest` と共有するリセットには世代の更新を入れず、`closeRequest` の側に書く。
- **`closeRequest` は保存しない**: 消えたリクエストへ保存すると必ず失敗するため。`activeRequestId` が null になるので、自動保存の effect は止まり、`http-provider.tsx:156` の effect が localStorage の選択を消す。
- `http-provider.tsx:156` の effect は `activeCollectionId` を読んでいるので、付け替えると localStorage の `wirexa:http:activeRequest` も新しいコレクション ID で書き直される（リロード後に復元される）。

### B7: ルートのフォルダの中のリクエスト

`findRequestLocation` を足しても、B1 の「フォルダごとルートへ移す」は直りきらない。付け替え先は `__root__` になるが、`findRequestById` と `patchRequest` は `__root__` のときルート直下しか見ないので、リロード後の復元も保存後のツリーの更新も効かない。この 2 つは B1 と関係なく今も起きる（ルートのフォルダに直接リクエストを足した場合）。

`__root__` を特別扱いする分岐をやめ、「コレクション ID からアイテムの並びを得る（`__root__` なら `rootItems`）→ 同じ再帰で歩く」に統一する。`findRequestById` の「`__root__` は子を持たない」というコメントも直す。`findRequestLocation` も同じ再帰を使う。

### B4: ドラッグ後のクリック抑止

フラグを降ろす責任を呼び出し側の `onClick` から `makeLongPressDragHandlers` に移す。ドラッグを始めたら document の `mouseup` を 1 回待ち、`setTimeout(0)` で降ろす。click は mouseup と同じタスクで配送されるので、同じ行の上で離した場合の誤クリックはこれまでどおり抑止され、別の場所で離した場合もフラグが残らない。呼び出し側 4 か所は `if (suppressRef.suppress) return;` だけになる。

### B5・B6

どちらも presentation の中で閉じる。B5 は URL の妥当性（`isValidHttpUrl`）が presentation にあるので、同じ場所で `canSend` を作る。B6 は「どのレスポンスに対して表示を選んだか」を持てば導出で済み、application の state にも effect にも触れない。

### 偽バックエンド（観点 N）

`e2e/fake-backend/install.ts` の HttpHandler を Go の `CollectionService` に合わせる。文言は `internal/domain/errors.go` の `"<resource> not found: <id>"`・`"invalid <field>: <message>"` と同じ形にする。

| # | 対象 | 合わせる内容 |
| --- | --- | --- |
| N-1 | `UpdateRequest` | 無い・リクエストでないノードは `request not found: <id>` |
| N-1 | `RenameItem`・`DeleteItem`・`MoveItem`・`MoveItemToSidebar` | 無いアイテムは `item not found: <id>` |
| N-1 | `DeleteCollection` | 無いコレクションは `collection not found: <id>` |
| N-1 | `AddFolder`・`AddRequest`・`MoveItem` | 親が無い・フォルダでないときは `parent not found: <id>`（`AppendItem`・`InsertItem` と同じ条件） |
| N-1 | `MoveSidebarEntry` | 無いエントリは `sidebar entry not found: <id>` |
| N-2 | `CreateCollection` | 空白だけの名前は `invalid name: is required` |
| N-2 | `SendRequest` | 未知のメソッドは `invalid method: <method>` |
| N-2 | `__root__` の拒否 | `invalid id: reserved collection cannot be modified` |
| N-3 | `MoveItem` | 自分の子孫への移動は、取り外す前に `invalid parent: cannot move an item into its own subtree` で拒否 |
| N-4 | `GetSidebarLayout`・`MoveSidebarEntry`・`snapshot().sidebar` | Go の `reconcileSidebarLayout` と同じ突合（無いエントリと重複を落とし、レイアウトに無いコレクションを名前順、ルートのアイテムをツリー順で末尾へ）を通す |
| N-5 | `GetCollections` | 名前順 |

seed は次のように変える（後方互換は残さず、既存の spec の `requests:` 8 か所を書き換える）。

```ts
type SeedItem =
  | ({ name: string; id?: string } & Partial<Omit<HttpRequest, "id" | "name">>) // リクエスト
  | { folder: string; id?: string; items?: SeedItem[] };                        // フォルダ

interface FakeSeed {
  collections?: Array<{ id?: string; name: string; items?: SeedItem[] }>;
  /** __root__ 直下のアイテム。サイドバーレイアウトの末尾に並ぶ。 */
  rootItems?: SeedItem[];
  /** HTTP のバインディング名 → 失敗させる文言。times を付けるとその回数だけ失敗する。 */
  httpRpcErrors?: Record<string, string | { message: string; times: number }>;
  /** SaveResponseBody が false を返す（保存ダイアログのキャンセル）。 */
  saveResponseBodyCancelled?: boolean;
  // ...
}
```

`httpRpcErrors` は HttpHandler の各メソッドの先頭で見る（`SendRequest` だけは今の `httpError` と同じく遅延のあと）。`httpError`・`updateRequestError`・`saveResponseError`・`getSidebarLayoutError` はこれに置き換える。MQTT・UDP の個別の注入口（`getTargetsError` など）は今回は触らない。

バインド API（Go）の振る舞いは変えないので、偽バックエンドの変更は「Go に寄せる」方向だけである。

## 永続化への影響

保存形式の変更は無し。stored DTO・golden・復旧方針には触れない。

localStorage はキーも値の形も変えない。書かれる内容だけが次のように変わる。

- `wirexa:http:activeRequest`: 開いているリクエストを移動すると新しい `collectionId` で書き直され、削除すると消える。
- `wirexa:http:expandedFolders`: ルートのアイテム（ルートへ出したフォルダ）の ID が再読み込みで消されなくなる。

## コード生成

不要（Go の構造体・ハンドラ・イベントを変えない）。

## テスト方針

### Go ユニット / Go 統合

変更なし。

### フロント ユニット（Vitest）

- `collections.test.ts`: `refreshCollections` が 3 つ目の取得に失敗したら state を一切変えないこと。`pruneExpandedIds` がルートのフォルダの ID を残すこと。`findRequestLocation`（コレクション直下・フォルダの中・ルート・ルートのフォルダの中・無い）。`findRequestById` と `patchRequest` がルートのフォルダの中のリクエストを見つける・書き換えること（B7）。直列化: 取得を手で完了させる API で、先の操作の再読み込みが終わるまで次の操作の RPC が呼ばれないこと、「ルート → コレクション」の移動を先の再読み込みの途中に重ねても、リクエストがどこにも無いツリーが一度も反映されないこと、失敗した操作のあとも次の操作が実行されること。
- `request.test.ts`: `closeRequest` が保存を呼ばず、レスポンスを破棄し、`saveError` を消すこと。`relocateActiveRequest` が新しいコレクション ID で `updateRequest` を呼ぶこと。保留した保存を `closeRequest` のあとで失敗させても `saveError` が null のままで例外も出ないこと。保留した古い保存先への保存を、`relocateActiveRequest` の保存が成功したあとで失敗させても `saveError` が null のままであること。捨てた保存の成功では `afterSave` が呼ばれないこと。`loadRequest` で切り替えたあとの、切り替え前の保存の失敗は今までどおり例外になること。
- `active-request.test.ts`（新規）: 読み込み前は何もしない / 別のコレクションへ移ったら付け替える / ルートへ移ったら `__root__` に付け替える / 親ごと消えたら閉じる / 所属が変わらなければ何も呼ばない。
- `use-long-press-drag.test.ts`: ドラッグ開始後に document で mouseup するとフラグが降りること、降りる前の click では立っていること。

### UI e2e（`task frontend:test:e2e`）

本体の修正に対応する回帰テスト（修正前は失敗することを確かめてから直す）:

| 不具合 | テスト名 | 追加先 |
| --- | --- | --- |
| B1 | `edits made after moving the open request to another collection are saved`（リロード後の選択の復元まで見る） | tree-drag-drop.spec.ts |
| B1 | `edits made after moving the open request to the sidebar root are saved` | tree-drag-drop.spec.ts |
| B2 | `deleting the open request clears the editor`、`deleting the folder that holds the open request clears the editor`（`UpdateRequest` が呼ばれず、バナーが出ず、localStorage の選択が消える） | tree.spec.ts |
| B3 | `a folder moved to the sidebar root stays expanded after adding a request to it` | tree-drag-drop.spec.ts |
| B4 | `a request can be selected with one click right after it was reordered`、`a collection can be collapsed with one click right after it was reordered` | tree-drag-drop.spec.ts |
| B5 | `pressing Enter with an invalid URL does not send` | form-validation.spec.ts |
| B5 | `pressing Enter while a request is in progress does not send a second one` | async-loading.spec.ts |
| B6 | `a new truncated response offers the save choice again after showing the previous one` | response-viewer.spec.ts |
| B7 | `edits to a request inside a folder moved to the sidebar root survive switching requests`（フォルダごとルートへ移す → 子を編集 → 別のリクエストを選んで戻る）、`a request inside a root folder is restored after reload` | tree-drag-drop.spec.ts、request-persistence.spec.ts |

レビューで不足とされたテスト:

- **tree-drag-drop.spec.ts**: C-1 `dropping a request on the lower half of a sibling moves it after that sibling` / `…upper half…before that sibling`（seed にリクエスト 3 件、`MoveItem` の引数と `fake.snapshot()` の並び）。C-3 `dragging a folder to another collection moves its children with it` / `dropping a folder into its own subfolder is rejected and keeps the tree`（`Failed to move item` のトースト）。C-4 `dragging a root request reorders it among collections` / `dragging a root request onto a collection moves it out of the sidebar root`。
- **tree.spec.ts**: A `creating a collection after a failed load reports the failure and keeps existing collections`、C-7 ルート直下のリクエストの削除・Add メニューが外側のクリックで閉じる、E-3 `a failed delete shows an error toast and keeps the item` ほか（追加に失敗したらリネーム入力を出さない）。
- **request-send.spec.ts**: I-2 `json body is sent with the request` / `an untouched json body sends no content`、D-3 `clearing the timeout sends 0 so the backend default applies`。
- **request-file.spec.ts**: I-4 `cancelling the file dialog keeps the typed path unconfirmed` / `clear selection removes the confirmed file` / `a file that needs reselecting can be sent after Browse`、`a failed file dialog shows an error toast`。
- **request-persistence.spec.ts**: I-5 `params, auth, settings and form rows are restored after reload`、F `a stale or malformed saved selection is ignored on startup`、サイドバーのメソッド表示が自動保存のあと更新される、リロードせずに別のリクエストから戻ると URL が入っている。
- **response-save.spec.ts**: E-4 `a failed save shows an error toast and keeps the save button`、保存ダイアログのキャンセルでボタンが残る、バイナリ保存の失敗。
- **response-viewer.spec.ts**: 1MB を超える JSON はハイライトしない。
- **http.spec.ts**: E-2 `repeated auto-save failures keep a single toast`、失敗のあと次の保存が成功したらバナーが消える（`times: 1`）、G のロケーターを `getByRole("button", { name: "Dismiss" })` に替える。
- **form-validation.spec.ts**: D-2 `request name with markup and emoji is shown as text`。
- **root-collection.spec.ts / tree-drag-drop.spec.ts**: 偽バックエンドの拒否を `fake.callHttp` で直接確かめる（N-1 の各 RPC、N-2 の文言、N-3）。Go の文言と同じ値で比べる。

観点 L の整理（振る舞いは変えない）:

- L-1: Select の操作を `app.chooseOption`・`app.selectMethod`・`app.chooseBodyType` に、`beforeEach` を `app.switchTo("HTTP")` に、URL 欄と Send を `app.urlInput`・`app.sendButton` に寄せる。バッジは `data-variant` で見る。
- L-2: `lastSent` → `fake.lastArgs("SendRequest")`、`storedCollection` → `fake.collection(...)`、バインディングの直接呼び出し → `fake.callHttp(...)`、`openFormBody`・`chooseBodyType` → `app.chooseBodyType`。
- L-3: `async-loading` の失敗のテストに `toContainText("connection refused")` を足す。`getByText("200")` を `app.responseViewer` でスコープする。`http.spec.ts:305`・`:342`・`:360` は `request-send.spec.ts` に含まれるので消す。`keyboard-accessibility.spec.ts:18` を `:29` に統合する。`sidebar-operations` の作成・削除で `fake.snapshot()` も見る。
- L-4: spec 4 本の移動、`integration/http/http.spec.ts:68`・`:266` の削除。

### フルスタック e2e（`task frontend:test:e2e:fullstack`、CI 非対象）

- **integration/http/http.spec.ts**: M-5 `a connection error from the backend is shown in the response viewer`（閉じたポートへ送り、`failed to send request:` を含むこと）。M-10 `saving an unknown execution ID is rejected with the message the UI treats as reclaimed`（`app.httpSaveResponseError` の戻り値を `RESPONSE_UNAVAILABLE_ERROR` と比べる）。M-6 `cancel aborts a request to a slow server` / `a response larger than the limit is shown as truncated`（テストサーバーに遅い応答と大きい応答のパスを足す）。I-1 `edits made after moving the open request to another collection are saved`（実バックエンドで `request not found` にならないこと）。
- **integration/common/backend-integration.spec.ts**: M-2 `renaming and deleting a collection updates and removes its file`、自動保存した内容がリロード後に残る。

## 副作用・注意事項

- **編集エリアが閉じるようになる**: 開いているリクエスト、またはそれを含むフォルダ・コレクションを削除すると、入力中の内容は保存されずに消える（削除の確認ダイアログを経ている）。送信中だった応答は表示されず、打ち切られた全文は破棄される。
- **画面から二重送信ができなくなる**: 送信中の Enter を無視するので、同時に実行中のリクエストは高々 1 件になる。`request.ts` の `inFlight` は配列のまま残す（単一の ID に畳むのは別の変更とする）。
- **`refreshCollections` の失敗時の見え方**: 3 つのうちどれかが失敗すると、これまでは途中まで反映されていたのが、何も反映されなくなる。起動時の失敗の既存テスト（`tree.spec.ts:233`、`No collections yet` のまま）は同じ結果になる。
- **コレクションの操作が順番待ちになる**: 前の操作の再読み込みが終わるまで次の操作の RPC を始めない。RPC はローカルなので体感は変わらない見込みだが、1 つの RPC が返らなければ後続も止まる。
- **付け替え・クローズより前に始めた保存の失敗は通知されなくなる**: 付け替えでは新しい保存先へ保存し直しており、その失敗は通知する。クローズでは保存する対象が無い。
- **`suppress` を降ろす場所の変更は OpenAPI のファイル一覧にも効く**: 同じ不具合が直る方向の変更で、フックのユニットテストで守る。OpenAPI の e2e は今回足さない。
- **偽バックエンドが厳しくなる**: 「見つからない対象」を黙って成功させていた箇所がエラーになる。既存の spec が、消したあとの対象へ保存する流れを暗黙に通っていれば落ちる。その場合は B2 の修正で解消するか、テストの側の誤りなので個別に直す。
- **seed の形の変更**: `requests` → `items`、失敗の注入の 4 フィールド → `httpRpcErrors`。HTTP の spec を一括で書き換える。`GetCollections` が名前順になるが、画面の並びはサイドバーレイアウトで決まるので既存のテストには影響しない見込み。
- **範囲外として残すもの**: `tree-drag-drop.spec.ts:90` の NOTE にある `InsertionZone.isNoOp` の誤り（コレクションの中のアイテムをドラッグすると、親の中の位置と同じ番号のサイドバーゾーンが隠れる）。レビューの指摘に無く、C-4 のテストはこの制約を避けて書ける。直す場合は別の計画にする。
- テストは実行していないので、B1〜B7 は「回帰テストを先に書いて失敗を確かめる → 直す」の順で進め、想定と違う結果が出たらその時点で報告する。

## Git運用

- **ブランチ名**: `fix/http-e2e-review-followup`
- **コミット分割方針**（上から順。各コミットで `task frontend:test:e2e` が通る状態を保つ）:
  1. `test(e2e): HTTP だけを扱う common の spec を ui/http へ移す`（`git mv` と、パスを書いているコメント・skill の更新だけ）
  2. `test(e2e): 偽バックエンドの seed にフォルダとルートのアイテムを置けるようにする`
  3. `test(e2e): 偽バックエンドの HTTP の失敗の注入を httpRpcErrors にまとめる`
  4. `test(e2e): 偽バックエンドのコレクション操作を Go に合わせる`（N-1〜N-5 と、それを直接確かめるテスト）
  5. `fix(http): ルートのフォルダの中のリクエストを復元・更新する`（B7。6 の「フォルダごとルートへ移す」が前提にする）
  6. `fix(http): コレクションの操作と再読み込みを直列にする`（`refreshCollections` のまとめての反映を含む。7 の追従が前提にする）
  7. `fix(http): 選択中のリクエストを移動・削除に追従させる`（B1・B2。古い保存の結果を捨てる世代を含む）
  8. `fix(http): ルートへ出したフォルダの開閉の記録を消さない`（B3）
  9. `fix(frontend): ドラッグのあとの最初のクリックを無視しない`（B4 と C-1 のテスト）
  10. `fix(http): URL 欄の Enter を Send ボタンと同じ条件にする`（B5）
  11. `fix(http): 打ち切り表示の選択をレスポンスごとに持つ`（B6）
  12. `fix(http): 保存失敗バナーの閉じるボタンに名前を付ける`（G。`Badge` の `data-variant` は 14 に含める）
  13. `test(e2e): HTTP の UI e2e に、レビューで不足していたフローを足す`（C-3・C-4・C-7・A・D・E・F・I。量が多ければツリー / 送信・レスポンス / 永続化で分ける）
  14. `test(e2e): HTTP の UI e2e のロケーターとヘルパーを fixture に寄せる`（観点 L）
  15. `test(e2e): HTTP のフルスタック e2e に、エラーの表示と Cancel・打ち切りを足す`（M-2・M-5・M-6・M-10・I-1、重複 2 件の削除）
  16. `docs: HTTP の e2e テストレビュー文書を削除する`
- **完了条件**: すべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task frontend:test:e2e`（UI の振る舞いと fake-backend を変えるため）
  - `task frontend:test:e2e:fullstack`（フルスタックの spec を足すため。CI 非対象なのでローカルで実行する）
  - `task go:test:integration`・`task go:test:race` は不要（Go を変えない）
