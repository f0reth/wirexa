# 変更計画書: コレクションキャッシュの返却値・入力値をキャッシュから切り離す

対象: [backend-architecture-review.md](./backend-architecture-review.md) の「6. コレクションキャッシュの可変参照がロック外へ公開される」

ステータス: **計画（未実装）**。本書は変更の計画であり、本書を追加しただけでは問題は解消しない。
以下の「本変更で達成される状態」は実装完了後の状態で、完了の判定は「完了条件」の節による。

## 概要

`CollectionService` はコレクションをメモリ上のキャッシュ（`map[string]*domain.Collection`）に保持し、
`mu` で保護している。しかし読み取り系の戻り値がキャッシュ内部のポインターを共有しているため、
呼び出し側（adapter、Wails の JSON 化、テスト、将来の別ユースケース）がロック外でキャッシュに触れられる。

### レビュー執筆後の状況（#3 の copy-on-write 導入後）

レビュー #3 の対応で、変更系メソッドは「キャッシュ上のコレクションを `Clone` し、コピーを変更し、
永続化成功後にエントリごと差し替える」copy-on-write になった。そのため **サービス自身は公開済みの
コレクションを書き換えない**。レビューが挙げた「JSON 化中に別 RPC が更新してデータ競合」は、
現在のサービスのコードだけを見れば起きない。

それでも以下が残っている。本変更はこれらを解消する。

1. **戻り値経由でキャッシュを書き換えられる**
   - `GetCollections` は `*c` の浅いコピー。`Items` 以下（`[]*TreeItem`、`*HTTPRequest`、`Headers` / `Params` / `FormData` / `FormURLEncoded` スライス、`Contents` map）はキャッシュと共有（`collection_service.go:142`）。
   - `GetRootItems` はキャッシュ内の `root.Items` をそのまま返す（`collection_service.go:158`）。
   - `AddFolder` / `AddRequest` は、キャッシュへ挿入した `*TreeItem` そのものを返す（`collection_service.go:241,260`）。
   - `CreateCollection` は `*c` を返し、`Items` スライスをキャッシュと共有する（`collection_service.go:184`）。
2. **入力値がキャッシュと共有される**
   - `AddRequest` / `UpdateRequest` は引数 `req` の値コピーをそのままキャッシュへ入れる（`collection_service.go:254,310`）。値コピーでも `Headers` などのスライスと `Contents` map は呼び出し側と共有される。
   - さらに `req.Body.DropFileContents()` は共有された `Contents` map から削除するため、**呼び出し側の map を書き換えている**。
3. **「公開済みエントリは不変」という前提が暗黙**
   - copy-on-write が並行安全性の根拠になっているのに、`cache` フィールドにその不変条件が書かれていない。
     将来誰かがキャッシュを直接書き換えると、ロック外で読まれている値と競合する。
4. **race test が実質動いていない**
   - 既存の `TestCollectionService_ConcurrentReadWrite` / `TestCollectionService_ConcurrentUpdateWithSaveFailures` は
     「CI は -race 付きで実行する」とコメントしているが、CI（`.github/workflows/ci.yml:79`）も `task go:test` も
     `-race` を付けていない。また戻り値を走査・JSON 化していないため、仮に `-race` で動かしても戻り値経由の共有を検出しない。

### 本変更で達成される状態

- `CollectionService` の公開メソッドが返す値は、キャッシュと可変状態を一切共有しない（ディープコピー）。
- `CollectionService` が引数で受け取った値は、ディープコピーしてから取り込む。呼び出し側の引数も書き換えない。
- 「キャッシュに載ったコレクションは公開後に変更しない」という不変条件をコードコメントで明文化する。
- 戻り値の JSON 化・戻り値の書き換え・更新を並行に走らせる race test を追加し、CI で `-race` 付きで実行する。

### 計画時点の事前確認（2026-09-23）

- 上記の行番号・挙動は `a64cbd2` 時点の `collection_service.go` と一致することを確認済み。
- `domain.Collection` / `TreeItem` / `HTTPRequest` / `RequestBody` の `Clone` は、参照型のフィールド（`Items`・`Children`・`Request`・`Headers`・`Params`・`Contents`・`FormData`・`FormURLEncoded`）をすべて複製する。`RequestAuth` / `RequestSettings` / `FileReference` / `KeyValuePair` / `FormRow` は値型のフィールドしか持たないため、既存の `Clone` で完全なディープコピーになる。
- `CollectionService` のキャッシュを読むのは本サービスだけ（`request_service.go` はキャッシュを参照しない）。adapter（`http_handler.go`）は戻り値を素通しで返す。
- Windows ローカル（scoop の gcc、`CGO_ENABLED=1`）で、変更前のコードに対して `go test -race ./...` が全パッケージ通ることを確認済み。したがって CI に `-race` を入れても、既存テストが原因で赤くなる見込みは低い。

### 本変更の対象外

- `CachedStore[T]`（MQTT profile / UDP target）の `GetAll` も浅いコピーを返す同種の問題がある。本計画では扱わず、別途提案とする（「副作用・注意事項」参照）。
- ポート・RPC のシグネチャ変更（`[]*TreeItem` → `[]TreeItem` など）は行わない。ディープコピーで同じ保証が得られ、生成物・フロント・偽バックエンドに波及させずに済むため。

## 変更対象ファイル

### バックエンド

| ファイル                                         | 層          | 変更種別 | 変更内容 |
| ------------------------------------------------ | ----------- | -------- | -------- |
| `internal/application/http/collection_service.go` | Application | 変更     | `GetCollections` は各コレクションを `Clone` して返す。`GetRootItems` は `root.Clone().Items` を返す。`CreateCollection` は `*c.Clone()` を返す。`AddRequest` / `UpdateRequest` は引数 `req` を `Clone` してから `DropFileContents` とキャッシュへの取り込みを行う。`AddFolder` / `AddRequest` はキャッシュへ挿入したノードではなく、その `Clone` を返す。`cache` フィールドと `snapshotForLayout` に「公開済みエントリは不変」の不変条件をコメントで明記する |

ドメイン型（`internal/domain/http/types.go`）は変更しない。既存の `Collection.Clone` / `TreeItem.Clone` / `HTTPRequest.Clone` がディープコピーを提供済みで、そのまま使える。

### バックエンドのテスト

| ファイル                                              | 系統        | 変更種別   | 変更内容 |
| ----------------------------------------------------- | ----------- | ---------- | -------- |
| `internal/application/http/collection_service_test.go` | Go ユニット | 変更・追加 | 戻り値の書き換えがキャッシュへ波及しないテスト、引数の書き換えがキャッシュへ波及しない・引数を書き換えないテスト、JSON 化と更新と戻り値の書き換えを並行に走らせる race test を追加。既存の concurrent test のコメントを実態に合わせる |

### CI・タスク・ドキュメント

| ファイル                   | 種別 | 変更内容 |
| -------------------------- | ---- | -------- |
| `.github/workflows/ci.yml` | CI   | `test-go-unit` の `go test ./...` を `go test -race ./...` にする（ubuntu ランナーは gcc があり cgo が有効） |
| `Taskfile.yml`             | タスク | `go:test:race`（`go test -race ./...`）を追加する。`go:test` は変更しない（Windows では `-race` に cgo/gcc が必要なため、既定のローカル実行の前提を増やさない） |
| `CLAUDE.md`                | ドキュメント | Go のコマンド一覧に `task go:test:race` を追記する |

### フロントエンド・生成物・偽バックエンド

変更なし。RPC のシグネチャも戻り値の内容（JSON）も変わらないため、`frontend/wailsjs/`、`frontend/src/`、`frontend/e2e/fake-backend/` はいずれも追従不要。
偽バックエンドは状態を sessionStorage に JSON で持ち、もともと毎回コピーを返しているため、意味論の差も無い。

## 実装方針

### 1. 「入口で複製、出口で複製、中は不変」

`CollectionService` の境界で所有権を切る。

- **出口**: 公開メソッドの戻り値は、ロック保持中に `Clone` したものを返す。
  - `GetCollections`: `result = append(result, *c.Clone())`
  - `GetRootItems`: `root.Clone().Items`（`cloneItems` は nil と空スライスを区別して保つので、`null` / `[]` の JSON 表現は現状と変わらない）。ルート全体を複製するのが無駄なら `domain` に `CloneItems` を足す選択肢もあるが、ルート 1 件の `Collection` ヘッダのコピーは無視できるため、domain の公開 API を増やさない方を採る。
  - `CreateCollection`: `return *c.Clone(), nil`
  - `AddFolder` / `AddRequest`: `addItem` 成功後に `item.Clone()` を返す。キャッシュに入ったノードの所有権はサービスに残す。
- **入口**: 引数で受けた可変データは取り込む前に複製する。
  - `AddRequest`: `r := req.Clone()`（`*HTTPRequest`）を作り、`r.ID` の採番と `r.Body.DropFileContents()` はコピーに対して行い、`Request: r` とする。`Name` も `r.Name` を使う。
  - `UpdateRequest`: 同様に `r := req.Clone()` に対して `Name` の上書きと `DropFileContents` を行い、`node.Request = r` とする。
  - これにより呼び出し側の `req.Body.Contents` から `file` キーが消える副作用もなくなる。
- **中**: キャッシュ上のコレクションは公開後に変更しない（既に #3 で成り立っている）。これを `cache` フィールドのコメントで不変条件として明記する。

`Clone` を戻り値ごとに行うため読み取りのたびに全ツリーを複製するが、コレクションは UI で扱う規模（数百〜数千ノード程度）で、変更系は既に毎回 `Clone` している。読み取りの複製コストは許容できる。

### 2. `snapshotForLayout` はポインター共有のまま残す

`snapshotForLayout` はキャッシュ上の `*Collection` / `*TreeItem` をロック外（レイアウト突合）で読む。これはパッケージ内部の経路で、application 層の外には出ない。
突合は `ID` と `Name` を読むだけで、公開済みエントリが不変である限り安全なので、コピーはしない。
代わりに関数コメントへ「公開済みエントリは不変なのでロック外で読んでよい。キャッシュを直接書き換える変更を入れるとこの前提が崩れる」と書き、不変条件の依存箇所を明示する。

（突合の入力を `ID`/`Name` の値スライスへ変える案もあるが、`reconcileSidebarLayout` と `reconcile_test.go` のシグネチャ変更が波及する割に得るものが少ないため採らない。）

### 3. 起動時の in-place 変更について

`NewCollectionService` 内の `normalizeItemForms` と、`recoverDuplicateItems` の差し替えはキャッシュを直接触るが、サービスを返す前（どのゴルーチンにも公開される前）なので不変条件に反しない。その旨をコメントで補う。

### 4. 依存方向

変更は application 層の `CollectionService` の内部と、そのテストに閉じる。ポート（`internal/domain/http/port.go`）、adapter、infrastructure のシグネチャは変えない。依存方向は現状のまま（adapters → domain ← application）。新規サービスの追加や `app.go` の配線変更は無い。

### 5. `-race` を CI で有効にする

- `ci.yml` の Go ユニットテストを `go test -race ./...` にする。ubuntu-latest には gcc があり、`CGO_ENABLED` は既定で 1 になる。
- 統合テストのステップ（`go test -tags integration`）は今回変更しない。
- ローカル用に `task go:test:race` を追加する。`task go:test` / `task test` は変更しない。
- `-race` は実行時間を数倍にし、タイミング依存のテスト（MQTT 組み込みブローカーなど）が揺れる可能性がある。マージ前にローカル（Windows、scoop の gcc）で `task go:test:race` を複数回通し、CI でも通ることを確認する。揺れるテストが見つかった場合は本変更に混ぜず、別途報告する。

## コード生成

不要。

- [ ] `task wails:generate` — 不要（バインド対象の Go 構造体・メソッドのシグネチャは変えない）
- [ ] `task go:generate:events` — 不要（イベント追加なし）

## テスト方針

### Go ユニット（`internal/application/http/collection_service_test.go`）

いずれも現状のコードで失敗し（1〜3 は通常実行で、4 は `-race` で）、修正後に通ることを確認する。

1. **戻り値の書き換えがキャッシュへ波及しない**
   - `TestCollectionService_GetCollections_ReturnsDeepCopy`: フォルダ配下に `Headers` / `Params` / `Contents` / `FormData` を持つリクエストを置いたコレクションを用意し、`GetCollections` の戻り値のあらゆる階層（`Name`、`Items` への append、子の `Name`、`Request.Headers[0].Value`、`Request.Body.Contents` への書き込み、`FormData[0].Value`）を書き換える。その後の `GetCollections` とキャッシュ（`cachedCollection`）が元のままであることを確認する。
   - `TestCollectionService_GetRootItems_ReturnsDeepCopy`: 同様に `__root__` 直下で確認する。
   - `TestCollectionService_AddedItems_AreNotAliased`: `CreateCollection` / `AddFolder` / `AddRequest` の戻り値を書き換えてもキャッシュが変わらないことを確認する。
2. **引数の書き換えがキャッシュへ波及しない**
   - `TestCollectionService_AddRequest_DoesNotRetainArgument` / `TestCollectionService_UpdateRequest_DoesNotRetainArgument`: 呼び出し後に引数 `req` の `Headers[0].Value` と `Body.Contents` を書き換えても、キャッシュが変わらないことを確認する。
3. **引数を書き換えない**
   - `TestCollectionService_AddRequest_DoesNotMutateArgument`: `Contents` に `file` キーを持つ `req` を渡した後も、呼び出し側の map から `file` キーが消えていないことを確認する（キャッシュ側から消えることは既存の `TestCollectionService_DropsFileContents` が確認済み）。
4. **race test**
   - `TestCollectionService_ConcurrentMarshalAndUpdate`: 以下を `sync.WaitGroup` で並行に回す。
     - `json.Marshal(svc.GetCollections())` と `json.Marshal(svc.GetRootItems())` を繰り返す読み手（Wails の JSON 化を模す）
     - `UpdateRequest` / `RenameItem` / `AddRequest` / `MoveItem` を繰り返す書き手
     - `GetCollections` / `GetRootItems` の戻り値を書き換える「行儀の悪い呼び出し側」
   - 修正前は、3 番目の書き換えが他の読み手の JSON 化と同じキャッシュ上のオブジェクトに触れるため `-race` で検出される。修正後は検出されない。copy-on-write が崩れてキャッシュを直接書き換える変更が入った場合も、1 番目と 2 番目の組み合わせで検出できる。
5. **既存テストのコメント修正**
   - `TestCollectionService_ConcurrentReadWrite` / `TestCollectionService_ConcurrentUpdateWithSaveFailures` の「CI は -race 付きで実行する」は、本変更の CI 修正で事実になる。`ConcurrentReadWrite` は戻り値を走査していないため、読み手で戻り値を JSON 化するように強化する。

### Go 統合・フロント ユニット・UI e2e・フルスタック e2e

変更なし。RPC の入出力（JSON）は変わらないため、既存テストがそのまま回帰テストとして働く。偽バックエンド（`frontend/e2e/fake-backend/`）の追従も不要。

### 実行確認

- `task go:test`
- `task go:test:race`（Windows ローカル、複数回）
- `task go:test:integration`
- `task test` / `task lint`

## 完了条件

以下をすべて満たすまで、本変更（レビュー #6 の対応）は完了とみなさない。計画書のコミットだけ、あるいは一部だけが入った状態を「修正済み」として扱わない。

1. **実装**: `collection_service.go` で以下が成り立っている。
   - `GetCollections` が各コレクションを `Clone` して返す（`*c` の浅いコピーを返す箇所が無い）。
   - `GetRootItems` がキャッシュ上の `root.Items` をそのまま返さず、複製を返す。
   - `CreateCollection` / `AddFolder` / `AddRequest` がキャッシュへ入れたオブジェクトではなく、その複製を返す。
   - `AddRequest` / `UpdateRequest` が引数 `req` を `Clone` してから取り込み、呼び出し側の `Contents` map を書き換えない。
   - `cache` フィールドと `snapshotForLayout` に「公開済みエントリは不変」の不変条件がコメントで書かれている。
2. **回帰テストの赤→緑を確認している**: テスト方針 1〜3 のテストが修正前のコードで失敗し、修正後に通ることを、実装中に一度確認している（修正を一時的に戻して `go test` で失敗を確認する）。テスト方針 4 の race test も、修正前のコードで `go test -race` が失敗し、修正後に通ることを確認している。
3. **CI で `-race` が有効**: `.github/workflows/ci.yml` の `test-go-unit` が `go test -race ./...` を実行し、ブランチの CI が通っている。
4. **ローカル確認**: `task format` → `task lint` → `task test` → `task go:test:race`（複数回）→ `task go:test:integration` がすべて通っている。

## 副作用・注意事項

- **読み取りのコスト増**: `GetCollections` / `GetRootItems` が毎回全ツリーをディープコピーする。UI で扱う規模では問題にならない想定だが、極端に大きなコレクションでは RPC 1 回あたりの割り当てが増える。
- **`AddFolder` / `AddRequest` の戻り値が別オブジェクトになる**: 戻り値はキャッシュ上のノードと同じ内容だが、別物になる。戻り値を書き換えてキャッシュへ反映させることを前提にしたコードは無いことを確認済み（adapter はそのまま返すだけで、テストにも書き換えは無い）。
- **呼び出し側の `req.Body.Contents` が書き換えられなくなる**: 現状は `AddRequest` / `UpdateRequest` が呼び出し側の map から `file` キーを削除している。Wails 経由では引数は毎回新しく作られるので影響は無い。Go の呼び出し側でこの削除に依存している箇所も無い。
- **CI の実行時間が延びる**: `-race` で Go ユニットテストの実行時間とメモリが数倍になる。タイミング依存のテストが揺れた場合は別途対処する（本変更には混ぜない）。
- **ローカルの `task go:test` は変わらない**: `-race` を使うには cgo（Windows では gcc）が必要なため、既定のローカル実行には含めない。
- **別途提案: `CachedStore[T]` の浅いコピー**: `internal/application/store/cached_store.go` の `GetAll` / `Save` の戻り値は `T` の値コピーで、MQTT profile や UDP target が持つスライス・map はキャッシュと共有される。`Save` も引数の中身をそのままキャッシュに持つ。同じ方針（型ごとの clone 関数を `NewCachedStore` に渡すなど）で対応できるが、本計画の対象外とする。

## Git運用

- **ブランチ名**: `fix/collection-cache-snapshot`
- **コミット分割方針**（コミットメッセージは日本語）:
  1. `docs: コレクションキャッシュの返却値を切り離す変更計画書を追加する` — 本計画書
  2. `fix(application): コレクションの返却値と入力値を複製し、キャッシュと共有しない` — `collection_service.go` の変更と、テスト方針 1〜3 のユニットテスト（修正とその回帰テストを 1 コミットにまとめ、bisect で赤いコミットを作らない）
  3. `test: コレクションの JSON 化と更新を並行実行しデータ競合が無いことを検証する` — テスト方針 4・5
  4. `ci: Go のユニットテストを -race 付きで実行する` — `ci.yml`、`Taskfile.yml`（`go:test:race`）、`CLAUDE.md`
- **計画書コミットの扱い**: コミット 1（計画書）は作業ブランチの先頭に置き、コミット 2〜4 と一緒にマージする。計画書だけを先に main へ入れない（main 上で「計画はあるが修正は無い」状態を作らず、計画書の存在を修正済みと誤読させないため）。
- **完了後**: 「完了条件」をすべて満たしたことを確認 → main へマージ
