# 変更計画書: コメント見直しで見つかったバックエンドの小さな整理

## 概要

`/comment-review backend` で、コメントではなくコードを直すべきものが 5 件見つかった。どれも「コメントで説明している内容を、名前・構造・定数で表す」変更で、RPC の形・書き出す保存形式・イベントは変えない。

触る箇所に残っていた旧形式への対応（コレクションの旧ファイルパスの移行、旧形式の一時ファイルの sweep）は、AGENTS.md の「後方互換を保たない・データを移行しない」に合わせて、移さずに消す。

| # | 対象 | いまの問題 | 変更 |
| --- | --- | --- | --- |
| 1 | `collection_repository.go` | 397 行のうち 330 行が永続化 DTO と変換関数で、`// ── 永続化 DTO ──` などの区切りコメント 3 つで区切っている。旧形式のファイルパスの移行（`legacyFilePath`・`UnmarshalJSON`・`legacyBaseName`）も残っている | 旧形式の移行を消し、DTO と変換関数を別ファイルへ移す |
| 2 | `SweepStaleTempFiles`・`sessionSecret` | 削除の失敗を黙って捨てる。呼び出し時点で logger は存在する。旧形式の一時ファイルの sweep は実際の `os.TempDir()` を対象にしていて、統合テストからも実環境のファイルを消す。長さが不正なシークレットファイルがあると、空のシークレットをエラーなしで返す | 旧形式の sweep を消す。logger を引数で受けて記録する。シークレットの長さを読み直しのあとも確かめる |
| 3 | `position` の `-1` | 「末尾に追加」を裸の `-1` で表している（フロントエンド 2 か所、Go テスト 14 か所） | 名前付き定数にする |
| 4 | `ValidateID`・`validateTopicString` | エラーメッセージに `128`・`65535` を直書きしていて、`maxIDLength`・`maxTopicBytes` を変えると食い違う | メッセージを定数から組み立てる |
| 5 | `errShuttingDown` | 同じ文言の sentinel が `httpapp` と `mqttapp` に 1 つずつある | `domain.ErrShuttingDown` に 1 つにまとめる |

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `internal/infrastructure/http/collection_repository.go` | Infrastructure | 変更 | #1: `CollectionRepository` と `New`・`Load`・`Save`・`Delete`・`Exists` だけを残す。`Load` の doc コメントから旧形式の移行の説明を消す |
| `internal/infrastructure/http/stored_collection.go` | Infrastructure | 新規 | #1: `storedXxx` DTO、`toStoredXxx` / `fromStoredXxx` / `toDomain` を移す。区切りコメントは消す。旧形式の移行（`legacyFilePath`・`storedFormRow.UnmarshalJSON`・`legacyBaseName`・`fromStoredFileRef` の `legacyPath` 引数）は移さずに消す |
| `internal/infrastructure/http/collection_repository_test.go` | Infrastructure (test) | 変更 | #1: `TestCollectionRepository_MigratesLegacyPaths` と `TestLegacyBaseName` を消す。`TestCollectionRepository_LoadedServiceHidesLegacyPaths` は「`Contents["file"]` にパスがあるファイルを読んでも RPC 応答にパスが出ない」ことだけを確かめる形に直す。#3: `-1` を定数に |
| `internal/infrastructure/http/response_store.go` | Infrastructure | 変更 | #2: `SweepStaleTempFiles(baseDir string, logger cmn.Logger)` にする。旧形式の sweep と `legacyResponseFilePrefix` を消す。`sessionSecret` は読み直した内容の長さも確かめる |
| `app.go` | 合成ルート | 変更 | #2: `SweepStaleTempFiles(sessionDir, logger)` |
| `internal/infrastructure/http/net_client_cleanup_test.go` | Infrastructure (test) | 変更 | #2: 引数の追加。旧形式の一時ファイルの検証と `isolateTempDir` を消す。失敗を記録するテストと、長さが不正なシークレットのテストを足す |
| `internal/integration/http_test.go` | Integration (test) | 変更 | #2: 引数の追加。`TestHTTP_StaleResponseFilesSweptOnRestart` の「注意: … 旧形式の一時ファイルも消す」のコメントを消す。#3: `-1` を定数に |
| `internal/domain/slices.go` | Domain | 変更 | #3: `PositionEnd = -1` を `InsertAt` の隣に足す |
| `internal/domain/http/types_test.go` | Domain (test) | 変更 | #3: `-1` を定数に |
| `internal/application/http/collection_service_test.go` | Application (test) | 変更 | #3: `-1` を定数に |
| `internal/application/openapi/file_service_test.go` | Application (test) | 変更 | #3: `-1` を定数に |
| `internal/adapters/http_handler.go`、`internal/application/http/collection_service.go` | Adapters / Application | 変更 | #3: doc コメントの「負または範囲外なら末尾」に定数名を添える（コードは変えない） |
| `frontend/src/application/http/collections.ts` | Frontend application | 変更 | #3: `APPEND_POSITION = -1` を `CollectionsApi` の隣に export する |
| `frontend/src/presentation/components/sidebar/use-tree-drag-drop.ts` | Frontend presentation | 変更 | #3: コレクション見出しへのドロップの `-1` 2 か所（`handleMouseMove` の `setDropTarget` と `handleMouseUp` の `deps.onMoveItem`）を `APPEND_POSITION` に。DOM 属性が無いことを表す `-1` は別の定数 `NO_POSITION` に分ける |
| `frontend/e2e/fake-backend/install.ts` | e2e | 変更 | #3: `MoveSidebarEntry` と `MoveItemToSidebar` の負の position を Go と同じく末尾にする（下の「副作用」を参照）。#4: トピックの上限のメッセージを `MAX_TOPIC_BYTES` から組み立てる |
| `frontend/e2e/ui/http/tree-drag-drop.spec.ts` | e2e | 変更 | #3: 偽バックエンドの `MoveSidebarEntry` と `MoveItemToSidebar` に負の position を渡すと末尾に入ることを確かめる回帰テストを足す |
| `internal/domain/id.go` | Domain | 変更 | #4: メッセージを `maxIDLength` から組み立てる |
| `internal/domain/mqtt/topic.go` | Domain | 変更 | #4: メッセージを `maxTopicBytes` から組み立てる |
| `internal/domain/id_test.go`、`internal/domain/mqtt/topic_test.go` | Domain (test) | 変更 | #4: メッセージの文言を固定するテストを足す。`id_test.go` の `128` を `maxIDLength` に、`129` を `maxIDLength+1` に |
| `internal/domain/errors.go` | Domain | 変更 | #5: `ErrShuttingDown` を足す |
| `internal/application/http/request_service.go` | Application | 変更 | #5: 自前の `errShuttingDown` を消して `cmn.ErrShuttingDown` を返す |
| `internal/application/mqtt/service.go`、`topic_scan.go` | Application | 変更 | #5: 同上 |
| `internal/application/mqtt/service_test.go`、`internal/application/http/request_service_test.go` | Application (test) | 変更 | #5: `errors.Is(err, cmn.ErrShuttingDown)` に。HTTP 側は終了後の送信のテストが sentinel を見ていなければ足す |

## 実装方針

### 1. `collection_repository.go` の分割

- 先に旧形式の移行を消し、そのあとでファイルを分ける（コミットも分ける）。分割のコミットが行の移動だけになる。
- 消すのは、旧形式のパスを basename へ変換して引き継ぐ処理だけ。
  - `storedFormRow` の `legacyFilePath` フィールドと `UnmarshalJSON`（旧 `"filePath"` キーの読み込み）
  - `storedRequestBody.toDomain` が `Contents["file"]` を `legacyPath` として取り出す処理と、`fromStoredFileRef` の `legacyPath` 引数
  - `legacyBaseName`
- token と実パスを永続化しない・RPC に出さない処理は残す。
  - `toStoredBody` の `delete(contents, domain.BodyTypeFile)` と `toStoredFileRef`（保存時）
  - `storedRequestBody.toDomain` の `delete(contents, domain.BodyTypeFile)`（読み込み時）。手で書き換えたファイルに残ったパスを RPC へ出さないため。basename へは変換せず、捨てるだけにする。
- 旧 `"filePath"` キーは、`UnmarshalJSON` を消すと `encoding/json` が未知のキーとして読み飛ばす。旧形式のファイルを持つ行は、ファイル未選択の行として読み込まれる。
- 同じパッケージ（`httpinfra`）の中でファイルを分けるだけにする。残す型名・関数名・シグネチャは変えない（`fromStoredFileRef` の引数だけ減る）。
- 分け方は「リポジトリの操作」と「保存形式」の 2 つ。DTO と変換関数をさらに分けると、フィールドを足すときに 3 ファイルを行き来することになるので分けない。
- 他の 4 つのリポジトリ（`sidebar_layout_repository.go` など）は 60〜90 行で区切りコメントも無いので、そのままにする。
- AGENTS.md は保存形式の持ち主を「`http/collection_repository.go` などの `storedXxx` DTO」と書いている。ファイル名の列挙に `http/stored_collection.go` を足す。
- この計画では、フォームの旧データ（`NormalizeForms`、`Kind` が空の行、`form_migration_test.go`）には触れない。domain と application の振る舞いで、コメント見直しの対象外。

### 2. `SweepStaleTempFiles` の整理

- **旧形式の sweep を消す。** `os.TempDir()` 直下の `wirexa-response-*` を消す処理と `legacyResponseFilePrefix` を消す。この接頭辞のファイルを作るコードはもう無い（使っているのは sweep の glob だけ）。これで sweep の対象は引数の `baseDir` だけになり、統合テストが実環境の一時ディレクトリを触らなくなる。
- **`sessionSecret` で長さを確かめ直す。** いまは、シークレットファイルが存在して長さが不正（空など）だと、`O_EXCL` が `ErrExist` で失敗し、読み直した内容を長さを見ずに返す。空のシークレットが `nil` エラーで返り、`isWirexaSessionDir` が常に false になるので、sweep は何も消さず、失敗としても記録されない。`reserveSpill` も同じ空のシークレットを marker に書く。
  - 読み直した内容が `sessionSecretSize` でなければ、新しいシークレットで上書きして返す。
  - 上書きの前に作られた session directory は marker が一致しないので残る（いまも残っている）。
- **logger を渡す。** `logger` は `cmn.Logger` で受ける。infrastructure → domain の依存で、既存の `NewCollectionRepository(dir, logger)` と同じ向き。
- nil を許容する（`store.LoadSingleFile`・`JSONStore` と同じ扱い）。nil なら記録しない。
- 記録するのは次の 2 つ。どちらも失敗しても sweep は続ける（振る舞いは今と同じ）。
  - シークレットを読み書きできず、session directory の sweep を諦めたとき
  - session directory の削除に失敗したとき
- **ログにパスを出さない。** `os` のエラーは `*fs.PathError` でパスを含むので、`errors.As` で取り出した `Err`（`access is denied` など）だけを記録する。対象は basename で示す。`docs/http-local-file-access-hardening.md` の方針（エラーとログに実パス・一時ファイルのパスを出さない）に合わせる。
- `app.go` の配線は引数を 1 つ足すだけ。logger は sweep より前に作っているので、順序は変えない。

### 3. 「末尾に追加」を表す定数

`-1` は 2 つの場所で別々に使われているので、それぞれに名前を付ける。値は RPC の引数（`int`）として運ぶだけで、Go の型は変えない。

- **Go**: `internal/domain/slices.go` に `PositionEnd = -1` を足す。`InsertAt` は今も「負または範囲外なら末尾」で、この契約は変えない。`PositionEnd` は「末尾を指すときに呼び出し側が渡す値」の名前で、非テストコードには渡す箇所が無い（渡すのはフロントエンドとテストだけ）。テスト 14 か所の `-1` を置き換える。
- **フロントエンド**: `application/http/collections.ts` に `APPEND_POSITION = -1` を export する。`moveItem` の `position` の意味を決めているのがこのファイルの `CollectionsApi` なので、ここに置く。`use-tree-drag-drop.ts` は presentation → application の import になり、依存方向を守る。
- `use-tree-drag-drop.ts` で `APPEND_POSITION` に置き換えるのは 2 か所。どちらもコレクション見出しへのドロップ。
  - `handleMouseMove` の `setDropTarget({ …, position: -1 })`
  - `handleMouseUp` の `deps.onMoveItem(di.collectionId, di.itemId, collectionId, "", -1)`
- 同じファイルには、同じ `-1` が「`DROP_POSITION_ATTR` が無い（ドロップ先ではない）」の意味でも 4 か所ある（`?? "-1"` と `=== -1` / `!== -1` が 2 組）。これは末尾追加とは別の意味なので、ファイル内の定数 `NO_POSITION` に分ける。`TREE_ITEM_INDEX_ATTR` の既定値 `"-1"` 2 か所は index の既定値で、どちらの意味でもないので触らない。
- `e2e/ui/http/tree-drag-drop.spec.ts` の期待値 `[ALPHA.id, REQUEST.id, BETA.id, "", -1]` は、RPC に渡った値そのものを確かめているので `-1` のままにする。
- Go の定数を TS へ生成する仕組みは足さない（イベント名と違い、値が 1 つで変わる見込みも無い）。両側の定数の doc コメントに、相手の定数名を書く。

### 4. 上限のメッセージを定数から組み立てる

- `fmt.Sprintf("must be at most %d characters", maxIDLength)` と `fmt.Sprintf("must be at most %d bytes", maxTopicBytes)` にする。出力される文言は今と同じ。
- `frontend/e2e/fake-backend/install.ts` も同じ文言 `"must be at most 65535 bytes"` を直書きしている。すぐ上に `MAX_TOPIC_BYTES` があるので、`` `must be at most ${MAX_TOPIC_BYTES} bytes` `` にする。Go の定数との一致は、今と同じく手で保つ。
- `id_test.go` は上限ちょうどの `128` と超過の `129` を直書きしている。`maxIDLength` と `maxIDLength+1` にする。

### 5. `ErrShuttingDown` を domain に置く

- `internal/domain/errors.go` に `var ErrShuttingDown = errors.New("application is shutting down")` を足す。文言は今と同じなので、フロントエンドに届くエラー文字列は変わらない。
- application の 2 パッケージは domain だけに依存しているので、依存方向は変わらない。`httpapp` と `mqttapp` の間に依存は作らない。
- 2 つの sentinel が 1 つになるので、`errors.Is` で HTTP と MQTT の終了エラーを区別できなくなる。区別している箇所は無い（grep で確認済み）。

## 永続化への影響

書き出す形式は変わらない。#1 で `storedXxx` DTO のフィールド・json タグは変えず、`testdata/collection.golden.json` も変えない。golden テストと `*_RoundTripKeepsEveryField` がそのまま通ることで確かめる。

読み込みは 1 点だけ変わる。旧形式のファイルパス（`Contents["file"]` と行の `"filePath"`）を basename と再選択待ちへ移行しなくなる。旧形式のファイルを読むと、パスは捨てられ、そのボディと行はファイル未選択になる。

## コード生成

不要。バインド対象の構造体・メソッドのシグネチャも、イベントも変えない。

- [ ] `task wails:generate`（不要。念のため実行して `git diff --exit-code frontend/wailsjs` が空であることだけ確かめる）
- [ ] `task go:generate:events`（不要）

## テスト方針

- **Go ユニット**
  - #1: 旧形式の移行のテスト 2 つを消し、1 つを直す（「変更対象ファイル」を参照）。残りの `collection_repository_test.go`（往復・golden・`AssertNoTypesFrom`・token を永続化しないこと）が通ること。
  - #2: `TestSweepStaleTempFiles` から旧形式の一時ファイルの検証を消す。削除できない session directory があるとき logger に 1 件記録され、記録にパスが含まれないことを足す。logger が nil でも panic しないことを足す。シークレットファイルが空のとき、`sessionSecret` が `sessionSecretSize` バイトのシークレットを返してファイルを上書きすることと、そのあとに作った session directory が sweep で消えることを足す。
  - #3: 追加なし。置き換えた定数で既存のテストが通ること。
  - #4: `ValidateID` と `ValidateTopicName` の長さ超過で、メッセージが `must be at most 128 characters` / `must be at most 65535 bytes` であることを固定する。
  - #5: 既存の `errors.Is(err, errShuttingDown)` を `cmn.ErrShuttingDown` に変える。HTTP 側は `Shutdown` 後の `SendRequest` が `cmn.ErrShuttingDown` を返すことを確かめる。
- **Go 統合**: #2 と #3 の置き換えだけ。新しいテストは足さない。
- **フロント ユニット**: 追加なし（`use-tree-drag-drop.ts` にユニットテストは無い）。`task frontend:tsc` と biome が通ること。
- **UI e2e**: `e2e/ui/http/tree-drag-drop.spec.ts` の既存のテストがそのまま通ること。既存のテストがサイドバーへ渡す position は `0` と `2` だけで、fake backend を直す前でも通る。負の position で末尾に入ることを確かめる回帰テストを同じ spec に足す。UI は負の position をサイドバーへ送らないので、`page.evaluate` で `window.go` の `MoveSidebarEntry` と `MoveItemToSidebar` を直接呼び、`fake.snapshot()` の `sidebar` の末尾を確かめる。
- **フルスタック e2e**: 対象外。

## 副作用・注意事項

- **fake backend と Go の食い違い（#3 を調べる途中で見つけた）**: `MoveSidebarEntry` と `MoveItemToSidebar` に負の position を渡すと、Go は末尾へ入れ（`cmn.InsertAt`）、fake backend は先頭へ入れる（どちらも `Math.max(0, …)`）。UI は負の position をサイドバーへ送らないので、いま実害は無い。Go に合わせて fake backend の両方を直す。これは振る舞いの変更なので、コミットを分ける。
- #1 で、旧形式のファイルパスを持つコレクションは、読み込むとファイルの basename が消える（上の「永続化への影響」）。
- #2 で起動時のログが増える。増えるのは sweep が失敗したときだけで、通常の起動では出ない。
- #2 で、`os.TempDir()` 直下に残っている `wirexa-response-*` は消されなくなる。
- #5 でエラーの同一性が変わる（上記）。文言は変わらない。
- #1 は行の移動なので、`git blame` が新しいファイルでは途切れる。`git log --follow` では追えない（1 ファイルから 2 ファイルへの分割のため）。
- どの項目も互いに独立している。一部だけ見送っても、残りはそのまま進められる。

## Git運用

- **ブランチ名**: `refactor/backend-comment-review-followups`
- **コミット分割方針**: 項目ごとに 1 コミット。振る舞いが変わるものは分ける。テストはそれぞれの変更と同じコミットに入れる。
  1. `refactor(http): コレクションの旧形式のファイルパスの移行をやめる`
  2. `refactor(http): コレクションの永続化 DTO と変換関数を stored_collection.go に分ける`（AGENTS.md の更新を含む）
  3. `refactor(http): 旧形式の一時ファイルの sweep をやめる`
  4. `fix(http): 長さが不正なセッションシークレットを作り直す`
  5. `fix(http): 起動時 sweep の失敗をログに記録する`
  6. `refactor(http): 末尾への追加を表す position に名前を付ける`（Go とフロントエンド）
  7. `fix(e2e): fake backend のサイドバーへの移動で負の position を末尾として扱う`（回帰テストを含む）
  8. `refactor(domain): ID とトピックの上限のメッセージを定数から組み立てる`（fake backend の文言を含む）
  9. `refactor(domain): 終了処理中のエラーを ErrShuttingDown にまとめる`
  10. `docs: 実装済みの変更計画書を削除する`（この計画書。これまでの運用に合わせる）
- **完了条件**: すべて通ってから main へマージする。
  - `task format` → `task lint` → `task test`
  - `task go:test:integration`（#2 と #3 が `internal/integration/` に触れる）
  - `task frontend:test:e2e`（#3 で `use-tree-drag-drop.ts` と fake backend を変える）
  - `task go:test:race` は不要（並行処理には触れない）。CI では常に -race で走る。
