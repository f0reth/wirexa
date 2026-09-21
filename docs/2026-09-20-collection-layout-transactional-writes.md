# 変更計画書: コレクションとサイドバーレイアウトの更新をトランザクション化する

## 概要

`docs/backend-architecture-review.md` の指摘 3「コレクションとサイドバーレイアウトの更新がトランザクションになっていない」に対応する。

現状の `CollectionService` には2種類の不整合がある。

**(a) キャッシュ先行更新** — `RenameCollection` / `UpdateRequest` / `RenameItem` / `MoveItem` / `MoveItemToSidebar` は、キャッシュ上の `*domain.Collection` を直接書き換えてから `repo.Save` を呼ぶ。保存が失敗すると API はエラーを返すのに、メモリ上の変更だけが残る。UI には反映され、再起動すると消える。

**(b) 複数ファイルの順次更新** — 1つの操作が複数の JSON ファイルを順に書き、途中で失敗しても巻き戻されない。

### 失敗クラスを分けて対処する

補償ロールバックだけでは届かない失敗がある。プロセスが消える場合は巻き戻しコードが走らないため、ロールバックに「クラッシュしても壊れない」責務を負わせない。

| 失敗クラス       | 例                                                                 | 対処                                               |
| ---------------- | ------------------------------------------------------------------ | -------------------------------------------------- |
| プロセス内の保存エラー | ディスクフル、権限、Windows の AV によるロック、一時的な rename 失敗 | 補償ロールバック（書き込み済みを逆順に書き戻す）  |
| プロセス消失     | クラッシュ、強制終了、電源断                                        | 巻き戻し不可能。**中間状態そのものを無害にする**  |

実際に遭遇する大半は前者で、`Save` がエラーを返すため巻き戻しが機能する。「エラーを返したなら何も変わっていない」という API 契約はこれで守る。後者に対しては、汎用の write-ahead log を導入するのではなく、次の2つで**中間状態から「恒久的なデータ喪失」という性質を除去する**。

### 対応の4本柱

1. **copy-on-write** — キャッシュを直接変更せず、ディープコピー上で変更し、永続化が成功してからキャッシュを差し替える。(a) の解消。
2. **unit of work（補償ロールバック）** — コレクション JSON 同士の複数書き込みを1単位にまとめ、プロセス内エラーで途中失敗したら書き込み済みを元へ戻す。
3. **レイアウトの導出値化** — `sidebar_layout.json` を並び順のヒントに降格し、読み出し時に実データと突合して自己修復する。これによりコレクション↔レイアウト間の原子性要求そのものが消える。
4. **「追加してから削除」の書き込み順序** — 移動系2操作の書き込み順を反転し、クラッシュ時の残骸を「喪失」から「重複」へ変える。重複は検出・回収できる。

汎用ジャーナルを採らないのは、ジャーナル形式・リカバリ経路・冪等性の担保と、各書き込み境界でのプロセス強制終了テストを Windows で安定して回す仕組みまで必要になり、守る対象（ローカル HTTP クライアントのアイテム配置と並び順）に対して恒久的なコストが釣り合わないため。3 と 4 を入れた後に残るのは「移動が取り消されて元の場所に戻っている」であり、ジャーナルを導入する理由としては弱い。

## 変更対象ファイル

| ファイル                                                   | 層          | 変更種別 | 変更内容                                                                                                                                 |
| ---------------------------------------------------------- | ----------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/domain/http/types.go`                            | Domain      | 変更     | `Collection.Clone` / `TreeItem.Clone` / `HTTPRequest.Clone` / `RequestBody.Clone` を追加                                                 |
| `internal/domain/http/types_test.go`                       | Domain      | 変更     | `Clone` がディープコピーであることの検証を追加                                                                                          |
| `internal/application/http/unit_of_work.go`                | Application | 追加     | `unitOfWork`（コレクション JSON の順次保存 ＋ undo スタック ＋ `Rollback`）                                                              |
| `internal/application/http/unit_of_work_test.go`           | Application | 追加     | undo の積み方・逆順実行・undo 失敗時のログ記録の検証                                                                                    |
| `internal/application/http/sidebar_layout_service.go`      | Application | 変更     | `Load` と `Update(mutate)` を追加し、`Append` / `Remove` / `Move` / `InsertItem` を純粋な mutator へ分解。`GetOrInit` を削除             |
| `internal/application/http/sidebar_layout_service_test.go` | Application | 追加     | 各 mutator の純関数テストと `Update` の保存失敗時にファイルが変わらないことの検証                                                       |
| `internal/application/http/reconcile.go`                   | Application | 追加     | `dropDuplicateItems`（コレクション間の重複アイテム回収）と `reconcileSidebarLayout`（レイアウト突合）の純関数                           |
| `internal/application/http/reconcile_test.go`              | Application | 追加     | 重複回収・stale / 重複 / 欠落エントリの修復・空レイアウトからの初期生成の検証                                                           |
| `internal/application/http/collection_service.go`          | Application | 変更     | 全変更系メソッドを copy-on-write ＋ `unitOfWork` 化。移動系の書き込み順序を反転。レイアウト更新を best-effort 化。起動時と読み出し時の突合 |
| `internal/application/http/collection_service_test.go`     | Application | 変更     | 保存失敗時にキャッシュ・ディスクが変化しないことの回帰テストと、書き込み順序の検証を追加。失敗注入用の fake リポジトリを拡張             |
| `app.go`                                                   | 合成ルート  | 変更     | `httpapp.NewCollectionService(collRepo, layoutRepo, logger)` へ logger を追加                                                            |
| `internal/integration/http_test.go`                        | Integration | 変更     | クラッシュ相当の残骸（重複アイテム、stale レイアウト）が起動時に回収されることの検証を追加                                              |

**変更不要:** `internal/adapters/http_handler.go`、`internal/domain/http/port.go`、`frontend/` 配下すべて、`frontend/e2e/fake-backend/`。理由は後述の「コード生成」「テスト方針」を参照。

## 実装方針

### 1. ディープコピーは domain に置く

`Collection` / `TreeItem` / `HTTPRequest` / `RequestBody` の `Clone` は、ツリー構造の不変条件を知っている domain 型自身の責務とする。domain は外部依存を持たないままなので依存方向は変わらない。

コピー対象は `Items` とその子孫、`Request` ポインタ、`Headers` / `Params` スライス、`Contents` マップ、`FormData` / `FormURLEncoded` スライス。`FileReference` は文字列と bool のみの値型なのでコピー不要。

```go
// Clone はコレクションのディープコピーを返す。
// 返り値は元のコレクションと一切の可変状態を共有しない。
// 変更系ユースケースはコピー上で変更し、永続化が成功してからキャッシュへ差し替える。
func (c *Collection) Clone() *Collection
```

### 2. unitOfWork はコレクション JSON だけを扱う

レイアウトを導出値へ降格する（後述 3）ため、ロールバックの対象はコレクション JSON 同士に絞られる。`internal/application/http/unit_of_work.go` に置き、依存するのは `domain.CollectionRepository` と `cmn.Logger` のみ。

```go
// unitOfWork は複数のコレクションファイルへの書き込みを1単位にまとめる。
// 各操作は成功したときだけ「直前の内容へ戻す」undo をスタックへ積み、
// 途中で失敗すると以降の操作を飛ばして最初のエラーを保持する。
// 呼び出し側は Err() を見て、失敗していれば Rollback() でディスクを戻し、
// 成功していればキャッシュを差し替える。
//
// 巻き戻せるのはプロセスが生きている間の保存エラーだけで、プロセスが消えた場合は
// undo が走らない。そのためクラッシュ耐性は書き込み順序 (追加してから削除) と
// 起動時の回収で担保し、この型には負わせない。
type unitOfWork struct {
	repo   domain.CollectionRepository
	logger cmn.Logger
	undos  []func() error
	err    error
}

// SaveCollection は next を保存する。prev は保存前の内容で、nil の場合 (新規作成) の
// 巻き戻しは削除になる。prev は巻き戻しでそのまま書き戻すため、呼び出し側は以後
// prev を変更してはならない (キャッシュ上の現行オブジェクトをそのまま渡す)。
func (u *unitOfWork) SaveCollection(prev, next *domain.Collection)

// DeleteCollection は prev.ID のファイルを削除する。巻き戻しは prev の再保存。
func (u *unitOfWork) DeleteCollection(prev *domain.Collection)

// Err は最初に発生したエラーを返す。
func (u *unitOfWork) Err() error

// Rollback は積まれた undo を逆順に実行する。
// undo 自体の失敗はこれ以上ディスクを戻す手段が無いためログに記録して続行し、
// 呼び出し側には元のエラーを返す。残った不整合は起動時の回収で拾う。
func (u *unitOfWork) Rollback()
```

呼び出し側の形は全メソッドで統一する。

```go
s.mu.Lock()
defer s.mu.Unlock()

c, ok := s.cache[id]
if !ok {
	return &cmn.NotFoundError{Resource: sidebarKindCollection, ID: id}
}
next := c.Clone()
next.Name = name

uow := s.begin()
uow.SaveCollection(c, next)
if err := uow.Err(); err != nil {
	uow.Rollback()
	return err
}
s.cache[id] = next // 永続化がすべて成功してから差し替える
return nil
```

書き込みが1ファイルだけの操作（`RenameCollection` / `UpdateRequest` / `RenameItem` / `DeleteItem` / `addItem`）では `Rollback` は実質何もしない（唯一の保存が失敗しているので undo スタックが空）。それでも同じ形に揃えるのは、後から2ファイル目の書き込みが増えたときに書き換え漏れを起こさないため。

`MoveItem` でコレクションを跨ぐ場合は src・dst の両方をクローンし、src のクローンから外したノードを dst のクローンへ挿入する。同一コレクション内移動では1つのクローンを src/dst 兼用にする。これにより、従来は src と dst がキャッシュ上の同じ `*TreeItem` を共有していた経路も解消される。

### 3. レイアウトを導出値へ降格する

`sidebar_layout.json` は並び順のヒントであって存在の正ではない。正をコレクション JSON に一本化し、レイアウトは読み出し時に実データと突合して自己修復する扱いにする。

これによりコレクション↔レイアウト間の原子性要求が消える。

- `CreateCollection` のレイアウト保存前にクラッシュ → コレクションはあるがレイアウトに無い → 突合が末尾へ追加
- `DeleteCollection` のレイアウト更新前にクラッシュ → stale エントリが残る → 突合が除去
- `addItem` / `DeleteItem`（`__root__` 直下）も同様

したがって**コレクション変更に伴うレイアウト更新は best-effort とする**。失敗してもログに残すだけで、コレクション本体の変更を巻き戻さず、操作自体は成功として返す。

```go
// applyLayoutBestEffort はレイアウトを更新し、失敗してもログに残して続行する。
// レイアウトは導出値で、欠落・stale は読み出し時の突合で修復されるため、
// コレクション本体の変更をレイアウトの保存失敗で巻き戻さない。
func (s *CollectionService) applyLayoutBestEffort(mutate layoutMutator)
```

例外は `MoveSidebarEntry` で、これはレイアウトの並び替えそのものが目的であるため、保存失敗はエラーとして呼び出し側へ返す（書き込みは1ファイルなので巻き戻しは不要）。

`SidebarLayoutService` の mutator は純関数へ分解し、ロード〜保存をレイアウトロック下で完結させる。ロールバック対象から外れたため `Update` は undo を返さない。

```go
// layoutMutator は読み込み済みレイアウトから保存すべきレイアウトを組み立てる。
// エントリが見つからないなどの業務エラーはここで返し、ファイルは書き換えない。
type layoutMutator func([]domain.SidebarEntry) ([]domain.SidebarEntry, error)

func layoutAppend(entry domain.SidebarEntry) layoutMutator
func layoutRemove(kind, id string) layoutMutator
func layoutMove(kind, id string, position int) layoutMutator
func layoutInsertItem(itemID string, position int) layoutMutator

// Update はレイアウトを読み込み、mutate の結果を保存する。
func (l *SidebarLayoutService) Update(mutate layoutMutator) error

// Load は保存済みレイアウトをそのまま読み込む。突合は呼び出し側が行う。
func (l *SidebarLayoutService) Load() ([]domain.SidebarEntry, error)
```

**読み出し時に突合する。** `GetSidebarLayout` はディスクから読んだレイアウトを、その時点のコレクションキャッシュと突合してから返す（保存はしない）。こうすることで、起動時の保存が失敗していても、トランザクション中のレイアウト書き込みが失敗していても、frontend は常に整合した並びを受け取る。突合対象はメモリ上のキャッシュと数十件のエントリなので、読み出しごとに行っても負荷は無視できる。

### 4. 書き込み順序を「追加してから削除」に変える

レイアウトを外すと、残る複数ファイル問題は移動系2操作だけになる。先に削除するから喪失するのであって、先に追加すれば最悪でも両方に存在する。

| 操作                         | 現行                | 変更後                      | クラッシュ時の残骸    |
| ---------------------------- | ------------------- | --------------------------- | --------------------- |
| `MoveItem`（コレクション間） | src → dst          | **dst → src**              | 喪失 → **重複**      |
| `MoveItemToSidebar`          | src → root → layout | **root → src** → layout | 喪失 → **重複**      |

重複は検出可能で回収できるが、喪失はどちらでもない。これが本質的な差になる。

プロセス内エラーに対する挙動は変わらない。dst の保存が成功して src の保存が失敗した場合、undo は dst を保存前の内容へ書き戻す。

```go
uow := s.begin()
uow.SaveCollection(dstPrev, dstNext) // 先に追加先を書く
uow.SaveCollection(srcPrev, srcNext) // 後から移動元を書く
if err := uow.Err(); err != nil {
	uow.Rollback()
	return err
}
s.cache[targetCollectionID] = dstNext
s.cache[sourceCollectionID] = srcNext
```

### 5. 起動時の回収と突合

`internal/application/http/reconcile.go` に2つの純関数を置く。

```go
// dropDuplicateItems は複数のコレクションに現れるアイテム ID を1つだけ残して除去する。
// 「追加してから削除」の途中でプロセスが消えた場合の残骸を回収する。
// 残す側は __root__ を先頭、以降はコレクション ID の昇順で走査した最初の出現とする。
// クラッシュ後にどちらが移動先だったかは判別できないため、この規則は
// 「決定的であること」だけを保証し、移動の意図までは復元しない。
// 返り値は内容が変わったコレクションの ID 集合。
func dropDuplicateItems(cols map[string]*domain.Collection) map[string]*domain.Collection

// reconcileSidebarLayout は保存済みレイアウトを実データと突合して正規化する。
//  1. コレクション / __root__ 直下アイテムとして存在しないエントリを除去する
//  2. 重複エントリを除去する (最初の出現を残す)
//  3. レイアウトに無いコレクション (名前順) と __root__ 直下アイテム (ツリー順) を末尾へ追加する
// 既知のエントリの並び順は保存されたものを維持する。
// 空レイアウトからの初期生成も同じ規則の特殊ケースとして処理する。
func reconcileSidebarLayout(
	layout []domain.SidebarEntry,
	cols []*domain.Collection,
	rootItems []*domain.TreeItem,
) (next []domain.SidebarEntry, changed bool)
```

`__root__` のフォルダ内にネストしたアイテムはサイドバー直下に並ばないため、item エントリは `__root__` の**直下の子**に限って有効と判定する（現行の `addItem` / `computeInitialLayout` の条件と一致する）。

`NewCollectionService` の起動手順は次のとおり。

1. `repo.Load()` → キャッシュ構築、`normalizeItemForms`、`__root__` の確保（現行どおり）
2. `dropDuplicateItems` で重複を回収し、変化したコレクションを保存する。**保存失敗は best-effort**（ログに記録して続行。メモリ上は回収済みなので動作は整合し、次回起動で再試行される）
3. レイアウトを読み込んで `reconcileSidebarLayout` を適用し、`changed` なら保存する。**保存失敗は best-effort**（ログに記録して続行）

レイアウトの**保存**失敗が起動を止めないことが、現行からの明確な改善点になる。読み取り専用ディレクトリやディスクフルの状況でも、コレクションデータが読めている限りアプリは起動し、突合済みのレイアウトで動作する。

レイアウトの**読み込み**失敗（JSON として壊れている場合）を初期化失敗として返す現行の振る舞いは変えない。再生成可能ファイルの破損で起動できない問題はレビュー #8 の対象であり、本変更では扱わない。

### 6. ロック順序の明文化

`GetSidebarLayout` が読み出し時にコレクションキャッシュとレイアウトの両方を参照するため、**`CollectionService.mu` → `SidebarLayoutService.mu` の一方向**をロック順序として定め、`collection_service.go` の冒頭コメントに明記する。

逆向き（レイアウトロック保持中にコレクションロックを取る）の唯一の経路だった `SidebarLayoutService.GetOrInit(computeInitial)` は削除する。レイアウトの初期生成は突合に吸収されるため、`computeInitialLayout` も不要になる。

### 7. logger の注入

`unitOfWork.Rollback` の undo 失敗、best-effort なレイアウト保存の失敗、起動時の重複回収の記録に `cmn.Logger` が必要になる。`NewCollectionService` に引数として追加する。合成ルート `app.go` の `initialize()` には既に `logger` があるので渡すだけでよい。サービスの新設ではないため二段階配線（`NewApp()` での空生成 → `SetupXxxHandler` 注入）の構造には変更なし。テストでは nil を許容し、nil の場合は記録をスキップする（`JSONStore.SetLogger` と同じ扱い）。

### 8. 依存方向

- `Clone` は domain 型のメソッドで、domain は引き続き他層へ依存しない。
- `unitOfWork` と突合の純関数は application 層に閉じ、`domain.CollectionRepository` と `cmn.Logger` というポート経由でのみ永続化とログに触れる。
- `adapters` の RPC 面・シグネチャは一切変わらないため、`adapters` → `domain` ポートという依存も変わらない。

## コード生成

**どちらも不要。**

- [ ] `task wails:generate` — **不要**。`HTTPHandler` のメソッドシグネチャと `domain` 型のフィールドはどちらも変わらない（追加するのはメソッドのみで、Wails のモデル生成はフィールドを見る）。ただし CI が `git diff --exit-code frontend/wailsjs` で落ちるため、作業完了時に一度実行して差分が出ないことを確認する。
- [ ] `task go:generate:events` — **不要**。イベントの追加・変更なし。

## テスト方針

### Go ユニット（`task go:test`）

**`internal/domain/http/types_test.go`（追加）**

- `Clone` した結果を変更しても元の `Collection` が変化しないこと（`Items` / ネストした `Children` / `Request.Headers` / `Body.Contents` / `Body.FormData` それぞれ）。

**`internal/application/http/unit_of_work_test.go`（追加）**

- 複数操作が成功した場合に undo が積まれるだけでディスクが最終状態になること。
- 2番目の操作が失敗した場合、以降の操作が実行されず、`Rollback` で1番目が逆順に戻ること。
- undo 自体が失敗した場合、`Rollback` は残りの undo を続行し、logger に記録されること。

**`internal/application/http/sidebar_layout_service_test.go`（追加）**

- 各 mutator の純関数としての振る舞い（追加・削除・移動・挿入、見つからない場合のエラー）。
- `Update` の保存失敗時にファイルが書き換わらないこと。

**`internal/application/http/reconcile_test.go`（追加）**

- `dropDuplicateItems`: 2つのコレクションに同じアイテム ID がある場合に1つだけ残ること。残す側が規則どおり決定的であること。重複が無い場合に何も変更されないこと。
- `reconcileSidebarLayout`: 存在しないコレクション ID / アイテム ID のエントリが除去される。重複エントリが除去され最初の出現の位置が保たれる。レイアウトに無いコレクションが名前順で、`__root__` 直下アイテムがツリー順で末尾に追加される。`__root__` のフォルダ内にネストしたアイテムのエントリは除去される。空レイアウト ＋ 既存コレクションから従来の `computeInitialLayout` と同じ並びが生成される。変更がない場合に `changed` が偽になる。

**`internal/application/http/collection_service_test.go`（変更）**

失敗注入のため fake リポジトリを拡張する。`inMemoryLayoutRepo` に「n 回目以降の Save を失敗させる」カウンタを、`inMemoryRepo` に「特定 ID の Save / Delete を失敗させる」設定と**保存順序の記録**を追加する。

*ロールバックの回帰テスト* — 「エラーを返し、かつキャッシュとリポジトリの中身が操作前と一致すること」を毎回検証する。

| テスト                                    | 注入する失敗                  | 期待                                                   |
| ----------------------------------------- | ----------------------------- | ------------------------------------------------------ |
| `RenameCollection` / 保存失敗              | collection Save               | キャッシュ上の名前が変わらない                        |
| `UpdateRequest` / 保存失敗                 | collection Save               | キャッシュ上のリクエストが変わらない                  |
| `RenameItem` / 保存失敗                    | collection Save               | キャッシュ上のアイテム名が変わらない                  |
| `DeleteItem` / 保存失敗                    | collection Save               | アイテムが消えない                                    |
| `MoveItem`（コレクション間）/ src 保存失敗 | src の collection Save        | dst が保存前の内容へ戻り、src からアイテムが消えない  |
| `MoveItemToSidebar` / src 保存失敗         | src の collection Save        | `__root__` が保存前の内容へ戻る                        |
| `MoveSidebarEntry` / 保存失敗              | layout Save                   | エラーを返し、レイアウトが変わらない                  |
| ロールバック失敗                           | src Save ＋ dst の巻き戻し Save | 元のエラーが返り、logger に記録される                 |

*書き込み順序のテスト* — 記録した保存順から、`MoveItem`（コレクション間）が dst → src の順、`MoveItemToSidebar` が `__root__` → src の順であることを検証する。これがクラッシュ時に喪失ではなく重複になることの根拠になるため、順序を直接固定する。

*レイアウト best-effort のテスト* — `CreateCollection` / `DeleteCollection` / `AddRequest`（`__root__` 直下）/ `DeleteItem`（`__root__` 直下）/ `MoveItemToSidebar` について、レイアウト保存が失敗してもコレクション本体の変更は成功として返り、キャッシュとコレクションファイルに反映されていること。そして `GetSidebarLayout` が突合済みの整合した並びを返すこと。

*起動時の回収テスト* — レイアウト保存が失敗する状態でも `NewCollectionService` が成功すること。重複アイテムを含むリポジトリから構築した場合に回収され、コレクションの保存が失敗しても起動が成功すること。

既存テストのうち `GetOrInit` の遅延初期化に依存するもの（レイアウト初期値生成のテスト）は突合のテストへ置き換える。それ以外の `TestCollectionService_*` は振る舞いが変わらないため原則そのまま通る。

並行性テストは、既存の並行テストに「保存失敗を注入しながら並行更新しても、キャッシュとリポジトリが常に一致する」ケースを追加する（CI は `-race` 付きで実行）。

### Go 統合（`task go:test:integration`）

`internal/integration/http_test.go` に追加する。プロセスを実際に強制終了させる代わりに、**クラッシュが残す状態をディスク上に直接作って**回収を検証する。この方が Windows / macOS の双方で決定的に回せる。

- 同一アイテム ID を2つのコレクション JSON に書いた状態からハンドラを組み立て、アイテムが1つだけ残り、ディスク上のコレクションファイルも回収済みになっていること。
- `sidebar_layout.json` に存在しないコレクション ID と重複エントリを書き、`collections/` には layout に無いコレクションを置いた状態から組み立て、`GetSidebarLayout` が修復済みの並びを返し、ディスク上の `sidebar_layout.json` も書き換わっていること。

ロールバック経路の統合テストは、実ファイルの書き込み失敗を Windows / macOS の双方で安定して注入する手段が無いため行わない。この経路はユニットテストの失敗注入で網羅する。

### フロント ユニット / UI e2e / フルスタック e2e

**すべて変更なし。** バインド API のシグネチャも、成功時の振る舞いも変わらない。変わるのは「保存に失敗したときに何が残るか」だけで、`frontend/e2e/fake-backend/` はインメモリで失敗を発生させないため、模倣すべき差分が生じない。

### 実行前提

`go test` / `go vet` は `main.go` の `//go:embed` のため `frontend/dist/index.html` の存在を要求する。

## 副作用・注意事項

- **エラー時の振る舞いが変わる。** これまで「エラーを返したが一部は保存済み」だった操作が「エラーを返し、何も変わっていない」になる。
- **レイアウト保存の失敗が操作の失敗ではなくなる。** コレクションの変更が成功していれば、レイアウトの書き込みに失敗しても操作は成功として返る。ユーザの意図（コレクションやアイテムの変更）は達成されており、並び順は導出値として自己修復されるため。ただし `MoveItemToSidebar` でレイアウト書き込みが失敗した場合、アイテムは指定位置ではなくサイドバー末尾に現れる。この劣化は意図したもので、ログに記録する。
- **クラッシュ時の残骸は重複になる。** 移動系操作の途中でプロセスが消えると、アイテムが移動元と移動先の両方に存在する状態が残る。起動時に1つへ回収されるが、どちらが移動先だったかは判別できないため、ユーザから見ると「ドラッグした移動が反映されていない」ことがありうる。アイテムが失われることはない。
- **ロールバック自体の失敗は残余リスク。** undo の書き戻しが失敗した場合はディスクを戻す手段が無く、ログに記録して元のエラーを返すにとどまる。この状態も起動時の回収で部分的に拾われる。
- **キャッシュのメモリ使用量が増える。** 変更のたびに対象コレクションをディープコピーする。コレクション1件は高々数百アイテム規模で、コピーは変更操作時のみ・短命なので実用上の問題は無いと判断する。
- **レビュー #6（可変参照がロック外へ公開される）が副次的に緩和される。** 変更がすべてコピー上で行われ、キャッシュのエントリは丸ごと差し替えられるようになるため、`GetCollections` / `GetRootItems` が返した参照が後から書き換えられることが無くなる。ただし返却値が浅いコピーである点自体は変わらないため、#6 を閉じるものではない。
- **fsync は対象外。** `infra.WriteJSONFile` は tmp + rename だが fsync していないため、電源断では rename が原子的でもファイルの内容が永続化されていない可能性がある。これは本変更が持ち込む問題ではなくアプリの全書き込みに既にある性質であり、中途半端に一部だけ変えるべきではないため、必要なら独立した変更として扱う。
- **レビュー #8 は対象外。** レイアウトファイルが JSON として壊れていて読み込めない場合に起動できない問題（隔離・再生成）は本変更では扱わない。本変更が修復するのは「JSON としては読めるが実データと食い違っている」レイアウトのみ。
- **予約コレクション `__root__` の保護は対象外。** レビュー「その他の観察事項」の指摘であり、本変更には含めない。

## Git運用

- **ブランチ名**: `fix/collection-layout-transactional`
- **コミット分割方針**: 層ごと・機能ごとに分割する。各コミット単体でテストが通る状態を保つ。

  1. `refactor(domain): Collection と TreeItem にディープコピーを追加する`
  2. `refactor(application): サイドバーレイアウトの更新を mutator へ分解する`
  3. `feat(application): コレクション保存をまとめる unit of work を追加する`
  4. `fix(application): コレクション更新を copy-on-write にして永続化成功後に反映する`（`app.go` の logger 注入を含む）
  5. `fix(application): 移動系の書き込みを追加してから削除の順序に変える`
  6. `refactor(application): サイドバーレイアウトを導出値として扱う`（読み出し時の突合と best-effort 保存）
  7. `feat(application): 起動時に重複アイテムとレイアウトを回収する`
  8. `test(integration): クラッシュ相当の残骸が起動時に回収されることを検証する`

- **完了後**: `task format` → `task lint` → `task test` がすべて通ることを確認し、`task wails:generate` で `frontend/wailsjs` に差分が出ないことを確認 → main へマージ
