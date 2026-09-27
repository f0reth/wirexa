# バックエンド統合テストレビュー

生成日時: 2026-09-27
対象: internal/integration/ の統合テスト（HTTP / MQTT / UDP / OpenAPI）
実行結果: 統合テスト 成功（`ok github.com/f0reth/Wirexa/internal/integration 19.824s`、`./internal/...` の文カバレッジ 64.2%） / `-race` 未実行（`go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`。この Windows 環境に gcc が無い）

- 計測コマンド: `go test -tags integration -coverpkg=./internal/... -coverprofile=<一時ディレクトリ>/integ.cover.out ./internal/integration/...`
- 計測は Windows 上で行ったので、`*_windows.go` が計測され、`file_open_other.go` は計測されていない。
- 改訂（2026-09-27）: 実コードと照らし合わせたレビューを受けて、HTTP A1・B3・G2・J1・J2、MQTT D1 の再現手順と記述、MQTT G1 の `ResumeSubs` の扱い、`-race` の記述（統合テストのジョブに限る）を修正した。

---

## サマリー

| ドメイン | テスト関数数 | RPC カバー数 / 全 RPC 数 | Handler の文カバレッジ | 不足ケース数 | 優先度(高/中/低) |
| --- | --- | --- | --- | --- | --- |
| HTTP | 40 | 17 / 20 | 42.9%（39/91） | 26 | 高 |
| MQTT | 22（`TestMain` を除く） | 9 / 9 | 100%（11/11） | 12 | 高 |
| UDP | 20 | 8 / 8 | 100%（11/11） | 10 | 中 |
| OpenAPI | 2 | 0 / 7（Service 経由 3） | 0%（0/18）※ | 5 | 中 |

> **RPC カバー数**: 対象 Handler の公開メソッドのうち、統合テストから呼ばれ、結果（戻り値・副作用・イベント）がアサーションで検証されているものの数。
> ※ OpenAPI の統合テストは Handler を通らず `FileService` を直接組み立てている（`newOpenAPIFileService`）ため、Handler の文カバレッジは 0%。Service 経由で検証されている RPC 相当は `OpenFilePicker`（`OpenSelected` で代用）・`ReadFile`・`GetRecents` の 3 つ。
> **Handler の文カバレッジ**: Step 0 のプロファイルから Handler ファイル単位で集計した値。

下位層で統合テストのカバレッジが特に低いもの（実 I/O に依存するもの）:

| ファイル | 文カバレッジ | 0% の主な関数 |
| --- | --- | --- |
| `internal/infrastructure/http/response_store.go` | 9.8%（21/214） | `Spill`・`AcquireSave`・`Discard`・`SaveTo`・`Release`・`SweepStaleTempFiles` |
| `internal/application/http/request_service.go` | 53.6%（52/97） | `Shutdown` |
| `internal/application/openapi/file_service.go` | 43.1%（53/123） | `SaveSelected`・`WriteFile`・`RemoveRecent`・`MoveRecent` |
| `internal/application/mqtt/service.go` | 89.7% | `onConnectionLost` |
| `internal/infrastructure/quarantine.go` | 66.7% | 退避先の衝突（`.corrupt` が既存の場合）の分岐 |
| `internal/application/store/recovery.go` | 64.7% | 退避失敗・破損以外の読み込み失敗の分岐 |

---

## ドメイン別詳細

### HTTP (`http_test.go`)

#### 対象メソッド（Handler の公開メソッド）

| メソッド名 | ファイル:行 | テスト済み? | テストケース名 |
| --- | --- | --- | --- |
| OpenFilePicker | http_handler.go:110 | 部分的 | TestHTTP_SendRequest_FileBodyViaDialogToken / TestHTTP_SendRequest_FormDataKinds（hint・キャンセル・再試行は単体テストのみ） |
| SendRequest | http_handler.go:161 | 済 | TestHTTP_SendRequest_* ほか多数 |
| CancelRequest | http_handler.go:170 | 済 | TestHTTP_CancelRequest / TestHTTP_CancelRequest_SameSavedRequestInParallel / TestHTTP_CancelRequest_BeforeSend |
| SaveResponseBody | http_handler.go:178 | 未テスト | - |
| DiscardResponseBody | http_handler.go:203 | 未テスト | - |
| SaveResponseBase64 | http_handler.go:213 | 未テスト | - |
| GetRootItems | http_handler.go:241 | 済 | TestHTTP_GetRootItems / TestHTTP_SidebarLayout |
| GetCollections | http_handler.go:246 | 済 | TestHTTP_CollectionCRUD ほか |
| CreateCollection | http_handler.go:251 | 済 | TestHTTP_CollectionCRUD |
| DeleteCollection | http_handler.go:256 | 済 | TestHTTP_CollectionCRUD / TestHTTP_DeleteCollection_NotFound / TestHTTP_RootCollection_DeleteAndRenameRejected |
| RenameCollection | http_handler.go:261 | 済 | TestHTTP_CollectionCRUD / TestHTTP_PersistenceRoundTrip |
| AddFolder | http_handler.go:266 | 済 | TestHTTP_FolderAndRequestTree / TestHTTP_GetRootItems |
| AddRequest | http_handler.go:271 | 済 | TestHTTP_FolderAndRequestTree / TestHTTP_AddRequest_AfterDeleteCollection（呼び出し側が指定した ID の重複は未検証） |
| UpdateRequest | http_handler.go:276 | 済 | TestHTTP_UpdateRequest |
| RenameItem | http_handler.go:281 | 済 | TestHTTP_RenameItem |
| DeleteItem | http_handler.go:286 | 部分的 | TestHTTP_DeleteItem（通常コレクション直下のみ。`__root__` 直下の削除とレイアウトからの除去、フォルダごとの削除は未検証） |
| MoveItem | http_handler.go:293 | 部分的 | TestHTTP_MoveItem（コレクション間・末尾への移動のみ。`MoveItem` の文カバレッジ 59.6%） |
| GetSidebarLayout | http_handler.go:298 | 済 | TestHTTP_SidebarLayout / TestHTTP_ReconcilesSidebarLayoutOnStartup |
| MoveSidebarEntry | http_handler.go:304 | 部分的 | TestHTTP_SidebarLayout（`collection` だけ。`item` と `__root__` の拒否は未検証） |
| MoveItemToSidebar | http_handler.go:310 | 済 | TestHTTP_SidebarLayout（再起動後の確認なし） |

**状態を持つ箇所**: 実行中リクエストのキャンセル関数と墓標（`HTTPRequestService.cancels` / `pending`、application）、切り詰めレスポンスの一時ファイル（`ResponseStore.entries`、infrastructure。`http-sessions/wirexa-http-*/response-*`）、ファイルトークン（`FileRegistry`、infrastructure）、コレクションのキャッシュ（`CollectionService.cache`、application）。イベントは発行しない。

**既存テストの読み取り**:
- アサーションは概ね戻り値の中身まで見ている。`errors.Is` で種類まで見ているのは `TestHTTP_SendRequest_RawPathsAreNeverRead` と `TestHTTP_CancelRequest_BeforeSend` だけで、他のエラー系（`TestHTTP_SendRequest_InvalidMethod`・`TestHTTP_DeleteCollection_NotFound`・`TestHTTP_AddRequest_AfterDeleteCollection` など）は `err != nil` しか見ていない。
- 再起動相当は `TestHTTP_PersistenceRoundTrip`・`TestHTTP_RootCollection_DeleteAndRenameRejected`・`TestHTTP_CorruptSidebarLayout`・`TestHTTP_UnloadableRootIsNotOverwritten`・`TestHTTP_RecoversDuplicateItemsOnStartup` で行っている。
- 非同期の待ち方はチャネルとタイムアウト（`TestHTTP_CancelRequest` など）で、`time.Sleep` 依存は無い。
- `buildHTTPHandler`（http_test.go:56）は `NetClient.Cleanup` だけを `t.Cleanup` に登録し、`HTTPRequestService.Shutdown` は呼ばない。`app.go:229` の `shutdown` の順序（`reqSvc.Shutdown` → `netClient.Cleanup`）は再現していない。
- `TestHTTP_CorruptStorage`（http_test.go:1195）は Handler を通さず、`collections/` ではなく `dir` 直下にリポジトリを作っている。正常なファイルが同居する場合の「残りで起動する」は確かめていない。

#### 不足テストケース

##### [観点 A] ゴールデンパスの完全性

**A1. 切り詰められたレスポンス本文の保存・破棄**
- **対象メソッド**: `SendRequest` / `SaveResponseBody` / `DiscardResponseBody`
- **不足内容**: 本文が `MaxResponseBodyMB` を超えて一時ファイルへ退避される流れが、統合テストで一度も通っていない。`SaveResponseBody`・`DiscardResponseBody` は 0%、`ResponseStore.Spill` / `AcquireSave` / `Discard` / `SaveTo` も 0%。Handler（`HTTPHandlerDeps.Responses`）と `NetClient`（`Do` の中の `c.responses.Spill`）が同じ `ResponseStore` を共有しているという配線は、単体テストでは確かめられない。
- **根拠コード**: `internal/adapters/http_handler.go:178-208`、`internal/infrastructure/http/net_client.go:249-272`（`size == maxBody` での 1 バイト先読みと Spill）、`internal/infrastructure/http/response_store.go:141`・`248`・`267`・`350`
- **実インフラで確かめる理由**: 実際に受信した本文が一時ファイルへ書かれ、保存先へコピーされた後に元の一時ファイルが削除される（`SaveTo` はリネームではなくコピー → 削除。`response_store.go:350-363`）という流れが、ソケットとファイル I/O をまたぐため。
- **推奨テスト名**: `TestHTTP_TruncatedResponse_SaveAndDiscard`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`fileDialog` に `savePath string`（`SaveFile` の戻り値）と `saveCalls atomic.Int32` を足す。一時ファイルの場所を確かめるためにテスト側で `dir` を持つ必要があるが、`newHTTPHandlerWithDialog`（http_test.go:40）は内部で `t.TempDir()` を作って外に出さない。`dir := t.TempDir()` を作って `buildHTTPHandler(t, dir, dialog)` を直接呼ぶか、`newHTTPHandlerWithDirAndDialog(t, dir, dialog)` を足す（**dialog に nil を渡す `newHTTPHandler` / `newHTTPHandlerWithDir` では `SaveResponseBody` を呼ばないこと。`wailsFileDialog` が使われてプロセスが終了する**）。サーバーは `1<<20 + 1024` バイトを返し、`Settings.MaxResponseBodyMB: 1` で送る。確かめること:
  1. `resp.BodyTruncated == true`、`len(resp.Body) == 1<<20`、`resp.Size` が全長になること。`dir/http-sessions/wirexa-http-*/response-*` が 1 つあること
  2. `savePath = ""`（キャンセル）で `SaveResponseBody` が `(false, nil)` を返し、一時ファイルが残ること
  3. `savePath` を設定して再度 `SaveResponseBody` を呼ぶと `(true, nil)` を返し、保存先の中身がサーバーの全文と一致し、元の一時ファイルが削除されていること
  4. 保存済みの ID への 2 回目の `SaveResponseBody` と `DiscardResponseBody` が `httpdomain.ErrResponseUnavailable`（`errors.Is`）になり、ダイアログが開かれない（`saveCalls` が増えない）こと
  5. 別の executionID で同じように送信し、`DiscardResponseBody` で一時ファイルが消えること。保持中に同じ executionID で `SendRequest` すると `ErrResponseBusy` になり、破棄後は成功すること
- **優先度**: 高

**A2. バイナリ本文の base64 保存**
- **対象メソッド**: `SendRequest` / `SaveResponseBase64`
- **不足内容**: 非 UTF-8 の本文が `BodyBase64` で返り、それを `SaveResponseBase64` でデコードして書き出す往復が通っていない（0%）。
- **根拠コード**: `internal/infrastructure/http/net_client.go:280`（`cmn.EncodeMaybeBase64`）、`internal/adapters/http_handler.go:213-226`
- **実インフラで確かめる理由**: 送信側のエンコード（infrastructure）と保存側のデコード（adapters）が別の層にあり、実際に受信したバイト列で往復して初めて一致を確かめられるため。
- **推奨テスト名**: `TestHTTP_SaveResponseBase64_BinaryBody`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。A1 で拡張した `fileDialog`（`savePath` あり）を `newHTTPHandlerWithDialog` に渡す。サーバーは `0x89 'P' 'N' 'G' 0x00 0xff ...` を `image/png` で返す。`resp.BodyBase64 == true` を確かめ、`SaveResponseBase64(resp.Body, resp.ContentType)` の後、保存先のバイト列がサーバーの送ったものと一致することを見る。
- **優先度**: 中

**A3. `MoveItem` のコレクション内の並び替えとフォルダへの移動**
- **対象メソッド**: `MoveItem`
- **不足内容**: コレクション内の並び替え（`sameCollection && position > 0` の位置の補正）、フォルダへの移動、自分のサブツリーへの移動の拒否、入れ子のフォルダの移動が通っていない（`MoveItem` 59.6%）。
- **根拠コード**: `internal/application/http/collection_service.go:444-482`
- **実インフラで確かめる理由**: 同じコレクション内では 1 ファイル、コレクションをまたぐと 2 ファイルを書く（移動先が先）。移動後にディスクから読み直した木構造が正しいことは、実ストレージを通して確かめる必要があるため。
- **推奨テスト名**: `TestHTTP_MoveItem_ReorderAndIntoFolder`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`newHTTPHandlerWithDir` で組み立て、リクエスト 3 件 [r1, r2, r3] と子を持つフォルダを作る。(1) `MoveItem(c, r1, c, "", 2)` で [r2, r1, r3] になる。(2) r3 をフォルダへ移動する。(3) フォルダを自分の子フォルダへ移動しようとすると `*domain.ValidationError`（`errors.As`）になり、ディスクの内容が変わらない。(4) 子を持つフォルダを別コレクションへ移動する。最後に同じ `dir` で Handler を作り直し、木構造が一致することを確かめる。
- **優先度**: 中

##### [観点 B] 再起動をまたぐ引き継ぎ

**B1. 移動・削除系の操作結果が再起動後に残るか**
- **対象メソッド**: `MoveItemToSidebar` / `MoveSidebarEntry` / `DeleteItem` / `MoveItem`
- **不足内容**: `TestHTTP_PersistenceRoundTrip`（http_test.go:605）が見ているのは Create・Rename・AddRequest だけで、2 ファイルとレイアウトを書く `MoveItemToSidebar`（`__root__` → 移動元 → レイアウトの順）と、`MoveSidebarEntry` の並び、`__root__` 直下の `DeleteItem` によるレイアウトからの除去は、再起動後に確かめていない。
- **根拠コード**: `internal/application/http/collection_service.go:599-617`（`MoveItemToSidebar` の書き込み順とレイアウトの best effort）、`collection_service.go:640-643`
- **実インフラで確かめる理由**: コレクションファイルと `sidebar_layout.json` の書き込みが別々に成功・失敗し、起動時の突合（`reconcileLayoutAtStartup`）を通して初めて最終的な並びが決まるため。
- **推奨テスト名**: `TestHTTP_SidebarOperations_SurviveRestart`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`newHTTPHandlerWithDir(t, dir)` で 2 コレクションとリクエストを作り、`MoveItemToSidebar(col, item, 0)` と `MoveSidebarEntry("collection", col1, 3)` の後、同じ `dir` で作り直した Handler の `GetSidebarLayout`・`GetRootItems`・`GetCollections` がすべて一致すること（アイテムが移動元に残っていないこと）を確かめる。続けて `DeleteItem(RootCollectionID, item)` の後にもう一度作り直し、レイアウトから `item` エントリが消えていることを見る。
- **優先度**: 中

**B2. 起動時の突合で `__root__` 直下のアイテムエントリを直すこと**
- **対象メソッド**: `GetSidebarLayout`（起動時の `reconcileLayoutAtStartup`）
- **不足内容**: `TestHTTP_ReconcilesSidebarLayoutOnStartup`（http_test.go:1501）が扱うのは `kind: "collection"` のエントリだけ。`kind: "item"`（`sidebarKindItem`）の stale・重複・欠落の修正を起動時に確かめていない。
- **根拠コード**: `internal/application/http/reconcile.go:83-85`・`113-115`、`internal/application/http/collection_service.go:147-162`
- **実インフラで確かめる理由**: `__root__.json` とレイアウトファイルの食い違いはクラッシュ（`MoveItemToSidebar` の途中など）でディスク上にだけ生じる状態で、起動時に修正してファイルへ書き戻すところまでが層をまたぐため。
- **推奨テスト名**: `TestHTTP_ReconcilesRootItemEntriesOnStartup`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`writeCollectionJSON(t, dir, RootCollectionID, collectionJSON(RootCollectionID, RootCollectionID, "r1"))` で `__root__.json` を置き、レイアウトを `[{"kind":"item","id":"gone"},{"kind":"item","id":"r1"},{"kind":"item","id":"r1"}]` にする。`r2` を持つ `__root__` でも試し、`GetSidebarLayout` が `[item r1, item r2]` になり、ディスク上のファイルからも `gone` が消えていることを確かめる。
- **優先度**: 中

**B3. 前回セッションの一時ファイルの回収とファイルトークンの失効**
- **対象メソッド**: `SendRequest`（切り詰め）/ `OpenFilePicker`、起動時の `httpinfra.SweepStaleTempFiles`
- **不足内容**: 本文を切り詰めたまま終了した（`Cleanup` が走らなかった）セッションディレクトリを sweep が回収すること、marker が一致しないディレクトリは残すことを確かめていない（`SweepStaleTempFiles` 0%）。前回のファイルトークンが再起動後に使えないことも確かめていない。
- **根拠コード**: `internal/infrastructure/http/response_store.go:459-493`、`app.go:113`・`134`・`164`、`internal/infrastructure/http/file_registry.go:89-99`
- **実インフラで確かめる理由**: 回収の判定は、ディスクに永続化した `.session-secret` と各ディレクトリの marker の比較で行うので、実ファイルで前回セッションを再現しなければ確かめられないため。
- **推奨テスト名**: `TestHTTP_StaleResponseFilesSweptOnRestart`
- **範囲**: このテストが確かめるのは「実際のセッションが残したファイルに対する `SweepStaleTempFiles` の判定」まで。`app.go` の起動処理がこの関数を `NetClient` の作成より前に、`NetClient` と同じ `sessionDir` で呼んでいること（`app.go:113`・`134`・`164`）は、テストから `SweepStaleTempFiles` を直接呼ぶ限り確かめられない。起動時の配線まで守るなら、`sessionDir` の決定・sweep・`NewNetClient` を `app.go` から 1 つの関数（例: `httpinfra.OpenSessionDir(cacheDir)` のような、sweep 済みの `NetClient` を返すもの）に切り出し、テストからはその関数を呼ぶ。切り出さない場合、配線は fullstack e2e に任せる。
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`buildHTTPHandler` は `NetClient.Cleanup` を必ず `t.Cleanup` に登録するので、クラッシュを再現するには Cleanup を登録しない組み立てが必要（例: `buildHTTPHandler` を `httpFixture{h, reqSvc, netClient}` を返す関数に分け、登録するかどうかを引数で選べるようにする）。1 回目で A1 と同じ切り詰めレスポンスを作り、`dir/http-sessions/wirexa-http-fake`（marker なし）も作っておく。`httpinfra.SweepStaleTempFiles(filepath.Join(dir, "http-sessions"))` を明示的に呼んだ後（上記の切り出しをした場合はその関数を呼ぶ）、Wirexa のセッションディレクトリが消え、`wirexa-http-fake` と `.session-secret` が残ることを確かめる。2 回目の Handler に 1 回目の token を渡すと `ErrFileAccessDenied` になることも見る。注意: `SweepStaleTempFiles` は `os.TempDir()` 直下の `wirexa-response-*`（旧形式）も消す。
- **優先度**: 中

##### [観点 C] リソースリークと終了処理

**C1. `app.go` の終了順序の再現（実行中リクエスト → 一時ファイル）**
- **対象メソッド**: `SendRequest` / `CancelRequest` と `HTTPRequestService.Shutdown` → `NetClient.Cleanup`
- **不足内容**: `HTTPRequestService.Shutdown` は統合テストで 0%。実行中のリクエストが Shutdown で止まり、その後の `SendRequest` が拒否され、`Cleanup` で一時ファイルとセッションディレクトリが消えるまでの流れを確かめていない。
- **根拠コード**: `internal/application/http/request_service.go:193-212`・`98-102`・`150-153`、`internal/infrastructure/http/response_store.go:287-305`、`app.go:235-243`
- **実インフラで確かめる理由**: ルート context のキャンセルが実 TCP 接続上のリクエストまで伝わること、その後に一時ファイルを消しても実行中の書き込みと競合しないことは、層を貫いて初めて確かめられるため。
- **推奨テスト名**: `TestHTTP_Shutdown_CancelsInFlightThenCleansTempFiles`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。B3 と同じく `buildHTTPHandler` を `reqSvc` と `netClient` も返す形に分ける（ヘルパーの変更が必要）。A1 と同じ切り詰めレスポンスを 1 つ保持し、応答しないサーバーへの `SendRequest` を goroutine で走らせる。`reqSvc.Shutdown(3*time.Second)` が true を返し、実行中の `SendRequest` がエラーで復帰し、Shutdown 後の `SendRequest` はサーバーに届かずにエラーになり（サーバー側のカウンタで確かめる）、Shutdown 後の `CancelRequest` は何もしないことを確かめる。続く `netClient.Cleanup()` で `http-sessions` 配下に `.session-secret` 以外が残らないことを見る。タイムアウトで false を返す分岐は `TestHTTPRequestService_Shutdown_ReturnsFalseOnTimeout`（単体テスト）で対応済み。
- **優先度**: 高

##### [観点 D] 入力値の境界

**D1. 不正な URL と、エラー後の実行予約の解放**
- **対象メソッド**: `SendRequest`
- **不足内容**: スキームの無い URL（`localhost:8080`）・空の URL・`://bad` がエラーになること、そのエラーの後に同じ executionID が再利用できる（`ResponseStore.Begin` の予約が `Finish` で解放される）ことを、層を通して確かめていない。
- **根拠コード**: `internal/infrastructure/http/net_client.go:90-105`（`Begin` と `defer Finish`、`invalid URL`）
- **実インフラで確かめる理由**: 予約（infrastructure の ResponseStore）と実行中登録（application の `cancels`）の 2 か所が、実際の失敗経路で両方とも解放されることは結合してしか確かめられないため。URL 解析自体は `TestNetClient_InvalidURL`（単体テスト）で対応済み。
- **推奨テスト名**: `TestHTTP_SendRequest_InvalidURLReleasesExecution`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。各不正 URL で同じ executionID を使って `SendRequest` がエラーになることを確かめ、最後に同じ ID で `httptest.Server` へ送って成功することを見る。
- **優先度**: 低

##### [観点 E] 状態遷移の境界

**E1. 予約コレクション `__root__` を通る移動とサイドバーのエントリ**
- **対象メソッド**: `MoveSidebarEntry` / `MoveItem` / `GetSidebarLayout` / `GetRootItems`
- **不足内容**: `MoveSidebarEntry("collection", "__root__", 0)` が `NotFoundError` になること、`MoveItem` で `__root__` へ移したアイテムが `GetRootItems` に現れ、`GetSidebarLayout` の読み出し時の突合で末尾に追加されること（`MoveItem` はレイアウトを更新しない）、逆向きの移動でエントリが消えることを確かめていない。
- **根拠コード**: `internal/application/http/collection_service.go:419-500`（レイアウト更新なし）・`506-518`（読み出し時の突合）・`545-551`、`internal/application/http/sidebar_layout_service.go:64-80`（`layoutMove` の NotFound）
- **実インフラで確かめる理由**: レイアウトファイルとコレクションファイルがずれた状態を、読み出し時の突合が正しく埋め合わせることは、実ファイルを挟んだ層の組み合わせでしか現れないため。
- **推奨テスト名**: `TestHTTP_MoveItem_ThroughRootCollection`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`newHTTPHandlerWithDir` で組み立てる。`MoveSidebarEntry("collection", RootCollectionID, 0)` のエラーを `errors.As(err, &nf)` で `*domain.NotFoundError` と確かめる。`MoveItem(col, item, RootCollectionID, "", -1)` の後、`GetSidebarLayout` の末尾が `{item, item.ID}` になり、`sidebar_layout.json` にはまだ無いこと（読み出し時の突合であること）を見る。`MoveItem(RootCollectionID, item, col, "", -1)` の後はエントリが消えることを確かめる。
- **優先度**: 中

**E2. 削除済みのアイテム・コレクションへの `MoveItem`**
- **対象メソッド**: `MoveItem` / `DeleteItem` / `DeleteCollection`
- **不足内容**: 削除済みのアイテムの移動、削除済みのコレクションを移動先にした移動が NotFound になり、移動元のファイルが変わらないことを確かめていない（`collection_service.go:425-430`・`444-445` が未到達）。
- **根拠コード**: `internal/application/http/collection_service.go:423-445`
- **実インフラで確かめる理由**: 失敗時にどのファイルも書き換えないこと（挿入先の検証を `RemoveNode` の前に行う）をディスク上の内容で確かめるため。
- **推奨テスト名**: `TestHTTP_MoveItem_AfterDelete`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。移動元のコレクションファイルのバイト列を操作の前後で比較する。
- **優先度**: 低

##### [観点 F] 組み合わせの境界

**F1. 認証とユーザー指定の `Authorization` ヘッダーの重なり**
- **対象メソッド**: `SendRequest`
- **不足内容**: `Auth.Type` が `basic` / `bearer` で、Headers にも `Authorization` がある場合に認証側が優先され、値が 1 つだけ送られることを確かめていない（単体テストにも無い）。
- **根拠コード**: `internal/infrastructure/http/net_client.go:190-201`（Headers は `Add`、認証は `Set` で上書き）
- **実インフラで確かめる理由**: サーバーが実際に受け取るヘッダーの値の数は、ワイヤ上でしか確かめられないため。
- **推奨テスト名**: `TestHTTP_SendRequest_AuthOverridesAuthorizationHeader`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`TestHTTP_SendRequest_Auth` と同じ形で、`Headers: [{Key: "Authorization", Value: "Custom x", Enabled: true}]` を足し、サーバー側で `r.Header.Values("Authorization")` が 1 件で認証側の値になることを見る。
- **優先度**: 中

##### [観点 G] 異常系の境界

**G1. 選択後に消えたファイルを送る**
- **対象メソッド**: `OpenFilePicker` / `SendRequest`
- **不足内容**: `OpenFilePicker` の後、送信前にファイルが削除された場合に `ErrSelectedFileUnavailable` になり、エラーにパスが含まれず、サーバーに本文が届かないことを確かめていない（`file_open_windows.go:24-28` が未到達）。
- **根拠コード**: `internal/infrastructure/http/file_registry.go:89-113`、`internal/infrastructure/http/file_open_windows.go:17-31` / `file_open_other.go:14-20`
- **実インフラで確かめる理由**: ダイアログで選んだ時点と送信時点の間に実ファイルが変わるという、実 I/O でしか起こらない状況のため。
- **推奨テスト名**: `TestHTTP_SendRequest_SelectedFileRemoved`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`newHTTPHandlerWithDialog(t, &fileDialog{path: p})` で token を得た後に `os.Remove(p)` し、file body と form-data の file 行の両方で `errors.Is(err, httpdomain.ErrSelectedFileUnavailable)`、`!strings.Contains(err.Error(), p)` を確かめる。
- **優先度**: 中

**G2. 送信中に選択ファイルが変更される**
- **対象メソッド**: `SendRequest`
- **不足内容**: 送信中のファイル変更の扱いが OS で分かれている（Windows は共有モードで書き込みを拒否、それ以外は `CheckUnchanged` で `ErrSelectedFileChanged`）が、どちらの経路も統合テストが無い。
- **根拠コード**: `internal/infrastructure/http/request_body.go:193-217`（`readFile`: 区間の途中の短い読み込みと、末尾での `CheckUnchanged`）・`237-243`、`internal/infrastructure/http/file_open_windows.go:12-31`
- **実インフラで確かめる理由**: 実ソケットの背圧で送信を途中で止め、その間に実ファイルを書き換えないと再現できないため。
- **推奨テスト名**: `TestHTTP_SendRequest_SelectedFileModifiedDuringSend`
- **同期の注意**: 「サーバーが 1 KiB 読んだ」ことは、クライアントがファイルをどこまで読んだかを保証しない。クライアントは送信バッファとサーバー側の受信バッファを埋めるまで読み進めるので、ファイルが小さいと、変更より先に末尾の `CheckUnchanged` まで済んで送信が成功し、テストが不安定になる。クライアントの読み取り位置を外から観測するフックは無いので、次の 2 点で「変更がクライアントの末尾の読み取りより先に起きる」状況を作る。
  1. **書き換えではなく 0 バイトへの切り詰めを使う**。切り詰めは末尾の `CheckUnchanged` を待たず、それ以降の `ReadAt` がすべて短い読み込みになり `ErrSelectedFileChanged` になる（`request_body.go:204-208`）。書き換え（同じサイズで中身を変える）は末尾の検査でしか気付かないので、同期がより難しい。
  2. **ファイルをソケットのバッファの合計より十分大きくする**（例: 64 MiB）。加えて、サーバーは `net.Listener` を包んで accept した接続に `(*net.TCPConn).SetReadBuffer(4 << 10)` を設定し、先読みされる量を減らす。
  それでも、ファイル全体がバッファに収まった場合は送信が成功してしまうので、その場合は `t.Fatalf` で「前提が崩れた」と分かるメッセージにする（成功を黙って許さない）。
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。上の 64 MiB のファイルを選び、サーバーは本文を 1 KiB 読んだ時点でチャネルで通知して待つ。テスト側は通知を受けてから `os.Truncate(p, 0)` し、サーバーを再開させる。`runtime.GOOS == "windows"` なら共有モード（`FILE_SHARE_READ`）のため切り詰め自体がエラーになり送信が成功すること、それ以外なら `errors.Is(err, httpdomain.ErrSelectedFileChanged)` になることを確かめる（CI の ubuntu では後者だけが検証される）。Windows 側は、送信が終わるまでハンドルを開いたままにしているので、サーバーが止まっている間の切り詰めは決定的に失敗する。
- **優先度**: 中

**G3. 本文の受信中に接続が切れる（切り詰めの閾値を超えた後を含む）**
- **対象メソッド**: `SendRequest`
- **不足内容**: サーバーが本文の途中で接続を切った場合のエラーと、閾値を超えて一時ファイルへ退避している最中に切れた場合に一時ファイルが残らず（`abortSpill`）、同じ executionID を再利用できることを確かめていない（`net_client.go:226-239`・`writeSpill` の読み込みエラー経路が未到達）。
- **根拠コード**: `internal/infrastructure/http/net_client.go:223-272`・`328-341`、`internal/infrastructure/http/response_store.go:207-223`
- **実インフラで確かめる理由**: 読み込みエラーと書き込みエラーの区別、作りかけの一時ファイルの削除は、実際に切れる TCP 接続と実ファイルの組み合わせでしか通らないため。
- **推奨テスト名**: `TestHTTP_SendRequest_ConnectionDroppedMidBody`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。サーバーは `Content-Length: 4MiB` を宣言し、(a) 512 KiB、(b) 2 MiB（`MaxResponseBodyMB: 1` を超える）を書いた後に `http.Hijacker` で接続を閉じる。どちらもエラーになり、`http-sessions` 配下に `response-*` が残らず、同じ executionID での再送が成功することを確かめる。
- **優先度**: 中

**G4. 保存先に書けない場合の再試行**
- **対象メソッド**: `SaveResponseBody`
- **不足内容**: 保存ダイアログが書けないパス（存在しないディレクトリ配下）を返したとき `ErrSaveResponseFailed` になり、一時ファイルが ready に戻って、正しいパスでの再保存が成功することを、実ファイルで確かめていない。
- **根拠コード**: `internal/adapters/http_handler.go:195-197`、`internal/infrastructure/http/response_store.go:350-389`
- **実インフラで確かめる理由**: 単体テスト（`TestResponseStore_CopyFailureRestoresReady`）は `ops` を差し替えて失敗を注入している。実 FS のエラーで同じ経路を通るかは統合でしか見られないため。
- **推奨テスト名**: `TestHTTP_SaveResponseBody_UnwritableDestinationThenRetry`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。A1 と同じ準備で、`savePath` を `filepath.Join(t.TempDir(), "missing", "out.bin")` → 正しいパスの順に変える。
- **優先度**: 低

##### [観点 H] 時間の境界

**H1. 本文の受信中のキャンセルとタイムアウト**
- **対象メソッド**: `SendRequest` / `CancelRequest`
- **不足内容**: `TestHTTP_CancelRequest` と `TestHTTP_SendRequest_Timeout` は、どちらもサーバーが応答ヘッダーを返す前に止めている。ヘッダーを返した後、本文の途中で止まるサーバーに対するキャンセルと、`request timed out after ...` への変換（`net_client.go:226-227`・`236-238` の `wrapTimeout`）が未到達。
- **根拠コード**: `internal/infrastructure/http/net_client.go:223-247`・`406-414`
- **実インフラで確かめる理由**: 読み込み中の context 取り消しがどの層でどのエラーになるかは、実際のストリーミング応答でしか決まらないため。
- **推奨テスト名**: `TestHTTP_SendRequest_CancelAndTimeoutDuringBody`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。サーバーはヘッダーと本文の一部を `Flush` した後、`r.Context().Done()` まで待つ。キャンセル版は `CancelRequest` 後にエラーで復帰すること、タイムアウト版（`TimeoutSec: 1`）はエラー文言に `timed out` を含むことを確かめる。
- **優先度**: 低

##### [観点 I] 並行性の境界

**I1. コレクションへの並行書き込み**
- **対象メソッド**: `AddRequest` / `CreateCollection` / `MoveItem`
- **不足内容**: 同じコレクションへの並行 `AddRequest` と、並行 `CreateCollection` で、更新が失われずにディスクへ残ることを確かめていない。CI の統合テストのジョブ（`test-go-integration`）は `-race` なしなので、層をまたぐこの経路で競合があっても検出されない（`CollectionService` の単体テストは `test-go-unit` の `-race` で回っている）。
- **根拠コード**: `internal/application/http/collection_service.go:323-351`（Clone → 保存 → キャッシュ差し替え）
- **実インフラで確かめる理由**: 実ファイルへの原子的な書き込み（`AtomicWriteFile`）が並行に走った結果を、再起動後の読み直しで確かめるため。
- **推奨テスト名**: `TestHTTP_ConcurrentAddRequest_NoLostUpdates`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。20 goroutine から同じコレクションへ `AddRequest` し、全件成功すること、同じ `dir` で作り直した Handler に 20 件あることを確かめる。
- **優先度**: 低

##### [観点 J] 環境の境界

**J1. Windows 固有の選択ファイルの開き方**
- **対象メソッド**: `OpenFilePicker` / `SendRequest`
- **不足内容**: 他のハンドルが書き込み用に開いているファイルが `ErrSelectedFileInUse` になること、`MAX_PATH` を超えるパスのファイルを送れること（`longPath` → `CreateFile`）を、Handler から確かめていない。`long_path.go` の `isWindowsAbs` は統合テストで 0%。
- **根拠コード**: `internal/infrastructure/http/file_open_windows.go:17-31`、`internal/infrastructure/http/long_path.go:8-17`
- **実インフラで確かめる理由**: Windows の共有モードと長いパスの扱いは OS の実 API でしか現れないため。infrastructure の単体テスト（`TestOpenSelectedFile_WindowsRejectsFileOpenForWriting`）はあるが、token からパスを引いて開くまでの経路は通っていない。
- **推奨テスト名**: `TestHTTP_SendRequest_WindowsSelectedFile`
- **実装ガイド**: 新しいファイル `http_windows_test.go`（`//go:build integration && windows` / `package integration`）に追加する。長いパスは `t.TempDir()` 配下にディレクトリを重ねて 300 文字超にする。作成は `os.MkdirAll` / `os.WriteFile` に**接頭辞の無い絶対パス**を渡せばよい（`os` パッケージが内部で `\\?\` を付ける）。`fileDialog` に登録するのも**接頭辞の無い絶対パス**にすること。`\\?\` 付きのパスを登録すると、`longPath` は既に接頭辞があるものとしてそのまま返す（`long_path.go:10`）ので、変換の分岐を通らない。**CI（ubuntu）では実行されない**ので、ローカルの Windows で回す旨をファイルのコメントに書く。
- **優先度**: 中

**J2. 複数ファイルへの書き込みの途中で失敗した場合の巻き戻し**
- **対象メソッド**: `MoveItem`（コレクション間）/ `CreateCollection` / `AddRequest`
- **不足内容**: 実ディスクへの書き込みが途中で失敗したとき、先に成功した書き込みが `uow.Rollback` でディスク上も元に戻り、キャッシュも変わらないことを確かめていない（`Rollback` 0%）。
- **根拠コード**: `internal/application/http/unit_of_work.go:35-48`（undo は保存に**成功したときだけ**積む）・`69-76`、`internal/application/http/collection_service.go:484-497`（コレクション間の `MoveItem` は移動先 → 移動元の順に 2 ファイルを書く）、`internal/infrastructure/atomic_write.go:13-40`
- **実インフラで確かめる理由**: 実ディスクの書き込み失敗で巻き戻しが走り、先に書いたファイルが元のバイト列に戻るかは、統合でしか見られないため。
- **注意（`collections/` を消す方法では `Rollback` を確かめられない）**: `CreateCollection` や `AddRequest` は 1 ファイルしか書かないので、最初の保存が失敗した時点で undo は 1 つも積まれておらず、`Rollback` は何もしない。`collections/` ごと消した場合も、最初の保存（`os.CreateTemp`）で失敗するので同じ。巻き戻しを通すには、**1 つ目の保存が成功し、2 つ目が失敗する**操作が必要。
- **推奨テスト名**: `TestHTTP_MoveItem_SecondWriteFailsRollsBack`（巻き戻し）と `TestHTTP_StorageDirRemovedAtRuntime`（1 ファイルの書き込み失敗でキャッシュが変わらないこと）
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。
  1. 巻き戻し: `newHTTPHandlerWithDir` で col-src（リクエスト r1 を持つ）と col-dst を作り、`collections/<col-dst>.json` のバイト列を控える。`collections/<col-src>.json` を削除して**同名の空ディレクトリ**を作る（`AtomicWriteFile` の最後の `os.Rename` が、ディレクトリへの置き換えとして Windows・Linux の両方で失敗する）。`MoveItem(col-src, r1, col-dst, "", -1)` がエラーになり、`collections/<col-dst>.json` が控えたバイト列に戻っていること（移動先は一度書き換わってから `Rollback` で戻る）、`GetCollections` で r1 が col-src に残り col-dst に無いこと、`collections/` に `.tmp-*` が残らないことを確かめる。
  2. 1 ファイルの失敗: `os.RemoveAll(filepath.Join(dir, "collections"))` の後に `CreateCollection` と `AddRequest` がエラーになり、`GetCollections` が変わらないことを確かめる（`Rollback` は通らないが、キャッシュを差し替えないことの確認になる）。
- **優先度**: 低

##### [観点 K] 保存データの復旧

**K1. 必須データ（collections）：正常なファイルとの同居と、壊れた `__root__.json`**
- **対象メソッド**: `GetCollections` / `GetRootItems`（起動時）
- **不足内容**: `TestHTTP_CorruptStorage` は Handler を通さず、壊れたファイル 1 つだけで確かめている。「そのファイルだけ退避してスキップし、残りで起動する」の「残り」と、壊れた `__root__.json` が退避された後に空の `__root__` が作り直されること（`ensureRootCollection` の「ファイルが本当に無い: 破損ファイルの退避に成功した後」）を確かめていない。
- **根拠コード**: `internal/infrastructure/json_store.go:47-57`、`internal/application/http/collection_service.go:100-128`
- **実インフラで確かめる理由**: 退避（リネーム）の結果として「ファイルが無い」状態になり、それを `Exists` で確かめてから作り直すという、実ファイルの状態に依存する分岐のため。
- **推奨テスト名**: `TestHTTP_CorruptCollectionAmongValidOnes`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`writeCollectionJSON` で正常な `col-a.json`、壊れた `col-b.json`、壊れた `__root__.json` を置き、`newHTTPHandlerWithDir` で起動する。`GetCollections` が col-a だけ、`GetRootItems` が空、`col-b.json.corrupt` と `__root__.json.corrupt` が元のバイト列のまま存在し、新しい `__root__.json` があって `AddRequest(RootCollectionID, ...)` が成功することを確かめる。
- **優先度**: 中

**K2. 退避先の衝突**
- **対象メソッド**: 起動時の読み込み（`GetCollections`）
- **不足内容**: `<path>.corrupt` が既にある場合に `<path>.corrupt.<unixnano>` へ退避し、既存の退避ファイルを上書きしないことを確かめていない（`quarantine.go:18-20` が未到達）。
- **根拠コード**: `internal/infrastructure/quarantine.go:16-25`
- **実インフラで確かめる理由**: 衝突の判定は実ファイルの `os.Stat` で行うため。
- **推奨テスト名**: `TestHTTP_CorruptCollection_ExistingQuarantineKept`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`collections/x.json.corrupt`（中身 "old"）と壊れた `collections/x.json` を置いて起動し、`x.json.corrupt` が "old" のまま、`x.json.corrupt.*` が 1 つできていることを確かめる。
- **優先度**: 低

**K3. 再生成可能データ（sidebar_layout.json）：破損以外の読み込み失敗**
- **対象メソッド**: `GetSidebarLayout` / `MoveSidebarEntry` / `CreateCollection`
- **不足内容**: 表の「再生成した値で動作を続け、ファイルは退避しない」のセルに対応するテストが無い（`collection_service.go:150-152`・`510-512`、`sidebar_layout_service.go:116-117` が未到達）。
- **根拠コード**: `internal/application/http/collection_service.go:147-162`・`506-518`、`internal/application/http/sidebar_layout_service.go:97-127`
- **実インフラで確かめる理由**: `sidebar_layout.json` を**ディレクトリにする**と、Windows・Linux の両方で `os.ReadFile` が ErrNotExist でも破損でもないエラーを返すので、権限やロックを使わずにこのセルを決定的に再現できるため。
- **推奨テスト名**: `TestHTTP_UnreadableSidebarLayout_RegeneratesWithoutQuarantine`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。コレクションを 2 つ置き、`os.Mkdir(filepath.Join(dir, "sidebar_layout.json"), 0o750)` してから `newHTTPHandlerWithDir` で起動する。起動が成功し、`GetSidebarLayout` が名前順の 2 件を返し、`sidebar_layout.json.corrupt` が無く、ディレクトリがそのまま残ることを確かめる。`MoveSidebarEntry` はエラーを返し、`CreateCollection` は成功する（レイアウトは best effort）ことも見る。
- **優先度**: 中

**K4. 必須データ：ディレクトリ自体を読めない場合の起動失敗**
- **対象メソッド**: 起動（`NewCollectionRepository` / `NewCollectionService`）
- **不足内容**: 表の「ディレクトリ自体を読めなければ起動失敗」を確かめていない（`json_store.go:29-30`・`38-39` が未到達）。
- **根拠コード**: `internal/infrastructure/json_store.go:27-39`
- **実インフラで確かめる理由**: `collections` を**通常のファイルにする**と `MkdirAll` が失敗するので、OS に依存せず再現できるため。
- **推奨テスト名**: `TestHTTP_CollectionsDirUnusable_FailsStartup`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`buildHTTPHandler` は失敗を `t.Fatalf` にするので、`TestHTTP_CorruptStorage` と同じく DI を直接組み立て、`NewCollectionRepository` か `NewCollectionService` がエラーを返すことを確かめる。
- **優先度**: 低

**観点 K で対象外にしたセル**: 「退避失敗時」の各セル（必須・best effort・再生成可能）は、実ファイルシステムでリネームだけを決定的に失敗させる方法が無く、`infrastructure.quarantine` はパッケージ外から差し替えられない。これらは単体テスト（`internal/infrastructure/json_store` の各テスト、`TestLoadSingleFile`、`TestNewFileService_CorruptQuarantineFailsDoesNotSave`）で対応済みとして、不足には数えない。**CLAUDE.md の表とコードの食い違いは見つからなかった。**

##### [観点 L] イベント発行

該当なし（HTTP はイベントを発行しない）。

##### [観点 M] RPC の信頼境界

**M1. `AddRequest` が呼び出し側の指定した ID を重複の確認なしに受け入れる**
- **対象メソッド**: `AddRequest`
- **不足内容**: `AddRequest` は `req.ID` が空でなければそのまま TreeItem の ID に使い、既存 ID との重複を確かめない。JS から既存アイテムと同じ ID で別のコレクションへ `AddRequest` すると、次回起動時の `recoverDuplicateItems` が片方（`__root__` を先頭に、コレクション ID 昇順で最初の出現以外）を**黙って削除**する。フロントエンドは常に `id: ""` を渡している（`frontend/src/presentation/components/sidebar/collection-tree.tsx:55`・`101`）ので、通常操作では起きないが、RPC の境界としては防がれていない。
- **根拠コード**: `internal/application/http/collection_service.go:305-321`（`if r.ID == "" { r.ID = uuid.NewString() }`）、`internal/domain/http/types.go:416-427`（`AppendItem` は重複を見ない）、`internal/application/http/reconcile.go:16-31`
- **実インフラで確かめる理由**: 被害（データの消失）は再起動時の回収処理で初めて現れ、層と再起動をまたがないと観測できないため。
- **推奨テスト名**: `TestHTTP_AddRequest_RejectsDuplicateItemID`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。col-a にリクエストを作り、その ID を指定して col-b へ `AddRequest` する。期待値は「エラーになる」か「サーバー側で新しい ID を採番する」のどちらかで、**現状のコードではこのテストは失敗する見込み**なので、仕様を決めてから直すこと。どちらの仕様でも、同じ `dir` で作り直した Handler で col-a のアイテムが残っていることを確かめる。
- **優先度**: 高

**M2. ストア外を指すコレクション ID**
- **対象メソッド**: `DeleteCollection` / `RenameCollection` / `AddRequest` / `MoveItem` / `MoveItemToSidebar`
- **不足内容**: MQTT・UDP にある `traversalIDs` のテストが HTTP には無い。コレクション ID はキャッシュ引きで弾かれる（`collection_service.go:242-245` など）ので現状は安全だが、層を貫いて確かめていない。
- **根拠コード**: `internal/application/http/collection_service.go:233-283`・`419-430`、`internal/infrastructure/json_store.go:88-99`
- **実インフラで確かめる理由**: ストアの外にファイルができないことは、実ディレクトリを見て確かめるため。
- **推奨テスト名**: `TestHTTP_CollectionOps_RejectTraversalID`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`base := t.TempDir()`、`dir := filepath.Join(base, "store")` で組み立て、各操作が `*domain.NotFoundError` になった後、`assertNoFilesOutside(t, base, "store")` と、`dir/collections` に `__root__.json` 以外が無いことを確かめる。
- **優先度**: 低

**M3. `SaveResponseBody` が保存元を持たない ID を、ダイアログを開く前に拒否すること**
- **対象メソッド**: `SaveResponseBody` / `DiscardResponseBody`
- **不足内容**: 未追跡・偽の executionID（`"../../etc"`、64 文字超など）で、実 `ResponseStore` がダイアログを開く前に拒否することを確かめていない（単体テスト `TestSaveResponseBody_RejectsBeforeDialog` は偽のストア）。
- **根拠コード**: `internal/adapters/http_handler.go:178-185`、`internal/infrastructure/http/response_store.go:248-264`
- **実インフラで確かめる理由**: 本物のストアと Handler の組み合わせで、ダイアログ（= ユーザーへの保存確認）が出ないことを確かめるため。
- **推奨テスト名**: `TestHTTP_SaveResponseBody_RejectsUntrackedBeforeDialog`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。A1 の `fileDialog.saveCalls` を使い、`newHTTPHandlerWithDialog` で組み立てる。`errors.Is(err, httpdomain.ErrResponseUnavailable)` と `saveCalls == 0` を確かめる。
- **優先度**: 低

---

### MQTT (`mqtt_test.go`)

#### 対象メソッド（Handler の公開メソッド）

| メソッド名 | ファイル:行 | テスト済み? | テストケース名 |
| --- | --- | --- | --- |
| Connect | mqtt_handler.go:41 | 済 | TestMQTT_ConnectDisconnect / TestMQTT_Connect_UnreachableBroker / TestMQTT_Connect_EmptyBroker ほか |
| Disconnect | mqtt_handler.go:46 | 済 | TestMQTT_ConnectDisconnect / TestMQTT_Disconnect_NotFound / TestMQTT_Disconnect_Twice |
| Publish | mqtt_handler.go:51 | 済 | TestMQTT_SubscribePublishQoS0/1/2 / TestMQTT_Publish_InvalidInput |
| Subscribe | mqtt_handler.go:56 | 済 | TestMQTT_SubscribePublishQoS0/1/2 / TestMQTT_Subscribe_Wildcard / TestMQTT_Subscribe_InvalidInput |
| Unsubscribe | mqtt_handler.go:61 | 済 | TestMQTT_Unsubscribe / TestMQTT_Unsubscribe_EmptyTopic |
| GetConnections | mqtt_handler.go:66 | 済 | TestMQTT_GetConnections / TestMQTT_Shutdown ほか（`Subscriptions` の中身は未検証） |
| GetProfiles | mqtt_handler.go:71 | 済 | TestMQTT_ProfileCRUD / TestMQTT_ProfilePersistenceRoundTrip |
| SaveProfile | mqtt_handler.go:77 | 済 | TestMQTT_ProfileCRUD / TestMQTT_SaveProfile_RejectsTraversalID |
| DeleteProfile | mqtt_handler.go:82 | 済 | TestMQTT_ProfileCRUD / TestMQTT_SaveProfile_RejectsTraversalID |

**状態を持つ箇所**: 接続マップ `MQTTService.conns` と各接続の `state` / `subs`（application）、paho クライアントの自動再接続（infrastructure）。発行するイベントは `mqtt:connected` / `mqtt:disconnected` / `mqtt:connection-lost` / `mqtt:connection-failed` / `mqtt:message`。

**既存テストの読み取り**:
- `mqttMockEmitter`（mqtt_test.go:79）が拾うのは `MQTTMessage` と `mqtt:connection-failed` だけで、`mqtt:connected` / `mqtt:disconnected` は総数（`total`）にしか現れない。これらのイベントの発行と中身はどのテストも確かめていない。
- 非同期の待ち方はチャネル＋タイムアウト。`waitConnected` は 50ms 間隔のポーリング、`assertSilent` / `noMessage` は「来ないこと」を確かめるための固定待ちで、目的に照らして妥当。
- `TestMQTT_Connect_Concurrent`（mqtt_test.go:793）は `Connect` のエラーを捨て、`Disconnect` の結果も見ていないので、アサーションが無い。
- クリーンアップはテストごとに `t.Cleanup(Disconnect)` か `svc.Shutdown` を登録しているが、ヘルパー（`newMQTTHandlerWithDir`）自体は `Shutdown` を登録しない。

#### 不足テストケース

##### [観点 A] ゴールデンパスの完全性

不足なし（`Connect` → `Subscribe` → `Publish` → 受信イベント → `Unsubscribe` → `Disconnect` は各テストで通っている）。

##### [観点 B] 再起動をまたぐ引き継ぎ

**B1. プロファイルの更新・削除が再起動後に残るか**
- **対象メソッド**: `SaveProfile`（更新）/ `DeleteProfile`
- **不足内容**: `TestMQTT_ProfilePersistenceRoundTrip`（mqtt_test.go:584）は新規作成だけ。更新と削除を同じ `dir` での作り直しで確かめていない。
- **根拠コード**: `internal/application/store/cached_store.go:74-103`
- **実インフラで確かめる理由**: 更新は同名ファイルの原子的な置き換え、削除は `os.Remove` で、どちらも実ファイルの結果を再読み込みで確かめるため。
- **推奨テスト名**: `TestMQTT_ProfileUpdateDelete_SurviveRestart`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。`newMQTTHandlerWithDir` を同じ `dir` で 2 回呼ぶ。2 件保存 → 1 件を更新・1 件を削除 → 作り直した Handler の `GetProfiles` を確かめる。
- **優先度**: 低

##### [観点 C] リソースリークと終了処理

**C1. ヘルパーでの `Shutdown` 登録とブローカー側の接続数**
- **対象メソッド**: `Connect` と `MQTTService.Shutdown`
- **不足内容**: ヘルパーが `Shutdown` を登録しないため、`Connect` の直後から `t.Cleanup` の登録までの間に `t.Fatal` すると、接続がブローカーに残る。テスト終了時にブローカー側の接続が残っていないことも確かめていない。
- **根拠コード**: `internal/integration/mqtt_test.go:145-159`
- **実インフラで確かめる理由**: 共有の埋め込みブローカーに接続が残ると、後続のテスト（同じトピック名を使うもの）へ影響しうるため。
- **推奨テスト名**: （ヘルパーの変更）`newMQTTHandlerWithDir` で `t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })` を登録する
- **実装ガイド**: `mqtt_test.go` のヘルパーを変更する（`//go:build integration` / `package integration`）。`Shutdown` は 2 回目以降に true を返す（`service.go:387-390`）ので、個別テストの `t.Cleanup` と重なっても安全。ブローカー側の確認が必要なら、`TestMain` の `server` をパッケージ変数にして `server.Clients.Len()` を見る。
- **優先度**: 低

##### [観点 D] 入力値の境界

**D1. 不正なブローカー URL**
- **対象メソッド**: `Connect`
- **不足内容**: 空文字（`TestMQTT_Connect_EmptyBroker`）以外の不正な URL を確かめていない。Service は空文字しか検証しない（`service.go:96-98`）。paho v1.5.1 の `AddBroker`（`options.go:170-184`）は、スキームが無ければ `tcp://` を補い、`url.Parse` に失敗した URL はログに出すだけでブローカーを追加しない。このため不正な URL は 2 通りに分かれる:
  1. **未対応のスキーム**（`http://127.0.0.1:1883`）: 解析には成功して追加され、接続時に `unknown protocol`（paho `netconn.go:100`）で失敗する。
  2. **解析できない URL**（`tcp://[::1`、`tcp://%zz`）: ブローカーが追加されず、接続時に `no servers defined to connect to`（paho `client.go:253-254`）で失敗する。
  なお、スキームの無い `127.0.0.1:1883` は `tcp://127.0.0.1:1883` として扱われる**正しい入力**なので、不正な URL の例には使わない（むしろ、スキームを省いても埋め込みブローカーに接続できることを正常系として固定してよい）。
- **根拠コード**: `internal/application/mqtt/service.go:95-131`、`internal/infrastructure/mqtt/paho_client.go:83-88`、paho v1.5.1 `options.go:170-184`・`client.go:253-254`・`netconn.go:100`
- **実インフラで確かめる理由**: 不正な URL がどう扱われるか（同期エラーか、非同期の `mqtt:connection-failed` か）は paho の実装で決まるため。
- **推奨テスト名**: `TestMQTT_Connect_InvalidBrokerURL`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。`newMQTTHandlerWithConfig` で `ConnectTimeout` / `TokenTimeout` を短くする。`http://127.0.0.1:1883` と `tcp://[::1` のそれぞれで「同期エラー」か「`waitConnectionFailed` で ID が一致し、`GetConnections` が空」のどちらかになり、`mqtt:connected` が出ないことを確かめる。別のケースとして、`127.0.0.1:<埋め込みブローカーのポート>`（スキームなし）で接続に成功することも確かめる。
- **優先度**: 中

**D2. ワイルドカードの位置違反とトピックへのワイルドカード**
- **対象メソッド**: `Subscribe` / `Publish`
- **不足内容**: 位置違反の購読（`a/#/b`）と、ワイルドカードを含むトピックへの publish（`a/+`）を確かめていない。コードから読める問題が 2 つある:
  1. `pahoClient.Subscribe` は `token.Error()` しか返さず SUBACK の結果（`SubscribeToken.Result()`）を見ないので、ブローカーが拒否した購読も成功扱いになり、`conn.subs` に残って `GetConnections` に表示される。
  2. Service も paho もトピックのワイルドカードを検証しない。埋め込みブローカー（mochi v2.7.9 の `processPublish`）は不正なトピックの PUBLISH を PUBACK なしで捨てるので、QoS 1/2 の `Publish` は `TokenTimeout`（既定 30 秒）まで復帰せず、その間 `opMu` を握って同じ接続の `Disconnect` を待たせる見込み。
- **根拠コード**: `internal/infrastructure/mqtt/paho_client.go:148-165`、`internal/application/mqtt/service.go:272-324`
- **実インフラで確かめる理由**: 拒否の仕方はブローカーの実装で決まり、実ブローカーでしか再現できないため。
- **推奨テスト名**: `TestMQTT_InvalidWildcardTopics`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。`newMQTTHandlerWithConfig` で `TokenTimeout: 2*time.Second` にする。`Subscribe(id, "a/#/b", 0)` がエラーを返すこと（または `GetConnections` の `Subscriptions` に残らないこと）、`Publish(id, "a/+", "x", 1, false)` が `TokenTimeout` より十分短くエラーで返ることを期待値とする。**現状のコードでは失敗する見込み**なので、Service でトピックを検証するか、SUBACK の結果を見るかを決めてから直すこと。上の挙動は実装からの推定で、実測で確かめる。
- **優先度**: 中

##### [観点 E] 状態遷移の境界

**E1. 切断後・Shutdown 後の操作**
- **対象メソッド**: `Publish` / `Subscribe` / `Unsubscribe` / `Connect`
- **不足内容**: `Disconnect` 後の `Publish` / `Subscribe` / `Unsubscribe` が `NotFoundError` になること、`Shutdown` 後の `Connect` が拒否され接続 goroutine を起動しない（`service.go:122-125` が未到達）ことを確かめていない。
- **根拠コード**: `internal/application/mqtt/service.go:119-130`・`252-269`
- **実インフラで確かめる理由**: 切断した paho クライアントに操作が届かないこと、Shutdown 後にブローカーへ新しい接続が張られないことは、実クライアントと実ブローカーで確かめるため。Service 単体では `TestMQTTService_Connect_AfterShutdown_Rejected` で対応済み。
- **推奨テスト名**: `TestMQTT_OperationsAfterDisconnectAndShutdown`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。`errors.As(err, &nf)` で `*cmndomain.NotFoundError` を確かめる。Shutdown 後の `Connect` はエラーになり、`emitter.assertSilent` でイベントが出ないことを見る。
- **優先度**: 中

##### [観点 F] 組み合わせの境界

**F1. 同じ接続での重なる購読**
- **対象メソッド**: `Subscribe` / `Publish`（`mqtt:message`）
- **不足内容**: 同じ接続で `test/#` と `test/specific` を購読したときに、1 回の publish で `mqtt:message` が何回出るかを確かめていない。paho のルーターは一致するハンドラーをすべて呼び、Service はハンドラーごとにイベントを出すので、重複して出る可能性がある。
- **根拠コード**: `internal/application/mqtt/service.go:295-322`、`internal/infrastructure/mqtt/paho_client.go:156-165`
- **実インフラで確かめる理由**: 重複の回数はブローカーの配信（MQTT 3.1.1 では重なる購読の配信回数は実装依存）と paho のルーティングの組み合わせで決まるため。
- **推奨テスト名**: `TestMQTT_OverlappingSubscriptions`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。1 回の publish の後、一定時間に受け取った件数を数え、期待する回数（UI として 1 回か、購読ごとか）を決めて固定する。
- **優先度**: 中

**F2. QoS の組み合わせと retained**
- **対象メソッド**: `Subscribe` / `Publish`
- **不足内容**: Subscribe と Publish の QoS が異なる場合（購読 0・送信 2 → 受信 QoS 0）と、`retain: true` のメッセージを後から購読したときに `Retained == true` で届くことを確かめていない。
- **根拠コード**: `internal/application/mqtt/service.go:306-314`
- **実インフラで確かめる理由**: QoS の引き下げと retained の配信はブローカーが行うため。
- **推奨テスト名**: `TestMQTT_QoSDowngradeAndRetained`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。retained は共有ブローカーに残るので、テスト名を含む一意なトピックを使い、最後に空ペイロードの retained で消す。
- **優先度**: 低

##### [観点 G] 異常系の境界

**G1. 接続断・自動再接続と購読の復元**
- **対象メソッド**: `Connect` / `Subscribe` / `GetConnections`（`mqtt:connection-lost` / `mqtt:connected`）
- **不足内容**: `onConnectionLost` は統合テストで 0%。ブローカーとの接続が切れたときに `mqtt:connection-lost` が出ること、その間の `GetConnections` の `Connected` が false になること、paho の自動再接続で `mqtt:connected` が再び出ることを確かめていない。さらに、`onConnected` は再接続時に購読し直さず、paho は既定で CleanSession のため、**再接続後はブローカー側の購読が失われているのに `GetConnections` の `Subscriptions` には残る**可能性がある。`SetResumeSubs(true)` を設定しているが、これは送信待ちのまま保存された SUBSCRIBE / UNSUBSCRIBE パケットを再送するだけで（paho `options.go:186-191`、`client.go` の `resume`）、**SUBACK まで済んだ購読を再接続後に張り直すものではない**。したがって `ResumeSubs` があることは、この不足の反証にならない。
- **根拠コード**: `internal/application/mqtt/service.go:179-204`、`internal/infrastructure/mqtt/paho_client.go:90-91`（`SetAutoReconnect(true)` / `SetResumeSubs(true)`）、paho v1.5.1 `options.go:186-191`
- **実インフラで確かめる理由**: 接続断と再接続は実ソケットと実ブローカーの再起動でしか起こせず、購読が残るかはブローカーのセッションの扱いで決まるため。
- **推奨テスト名**: `TestMQTT_ConnectionLostAndReconnect`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。共有ブローカーを止めるわけにはいかないので、テスト内で専用の mochi サーバーを `127.0.0.1:<port>` で起動する（`TestMain` と同じ組み立て）。接続・購読の後に `server.Close()` し、`mqtt:connection-lost` を待つ（`mqttMockEmitter` にイベント名ごとのチャネルを足す）。同じポートでサーバーを起動し直し、`mqtt:connected` を待ってから別接続で publish し、受信できるかを確かめる。受信できない場合は、`onConnected` で `conn.subs` を購読し直す修正の要否を決める。
- **優先度**: 高

##### [観点 H] 時間の境界

**H1. 応答しないブローカーへの接続タイムアウト**
- **対象メソッド**: `Connect`（`mqtt:connection-failed`）
- **不足内容**: `blackholeBroker` は Shutdown のテストにしか使われていない。接続を受け付けるが CONNACK を返さないブローカーで `TokenTimeout` が切れ、`mqtt:connection-failed`（エラー `connection timed out`）が出て接続が消えることを確かめていない（閉じたポートは即座に失敗するので、この経路を通らない）。
- **根拠コード**: `internal/infrastructure/mqtt/paho_client.go:115-146`、`internal/application/mqtt/service.go:155-169`
- **実インフラで確かめる理由**: dial は成功するが応答が無いという状況は、実 TCP リスナーでしか作れないため。
- **推奨テスト名**: `TestMQTT_Connect_BlackholeBrokerTimesOut`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。`newMQTTHandlerWithConfig` で `ConnectTimeout: 1s` / `TokenTimeout: 1s` にし、`waitConnectionFailed` で ID を確かめ、イベントの `error` に `timed out` が含まれることも見る（`waitConnectionFailed` をイベントの map ごと返すように拡張する）。
- **優先度**: 低

##### [観点 I] 並行性の境界

**I1. 並行 Connect / Publish のアサーション**
- **対象メソッド**: `Connect` / `Disconnect` / `Publish`
- **不足内容**: `TestMQTT_Connect_Concurrent` にはアサーションが無く、CI の統合テストのジョブ（`test-go-integration`）は `-race` なしなので実質何も検証していない。同じ接続からの並行 `Publish` も無い。
- **根拠コード**: `internal/integration/mqtt_test.go:793-820`、`internal/application/mqtt/service.go:250-285`（`opMu` による直列化）
- **実インフラで確かめる理由**: paho のトークンと `opMu` の直列化の組み合わせで、並行 publish が全件ブローカーに届くかは実クライアントでしか確かめられないため。
- **推奨テスト名**: `TestMQTT_Connect_Concurrent` の強化と `TestMQTT_ConcurrentPublish`
- **実装ガイド**: `mqtt_test.go` を変更・追加する（`//go:build integration` / `package integration`）。全 `Connect` の成功・ID の一意性・全接続の `Connected`・全 `Disconnect` の nil・最後に `GetConnections` が空を確かめる。並行 publish は QoS 1 で 50 件送り、受信数が 50 件になることを確かめる（`mqttMockEmitter` のバッファ 16 は溢れると黙って捨てるので、バッファを広げる）。
- **優先度**: 中

##### [観点 J] 環境の境界

該当なし（MQTT 固有の OS 別コードは無い。IPv6 のループバックは CI のコンテナで無効なことがあり、決定的に再現できない）。

##### [観点 K] 保存データの復旧

**K1. 必須データ（mqtt-profiles）の破損と読み込み失敗**
- **対象メソッド**: `GetProfiles` / `SaveProfile`（起動時）
- **不足内容**: MQTT プロファイルの破損ファイルのテストが 1 つも無い（HTTP・UDP にはある）。
- **根拠コード**: `internal/infrastructure/mqtt/profile_repository.go:21-37`、`internal/infrastructure/json_store.go:47-72`
- **実インフラで確かめる理由**: 退避のリネームと、残りのファイルでの起動は実ファイルで確かめるため。
- **推奨テスト名**: `TestMQTT_CorruptProfileAmongValidOnes`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。`dir` に正常なプロファイル（1 回目の Handler で保存）、壊れた `x.json`、中身の ID が `../escape` のファイル（破損以外で読み飛ばされる代用）を置き、`newMQTTHandlerWithDir` で起動する。`GetProfiles` が正常な 1 件だけ、`x.json.corrupt` があり、ID が不正なファイルは退避されずに残り、新規の `SaveProfile` が成功することを確かめる。`dir` を通常のファイルにした場合の起動失敗は DI を直接組み立てて確かめる。
- **優先度**: 中

##### [観点 L] イベント発行

**L1. `mqtt:connected` / `mqtt:disconnected` の発行と中身、失敗時に `connected` が出ないこと**
- **対象メソッド**: `Connect` / `Disconnect`
- **不足内容**: 接続・切断のイベントを、どのテストも名前と `connectionId` まで確かめていない。接続失敗で `mqtt:connection-failed` だけが出て `mqtt:connected` が出ないことも確かめていない。
- **根拠コード**: `internal/application/mqtt/service.go:179-190`・`207-227`・`155-162`
- **実インフラで確かめる理由**: paho は `onConnected` を `Connect` の復帰より先に呼ぶことがあり（`service.go:171-174`）、イベントの順序と回数は実クライアントのコールバックで決まるため。
- **推奨テスト名**: `TestMQTT_LifecycleEvents`
- **実装ガイド**: `mqtt_test.go` に追加する（`//go:build integration` / `package integration`）。`mqttMockEmitter` を「イベント名とデータの組」を順に記録する形に拡張する。`Connect` → `mqtt:connected` が 1 回（`connectionId` 一致）、`Disconnect` → `mqtt:disconnected` が 1 回で、その後は `assertSilent`。`TestMQTT_Connect_UnreachableBroker` と同じ条件では `mqtt:connected` が 0 回であることを確かめる。
- **優先度**: 中

##### [観点 M] RPC の信頼境界

不足なし（`TestMQTT_SaveProfile_RejectsTraversalID` でストア外を指す ID を確かめている。ファイル中の不正 ID の読み飛ばしは K1 に含めた）。

---

### UDP (`udp_test.go`)

#### 対象メソッド（Handler の公開メソッド）

| メソッド名 | ファイル:行 | テスト済み? | テストケース名 |
| --- | --- | --- | --- |
| Send | udp_handler.go:42 | 済 | TestUDP_SendRaw / TestUDP_SendFixed / TestUDP_SendJSON / TestUDP_Send_EmptyHost / TestUDP_Send_InvalidPort / TestUDP_Send_UnreachableHost |
| GetTargets | udp_handler.go:47 | 済 | TestUDP_TargetCRUD / TestUDP_TargetPersistenceRoundTrip |
| SaveTarget | udp_handler.go:52 | 部分的 | TestUDP_TargetCRUD（新規作成のみ。更新と不正な host/port は未検証） |
| DeleteTarget | udp_handler.go:57 | 済 | TestUDP_TargetCRUD / TestUDP_SaveTarget_RejectsTraversalID |
| StartListen | udp_handler.go:64 | 済 | TestUDP_ListenerStartStop / TestUDP_StartListen_* |
| StopListen | udp_handler.go:69 | 済 | TestUDP_ListenerStartStop（二重停止を含む） |
| GetListeners | udp_handler.go:74 | 済 | TestUDP_GetListeners |
| Shutdown | udp_handler.go:79 | 部分的 | TestUDP_Shutdown（`GetListeners` が空になることだけ。ポートの解放とイベントの停止は未検証） |

**状態を持つ箇所**: リスナーセッション `UDPListenerService.sessions` と受信 goroutine（application）、`net.PacketConn`（infrastructure）。発行するイベントは `udp:message`。

**既存テストの読み取り**:
- `mockEmitter`（udp_test.go:30）はイベント名を無視して `UDPReceivedMessage` だけを拾う。`RemoteAddr`・`Encoding`・`Timestamp` はどのテストも見ていない。
- `TestUDP_SendJSON`（udp_test.go:471）は受信ペイロードを確かめていない（`SessionID` だけ）。
- `TestUDP_StopListen_RaceWithSend`・`TestUDP_Concurrent_StartStopListen` はアサーションが無く、コメントどおり race detector に頼っているが、CI の統合テストのジョブ（`test-go-integration`）は `-race` なしで回している（`-race` 付きの `test-go-unit` は `integration` タグを付けないので、これらのテストを含まない）。
- 非同期の待ち方はチャネル＋タイムアウト。クリーンアップは `t.Cleanup(StopListen)` / `Shutdown` で行っている。

#### 不足テストケース

##### [観点 A] ゴールデンパスの完全性

不足なし（`StartListen` → `Send` → 受信イベント → `StopListen` は `TestUDP_SendRaw` などで通っている）。

##### [観点 B] 再起動をまたぐ引き継ぎ

**B1. ターゲットの更新・削除が再起動後に残るか**
- **対象メソッド**: `SaveTarget`（更新）/ `DeleteTarget`
- **不足内容**: `SaveTarget` の更新経路（既存 ID）は統合テストで一度も通っていない。削除後の再起動も確かめていない。
- **根拠コード**: `internal/application/udp/target_service.go:34-44`、`internal/application/store/cached_store.go:74-103`
- **実インフラで確かめる理由**: MQTT B1 と同じ（同名ファイルの置き換えと削除を再読み込みで確かめる）。
- **推奨テスト名**: `TestUDP_TargetUpdateDelete_SurviveRestart`
- **実装ガイド**: `udp_test.go` に追加する（`//go:build integration` / `package integration`）。`newUDPHandlerWithDir` を同じ `dir` で 2 回呼ぶ。不正な host/port での更新が `*cmndomain.ValidationError` になりファイルが変わらないことも確かめる。
- **優先度**: 低

##### [観点 C] リソースリークと終了処理

**C1. `Shutdown` 後のポートの解放とイベントの停止**
- **対象メソッド**: `Shutdown` / `StartListen`
- **不足内容**: `TestUDP_Shutdown`（udp_test.go:287）は `GetListeners` が空になることしか見ていない。ソケットが閉じてポートが再バインドできること、Shutdown 後に送ったパケットがイベントにならないことを確かめていない。
- **根拠コード**: `internal/application/udp/listener_service.go:103-115`・`118-125`
- **実インフラで確かめる理由**: ソケットが OS から本当に解放されたかは、実ポートの再バインドでしか確かめられないため。
- **推奨テスト名**: `TestUDP_Shutdown_ReleasesPortsAndStopsEvents`
- **実装ガイド**: `udp_test.go` に追加する（`//go:build integration` / `package integration`）。2 ポートで待ち受け、`Shutdown` の後、同じポートでの `StartListen` が成功すること（新しいセッションは `t.Cleanup` で止める）と、その前に `Send` したパケットについて `mockEmitter` にメッセージが来ないこと（`noMessage` 相当のヘルパーを足す）を確かめる。
- **優先度**: 中

##### [観点 D] 入力値の境界

**D1. ペイロードのサイズ（0 バイト / 最大データグラム超）**
- **対象メソッド**: `Send`
- **不足内容**: 0 バイトの送信（空のメッセージとして届くか）と、UDP の最大ペイロード（IPv4 で 65,507 バイト）を超える送信のエラーを確かめていない。Validate はサイズを見ず、`conn.Write` のエラーに任せている。
- **根拠コード**: `internal/domain/udp/types.go:109-117`、`internal/infrastructure/udp/net_socket.go:23-31`、`internal/application/udp/send_service.go:41-45`
- **実インフラで確かめる理由**: 上限超過は OS のソケットが返すエラーで決まり、0 バイトのデータグラムの受信可否も実ソケットでしか確かめられないため。
- **推奨テスト名**: `TestUDP_Send_PayloadSizeBoundaries`
- **実装ガイド**: `udp_test.go` に追加する（`//go:build integration` / `package integration`）。`Payload: ""` で `BytesSent == 0` と空ペイロードの受信、`strings.Repeat("a", 65508)` でエラーかつリスナーに何も届かないことを確かめる。
- **優先度**: 中

**D2. 未知のエンコーディングと不正なペイロード**
- **対象メソッド**: `StartListen` / `Send`
- **不足内容**: `StartListen` は `encoding` 文字列を検証せず、未知の値でも成功して、そのままイベントの `Encoding` に載る（受信は text と同じ扱い）。`Send` の未知のエンコーディング・不正な JSON・不正な hex が `ValidationError` になり、何も送信されないこと（`send_service.go:37-39` が未到達）も確かめていない。
- **根拠コード**: `internal/adapters/udp_handler.go:64-66`、`internal/application/udp/listener_service.go:40-43`、`internal/domain/udp/encoding.go:40-65`
- **実インフラで確かめる理由**: RPC の文字列がドメイン型へ変換されて受信ループのエンコードとイベントまで届く流れは、層を貫いて初めて観測できるため。
- **推奨テスト名**: `TestUDP_UnknownEncodingAndInvalidPayload`
- **実装ガイド**: `udp_test.go` に追加する（`//go:build integration` / `package integration`）。`StartListen(port, "bogus")` の期待値（エラーにするか、受け入れるか）を決めて固定する。現状は受け入れるので、エラーにする仕様ならこのテストは失敗する。`Send` の各不正入力は `errors.As` で `*cmndomain.ValidationError` を確かめ、リスナーに何も届かないことを見る。
- **優先度**: 中

##### [観点 E] 状態遷移の境界

不足なし（二重の `StopListen`・停止後の同じポートでの再開始は既存テストで確かめている。停止後のイベントは L1 で扱う）。

##### [観点 F] 組み合わせの境界

**F1. JSON リスナーの受信内容**
- **対象メソッド**: `StartListen`（`json`）/ `Send`
- **不足内容**: `TestUDP_SendJSON` は受信ペイロードを確かめていない。JSON リスナーは受信データをインデント付きに整形し、JSON でなければそのまま返すので、両方の分岐を見るべき。
- **根拠コード**: `internal/domain/udp/encoding.go:23-37`
- **実インフラで確かめる理由**: 送信側と受信側のエンコードの組み合わせを実ソケットを通して確かめるため（`EncodePayload` 単体は単体テストで対応済み）。
- **推奨テスト名**: `TestUDP_SendJSON` の強化
- **実装ガイド**: `udp_test.go` を変更する（`//go:build integration` / `package integration`）。`{"key":"value"}` が `"{\n  \"key\": \"value\"\n}"` で届くこと、`not json` がそのまま届くことを確かめる。
- **優先度**: 低

##### [観点 G] 異常系の境界

不足なし（使用中のポート・解決できないホストは既存テストで確かめている。ループバックの閉じたポートへの送信は ICMP の扱いが OS で違い、決定的に再現できない）。

##### [観点 H] 時間の境界

不足なし（受信順序は UDP では保証されないので対象外。`Send` 直後の `StopListen` は I1 で扱う）。

##### [観点 I] 並行性の境界

**I1. 並行テストのアサーション**
- **対象メソッド**: `StartListen` / `StopListen` / `Send`
- **不足内容**: `TestUDP_Concurrent_StartStopListen`・`TestUDP_StopListen_RaceWithSend` は結果を見ていないので、CI の統合テストのジョブ（`-race` なし）では何も検証していない。並行 `Send` のテストも無い。
- **根拠コード**: `internal/integration/udp_test.go:543-605`、`internal/application/udp/listener_service.go:45-67`
- **実インフラで確かめる理由**: 実ソケットの開閉と受信 goroutine の終了が並行に起きたときの結果は、実ソケットでしか確かめられないため。
- **推奨テスト名**: `TestUDP_Concurrent_StartStopListen` / `TestUDP_StopListen_RaceWithSend` の強化と `TestUDP_ConcurrentSend`
- **実装ガイド**: `udp_test.go` を変更・追加する（`//go:build integration` / `package integration`）。全 `StartListen` の成功、`GetListeners` の件数、全 `StopListen` の nil、最後に `GetListeners` が空を確かめる。RaceWithSend は両方の戻り値と、終了後に `GetListeners` が空であることを見る。並行 Send は 20 goroutine から送り、全件成功することを確かめる（受信数はループバックでも落ちうるので、受信数の完全一致は求めない。`mockEmitter` のバッファ 16 は溢れると黙って捨てる点にも注意）。
- **優先度**: 中

##### [観点 J] 環境の境界

**J1. IPv6 の宛先**
- **対象メソッド**: `Send` / `StartListen`
- **不足内容**: `NetSocket` は送信・受信とも `udp4` 固定なので、`::1` への送信はエラーになり、IPv6 のパケットは受信されない。この挙動を確かめて固定しているテストが無い。
- **根拠コード**: `internal/infrastructure/udp/net_socket.go:25`・`35`
- **実インフラで確かめる理由**: アドレスファミリーの扱いは実ソケットで決まるため。
- **推奨テスト名**: `TestUDP_Send_IPv6Rejected`
- **実装ガイド**: `udp_test.go` に追加する（`//go:build integration` / `package integration`）。`Host: "::1"` の `Send` がエラーになり、`Host: "localhost"` は 127.0.0.1 のリスナーに届くことを確かめる。IPv6 に対応する予定なら、このテストで仕様を決める。
- **優先度**: 低

##### [観点 K] 保存データの復旧

**K1. 必須データ（udp-targets）：Handler からの確認と正常なファイルとの同居**
- **対象メソッド**: `GetTargets`（起動時）
- **不足内容**: `TestUDP_CorruptStorage`（udp_test.go:609）は Handler を通さず、壊れたファイル 1 つだけで確かめている。正常なファイルと同居したときの「残りで起動する」と、`dir` を読めないときの起動失敗を確かめていない。
- **根拠コード**: `internal/infrastructure/udp/target_repository.go:21-37`、`internal/infrastructure/json_store.go:27-72`
- **実インフラで確かめる理由**: MQTT K1 と同じ。
- **推奨テスト名**: `TestUDP_CorruptTargetAmongValidOnes`
- **実装ガイド**: `udp_test.go` に追加する（`//go:build integration` / `package integration`）。MQTT K1 と同じ構成で `newUDPHandlerWithDir` を使う。
- **優先度**: 低

##### [観点 L] イベント発行

**L1. `udp:message` の名前と中身、停止後に発行されないこと**
- **対象メソッド**: `StartListen` / `Send` / `StopListen` / `Shutdown`
- **不足内容**: イベント名、`RemoteAddr`（送信元が `127.0.0.1:<port>` であること）、`Encoding` を確かめていない。`StopListen` が返った後に送ったパケットがイベントにならないことも確かめていない。
- **根拠コード**: `internal/application/udp/listener_service.go:118-140`、`internal/domain/events.go:15`
- **実インフラで確かめる理由**: 送信元アドレスは実ソケットの `ReadFrom` の結果で、停止後の沈黙は実ソケットの Close と受信ループの終了の組み合わせで決まるため。
- **推奨テスト名**: `TestUDP_MessageEventPayloadAndSilenceAfterStop`
- **実装ガイド**: `udp_test.go` に追加する（`//go:build integration` / `package integration`）。`mockEmitter` にイベント名を記録させ、`cmndomain.EventUDPMessage` と一致することを確かめる。`RemoteAddr` は `net.SplitHostPort` で host が `127.0.0.1` であることを見る。`StopListen` の後に `Send` し、一定時間メッセージが来ないことを確かめる（`StopListen` は受信 goroutine の終了を待たないので、**`StopListen` より前に読まれたパケット**が後から出る可能性は残る。これはテストでは決定的に作れないので「主なリスク」に書いた）。
- **優先度**: 中

##### [観点 M] RPC の信頼境界

**M1. `UDPHandler.Shutdown` が RPC に出ている**
- **対象メソッド**: `Shutdown`
- **不足内容**: `Shutdown` は `main.go` の `Bind` で RPC になり、`frontend/wailsjs/go/adapters/UDPHandler.d.ts` にバインディングがある（フロントエンドからは未使用）。WebView の JS から呼ぶと、全リスナーが黙って止まる（イベントも出ない）。MQTT・HTTP は終了処理をサービス側に置いて RPC から外している（`service.go:384` のコメント）。
- **根拠コード**: `internal/adapters/udp_handler.go:78-81`、`app.go:236`
- **実インフラで確かめる理由**: テストで守るより、`StopAll` を `UDPListenerService` から `app.go` が直接呼ぶ形に変えて RPC 面から外すのが本筋。外した後は、`TestUDP_Shutdown` をサービス経由の終了処理のテストに書き換える。
- **推奨テスト名**: （設計変更）`TestUDP_Shutdown` をサービスの `StopAll` 経由に書き換える
- **実装ガイド**: `udp_test.go` を変更する（`//go:build integration` / `package integration`）。`newUDPHandlerWithDir` が `*udpapp.UDPListenerService` も返すようにし（MQTT のヘルパーと同じ形）、`StopAll` を呼ぶ。RPC から外すと `task wails:generate` でバインディングも更新が必要。
- **優先度**: 低（テストとしては低。設計上のリスクは「主なリスク」に記載）

---

### OpenAPI (`openapi_test.go`)

#### 対象メソッド（Handler の公開メソッド）

| メソッド名 | ファイル:行 | テスト済み? | テストケース名 |
| --- | --- | --- | --- |
| OpenFilePicker | openapi_handler.go:72 | 部分的（Service 経由） | TestOpenAPI_CorruptRecents / TestOpenAPI_RecentsSeedAcrossRestart（`OpenSelected` を直接呼ぶ） |
| SaveFileAs | openapi_handler.go:86 | 未テスト | - |
| ReadFile | openapi_handler.go:98 | 部分的（Service 経由） | TestOpenAPI_RecentsSeedAcrossRestart（許可済みパスのみ） |
| WriteFile | openapi_handler.go:103 | 未テスト | - |
| GetRecents | openapi_handler.go:108 | 部分的（Service 経由） | TestOpenAPI_CorruptRecents |
| RemoveRecent | openapi_handler.go:113 | 未テスト | - |
| MoveRecent | openapi_handler.go:118 | 未テスト | - |

**状態を持つ箇所**: 許可リスト `FileService.granted` と recents（application）、`openapi-recents.json`（infrastructure）。イベントは発行しない。

**既存テストの読み取り**: 2 テストとも `newOpenAPIFileService` で Service を直接組み立てる。再起動相当（同じ `recentsPath` で 2 回組み立て）と破損時の退避はディスクの中身まで確かめている。

#### 不足テストケース

##### [観点 A] ゴールデンパスの完全性

**A1. Handler を通した開く・保存・読み書きの流れ**
- **対象メソッド**: `OpenFilePicker` / `SaveFileAs` / `ReadFile` / `WriteFile` / `GetRecents`
- **不足内容**: Handler の公開メソッドがすべて 0%。`SetupOpenAPIHandler` で組み立てた Handler から、ダイアログの戻り値だけが許可リストに入り、実ファイルを読み書きできる流れを確かめていない。`SaveSelected` と `WriteFile` は Service でも 0%。
- **根拠コード**: `internal/adapters/openapi_handler.go:58-120`、`internal/application/openapi/file_service.go:83-119`
- **実インフラで確かめる理由**: ダイアログ（adapters）→ 許可リスト（application）→ `AtomicWriteFile`（infrastructure）→ recents ファイルという流れを、実ファイルで端まで通すため。Handler の分岐は単体テスト（`TestOpenAPIOpenFilePicker_*` / `TestOpenAPISaveFileAs_*`）で対応済み。
- **推奨テスト名**: `TestOpenAPI_HandlerOpenSaveReadWrite`
- **実装ガイド**: `openapi_test.go` に追加する（`//go:build integration` / `package integration`）。`newOpenAPIHandler(t, recentsPath, dialog)` ヘルパーを作り、`adapters.SetupOpenAPIHandler(context.Background(), h, adapters.OpenAPIHandlerDeps{Files: newOpenAPIFileService(recentsPath), Dialog: dialog})` で組み立てる（**Dialog に nil を渡さないこと**。`wailsFileDialog` が使われる）。http_test.go の `fileDialog` は `SaveFile` が常に `""` なので、`openPath` / `savePath` を持つ OpenAPI 用のダイアログを用意する。`OpenFilePicker` → `ReadFile` → `WriteFile` → ディスクの中身、`SaveFileAs("spec.yaml", content)` → ファイルと recents、キャンセル（`""`）で `("", nil)` になり recents が変わらないことを確かめる。
- **優先度**: 中

##### [観点 B] 再起動をまたぐ引き継ぎ

**B1. `RemoveRecent` / `MoveRecent` の結果と許可の取り消しが再起動後に残るか**
- **対象メソッド**: `RemoveRecent` / `MoveRecent` / `ReadFile`
- **不足内容**: 並び替えと削除が保存され、削除したパスが再起動後の seed でも許可されない（`ReadFile` が `ErrFileAccessDenied`）ことを確かめていない。
- **根拠コード**: `internal/application/openapi/file_service.go:62-66`（seed）・`133-171`
- **実インフラで確かめる理由**: 許可リストは recents ファイルから再構築されるので、ファイルの中身を通して取り消しが永続することを確かめる必要があるため。
- **推奨テスト名**: `TestOpenAPI_RemoveAndMoveRecent_SurviveRestart`
- **実装ガイド**: `openapi_test.go` に追加する（`//go:build integration` / `package integration`）。A1 のヘルパーで 3 ファイルを開き、1 つを `MoveRecent`、1 つを `RemoveRecent` した後、同じ `recentsPath` で作り直す。`GetRecents` の順序と、削除したパスの `ReadFile` が `errors.Is(err, openapidomain.ErrFileAccessDenied)` になることを確かめる。
- **優先度**: 中

##### [観点 C] リソースリークと終了処理

該当なし（OpenAPI は接続や一時ファイルなどの資源を持たない）。

##### [観点 D] 入力値の境界

該当なし（`MoveRecent` の範囲外 index・未知のパスは単体テスト `TestMoveRecent*` で対応済み）。

##### [観点 E] 状態遷移の境界

該当なし（削除後の読み書きの拒否は B1 と M1 に含めた）。

##### [観点 F] 組み合わせの境界

該当なし（組み合わせになる入力が無い）。

##### [観点 G] 異常系の境界

**G1. 書けない保存先への `SaveFileAs`**
- **対象メソッド**: `SaveFileAs`
- **不足内容**: 存在しないディレクトリ配下を保存先にしたとき、エラーになり recents に追加されないことを実ファイルで確かめていない（許可は残る設計: `file_service.go:82`）。
- **根拠コード**: `internal/application/openapi/file_service.go:83-97`
- **実インフラで確かめる理由**: `AtomicWriteFile` の実際の失敗で同じ経路を通るかを確かめるため（単体テストは偽の `FileAccess`）。
- **推奨テスト名**: `TestOpenAPI_SaveFileAs_UnwritableDestination`
- **実装ガイド**: `openapi_test.go` に追加する（`//go:build integration` / `package integration`）。A1 のヘルパーで `savePath` を `filepath.Join(t.TempDir(), "missing", "spec.yaml")` にし、エラー・`GetRecents` が空・`missing` ディレクトリに一時ファイルが残らないことを確かめる。
- **優先度**: 低

##### [観点 H] 時間の境界

該当なし（非同期処理が無い）。

##### [観点 I] 並行性の境界

該当なし（`FileService` の排他は単体テスト `TestConcurrentReadAndRecentsUpdates` で対応済み。実 I/O 固有の並行性リスクは無い）。

##### [観点 J] 環境の境界

該当なし（OS 別のコードが無い）。

##### [観点 K] 保存データの復旧

**K1. best effort（openapi-recents.json）：破損以外の読み込み失敗と退避先の衝突**
- **対象メソッド**: `GetRecents` / `OpenFilePicker`（起動時）
- **不足内容**: 表の「破損以外の読み込み失敗: 空で始め、このセッションでは保存しない」のセルに対応するテストが無い（`file_service.go:57-59`・`recovery.go:58-60` が未到達）。`openapi-recents.json.corrupt` が既にある場合の退避先の衝突も確かめていない。
- **根拠コード**: `internal/application/openapi/file_service.go:47-68`・`222-230`、`internal/application/store/recovery.go:52-76`
- **実インフラで確かめる理由**: `openapi-recents.json` を**ディレクトリにする**と、OS に依存せず「破損ではない読み込み失敗」を再現できるため。
- **推奨テスト名**: `TestOpenAPI_UnreadableRecents_StartsEmptyWithoutSaving`
- **実装ガイド**: `openapi_test.go` に追加する（`//go:build integration` / `package integration`）。`os.Mkdir(recentsPath, 0o750)` の後に組み立て、`GetRecents` が空、`OpenFilePicker` で開いたパスがこのセッションでは `ReadFile` でき、`recentsPath` がディレクトリのまま（`.corrupt` も無い）であることを確かめる。衝突は、`recentsPath + ".corrupt"`（中身 "old"）と壊れた `recentsPath` を置いて組み立て、"old" が残り `.corrupt.*` ができることを見る。退避失敗のセルは単体テスト `TestNewFileService_CorruptQuarantineFailsDoesNotSave` で対応済み。
- **優先度**: 中

##### [観点 L] イベント発行

該当なし（イベントを発行しない）。

##### [観点 M] RPC の信頼境界

**M1. 許可リスト外のパスへの `ReadFile` / `WriteFile`**
- **対象メソッド**: `ReadFile` / `WriteFile`
- **不足内容**: ダイアログを通していない実在のパスへの読み書きが Handler 経由で拒否され、ファイルが変わらないことを確かめていない。許可済みのパスから `..` でたどった別のファイル（`<dir>/sub/../other.yaml`）も拒否されることを見るべき。
- **根拠コード**: `internal/application/openapi/file_service.go:99-119`・`175-181`、`internal/adapters/openapi_handler.go:95-102`
- **実インフラで確かめる理由**: 実ファイルが存在していても、許可リストの判定だけで読み書きが止まることを確かめるため（単体テスト `TestReadFileDeniesUngrantedPath` は偽の `FileAccess`）。
- **推奨テスト名**: `TestOpenAPI_UngrantedPathsRejected`
- **実装ガイド**: `openapi_test.go` に追加する（`//go:build integration` / `package integration`）。A1 のヘルパーで `spec.yaml` だけを開き、同じディレクトリの `secret.yaml` への `ReadFile` / `WriteFile` が `errors.Is(err, openapidomain.ErrFileAccessDenied)` になり、`secret.yaml` の中身が変わらないことを確かめる。
- **優先度**: 中

---

## 全体評価

### 観点別カバレッジ

> **カバー率**: 各観点のチェック項目のうち、既存テストで少なくとも 1 ケース確認できるものの割合（該当しない項目は分母から除外）。Step 0 の文カバレッジとは別の指標。

- **[A] ゴールデンパスの完全性**: 50% カバー済み（4/8。不足 4 件）
- **[B] 再起動をまたぐ引き継ぎ**: 50% カバー済み（6/12。不足 6 件）
- **[C] リソースリークと終了処理**: 43% カバー済み（3/7。不足 3 件）
- **[D] 入力値の境界**: 50% カバー済み（5/10。不足 5 件）
- **[E] 状態遷移の境界**: 36% カバー済み（4/11。不足 3 件。残りは HTTP A1・C1 で扱う）
- **[F] 組み合わせの境界**: 43% カバー済み（3/7。不足 4 件）
- **[G] 異常系の境界**: 33% カバー済み（3/9。不足 6 件）
- **[H] 時間の境界**: 67% カバー済み（4/6。不足 2 件）
- **[I] 並行性の境界**: 50% カバー済み（3/6。不足 3 件）
- **[J] 環境の境界**: 25% カバー済み（1/4。不足 3 件）
- **[K] 保存データの復旧**: 45% カバー済み（5/11。退避失敗時のセルは単体テストの担当として除外。不足 7 件）
- **[L] イベント発行**: 45% カバー済み（5/11。不足 2 件。`mqtt:connection-lost` は MQTT G1 で扱う）
- **[M] RPC の信頼境界**: 43% カバー済み（3/7。不足 5 件）

### CI で検証されない範囲

- **Windows 専用の経路**: `internal/infrastructure/http/file_open_windows.go`（共有モードでの open、`ErrSelectedFileInUse`）と、それが使う `longPath` の変換。統合テストでは成功経路だけがローカル（Windows）で通っていて（文カバレッジ 55.6%）、CI の ubuntu では代わりに `file_open_other.go` が使われる。HTTP J1・G2 の Windows 側はローカルでしか確かめられない。
- **統合テストでのデータ競合**: CI の単体テストのジョブ（`test-go-unit`）は `go test -race ./...` で race detector を有効にしている（`.github/workflows/ci.yml:79`）が、`integration` ビルドタグを付けないので `internal/integration` のテストは含まれない。統合テストのジョブ（`test-go-integration`）は `go test -tags integration -v ./internal/integration/...` で `-race` なし（`ci.yml:115`）。今回のローカル実行でも `-race` は動かなかった（cgo / gcc が無い）。`TestUDP_StopListen_RaceWithSend`・`TestUDP_Concurrent_StartStopListen`・`TestMQTT_Connect_Concurrent` はアサーションを持たず race detector に頼っているので、**これら 3 テストは CI でも今回のローカル実行でも実質何も検証していない**。各サービス単体の競合は単体テストの `-race` で検出されうるが、実ソケット・実クライアントをつないだ層をまたぐ経路の競合は検出されない。統合テストの CI ジョブに `-race` を足すか（ubuntu には gcc がある）、I1 のとおりアサーションを足す。

### 最優先で追加すべきテスト TOP5

1. **HTTP A1 `TestHTTP_TruncatedResponse_SaveAndDiscard`**（根拠: `SaveResponseBody`・`DiscardResponseBody` は 0%、`response_store.go` は 9.8%。Handler と `NetClient` が同じ `ResponseStore` を共有している配線が一度も通っていない。配線が崩れると、大きなレスポンスが保存できない・一時ファイルが最大 2 GiB までディスクに溜まる、といった障害になる）
2. **HTTP M1 `TestHTTP_AddRequest_RejectsDuplicateItemID`**（根拠: `collection_service.go:305-321` が呼び出し側の ID を重複の確認なしに受け入れる。RPC から既存 ID を指定されると、次回起動の `recoverDuplicateItems` が片方を黙って削除し、ユーザーのリクエストが消える。現状のコードでは失敗する見込み）
3. **MQTT G1 `TestMQTT_ConnectionLostAndReconnect`**（根拠: `onConnectionLost` は 0%。再接続時に購読し直さないので、ブローカーの再起動やネットワーク断の後、画面上は購読中なのにメッセージが届かない状態になりうる）
4. **HTTP C1 `TestHTTP_Shutdown_CancelsInFlightThenCleansTempFiles`**（根拠: `HTTPRequestService.Shutdown` は 0%で、`app.go:235-243` の終了順序を統合テストが再現していない。順序が崩れると、実行中の書き込みと一時ファイルの削除が競合して、ファイルが残るか書きかけが公開される）
5. **MQTT D2 `TestMQTT_InvalidWildcardTopics`**（根拠: `paho_client.go:156-165` が SUBACK の結果を見ず、Service はトピックのワイルドカードを検証しない。拒否された購読が成功扱いで表示され、QoS 1/2 の publish は最大 30 秒ブロックして同じ接続の他の操作を待たせる見込み）

### 総合評価

**信頼度**: 中

**主なリスク**:
- 切り詰めレスポンスの保存・破棄・終了時の回収（HTTP の一時ファイルの経路全体）が統合テストで一度も通っておらず、層をまたぐ配線が壊れても CI で検出されない。
- `AddRequest` が RPC 引数の ID を信頼しているため、重複 ID を経由して再起動時にデータが消える経路がある（通常の UI 操作では起きない）。
- MQTT の自動再接続後に購読が失われても、`GetConnections` は購読中と返し続ける可能性がある。不正なワイルドカードの購読も成功扱いになる。
- `UDPHandler.Shutdown` が RPC に出ており、WebView の JS から全リスナーを黙って止められる。また `StopListen` / `Shutdown` は受信 goroutine の終了を待たないので、停止の直前に読まれたパケットが停止後に `udp:message` として届く可能性がある（`listener_service.go:76-88`・`118-140`）。後者は決定的に再現できないため、テストではなく、停止時に受信ループの終了を待つ設計への変更で対処する。
- 統合テストの並行性のテストの一部がアサーションを持たず、CI の統合テストのジョブも `-race` なしなので、層をまたぐ経路のデータ競合は CI で検出されない（単体テストは `-race` 付きで回っているので、サービス単体の競合は検出されうる）。

**推奨アクション**:
1. `buildHTTPHandler` を `reqSvc`・`netClient` も返す fixture に分け、`dir` と `dialog` を両方指定して組み立てられるようにし（`newHTTPHandlerWithDialog` は `dir` を外に出さない）、`fileDialog` に保存先と呼び出し回数を持たせる（A1・B3・C1・G4・M3 の前提）。
2. TOP5 を追加する。M1・D2 は現状のコードで失敗する見込みなので、先に仕様（ID の重複時の扱い、トピックの検証をどこで行うか）を決める。
3. `mqttMockEmitter` / `mockEmitter` をイベント名とデータを順に記録する形に変え、L1（MQTT・UDP）と G1 を追加する。
4. `openapi_test.go` に `SetupOpenAPIHandler` で組み立てる Handler のヘルパーを作り、A1・B1・M1 を Handler 経由にする。
5. CI の `test-go-integration` に `-race` を足し、アサーションの無い並行テストに結果の確認を足す。
6. Windows 専用のテストを `http_windows_test.go`（`//go:build integration && windows`）にまとめ、ローカルで回す手順（`task go:test:integration`）を明記する。
