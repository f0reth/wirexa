# E2E テストレビュー

生成日時: 2026-10-05
対象: HTTP（`presentation/components/http/`、サイドバーのコレクションツリー、`http-provider.tsx`、`application/http/`、`internal/adapters/http_handler.go`）と、`e2e/ui/common/` のうち HTTP に関わるテスト
スイート: UI e2e（spec 15 本・テスト 106 件）/ フルスタック e2e（spec 2 本・テスト 13 件）

> 件数の内訳: UI e2e は `e2e/ui/http/` の 8 本 74 件と、`e2e/ui/common/` のうち HTTP を扱う 7 本 32 件（`async-loading` 4、`form-validation` 9、`request-persistence` 3、`sidebar-operations` 6 は全件、`keyboard-accessibility` は 13 件中 6 件、`initial-state` は 5 件中 2 件、`protocol-switching` は 7 件中 2 件）。フルスタックは `integration/http/http.spec.ts` 9 件と `integration/common/backend-integration.spec.ts` 11 件中 4 件（M-1 のコレクション、M-2 の 2 件、M-8）。
>
> テストは実行していない。すべてコードを読んだ結果で、「コードを読む限り」と書いた箇所は実際の動作を確かめていない。

---

## ユーザーフロー一覧

パスは本体が `frontend/src/` から、spec が `frontend/e2e/` からの相対。

| フロー | コンポーネント（ファイル:行） | UI e2e | フルスタック |
| --- | --- | --- | --- |
| 空状態（`No collections yet`、`Send a request to see the response`） | `presentation/components/sidebar/collection-tree.tsx:289`、`http/response-viewer.tsx:109` | 済（`ui/common/initial-state.spec.ts:33`） | 対象外 |
| 起動時の読み込み失敗（トースト、選択中リクエストを復元しない） | `presentation/providers/http-provider.tsx:184` | 済（`ui/http/tree.spec.ts:233`） | 対象外（Go の `GetSidebarLayout` はエラーを返さない） |
| コレクションの作成 | `sidebar/collection-tree.tsx:70` | 済（`ui/common/sidebar-operations.spec.ts:7`） | 済（`integration/common/backend-integration.spec.ts:109`） |
| ルート直下のリクエストの作成 | `sidebar/collection-tree.tsx:51` | 済（`ui/http/tree.spec.ts:139`） | 未テスト |
| フォルダ・リクエストの追加（コレクション直下・フォルダ内） | `sidebar/collection-node.tsx:119`、`sidebar/tree-item-node.tsx:187` | 済（`ui/http/tree.spec.ts:154`・`:168`） | 部分的（`backend-integration.spec.ts:118`、リクエストだけ） |
| リネーム（Enter・blur・Escape・空白だけ） | `sidebar/rename-input.tsx:24`、`sidebar/collection-node.tsx:43` | 済（`ui/common/sidebar-operations.spec.ts:12`・`:24`、`ui/http/tree.spec.ts:82`・`:104`、`ui/common/form-validation.spec.ts:110`） | 対象外 |
| 削除と確認ダイアログ（Delete・Cancel・Escape・子を持つフォルダ） | `sidebar/collection-tree.tsx:296` | 済（`ui/http/tree.spec.ts:27`・`:41`・`:63`、`ui/common/keyboard-accessibility.spec.ts:57`） | 未テスト |
| 選択中のリクエスト（またはその親）を削除する | `sidebar/collection-tree.tsx:118`（削除だけで、編集エリアは切り替えない） | 未テスト | 未テスト |
| リクエストの選択（クリック・Enter） | `sidebar/tree-item-node.tsx:290` | 済（`ui/common/sidebar-operations.spec.ts:57`、`ui/common/keyboard-accessibility.spec.ts:159`） | 済（`integration/http/http.spec.ts:238`） |
| コレクション・フォルダの開閉と復元 | `sidebar/collection-node.tsx:25`、`application/http/collections.ts:106` | 済（`ui/common/sidebar-operations.spec.ts:71`、`ui/http/tree.spec.ts:120`・`:184`・`:195`） | 対象外 |
| D&D: リクエストを別コレクションの見出しへ | `sidebar/use-tree-drag-drop.ts:113` | 済（`ui/http/tree-drag-drop.spec.ts:30`） | 未テスト |
| D&D: コレクションの並び替え | `sidebar/collection-tree.tsx:133` | 済（`ui/http/tree-drag-drop.spec.ts:58`） | 済（`backend-integration.spec.ts:268`） |
| D&D: リクエストをルートへ | `sidebar/collection-tree.tsx:147` | 済（`ui/http/tree-drag-drop.spec.ts:85`） | 未テスト |
| D&D: リクエストをフォルダの中へ（挿入ゾーン） | `sidebar/use-tree-drag-drop.ts:131` | 済（`ui/http/tree-drag-drop.spec.ts:118`） | 未テスト |
| D&D: 同じ親の中での並び替え（行の上半分・下半分に落とす） | `sidebar/use-tree-drag-drop.ts:83`・`:147` | 未テスト | 未テスト |
| D&D: ルートのアイテムの並び替え、ルートからコレクションへ | `sidebar/collection-tree.tsx:142` | 未テスト | 未テスト |
| D&D: フォルダの移動（自分の子孫へ落とす場合を含む） | `sidebar/tree-item-node.tsx:105` | 未テスト | 未テスト |
| メソッドの選択 | `http/request-bar.tsx:56` | 済（`ui/http/http.spec.ts:14`、`ui/http/request-send.spec.ts:74`） | 未テスト |
| URL の検証と Send の有効・無効 | `http/request-bar.tsx:42`・`:99` | 済（`ui/common/form-validation.spec.ts:12`〜`:44`） | 対象外 |
| URL 欄の Enter で送信 | `http/request-bar.tsx:82` | 部分的（`ui/common/keyboard-accessibility.spec.ts:45`、正しい URL だけ） | 対象外 |
| 送信・送信中の表示・Cancel | `application/http/request.ts:206`・`:270` | 済（`ui/common/async-loading.spec.ts:16`・`:30`） | 部分的（`integration/http/http.spec.ts:68`、Cancel は無し） |
| Params・Headers（追加・削除・有効/無効） | `http/key-value-editor.tsx:16` | 済（`ui/common/form-validation.spec.ts:48`〜`:106`、`ui/http/request-send.spec.ts:21`） | 済（`integration/http/http.spec.ts:181`） |
| 認証（Basic・Bearer） | `http/request-editor.tsx:164` | 済（`ui/http/http.spec.ts:264`〜`:301`、`ui/http/request-send.spec.ts:21`・`:53`） | 済（`integration/http/http.spec.ts:181`・`:216`） |
| ボディの種別の切り替え | `http/request-editor.tsx:100` | 済（`ui/http/http.spec.ts:44`〜`:74`・`:247`） | 対象外 |
| Text ボディの入力と送信 | `http/request-editor.tsx:144` | 済（`ui/http/request-send.spec.ts:74`） | 未テスト |
| JSON ボディの入力と送信 | `http/json-body-editor.tsx:17`、`application/http/request.ts:130` | 部分的（`ui/http/http.spec.ts:44`、エディタが出ることだけ） | 未テスト |
| Form Data・Form URL Encoded の行（種別、Content-Type、展開） | `http/form-row-editor.tsx:30` | 済（`ui/http/http.spec.ts:86`〜`:245`） | 済（`integration/http/http.spec.ts:127`） |
| ファイル参照（未確定・Browse・Enter・編集で取り消し・再選択） | `http/file-reference-input.tsx:36`、`domain/http/types.ts:48` | 済（`ui/http/request-file.spec.ts:34`〜`:193`） | 済（`integration/http/http.spec.ts:127`） |
| ファイル参照（ダイアログのキャンセル、Clear selection、ダイアログの失敗） | `http/file-reference-input.tsx:38`・`:74`、`application/http/request.ts:261` | 未テスト | 対象外 |
| リクエスト設定（タイムアウト、上限、プロキシ、TLS、リダイレクト） | `http/request-settings-panel.tsx:20` | 済（`ui/http/http.spec.ts:305`〜`:338`、`ui/http/request-send.spec.ts:93`） | 未テスト |
| Doc タブ | `http/doc-editor.tsx:15` | 済（`ui/http/http.spec.ts:393`） | 対象外 |
| レスポンス表示（ステータス、Body / Headers / Timing、画像、hex、空、HTML を含む JSON） | `http/response-viewer.tsx:142`〜`:341` | 済（`ui/http/response-viewer.spec.ts` 全体） | 済（`integration/http/http.spec.ts:81`〜`:119`） |
| 大きなレスポンス（打ち切り表示、保存、回収済み、送り直しで破棄） | `http/response-viewer.tsx:184`、`application/http/request.ts:175`・`:278` | 済（`ui/http/response-save.spec.ts`、`ui/http/response-viewer.spec.ts:210`・`:249`） | 未テスト |
| 打ち切り表示を選んだあとの、次の打ち切りレスポンス | `http/response-viewer.tsx:42` | 未テスト | 対象外 |
| レスポンスボディのコピー | `http/response-viewer.tsx:44` | 済（`ui/http/response-viewer.spec.ts:302`） | 済（`integration/http/http.spec.ts:266`） |
| 自動保存と保存失敗のバナー | `application/http/request.ts:406`、`http/index.tsx:30` | 済（`ui/common/request-persistence.spec.ts:48`、`ui/http/http.spec.ts:471`） | 済（`integration/http/http.spec.ts:238`） |
| 選択中のリクエストを別のコレクションへ移してから編集する | `application/http/request.ts:338`（保存先は選択時のコレクション ID）、`application/http/collections.ts:223` | 未テスト | 未テスト |
| 選択中リクエストのリロード後の復元 | `presentation/providers/http-provider.tsx:156`・`:168` | 済（`ui/common/request-persistence.spec.ts:22`・`:65`） | 未テスト |
| 送信中に別のリクエストへ切り替える | `application/http/request.ts:198` | 済（`ui/http/http.spec.ts:538`・`:559`） | 対象外 |
| レスポンスパネルの開閉、送信時に開く | `http/index.tsx:18`、`http/request-bar.tsx:107` | 済（`ui/http/http.spec.ts:425`・`:441`） | 対象外 |
| プロトコル切り替えで入力が残る、サイドバーの開閉 | `App.tsx:69`・`:79` | 済（`ui/common/protocol-switching.spec.ts:92`・`:112`） | 対象外 |

> 各列の値: `済（ファイル名:行番号）` / `部分的（ファイル名:行番号）` / `未テスト` / `対象外`（そのスイートで検証する意味が無い）

HTTP が公開する RPC は 20 個（`internal/adapters/http_handler.go`）。HTTP のイベントは無い（`internal/domain/events.go` は MQTT・UDP・終了確認だけ）。localStorage のキーは `wirexa:http:activeRequest` と `wirexa:http:expandedFolders`（`infrastructure/storage/local-storage.ts:15`）。

---

## 既存テストの概要

| spec | テスト数 | 検証している内容（要約） |
| --- | --- | --- |
| ui/http/http.spec.ts | 29 | メソッド選択、ボディ種別の切り替え、Form Data / URL Encoded の行（追加・無効化・空キー・種別・Content-Type・独立性）、認証の入力欄、設定の入力欄、ヘッダーの追加、Doc タブの保存と復元、レスポンスパネルの開閉、自動保存の失敗バナー、送信中に切り替えたときの応答の扱い。 |
| ui/http/request-file.spec.ts | 8 | ファイル参照。手入力のパスは未確定で送れない、Browse と Enter で確定、編集でトークンを捨てる、Form Data の file 行、自動判定の Content-Type、再起動後の `Reselect file`。 |
| ui/http/request-send.spec.ts | 4 | 入力した値が `SendRequest` の引数に載ること（Params・Headers・無効行・Bearer・Basic・メソッド・Text ボディ・設定）。 |
| ui/http/response-save.spec.ts | 3 | 打ち切られたボディの保存は execution ID だけを渡し 1 回だけ、送り直すと前のボディを破棄、回収済みのときの案内。 |
| ui/http/response-viewer.spec.ts | 11 | ステータスのバッジ（404・503・200）、空ボディ、Headers / Timing、画像、バイナリの hex と保存、打ち切り表示、上限超過の案内、HTML を含む JSON、コピー。 |
| ui/http/root-collection.spec.ts | 1 | 偽バックエンドが `__root__` の削除とリネームを拒否する（バインディングを直接呼ぶ）。 |
| ui/http/tree-drag-drop.spec.ts | 5 | D&D の 4 経路（別コレクションの見出し、コレクションの並び替え、ルートへ、フォルダの中へ）と、負の position の扱い（バインディングを直接呼ぶ）。 |
| ui/http/tree.spec.ts | 13 | リクエスト・フォルダの削除とキャンセル、リネーム（Enter・blur）、フォルダの開閉、ルート直下のリクエスト、入れ子の追加、開閉の復元、起動時の読み込み失敗。 |
| ui/common/async-loading.spec.ts | 4 | Send が Cancel に替わる、Cancel が同じ execution ID で `CancelRequest` を呼ぶ、成功時の表示、失敗時にエラー欄が出る。 |
| ui/common/form-validation.spec.ts | 9 | URL が空・不正なら Send が無効、key-value エディタの行の追加・削除・有効/無効、空白だけのリネーム、Unicode のコレクション名。 |
| ui/common/request-persistence.spec.ts | 3 | 選択中のリクエストと URL・メソッド・ヘッダー・Text ボディのリロード後の復元。 |
| ui/common/sidebar-operations.spec.ts | 6 | コレクションの作成・リネーム・Escape での取り消し・削除、リクエストの選択、コレクションの開閉。 |
| ui/common/keyboard-accessibility.spec.ts | 6（全 13） | URL 欄からの Tab、Enter で送信、確認ダイアログの Escape、Enter でリクエストを選択、HTTP パネルの id が重複しないこと。 |
| ui/common/initial-state.spec.ts | 2（全 5） | HTTP パネルの表示、HTTP の空状態。 |
| ui/common/protocol-switching.spec.ts | 2（全 7） | 別プロトコルを往復しても URL とタブの選択が残る、選択中のプロトコルの再クリックでサイドバーが開閉する。 |
| integration/http/http.spec.ts | 9 | 実サーバーへの送信とレスポンス表示、複数値ヘッダー、multipart の中身と手入力のパスが送られないこと、クエリ・ヘッダー・Bearer・Basic のワイヤ形式、保存した URL の読み直し、コピー。 |
| integration/common/backend-integration.spec.ts | 4（全 11） | M-1（バインディングで作ったコレクションが一覧に出る）、M-2（リロード後に残る、`collections/<id>.json` の形が golden と一致）、M-8（並び替えが `sidebar_layout.json` に保存される）。 |

---

## サマリー

| 観点 | チェック項目数 | 済 | 部分的 | 不足 |
| --- | --- | --- | --- | --- |
| [A] 初期状態 | 2 | 1 | 1 | 0 |
| [B] プロトコル切り替え | 3 | 3 | 0 | 0 |
| [C] サイドバー操作 | 7 | 4 | 3 | 0 |
| [D] 入力・バリデーション・境界値 | 5 | 3 | 2 | 0 |
| [E] 非同期操作 | 5 | 3 | 2 | 0 |
| [F] 画面側の永続化 | 3 | 2 | 0 | 1 |
| [G] キーボード操作・アクセシビリティ | 4 | 3 | 1 | 0 |
| [H] MQTT 固有フロー | 0 | — | — | — |
| [I] HTTP 固有フロー | 10 | 6 | 4 | 0 |
| [J] UDP 固有フロー | 0 | — | — | — |
| [K] OpenAPI 固有フロー | 0 | — | — | — |
| [L] テスト構造 | 9 | 5 | 4 | 0 |
| [M] バックエンド統合 | 6 | 2 | 3 | 1 |
| [N] 偽バックエンドの忠実さ | 5 | 2 | 1 | 2 |

> チェック項目数は各観点の表の行数。該当なしの行は数えない。観点 L は「済」を問題なし、「部分的」を改善の余地ありとして数えた。

---

## 観点別詳細

### [A] 初期状態

起動時の初期プロトコルとテーマは common の範囲なので該当なし。

#### テスト済み

- `ui/common/initial-state.spec.ts:33` — `No collections yet` と `Send a request to see the response`
- `ui/http/tree.spec.ts:233` — `GetSidebarLayout` が失敗したときにトーストが 1 件だけ出て、保存済みの選択中リクエストを復元も消去もしない

#### 不足

- **不足内容**: 起動時の読み込みに失敗したあとの操作。コレクションは読めているのにサイドバーは空なので、ここでコレクションを作ると `refreshCollections` がまた失敗し、作成は成功しているのに `Failed to create collection` が出る
- **根拠コード**: `application/http/collections.ts:147`（3 つの取得を順に待ち、最後で失敗する）、`:156`
- **スイート**: UI e2e
- **推奨テスト名**: `test("creating a collection after a failed load reports the failure and keeps existing collections", ...)`
- **追加先**: `ui/http/tree.spec.ts`
- **優先度**: 低（Go の `GetSidebarLayout` はエラーを返さないので、起きるのは RPC 自体が失敗したときだけ）

### [B] プロトコル切り替え

#### テスト済み

- `ui/common/protocol-switching.spec.ts:40` — 見出しが `Collections` に変わる
- `ui/common/protocol-switching.spec.ts:92` — MQTT を往復しても URL と選択中のタブが残る
- `ui/common/protocol-switching.spec.ts:112`・`:136` — HTTP の再クリックでサイドバーが閉じて開く、別プロトコルで開き直す

#### 不足

なし。

### [C] サイドバー操作

#### テスト済み

- 追加: `ui/common/sidebar-operations.spec.ts:7`、`ui/http/tree.spec.ts:139`・`:154`・`:168`
- リネーム: `ui/common/sidebar-operations.spec.ts:12`・`:24`、`ui/http/tree.spec.ts:82`・`:92`・`:104`
- 削除: `ui/common/sidebar-operations.spec.ts:38`、`ui/http/tree.spec.ts:27`・`:41`・`:63`、`ui/common/keyboard-accessibility.spec.ts:57`
- 選択: `ui/common/sidebar-operations.spec.ts:57`、`ui/common/keyboard-accessibility.spec.ts:159`
- 移動: `ui/http/tree-drag-drop.spec.ts:30`・`:58`・`:85`・`:118`
- 開閉: `ui/common/sidebar-operations.spec.ts:71`、`ui/http/tree.spec.ts:120`・`:184`・`:195`
- `__root__`: `ui/http/root-collection.spec.ts:32`（偽バックエンドの拒否だけ）

#### 不足

**1. 同じ親の中での並び替え**

- **不足内容**: リクエストを同じコレクションの別の位置へ動かす操作。既存の 4 件はどれも挿入ゾーンかコレクションの見出しに落としていて、行の上半分・下半分で前後を決める分岐を通るテストが無い。同じ親の中で後ろへ動かすときの position の補正も通らない
- **根拠コード**: `sidebar/use-tree-drag-drop.ts:83`〜`:100`・`:147`〜`:169`（行の上に落とす分岐）、`internal/application/http/collection_service.go:445`（補正）、`e2e/fake-backend/install.ts:439`（偽バックエンドの補正）
- **スイート**: UI e2e（seed にリクエストを 3 件置き、`fake.args("MoveItem")` と `fake.snapshot()` の並びを見る）
- **推奨テスト名**: `test("dropping a request on the lower half of a sibling moves it after that sibling", ...)`、`test("dropping a request on the upper half of a sibling moves it before that sibling", ...)`
- **追加先**: `ui/http/tree-drag-drop.spec.ts`
- **優先度**: 高（ツリーの D&D で最もよく使う操作が、どちらのスイートでも一度も通っていない）

**2. ドラッグしたあとの最初のクリック**

- **不足内容**: 並び替えた行を、そのあとクリックして開閉・選択できること。コードを読む限り、ドラッグを始めると `suppressRef.suppress` が立ち、降ろすのは同じ行の `onClick` だけ。別の要素の上でマウスを離すと行に click が届かないので、フラグが立ったまま残り、次のクリックが 1 回無視される可能性がある。行が作り直される移動（別コレクションへ、フォルダの中へ）では起きず、同じ親の中の並び替えとコレクションの並び替えで起きうる
- **根拠コード**: `sidebar/use-long-press-drag.ts:24`（立てる）、`sidebar/collection-node.tsx:81`・`sidebar/tree-item-node.tsx:149`・`:290`（降ろす）
- **スイート**: UI e2e
- **推奨テスト名**: `test("a collection can be collapsed with one click right after it was reordered", ...)`
- **追加先**: `ui/http/tree-drag-drop.spec.ts`（既存の `dragging a collection reorders the sidebar` の続きに足してもよい）
- **優先度**: 中

**3. フォルダの移動**

- **不足内容**: フォルダを子ごと別のコレクションへ動かす、フォルダを自分の子孫の中へ落とす。後者は Go が `cannot move an item into its own subtree` で拒否し `Failed to move item` のトーストが出るはずだが、偽バックエンドは拒否せずフォルダごと消す（観点 N の 3）
- **根拠コード**: `sidebar/tree-item-node.tsx:105`（フォルダもドラッグ元になる）、`internal/application/http/collection_service.go:438`
- **スイート**: UI e2e（偽バックエンドの拡張が必要: `MoveItem` に子孫への移動の拒否を足す）
- **推奨テスト名**: `test("dragging a folder to another collection moves its children with it", ...)`、`test("dropping a folder into its own subfolder is rejected and keeps the tree", ...)`
- **追加先**: `ui/http/tree-drag-drop.spec.ts`
- **優先度**: 中

**4. ルートのアイテムの並び替えと、ルートからの移動**

- **不足内容**: ルート直下のリクエストをサイドバー上で並び替える（`MoveSidebarEntry("item", ...)`）、ルートのリクエストをコレクションへ入れる
- **根拠コード**: `sidebar/collection-tree.tsx:142`〜`:144`、`sidebar/use-tree-drag-drop.ts:113`
- **スイート**: UI e2e（ルートのアイテムは seed で置けないので、`app.createRootRequest` で作る。seed にルートのアイテムを足せるとよい）
- **推奨テスト名**: `test("dragging a root request reorders it among collections", ...)`、`test("dragging a root request onto a collection moves it out of the sidebar root", ...)`
- **追加先**: `ui/http/tree-drag-drop.spec.ts`
- **優先度**: 中（後者は観点 N の 4 のずれを直してから書く）

**5. ルートへ出したフォルダの開閉**

- **不足内容**: フォルダをルートへ出したあと、開いた状態が保たれること。コードを読む限り、`pruneExpandedIds` が有効な ID を集めるのは通常のコレクションの中だけで、ルートのアイテムを見ていない。ルートのフォルダを開いても、次の再読み込み（追加・リネーム・移動のたびに走る）で開閉の記録が消え、フォルダが閉じる可能性がある。その場合、開いたフォルダに `Add request` をしてもリネーム入力が出ない
- **根拠コード**: `application/http/collections.ts:115`〜`:135`（`cols` だけを歩く）、`:147`〜`:150`
- **スイート**: UI e2e
- **推奨テスト名**: `test("a folder moved to the sidebar root stays expanded after adding a request to it", ...)`
- **追加先**: `ui/http/tree-drag-drop.spec.ts`
- **優先度**: 中

**6. 選択中のリクエストの削除**

- **不足内容**: 編集エリアに開いているリクエスト、またはそれを含むフォルダ・コレクションを削除したあとの挙動。削除は一覧から消すだけで、編集エリアは消えたリクエストを指したまま残る。そのあと入力すると自動保存が Go で `request not found` になり `Save failed` のバナーが出るはずだが、偽バックエンドは黙って成功する（観点 N の 1）
- **根拠コード**: `sidebar/collection-tree.tsx:118`〜`:120`、`application/http/request.ts:338`、`internal/application/http/collection_service.go:351`
- **スイート**: UI e2e（偽バックエンドの拡張が必要: `UpdateRequest` が見つからないリクエストを拒否する）
- **推奨テスト名**: `test("editing after deleting the open request reports that it can no longer be saved", ...)`
- **追加先**: `ui/http/tree.spec.ts`
- **優先度**: 中（期待する挙動そのもの、つまり編集エリアを空に戻すかどうかを先に決める必要がある）

**7. その他**

- ルート直下のリクエストの削除（`DeleteItem` がサイドバーレイアウトからも外す）— `ui/http/tree.spec.ts` に追加、優先度 低
- Add メニューが外側のクリックで閉じる（`sidebar/collection-tree.tsx:28`）— 優先度 低
- 長押し（250ms）でのドラッグ開始 — `use-long-press-drag.test.ts` で検証済みなので優先度 低

### [D] 入力・バリデーション・境界値

ポート番号、MQTT トピック、`config/limits.ts` の上限、UDP の固定長フィールドは HTTP に無いので該当なし。

#### テスト済み

- `ui/common/form-validation.spec.ts:12`・`:18`・`:27`・`:36` — URL が空・不正なら Send が無効
- `ui/common/form-validation.spec.ts:110` — 空白だけのリネームは元の名前のまま
- `ui/common/form-validation.spec.ts:48`・`:71`・`:92`、`ui/http/http.spec.ts:86`〜`:245` — key-value エディタと Form Data の行
- `ui/common/form-validation.spec.ts:125` — 日本語のコレクション名
- `ui/http/response-viewer.spec.ts:280` — HTML を含む JSON レスポンスがマークアップとして解釈されない

#### 不足

**1. 不正な URL での Enter**

- **不足内容**: URL が空・不正なときに URL 欄で Enter を押した場合。Send ボタンは無効になるが、Enter のハンドラは URL を検査せずに送信する。空の URL でも `SendRequest` が呼ばれる
- **根拠コード**: `http/request-bar.tsx:82`〜`:84`（`urlValid()` を見ていない）、`:99`（ボタンだけ無効）
- **スイート**: UI e2e（`fake.calls("SendRequest")`）
- **推奨テスト名**: `test("pressing Enter with an invalid URL does not send", ...)`
- **追加先**: `ui/common/form-validation.spec.ts`
- **優先度**: 中（現状の実装ではこのテストは失敗するはずで、送らないのが正しいなら本体の修正が要る）

**2. 特殊文字**

- **不足内容**: リクエスト名・フォルダ名に `<script>` や絵文字を入れた場合。コレクション名の日本語だけを見ている
- **根拠コード**: `sidebar/tree-item-node.tsx:320`（テキストとして描く）
- **スイート**: UI e2e
- **推奨テスト名**: `test("request name with markup and emoji is shown as text", ...)`
- **追加先**: `ui/common/form-validation.spec.ts`
- **優先度**: 低（Solid がテキストとして描くので壊れにくい）

**3. 設定の数値欄**

- **不足内容**: タイムアウトを空にしたときに 0（既定）として送られること、負の値
- **根拠コード**: `http/request-settings-panel.tsx:36`〜`:42`
- **スイート**: UI e2e
- **推奨テスト名**: `test("clearing the timeout sends 0 so the backend default applies", ...)`
- **追加先**: `ui/http/request-send.spec.ts`
- **優先度**: 低

### [E] 非同期操作

#### テスト済み

- `ui/common/async-loading.spec.ts:16` — Send が Cancel に替わる
- `ui/common/async-loading.spec.ts:30` — Cancel が送信時と同じ execution ID で `CancelRequest` を呼び、表示が戻る
- `ui/common/async-loading.spec.ts:54`、`ui/http/response-viewer.spec.ts` — 成功後の表示
- `ui/common/async-loading.spec.ts:70`、`ui/http/http.spec.ts:471` — 送信の失敗、自動保存の失敗のバナーとトースト
- `ui/http/http.spec.ts:538`・`:559` — 送信中に切り替えたら古い応答を出さず、打ち切られた応答は破棄する

#### 不足

**1. 送信中の Enter**

- **不足内容**: 送信中に URL 欄で Enter を押した場合。ボタンは Cancel に替わるのでクリックでは二重に送れないが、Enter は送信中かどうかを見ないので 2 件目が送られる
- **根拠コード**: `http/request-bar.tsx:82`〜`:84`、`application/http/request.ts:221`（同時送信を前提に ID を配列で持つ）
- **スイート**: UI e2e（`httpResponseDelayMs` と `fake.calls("SendRequest")`）
- **推奨テスト名**: `test("pressing Enter while a request is in progress does not send a second one", ...)`
- **追加先**: `ui/common/async-loading.spec.ts`
- **優先度**: 中（二重実行を許すのが意図なら、その挙動を固定するテストにする）

**2. 自動保存の失敗トーストが重複しないこと**

- **不足内容**: 失敗が続いてもトーストが 1 件のままであること。既存のテストは 1 回目の表示しか見ていない
- **根拠コード**: `application/http/request.ts:428`（`key: "http-autosave"`）
- **スイート**: UI e2e
- **推奨テスト名**: `test("repeated auto-save failures keep a single toast", ...)`
- **追加先**: `ui/http/http.spec.ts`（`when saving the request fails` の中）
- **優先度**: 低（`request.test.ts:1140` で key を渡すことは検証済み）

**3. コレクション操作の失敗**

- **不足内容**: 作成・削除・リネーム・追加・移動が失敗したときのトーストと、そのあとの画面（追加に失敗したらリネーム入力を出さない、削除に失敗したら行が残る）
- **根拠コード**: `application/http/collections.ts:156`〜`:262`、`sidebar/collection-tree.tsx:51`〜`:116`
- **スイート**: UI e2e（偽バックエンドの拡張が必要: コレクション操作の RPC を失敗させる注入口。観点 N の 6）
- **推奨テスト名**: `test("a failed delete shows an error toast and keeps the item", ...)`
- **追加先**: `ui/http/tree.spec.ts`
- **優先度**: 低（`collections.test.ts:297`〜`:357` で通知と再送出は検証済み）

**4. レスポンスの保存の失敗**

- **不足内容**: 回収済み以外の理由で保存に失敗したときの `Failed to save response` のトースト
- **根拠コード**: `application/http/request.ts:289`〜`:296`
- **スイート**: UI e2e（`saveResponseError` に `response body unavailable` 以外の文言を入れる）
- **推奨テスト名**: `test("a failed save shows an error toast and keeps the save button", ...)`
- **追加先**: `ui/http/response-save.spec.ts`
- **優先度**: 低（`request.test.ts:790` で検証済み）

### [F] 画面側の永続化

テーマと MQTT・UDP のキーは該当なし。

#### テスト済み

- `ui/common/request-persistence.spec.ts:22`・`:48`・`:65` — `wirexa:http:activeRequest`
- `ui/http/tree.spec.ts:184`・`:195` — `wirexa:http:expandedFolders`

#### 不足

- **不足内容**: 形の壊れた保存値、および消えたリクエストを指す `wirexa:http:activeRequest` で起動できること
- **根拠コード**: `infrastructure/storage/local-storage.ts:89`・`:105`、`presentation/providers/http-provider.tsx:174`〜`:180`
- **スイート**: UI e2e（`page.evaluate` で値を書いて `page.reload()`、`pageErrors` が空であること）
- **推奨テスト名**: `test("a stale or malformed saved selection is ignored on startup", ...)`
- **追加先**: `ui/common/request-persistence.spec.ts`
- **優先度**: 低（`local-storage.test.ts` と `collections.test.ts:148` で検証済み）

### [G] キーボード操作・アクセシビリティ

ショートカットは HTTP に無いので該当なし。

#### テスト済み

- `ui/common/keyboard-accessibility.spec.ts:18`・`:29` — URL 欄から Send へ Tab で移る
- `ui/common/keyboard-accessibility.spec.ts:45`、`ui/http/request-file.spec.ts:87` — Enter で送信、Enter でファイルダイアログ
- `ui/common/sidebar-operations.spec.ts:24`、`ui/common/keyboard-accessibility.spec.ts:57` — Escape でリネームと確認ダイアログを取り消す
- `ui/common/keyboard-accessibility.spec.ts:200` — HTTP パネルの id が重複せず、tabpanel を名前で引ける

#### 不足

- **不足内容**: 保存失敗バナーの閉じるボタンにアクセシブル名が無い。テストは名前で引けず、バナーの親要素から辿っている。名前を付ければテストも `getByRole("button", { name: "Dismiss" })` で書ける
- **根拠コード**: `http/index.tsx:33`〜`:39`（中身が `✕` だけ）、`e2e/ui/http/http.spec.ts:489`
- **スイート**: UI e2e（本体に `aria-label` を足したうえで）
- **推奨テスト名**: 既存の `auto-save failure shows a dismissible banner` のロケーターを置き換える
- **追加先**: `ui/http/http.spec.ts`
- **優先度**: 低

### [H] MQTT 固有フロー

該当なし（対象が HTTP）。

### [I] HTTP 固有フロー

#### テスト済み

- メソッド・URL・送信: `ui/http/http.spec.ts:14`、`ui/http/request-send.spec.ts:74`、`integration/http/http.spec.ts:68`
- ヘッダー・クエリ・認証: `ui/http/request-send.spec.ts:21`・`:53`、`integration/http/http.spec.ts:181`・`:216`
- リクエスト設定: `ui/http/http.spec.ts:305`〜`:338`、`ui/http/request-send.spec.ts:93`
- レスポンス表示: `ui/http/response-viewer.spec.ts` 全体、`integration/http/http.spec.ts:81`〜`:119`
- コピー: `ui/http/response-viewer.spec.ts:302`、`integration/http/http.spec.ts:266`
- レスポンスパネルの開閉: `ui/http/http.spec.ts:425`・`:441`

#### 不足

**1. 選択中のリクエストを移動したあとの自動保存**

- **不足内容**: 編集エリアに開いているリクエストを別のコレクション（またはルート）へ D&D で移し、そのあと入力した内容が保存されること。コードを読む限り、保存先のコレクション ID は選択したときの値のままで、移動しても更新されない。Go は元のコレクションでリクエストを探して `request not found` を返すので、移動後の編集は保存されず `Save failed` のバナーが出続ける。localStorage の選択中リクエストも古いコレクション ID のままなので、リロード後に復元されない。偽バックエンドは見つからないリクエストの更新を黙って成功させるので、UI e2e を書いても今のままでは検出できない（観点 N の 1）
- **根拠コード**: `application/http/request.ts:316`〜`:317`（選択時に設定）・`:338`〜`:355`（その ID で保存）、`application/http/collections.ts:223`〜`:240`（移動はコレクションの再読み込みだけ）、`sidebar/collection-tree.tsx:155`〜`:170`、`internal/application/http/collection_service.go:351`〜`:354`、`e2e/fake-backend/install.ts:398`〜`:399`
- **スイート**: UI e2e（偽バックエンドの拡張が必要: `UpdateRequest` が見つからないリクエストを `request not found: <id>` で拒否する）。Go との組み合わせを確かめるためフルスタックにも 1 件あるとよい
- **推奨テスト名**: `test("edits made after moving the open request to another collection are saved", ...)`
- **追加先**: `ui/http/tree-drag-drop.spec.ts`
- **優先度**: 高（保存されない編集が出る。現状の実装ではこのテストは失敗するはずなので、本体の修正とセットになる）

**2. JSON ボディの内容**

- **不足内容**: JSON ボディに入力した内容が `SendRequest` の引数に載ること。既存のテストは JSON を選ぶとエディタが出ることしか見ていない。あわせて、JSON を選んだだけで触っていないときは雛形（`{ "": "" }`）が表示されるが送られないこと
- **根拠コード**: `application/http/request.ts:130`〜`:137`（雛形は表示だけ）、`http/request-editor.tsx:137`〜`:142`
- **スイート**: UI e2e（`app.editor(bodyPanel).fill(...)` と `fake.args("SendRequest")`）
- **推奨テスト名**: `test("json body is sent with the request", ...)`、`test("an untouched json body sends no content", ...)`
- **追加先**: `ui/http/request-send.spec.ts`
- **優先度**: 中（POST で最もよく使うボディ種別）

**3. 打ち切り表示を選んだあとの次のレスポンス**

- **不足内容**: `Show truncated body` を押したあと、もう一度送信して打ち切られたレスポンスを受けたときに、選択の画面（`Save body to file` を含む）がまた出ること。コードを読む限り、表示を選んだ状態はレスポンスが替わっても戻らないので、2 回目以降は打ち切られた本文がそのまま出て、全文を保存するボタンが出ない可能性がある
- **根拠コード**: `http/response-viewer.tsx:42`（リセットする箇所が無い）、`:184`〜`:228`
- **スイート**: UI e2e
- **推奨テスト名**: `test("a new truncated response offers the save choice again after showing the previous one", ...)`
- **追加先**: `ui/http/response-viewer.spec.ts`
- **優先度**: 中

**4. ファイル参照の残りの経路**

- **不足内容**: ダイアログをキャンセルしたとき入力したヒントが残り未確定のままであること、`Clear selection` で未選択に戻ること、`Reselect file` の状態から Browse で確定して送れること
- **根拠コード**: `http/file-reference-input.tsx:36`〜`:39`・`:74`〜`:84`、`domain/http/types.ts:36`〜`:44`
- **スイート**: UI e2e（キャンセルは `pickedFile` を省くだけでよい）
- **推奨テスト名**: `test("cancelling the file dialog keeps the typed path unconfirmed", ...)`、`test("clear selection removes the confirmed file", ...)`、`test("a file that needs reselecting can be sent after Browse", ...)`
- **追加先**: `ui/http/request-file.spec.ts`
- **優先度**: 中

**5. リロード後に復元される項目**

- **不足内容**: Params・認証・設定・Form Data の行がリロード後に復元されること。復元を見ているのは URL・メソッド・ヘッダー・Text ボディ・Doc だけ。Go から読み直した値は `infrastructure/http/client.ts` の変換を通るので、設定や form 行の変換の取りこぼしはここでしか見つからない
- **根拠コード**: `infrastructure/http/client.ts:53`〜`:119`
- **スイート**: UI e2e
- **推奨テスト名**: `test("params, auth, settings and form rows are restored after reload", ...)`
- **追加先**: `ui/common/request-persistence.spec.ts`
- **優先度**: 中

**6. その他**

- サイドバーのメソッド表示が自動保存のあと更新される（`application/http/collections.ts:264`）— `ui/common/request-persistence.spec.ts` に追加、優先度 低
- 送信後に別のリクエストを選ぶとレスポンスが消える（送信中ではない場合）— `request.test.ts:425` で検証済み、優先度 低
- Text と JSON のボディが種別ごとに保たれる — `request.test.ts:248` で検証済み、優先度 低
- 入力の直後（500ms 以内）に別のリクエストへ切り替えても保存される — `request.test.ts:953` で検証済み、優先度 低
- 1MB を超える JSON はハイライトしない（`http/response-viewer.tsx:282`）— 優先度 低
- 保存ダイアログをキャンセルしたら保存ボタンが残る — 偽バックエンドの拡張が必要（観点 N の 6）、`request.test.ts:391` で検証済み、優先度 低

### [J] UDP 固有フロー

該当なし（対象が HTTP）。

### [K] OpenAPI 固有フロー

該当なし（対象が HTTP）。

### [L] テスト構造

#### 問題なし

- 固定 sleep（`waitForTimeout`）は無い。デバウンスされた自動保存は `fake.waitForCalls` と `expect.poll` で待っている
- `test.only`・`test.skip`・`test.fixme` は無い
- 個別の `timeout` 指定は無い
- URL・ポートの直書きは無い（UI e2e の `http://127.0.0.1:9999/...` は偽バックエンドに渡すだけの値。フルスタックは `server.port` を使う）
- フルスタックの後始末: HTTP は Go 側のメモリに残る状態を作らないので `afterEach` は不要。保存データは名前を一意にして絞り込んでいる。`backend-integration.spec.ts:278` は行の並びを完全一致で見るが、名前で絞ったロケーターなので前のテストのデータの影響は受けない

#### 改善の余地

**1. ページオブジェクトを通さないロケーター**

- `ui/http/http.spec.ts:46`・`:56`・`:68`・`:79`・`:249`・`:268`・`:318`、`ui/http/request-file.spec.ts:20`、`integration/http/http.spec.ts:136` — `panel.getByRole("button").first()` で Select のトリガーを位置で取っている。`app.chooseOption(panel, current, label)` が既にある
- `ui/http/http.spec.ts:15` — `page.getByTestId("method-select")` を直接使っている。`app.selectMethod` がある
- `ui/http/http.spec.ts:5`〜`:10`、`ui/http/response-save.spec.ts:6`〜`:11`、`ui/common/form-validation.spec.ts:3`〜`:8` — `beforeEach` が `app.switchTo("HTTP")` を使わず同じ処理を書いている。`form-validation.spec.ts` は URL 欄と Send も `app.urlInput`・`app.sendButton` を使わず、Send は `exact` 無しで引いている
- `ui/http/http.spec.ts:490` — `banner.locator("xpath=..")`（観点 G のとおり、本体に名前を付ければ不要）
- `ui/http/response-viewer.spec.ts:39`・`:55` — `toHaveClass(/variantDestructive/)` が CSS Modules のクラス名に依存している。バッジに `data-variant` を付けるなど、見た目の種別を属性で出すと安定する
- 優先度: 低

**2. ヘルパーの重複**

- `lastSent` — `ui/http/request-file.spec.ts:26` と `ui/http/request-send.spec.ts:15`（後者だけ呼び出しを待つ）。`FakeControl` に `lastArgs(name)` を足して寄せる
- `storedCollection` — `ui/http/tree.spec.ts:18` と `ui/http/tree-drag-drop.spec.ts:21`
- バインディングの直接呼び出し — `ui/http/root-collection.spec.ts:11` と `ui/http/tree-drag-drop.spec.ts:150`
- ボディ種別の選択 — `ui/http/http.spec.ts:77`（`openFormBody`）と `ui/http/request-file.spec.ts:18`（`chooseBodyType`）
- 優先度: 低

**3. 結果を十分に見ていないテスト**

- `ui/common/async-loading.spec.ts:70` — テスト名は「エラーメッセージを表示する」だが、エラー欄が見えることしか見ていない。`toContainText("connection refused")` を足す
- `ui/common/async-loading.spec.ts:54`、`integration/http/http.spec.ts:75` ほか — `page.getByText("200", { exact: true })` がレスポンス表示エリアでスコープされていない。`app.responseViewer` を使う
- `ui/http/http.spec.ts:305`・`:342`・`:360` — 入力した値が入力欄に入っていることしか見ていない。値が RPC に載ることは `request-send.spec.ts` が見ているので、表示の切り替えを見るテスト（`:314` など）以外は統合できる
- `ui/common/keyboard-accessibility.spec.ts:18` — 直後の `:29`（Send にフォーカスが移る）に含まれる
- `ui/common/sidebar-operations.spec.ts:7`・`:38` — 画面だけを見ていて、`fake.snapshot()` で偽バックエンドの状態を見ていない
- 優先度: 低

**4. スイートの選び方と置き場所**

- `integration/http/http.spec.ts:266`（コピー）は `ui/http/response-viewer.spec.ts:302` と同じ内容で、実バックエンドでしか確かめられないことを含まない。`:68` は `:81` に含まれる
- `integration/http/http.spec.ts:238`（リロードせずに別のリクエストを選んでから戻ると URL が入っている）に当たる UI e2e が無い。CI で回すため `ui/common/request-persistence.spec.ts` にも置く
- `ui/common/` の `async-loading`・`form-validation`・`request-persistence`・`sidebar-operations` は全件が HTTP のテスト。`ui/http/` へ移すと、プロトコル単位で読む・実行するときに漏れない
- 優先度: 低

### [M] バックエンド統合

M-3・M-4・M-9 は他のプロトコル、M-7 は HTTP がイベントを発火しないので該当なし。

#### テスト済み

- M-1: `integration/common/backend-integration.spec.ts:36`
- M-2: `integration/common/backend-integration.spec.ts:109`（リロード後に残る）・`:118`（ファイル名と、リクエストのアイテムの形が golden と一致）
- M-6: `integration/http/http.spec.ts:127`（multipart）・`:181`（クエリ・ヘッダー・Bearer）・`:216`（Basic）
- M-8: `integration/common/backend-integration.spec.ts:268`
- M-10: `integration/http/http.spec.ts:127`（手入力のパスのファイルが送られない）

#### 不足

**1. M-5: バックエンドのエラーの表示**

- **不足内容**: 実バックエンドが返したエラーが画面に出ること。繋がらないポートへ送ったときの `failed to send request: ...` を見るテストがフルスタックに無い。Wails はエラーを `Error` ではなく文字列で返すので、偽バックエンド（`Error` を投げる）とは経路が違う
- **根拠コード**: `internal/application/http/request_service.go:124`、`application/http/request.ts:244`〜`:249`
- **スイート**: フルスタック
- **推奨テスト名**: `test("a connection error from the backend is shown in the response viewer", ...)`
- **追加先**: `integration/http/http.spec.ts`
- **優先度**: 中

**2. M-10: 回収済みを表す文言の一致**

- **不足内容**: `response body unavailable` という文言が Go と TypeScript で一致していること。画面は文言の部分一致で「回収済み」を判定していて、UI e2e ではテストが自分で同じ文字列を seed に入れているので、Go 側の文言が変わっても検出できない。未知の execution ID で `SaveResponseBody` を直接呼べば、保存ダイアログが開く前に拒否されるのでフルスタックで確かめられる
- **根拠コード**: `domain/http/types.ts:177`、`internal/domain/http/errors.go:10`、`internal/adapters/http_handler.go:180`
- **スイート**: フルスタック（`app.mqttConnectError` と同じ形のヘルパーを `App` に足す）
- **推奨テスト名**: `test("saving an unknown execution ID is rejected with the message the UI treats as reclaimed", ...)`
- **追加先**: `integration/http/http.spec.ts`
- **優先度**: 中

**3. M-6: Cancel と打ち切り**

- **不足内容**: 実バックエンドでの Cancel（遅いエンドポイントへ送って Cancel を押すと送信が止まり表示が戻る）、`Max Response Body (MB)` を超えるレスポンスで打ち切りの案内が出ること
- **根拠コード**: `internal/application/http/request_service.go:134`、`internal/infrastructure/http/net_client.go`
- **スイート**: フルスタック
- **推奨テスト名**: `test("cancel aborts a request to a slow server", ...)`、`test("a response larger than the limit is shown as truncated", ...)`
- **追加先**: `integration/http/http.spec.ts`
- **優先度**: 低（Go 側は `TestHTTP_CancelRequest`・`TestHTTP_TruncatedResponse_SaveAndDiscard` で検証済み。画面との組み合わせだけが残る）

**4. M-2: 更新と削除のディスクへの反映**

- **不足内容**: コレクションのリネーム・削除でファイルが書き換わる・消えること（MQTT には `backend-integration.spec.ts:181` があるが HTTP には無い）、自動保存した内容がリロード後に残ること（`integration/http/http.spec.ts:238` はリロードしない）
- **根拠コード**: `internal/infrastructure/http/collection_repository.go`
- **スイート**: フルスタック
- **推奨テスト名**: `test("renaming and deleting a collection updates and removes its file", ...)`
- **追加先**: `integration/common/backend-integration.spec.ts`
- **優先度**: 低（Go 側は `TestHTTP_CollectionCRUD`・`TestHTTP_PersistenceRoundTrip` で検証済み）

### [N] 偽バックエンドの忠実さ

HTTP はイベントを発火しないので「イベントの一致」は該当なし。

#### 問題なし

- RPC の網羅: `internal/adapters/http_handler.go` の公開メソッド 20 個がすべて `e2e/fake-backend/install.ts:319`〜`:505` にある
- 型の追従: `e2e/fake-backend/types.ts:3`〜`:9` と `install.ts:8`〜`:17` が domain 型を使っている

#### ずれ

**1. 見つからない対象を黙って成功させる**

- Go は `NotFoundError` を返すが、偽バックエンドは何もせず成功する:
  - `UpdateRequest` — Go `internal/application/http/collection_service.go:351`〜`:354` / 偽 `install.ts:398`〜`:399`
  - `RenameItem` — Go `:382`〜`:385` / 偽 `install.ts:408`〜`:409`
  - `DeleteItem` — Go `:616`〜`:618` / 偽 `install.ts:415`〜`:416`
  - `DeleteCollection` — Go `:230`〜`:233` / 偽 `install.ts:343`〜`:349`
  - `AddFolder`・`AddRequest`（親が無い）— Go `:321`〜`:323` / 偽 `install.ts:365`・`:385`（挿入せずにアイテムを返す）
  - `MoveItem`（アイテムが無い）— Go `:427`〜`:430` / 偽 `install.ts:435`〜`:436`
  - `MoveSidebarEntry` — Go `internal/application/http/sidebar_layout_service.go:69`〜`:71` / 偽 `install.ts:459`
- 影響: 観点 I の 1 と観点 C の 6 が UI e2e で検出できない。特に `UpdateRequest`
- 優先度: 高（`UpdateRequest`）、中（それ以外）

**2. 検証の不足**

- `CreateCollection` が空白だけの名前を拒否しない — Go `collection_service.go:199`〜`:201` / 偽 `install.ts:336`
- `SendRequest` がメソッドを検証しない — Go `internal/application/http/request_service.go:86`〜`:88` / 偽 `install.ts:288`〜`:317`
- `__root__` を拒否するときの文言が違う — Go は `invalid id: reserved collection cannot be modified`（`collection_service.go:277`）/ 偽は `reserved collection cannot be modified: __root__`（`install.ts:208`）
- 影響: どれも画面からは送れない値なので小さい
- 優先度: 低

**3. `MoveItem` が子孫への移動でアイテムを失う**

- Go は `cannot move an item into its own subtree` で拒否する（`collection_service.go:438`〜`:441`）。偽バックエンドは移動元から外したあと移動先が見つからず、そのまま戻るのでフォルダごと消える（`install.ts:446`〜`:448`）
- 影響: 観点 C の 3 を書くと、実物とは違う結果でテストが通る・落ちる
- 優先度: 中

**4. ルートから出したアイテムがサイドバーレイアウトに残る**

- Go の `GetSidebarLayout` は読み出すたびにコレクションと突き合わせて、存在しないエントリを落とす（`collection_service.go:491`〜`:500`）。偽バックエンドの `MoveItem` はルートから出したアイテムのエントリを消さず（`install.ts:424`〜`:451`）、`GetSidebarLayout` も突き合わせない（`install.ts:329`〜`:334`）
- 影響: 画面は存在しないエントリを描かないが、挿入ゾーンの position は残ったエントリを数えるので、そのあとの並び替えの position が Go とずれる
- 優先度: 中

**5. `GetCollections` の並び**

- Go は名前順（`collection_service.go:180`〜`:182`）、偽バックエンドは作成順（`install.ts:320`〜`:322`）
- 影響: 画面の並びはサイドバーレイアウトで決まるので、今は見えない
- 優先度: 低

#### 忠実さを確かめるテスト

- 両方のスイートにあるシナリオ: 入力値が送信に載る（`ui/http/request-send.spec.ts` と `integration/http/http.spec.ts:181`）、手入力のパスを送らない（`ui/http/request-file.spec.ts:34` と `integration/http/http.spec.ts:127`）、コレクションの並び替え（`ui/http/tree-drag-drop.spec.ts:58` と `backend-integration.spec.ts:268`）
- `__root__` の拒否は偽バックエンド側（`ui/http/root-collection.spec.ts:32`）と Go の統合テスト（`TestHTTP_RootCollection_DeleteAndRenameRejected`）にあり、フルスタック e2e には無い。偽バックエンドのテストはメッセージが `null` でないことしか見ていないので、ずれ 2 の文言の違いは検出されない
- 上のずれ 1・3・4 を検出するテストは、どちらのスイートにも無い

#### 6. 注入口の不足

`FakeSeed`（`e2e/fake-backend/types.ts:18`）に無いもの:

| 必要な注入 | 使う観点 | 優先度 |
| --- | --- | --- |
| コレクション操作の RPC（`CreateCollection`・`DeleteItem`・`MoveItem` など）の失敗 | E-3 | 低 |
| `OpenFilePicker` の失敗（`Failed to open file picker` のトースト） | E | 低 |
| `SaveResponseBody` が `false` を返す（保存ダイアログのキャンセル） | I-6 | 低 |
| `SaveResponseBase64` の失敗 | E | 低 |
| `UpdateRequest` を 1 回だけ失敗させる（次の保存でバナーが消えること） | E | 低 |
| フォルダ、ルート直下のアイテム、メソッド・ヘッダー・認証・設定を持つリクエストの seed（今は名前・URL・ボディだけ。`ui/http/tree.spec.ts:5` に「フォルダは seed で仕込めない」とある） | C・I-5 | 中 |

---

## 優先して追加すべきテスト

1. **選択中のリクエストを別のコレクションへ移したあとの編集が保存されること**（観点 I-1）— 根拠: `application/http/request.ts:338`、`application/http/collections.ts:223`、`internal/application/http/collection_service.go:351`、スイート: UI e2e（偽バックエンドの `UpdateRequest` を Go に合わせて、見つからないリクエストを拒否するようにしてから）。コードを読む限り現状は失敗するはずなので、まず 1 件書いて実際の挙動を確かめる
2. **同じ親の中でのリクエストの並び替え**（観点 C-1）— 根拠: `sidebar/use-tree-drag-drop.ts:83`・`:147`、スイート: UI e2e。あわせて、並び替えた行が直後のクリックに反応すること（観点 C-2）を同じテストで見る

---

## 総合評価

**主なリスク**:

- 偽バックエンドが「見つからない対象」を黙って成功させるため、選択中のリクエストを移動・削除したあとの自動保存の失敗を UI e2e が検出できない。フルスタックにもこのフローは無いので、どのテストにも掛からない
- ツリーの D&D は「別の場所へ移す」4 経路だけを見ていて、同じ親の中の並び替え、フォルダの移動、ルートのアイテムの並び替えが通っていない。ドラッグ後のクリック抑止とルートのフォルダの開閉は、コードを読む限り不具合の疑いがあるが、該当するテストが無い
- レスポンス表示と送信の引数は厚く検証されている。残っているのは JSON ボディの内容、打ち切り表示を選んだあとの次のレスポンス、URL 欄の Enter が検証と送信中の判定を通らない点
- フルスタックは正常系の送信と保存形式を見ているが、実バックエンドのエラーが画面に出る経路（M-5）が 1 件も無い

**推奨アクション**:

1. `e2e/fake-backend/install.ts` の `UpdateRequest`・`RenameItem`・`DeleteItem`・`MoveItem` を Go に合わせ、見つからない対象と子孫への移動を拒否する。ルートから出したアイテムのレイアウトエントリも外す（観点 N の 1・3・4）
2. 「優先して追加すべきテスト」の 2 件を書く。1 件目が失敗したら、移動時に保存先のコレクション ID を更新するよう本体を直す
3. コードを読んで疑いが出た 4 点を、テストを書いて確かめる: ドラッグ後の最初のクリック（C-2）、ルートのフォルダの開閉（C-5）、打ち切り表示のあとの保存ボタン（I-3）、不正な URL・送信中の Enter（D-1・E-1）。期待する挙動が決まっていないもの（選択中のリクエストの削除、送信中の Enter）は先に決める
4. `ui/http/request-send.spec.ts` に JSON ボディ、`ui/http/request-file.spec.ts` にキャンセル・Clear・再選択、`ui/common/request-persistence.spec.ts` に Params・認証・設定・form 行の復元を足す
5. フルスタックに、接続エラーの表示（M-5）と `response body unavailable` の文言の一致（M-10）を足す
6. 余力があれば観点 L を片付ける: Select の操作を `app.chooseOption` に寄せる、`lastSent`・`storedCollection` を fixture へ移す、HTTP だけを扱う `ui/common/` の 4 本を `ui/http/` へ移す
