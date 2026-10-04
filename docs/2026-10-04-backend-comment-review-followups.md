# 変更計画書: コメント見直しで見つかったバックエンドの小さな整理

## 概要

`/comment-review backend` で、コメントではなくコードを直すべきものが 5 件見つかった。どれも「コメントで説明している内容を、名前・構造・定数で表す」変更で、RPC の形・保存形式・イベントは変えない。

| # | 対象 | いまの問題 | 変更 |
| --- | --- | --- | --- |
| 1 | `collection_repository.go` | 397 行のうち 330 行が永続化 DTO と変換関数で、`// ── 永続化 DTO ──` などの区切りコメント 3 つで区切っている | DTO と変換関数を別ファイルへ移す |
| 2 | `SweepStaleTempFiles` | 削除の失敗を黙って捨てる。呼び出し時点で logger は存在する | logger を引数で受けて記録する |
| 3 | `position` の `-1` | 「末尾に追加」を裸の `-1` で表している（フロントエンド 1 か所、Go テスト 14 か所） | 名前付き定数にする |
| 4 | `ValidateID`・`validateTopicString` | エラーメッセージに `128`・`65535` を直書きしていて、`maxIDLength`・`maxTopicBytes` を変えると食い違う | メッセージを定数から組み立てる |
| 5 | `errShuttingDown` | 同じ文言の sentinel が `httpapp` と `mqttapp` に 1 つずつある | `domain.ErrShuttingDown` に 1 つにまとめる |

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `internal/infrastructure/http/collection_repository.go` | Infrastructure | 変更 | #1: `CollectionRepository` と `New`・`Load`・`Save`・`Delete`・`Exists` だけを残す |
| `internal/infrastructure/http/stored_collection.go` | Infrastructure | 新規 | #1: `storedXxx` DTO、`toStoredXxx` / `fromStoredXxx` / `toDomain`、`legacyBaseName` を移す。区切りコメントは消す |
| `internal/infrastructure/http/response_store.go` | Infrastructure | 変更 | #2: `SweepStaleTempFiles(baseDir string, logger cmn.Logger)` にする |
| `app.go` | 合成ルート | 変更 | #2: `SweepStaleTempFiles(sessionDir, logger)` |
| `internal/infrastructure/http/net_client_cleanup_test.go` | Infrastructure (test) | 変更 | #2: 引数の追加と、失敗を記録するテスト |
| `internal/integration/http_test.go` | Integration (test) | 変更 | #2: 引数の追加。#3: `-1` を定数に |
| `internal/domain/slices.go` | Domain | 変更 | #3: `PositionEnd = -1` を `InsertAt` の隣に足す |
| `internal/domain/http/types_test.go` | Domain (test) | 変更 | #3: `-1` を定数に |
| `internal/application/http/collection_service_test.go` | Application (test) | 変更 | #3: `-1` を定数に |
| `internal/application/openapi/file_service_test.go` | Application (test) | 変更 | #3: `-1` を定数に |
| `internal/infrastructure/http/collection_repository_test.go` | Infrastructure (test) | 変更 | #3: `-1` を定数に |
| `internal/adapters/http_handler.go`、`internal/application/http/collection_service.go` | Adapters / Application | 変更 | #3: doc コメントの「負または範囲外なら末尾」に定数名を添える（コードは変えない） |
| `frontend/src/application/http/collections.ts` | Frontend application | 変更 | #3: `APPEND_POSITION = -1` を `CollectionsApi` の隣に export する |
| `frontend/src/presentation/components/sidebar/use-tree-drag-drop.ts` | Frontend presentation | 変更 | #3: コレクション見出しへのドロップの `-1` を `APPEND_POSITION` に。DOM 属性が無いことを表す `-1` は別の定数 `NO_POSITION` に分ける |
| `frontend/e2e/fake-backend/install.ts` | e2e | 変更 | #3: `MoveSidebarEntry` の負の position を Go と同じく末尾にする（下の「副作用」を参照） |
| `internal/domain/id.go` | Domain | 変更 | #4: メッセージを `maxIDLength` から組み立てる |
| `internal/domain/mqtt/topic.go` | Domain | 変更 | #4: メッセージを `maxTopicBytes` から組み立てる |
| `internal/domain/id_test.go`、`internal/domain/mqtt/topic_test.go` | Domain (test) | 変更 | #4: メッセージの文言を固定するテストを足す。`id_test.go` の `128` を `maxIDLength` に |
| `internal/domain/errors.go` | Domain | 変更 | #5: `ErrShuttingDown` を足す |
| `internal/application/http/request_service.go` | Application | 変更 | #5: 自前の `errShuttingDown` を消して `cmn.ErrShuttingDown` を返す |
| `internal/application/mqtt/service.go`、`topic_scan.go` | Application | 変更 | #5: 同上 |
| `internal/application/mqtt/service_test.go`、`internal/application/http/request_service_test.go` | Application (test) | 変更 | #5: `errors.Is(err, cmn.ErrShuttingDown)` に。HTTP 側は終了後の送信のテストが sentinel を見ていなければ足す |

## 実装方針

### 1. `collection_repository.go` の分割

- 同じパッケージ（`httpinfra`）の中でファイルを分けるだけにする。型名・関数名・シグネチャは変えない。
- 分け方は「リポジトリの操作」と「保存形式」の 2 つ。DTO と変換関数をさらに分けると、フィールドを足すときに 3 ファイルを行き来することになるので分けない。
- 他の 4 つのリポジトリ（`sidebar_layout_repository.go` など）は 60〜90 行で区切りコメントも無いので、そのままにする。
- AGENTS.md は保存形式の持ち主を「`http/collection_repository.go` などの `storedXxx` DTO」と書いている。ファイル名の列挙に `http/stored_collection.go` を足す。

### 2. `SweepStaleTempFiles` に logger を渡す

- `logger` は `cmn.Logger` で受ける。infrastructure → domain の依存で、既存の `NewCollectionRepository(dir, logger)` と同じ向き。
- nil を許容する（`store.LoadSingleFile`・`JSONStore` と同じ扱い）。nil なら記録しない。
- 記録するのは次の 3 つ。どれも失敗しても sweep は続ける（振る舞いは今と同じ）。
  - シークレットを読み書きできず、session directory の sweep を諦めたとき
  - session directory の削除に失敗したとき
  - 旧形式の一時ファイルの削除に失敗したとき
- **ログにパスを出さない。** `os` のエラーは `*fs.PathError` でパスを含むので、`errors.As` で取り出した `Err`（`access is denied` など）だけを記録する。対象は basename で示す。`docs/http-local-file-access-hardening.md` の方針（エラーとログに実パス・一時ファイルのパスを出さない）に合わせる。
- `app.go` の配線は引数を 1 つ足すだけ。logger は sweep より前に作っているので、順序は変えない。

### 3. 「末尾に追加」を表す定数

`-1` は 2 つの場所で別々に使われているので、それぞれに名前を付ける。値は RPC の引数（`int`）として運ぶだけで、Go の型は変えない。

- **Go**: `internal/domain/slices.go` に `PositionEnd = -1` を足す。`InsertAt` は今も「負または範囲外なら末尾」で、この契約は変えない。`PositionEnd` は「末尾を指すときに呼び出し側が渡す値」の名前で、非テストコードには渡す箇所が無い（渡すのはフロントエンドとテストだけ）。テスト 14 か所の `-1` を置き換える。
- **フロントエンド**: `application/http/collections.ts` に `APPEND_POSITION = -1` を export する。`moveItem` の `position` の意味を決めているのがこのファイルの `CollectionsApi` なので、ここに置く。`use-tree-drag-drop.ts` は presentation → application の import になり、依存方向を守る。
- `use-tree-drag-drop.ts` には、同じ `-1` が「DOM 属性に position が無い（ドロップ先ではない）」の意味でも 4 か所ある。これは末尾追加とは別の意味なので、ファイル内の定数 `NO_POSITION` に分ける。
- Go の定数を TS へ生成する仕組みは足さない（イベント名と違い、値が 1 つで変わる見込みも無い）。両側の定数の doc コメントに、相手の定数名を書く。

### 4. 上限のメッセージを定数から組み立てる

- `fmt.Sprintf("must be at most %d characters", maxIDLength)` と `fmt.Sprintf("must be at most %d bytes", maxTopicBytes)` にする。出力される文言は今と同じ。
- `frontend/e2e/fake-backend/install.ts:632` は同じ文言 `"must be at most 65535 bytes"` を直書きしている。fake backend は Go を import できないので、そのままにする。

### 5. `ErrShuttingDown` を domain に置く

- `internal/domain/errors.go` に `var ErrShuttingDown = errors.New("application is shutting down")` を足す。文言は今と同じなので、フロントエンドに届くエラー文字列は変わらない。
- application の 2 パッケージは domain だけに依存しているので、依存方向は変わらない。`httpapp` と `mqttapp` の間に依存は作らない。
- 2 つの sentinel が 1 つになるので、`errors.Is` で HTTP と MQTT の終了エラーを区別できなくなる。区別している箇所は無い（grep で確認済み）。

## 永続化への影響

なし。#1 は `storedXxx` DTO と変換関数をファイル間で移すだけで、フィールド・json タグ・変換の内容は変えない。`testdata/collection.golden.json` も変えない。golden テストと `*_RoundTripKeepsEveryField` がそのまま通ることで確かめる。

## コード生成

不要。バインド対象の構造体・メソッドのシグネチャも、イベントも変えない。

- [ ] `task wails:generate`（不要。念のため実行して `git diff --exit-code frontend/wailsjs` が空であることだけ確かめる）
- [ ] `task go:generate:events`（不要）

## テスト方針

- **Go ユニット**
  - #1: 追加なし。既存の `collection_repository_test.go`（往復・golden・`AssertNoTypesFrom`・旧形式の移行）が通ること。
  - #2: `TestSweepStaleTempFiles` に、削除できない session directory があるとき logger に 1 件記録され、記録にパスが含まれないことを足す。logger が nil でも panic しないことを足す。
  - #3: 追加なし。置き換えた定数で既存のテストが通ること。
  - #4: `ValidateID` と `ValidateTopicName` の長さ超過で、メッセージが `must be at most 128 characters` / `must be at most 65535 bytes` であることを固定する。
  - #5: 既存の `errors.Is(err, errShuttingDown)` を `cmn.ErrShuttingDown` に変える。HTTP 側は `Shutdown` 後の `SendRequest` が `cmn.ErrShuttingDown` を返すことを確かめる。
- **Go 統合**: #2 と #3 の置き換えだけ。新しいテストは足さない。
- **フロント ユニット**: 追加なし（`use-tree-drag-drop.ts` にユニットテストは無い）。`task frontend:tsc` と biome が通ること。
- **UI e2e**: `e2e/ui/http/tree-drag-drop.spec.ts` がそのまま通ること。fake backend の `MoveSidebarEntry` を直すので、サイドバーの並び替えの spec も確かめる。
- **フルスタック e2e**: 対象外。

## 副作用・注意事項

- **fake backend と Go の食い違い（#3 を調べる途中で見つけた）**: `MoveSidebarEntry` に負の position を渡すと、Go は末尾へ移し（`cmn.InsertAt`）、fake backend は先頭へ移す（`Math.max(0, …)`）。UI は負の position をサイドバーへ送らないので、いま実害は無い。Go に合わせて fake backend を直す。これだけは振る舞いの変更なので、コミットを分ける。
- #2 で起動時のログが増える。増えるのは sweep が失敗したときだけで、通常の起動では出ない。
- #5 でエラーの同一性が変わる（上記）。文言は変わらない。
- #1 は行の移動なので、`git blame` が新しいファイルでは途切れる。`git log --follow` では追えない（1 ファイルから 2 ファイルへの分割のため）。
- どの項目も互いに独立している。一部だけ見送っても、残りはそのまま進められる。

## Git運用

- **ブランチ名**: `refactor/backend-comment-review-followups`
- **コミット分割方針**: 項目ごとに 1 コミット。テストはそれぞれの変更と同じコミットに入れる。
  1. `refactor(http): コレクションの永続化 DTO と変換関数を stored_collection.go に分ける`（AGENTS.md の更新を含む）
  2. `fix(http): 起動時 sweep の失敗をログに記録する`
  3. `refactor(http): 末尾への追加を表す position に名前を付ける`（Go とフロントエンド）
  4. `fix(e2e): fake backend の MoveSidebarEntry で負の position を末尾として扱う`
  5. `refactor(domain): ID とトピックの上限のメッセージを定数から組み立てる`
  6. `refactor(domain): 終了処理中のエラーを ErrShuttingDown にまとめる`
  7. `docs: 実装済みの変更計画書を削除する`（この計画書。これまでの運用に合わせる）
- **完了条件**: すべて通ってから main へマージする。
  - `task format` → `task lint` → `task test`
  - `task go:test:integration`（#2 と #3 が `internal/integration/` に触れる）
  - `task frontend:test:e2e`（#3 で `use-tree-drag-drop.ts` と fake backend を変える）
  - `task go:test:race` は不要（並行処理には触れない）。CI では常に -race で走る。
