---
name: backend-integration-test-review
description: バックエンド（Go）の統合テストケースの網羅性を多角的な観点で分析し、Markdownレポートを出力する
disable-model-invocation: true
argument-hint: "[http | mqtt | udp | openapi]"
---

バックエンドの統合テスト（`internal/integration/`）が十分かどうかを分析し、`docs/backend-integration-test-review.md` にレポートを出力してください。

## プロジェクト固有情報

> ここに書いたファイル名・関数名・テスト名は**執筆時点の例**である。必ず現在のコードと Step 0 の結果で確かめ、食い違えば現在のコードを正とする。

### 統合テストの性質

- **実インフラを使う**: 埋め込み mochi ブローカー（`mqtt_test.go` の `TestMain` で起動）・`httptest.Server`・ループバックの実 UDP ソケット・`t.TempDir()` 上の実 JSON ストレージ。外部ネットワークは不要。
- **DI を手動で組み立てる**: `app.go` と同じく空の Handler を作り、`adapters.SetupXxxHandler` で注入する。
- **Handler から貫通する**: Handler → Application → Infrastructure を実装どおりに通す。差し替えるのは Wails ランタイムに依存する部分（Emitter・ファイルダイアログ）と Logger だけ。
- **単体テストとの役割分担**: 純粋なロジックの境界や保存形式のフィールド単位の往復（`*_RoundTripKeepsEveryField`・`*_GoldenFormat`）は単体テスト（`backend-unit-test-review`）の担当。統合テストでは層をまたぐときや実 I/O でだけ現れる振る舞いを見る。

### 対象の Handler と組み立てヘルパー

| ドメイン | テストファイル | Handler（`internal/adapters/`） | 組み立てヘルパー | 終了処理 |
| --- | --- | --- | --- | --- |
| HTTP | `http_test.go` | `http_handler.go` | `newHTTPHandler` / `newHTTPHandlerWithDir` / `newHTTPHandlerWithDialog`（実体は `buildHTTPHandler`。Handler だけを返す） | `HTTPRequestService.Shutdown(timeout)`（テストから呼ぶにはヘルパーの変更が必要）。`NetClient.Cleanup` は `t.Cleanup` で登録 |
| MQTT | `mqtt_test.go` | `mqtt_handler.go` | `newMQTTHandler` / `newMQTTHandlerWithDir` / `newMQTTHandlerWithConfig`（`*MQTTService` も返す） | `MQTTService.Shutdown(timeout)` |
| UDP | `udp_test.go` | `udp_handler.go` | `newUDPHandler` / `newUDPHandlerWithDir` | `UDPHandler.Shutdown()`（Handler にある） |
| OpenAPI | `openapi_test.go` | `openapi_handler.go` | `newOpenAPIFileService`（**Handler を通さず** `FileService` を直接組み立てる） | なし |

- **終了処理の所在**: `Shutdown` を持つ Handler は `UDPHandler` だけで、MQTT と HTTP はサービスにある。本番では `app.go` の `shutdown` が `mqttSvc.Shutdown` → `udpHandler.Shutdown` → `reqSvc.Shutdown` → `netClient.Cleanup` の順に呼ぶ（実行中の HTTP リクエストを止めてから一時ファイルを回収する）。統合テストはこの順序を再現していない。
- **Handler の公開メソッドはすべて RPC になる**（`main.go` の `Bind` に Handler ごと渡すため）。`UDPHandler.Shutdown` も `frontend/wailsjs/go/adapters/UDPHandler.d.ts` にバインディングがある（フロントエンドからは未使用）。Step 1 に含め、観点 M で扱う。
- **OpenAPI は Handler を通らない**ので、Step 0 では Handler の公開メソッドがすべて 0% になる。これを「未テストの RPC」と数えず、`FileService` で検証されている RPC とそうでない RPC を分ける。`SetupOpenAPIHandler` に偽の `Dialog` を渡せば、Handler から組み立てるヘルパーも作れる。
- 共通ヘルパーは `helper_test.go`（`freePort` / `freeUDPPort` / `assertNoFilesOutside` / `traversalIDs`）。`connectBroker` / `waitConnected` / `blackholeBroker` は `mqtt_test.go` にある。
- **対象外**: `log_handler.go`（実 I/O の境界がない）と `adapters/file_dialog.go` の `wailsFileDialog`（Wails ランタイムが必要）。

### テストダブル

- **Emitter**: MQTT は `mqttMockEmitter`（`receiveMessage` / `noMessage` / `assertSilent` / `waitConnectionFailed`）、UDP は `mockEmitter`（`receiveMessage`）。イベントをチャネルで受ける。
- **FileDialog**: HTTP は `fileDialog`（`OpenFile` は固定パス、`SaveFile` は常にキャンセルの `""`）。`adapters.FileDialog` を差し替えれば、ダイアログを使う RPC（`OpenFilePicker` / `SaveResponseBody` / `SaveResponseBase64`、OpenAPI の `OpenFilePicker` / `SaveFileAs`）も通せる。
  - **注意**: `newHTTPHandler` / `newHTTPHandlerWithDir` は dialog に `nil` を渡すので本番の `wailsFileDialog` が使われ、ダイアログを使う RPC を呼ぶと `log.Fatalf` で**テストプロセス全体が終了する**。そうした RPC のテストは必ず `newHTTPHandlerWithDialog` など dialog を渡すヘルパーで組み立てる。
- **Logger**: `testutil.NoopLogger{}`。
- testify / gomock / goleak などは依存になく、アサーションは標準の `testing`。goleak を使う提案には依存の追加が必要な旨を書く。

### 保存データの組み立て先

観点 K の基準は CLAUDE.md の「設定データの分類と復旧方針」の表。統合テストでの対応は次のとおり。

| 分類 | 対象ファイル | 統合テストでの組み立て |
| --- | --- | --- |
| 必須 | `collections/*.json`・`mqtt-profiles/*.json`・`udp-targets/*.json` | 各ヘルパーの `dir`（HTTP は `dir/collections`） |
| best effort | `openapi-recents.json` | `newOpenAPIFileService(recentsPath)` |
| 再生成可能 | `sidebar_layout.json`・`window-state.json` | HTTP は `dir/sidebar_layout.json`。`window-state.json` は Wails ランタイム依存のため対象外 |

- 予約コレクション `__root__` は削除・リネームを拒否し、ファイルが存在する限り作り直さない。

### CI

- CI（`.github/workflows/ci.yml` の `test-go-integration`）は **ubuntu で `-race` なし**で `go test -tags integration -v ./internal/integration/...` を実行する。
- そのため Windows 専用のコード（`*_windows.go`。例: `internal/infrastructure/http/file_open_windows.go`）を通る経路と、データ競合は CI で検証されない。レポートでは「CI で検証されない範囲」に分けて書く。

## 対象スコープ

- **分析対象**: `internal/integration/*_test.go`
- **本体コード**: 上の表の Handler と、それが呼ぶ `internal/application/<ドメイン>/`・`internal/infrastructure/<ドメイン>/`
- **`$ARGUMENTS`**: `http` / `mqtt` / `udp` / `openapi` が指定されたら、Step 0〜3 とレポートのサマリー・全体評価をそのドメインだけに絞る（Step 0 は `-run '^TestHTTP_'` のように絞る）。指定がなければ 4 ドメインすべて

## 分析手順

### Step 0: 統合テストの実行とカバレッジの取得

1. リポジトリの外（OS の一時ディレクトリなど）にプロファイルを出力する:
   `go test -tags integration -coverpkg=./internal/... -coverprofile=<一時ディレクトリ>/integ.cover.out ./internal/integration/...`
2. `go tool cover -func=<一時ディレクトリ>/integ.cover.out` で関数ごとのカバー率を取る。
   - レポートのパスは先頭の `github.com/f0reth/Wirexa/` を除いた相対パスにする。
   - 対象 Handler の公開メソッドで 0% のものが「統合テストから一度も呼ばれていない RPC」になる。
   - `internal/application/`・`internal/infrastructure/` で 0% または低い関数のうち、実 I/O（ソケット・ブローカー・ファイル・一時ファイル）に依存するものを Step 3 の候補にする。
3. 可能なら `go test -race -tags integration ./internal/integration/...` も実行する。Windows では cgo / gcc が必要なので、実行できなければレポート冒頭に書く。

- `frontend/dist/index.html` のスタブは不要（`//go:embed` は `main.go` にしかない）。
- テストが失敗したら、レポート冒頭に出力を記載し、分析は続ける。
- 実行されていても検証されているとは限らない。検証の有無は Step 2 で判断する。
- OS 別ファイルは実行した OS のものだけが計測される。

### Step 1: Handler と下位層の把握

対象の Handler を読み、以下をレポートの「対象メソッド」テーブル用に列挙する。

- 公開メソッド名と引数・戻り値の型
- 呼び出す入力ポートのメソッド
- Handler 自身の分岐（例: `HTTPHandler.OpenFilePicker` の hint とダイアログの再試行、`SaveResponseBody` のリース取得とキャンセル時の解放）。委譲だけなら Service の分岐を読む
- 状態を持つ箇所（接続マップ・リスナーセッション・実行中リクエスト・レスポンス本文の一時ファイル・ファイルトークンの登録簿など）とその層
- 発行するイベント（`internal/domain/events.go` の定数）

### Step 2: 既存の統合テストの読み取り

- テスト関数名と実際に確かめているシナリオ
- アサーションの深さ（戻り値の中身か、エラーの有無だけか。`errors.Is` で種類まで見ているか）
- 副作用の確認（ファイルの内容・退避ファイル・イベントの発行・「発行されないこと」）
- 再起動相当の確認（同じ `dir` でヘルパーを 2 回呼んでいるか）
- 非同期の待ち方（チャネル + タイムアウトか、`time.Sleep` か）
- クリーンアップ（`t.Cleanup` / `defer` で接続・リスナー・サービスを閉じているか）

### Step 3: 不足ケースの検出

以下の観点ごとに、不足しているテストケースをファイル名と行番号を根拠に検出する。

- 対象ドメインに該当しない観点（例: OpenAPI への H・I、HTTP への L）は「該当なし（理由）」と書いてスキップする。
- チェック表の例は観点の意味を示すもので、網羅の基準ではない。既存テストで確かめられているものは不足として報告しない。

#### 観点 A: ゴールデンパスの完全性（Golden Path Coverage）

境界テストより先に確認する。各ドメインの正常系が Handler から端まで通っているか。

| チェック項目 | 例 |
| --- | --- |
| RPC が一度も呼ばれていない | Step 0 で 0% の Handler メソッド（例: `SaveResponseBody` / `DiscardResponseBody` / `SaveResponseBase64`） |
| CRUD の一連の流れ | コレクション・フォルダ・リクエストの作成 → 取得 → 更新 → 移動 → 削除 |
| 接続〜切断（MQTT） | `Connect` → `Subscribe` → `Publish` → 受信イベント → `Unsubscribe` → `Disconnect` |
| リスナー（UDP） | `StartListen` → `Send` → 受信イベント → `StopListen` |
| 送信〜レスポンス保存（HTTP） | 大きなレスポンスで本文が切り詰められる → `SaveResponseBody` で保存 / `DiscardResponseBody` で破棄 |
| ファイル選択〜送信（HTTP） | `OpenFilePicker` で得たトークンを body に入れて `SendRequest` |

#### 観点 B: 再起動をまたぐ引き継ぎ（Restart Round-Trip）

同じ `dir` でヘルパーを呼び直したとき（= 再起動相当）に、Handler から見た状態が正しく復元されるか。フィールド単位の往復ではなく、起動時の整合処理を含む層をまたいだ結果を見る。

| チェック項目 | 例 |
| --- | --- |
| 操作の結果が残るか | `RenameCollection` / `MoveItem` / `MoveItemToSidebar` の後、作り直した Handler で取得する |
| 起動時の整合処理 | `internal/application/http/reconcile.go` の各分岐（重複アイテムの回収、sidebar_layout の欠落・stale・重複エントリの修正。コレクションのエントリと `__root__` 直下のアイテムのエントリ（`sidebarKindItem`）の両方） |
| 保存されない状態が消えるか | 接続・リスナー・実行中リクエスト・ファイルトークンが残らないこと。一時ファイルが次回起動時の sweep で回収されること（`SweepStaleTempFiles` は `buildHTTPHandler` が呼ばないので、テストで明示的に呼ぶ） |

#### 観点 C: リソースリークと終了処理（Resource Leak / Shutdown）

既存の観測手段（`GetConnections` / `GetListeners` が空になるか、Emitter が静かになるか、一時ファイルが消えるか）で確かめられるものを優先する。

| チェック項目 | 例 |
| --- | --- |
| `Shutdown` で全資源が閉じるか | 複数の接続・リスナーを作ってから `MQTTService.Shutdown` / `UDPHandler.Shutdown` を呼ぶ |
| `Shutdown` の戻り値 | `MQTTService.Shutdown(timeout)` / `HTTPRequestService.Shutdown(timeout)` がタイムアウト時に false を返すか（HTTP はヘルパーの変更が必要な旨を実装ガイドに書く） |
| 一時ファイルの後始末 | 切り詰められた本文の一時ファイル（`http-sessions` 配下）が破棄・保存・`Cleanup` で消えるか |
| 切断せずに終わったとき | テスト終了時にブローカー側の接続・UDP ソケットが残らないか（`t.Cleanup` の有無） |

#### 観点 D: 入力値の境界（Input Boundary）

検証がどの層にあるか（domain の `Validate`、Service、Infrastructure）を確かめ、層をまたいで初めて表れるものだけを扱う。

| チェック項目 | 例 |
| --- | --- |
| 空文字列 | Broker URL・トピック・コレクション名・UDP のホストが `""` |
| 不正な URL・ホスト | スキームなし / ポートなし / 解決できないホスト |
| ポート番号 | 0 / 65536 / -1（`StartListen` と `Send` の両方） |
| ペイロードのサイズ | 0 バイト / UDP の最大データグラム超 / HTTP の本文が切り詰めの閾値超 |
| MQTT のトピック・QoS | `#` / `+` の位置違反 / QoS が 3 以上 |
| HTTP の設定値 | `Settings.TimeoutSec` が 0（既定値）・負数、`Auth.Type` が未知の値 |
| 存在しない ID | 存在しない connectionID / sessionID / collectionID / executionID への操作 |

#### 観点 E: 状態遷移の境界（State Transition Boundary）

| チェック項目 | 例 |
| --- | --- |
| 切断後の操作 | `Disconnect` 後の `Publish` / `Subscribe` / `Unsubscribe` |
| 二重操作 | `Disconnect` / `StopListen` / `DiscardResponseBody` を 2 回 |
| 終了後の操作 | `Shutdown` 後の `Connect` / `StartListen` / `SendRequest` |
| 削除後の操作 | 削除済みコレクションへの `AddRequest` / 削除済みアイテムの `MoveItem` |
| 保存・破棄の順序（HTTP） | 保存済み・破棄済みの executionID への `SaveResponseBody`、キャンセル後の再保存 |
| キャンセル後の再送信 | `CancelRequest` 後に同じ executionID で `SendRequest` |
| 予約コレクションへの拒否 | `DeleteCollection` / `RenameCollection` に `__root__` を渡すとエラーになりファイルが残ること。`MoveSidebarEntry("collection", "__root__", …)` が `NotFoundError` になること |
| 予約コレクションを通る移動 | `MoveItem` の移動元・移動先に `__root__` を指定しても成功する。`MoveItem` はレイアウトを更新しないので、`__root__` へ移したアイテムが `GetRootItems` に現れ、`GetSidebarLayout` では読み出し時の突合で末尾に追加されること（逆向きではエントリが消えること） |

#### 観点 F: 組み合わせの境界（Combination Boundary）

実装に存在する組み合わせだけを扱う（例: `RequestAuth.Type` は 1 つだけなので、Basic と Bearer の同時指定は存在しない）。

| チェック項目 | 例 |
| --- | --- |
| 有効/無効フラグ | `Enabled: false` のヘッダー・クエリパラメータ・フォーム行が送信されないこと |
| 認証とヘッダーの重なり | `Auth.Type` が `basic` / `bearer` で、`Authorization` ヘッダーも指定した場合 |
| body の種類と Content-Type | 自動設定とユーザー指定の優先順位 |
| MQTT の購読の重なり | 同一接続で `test/#` と `test/specific` を購読したときの受信回数 |
| QoS の組み合わせ | Subscribe と Publish の QoS が異なる場合 |
| UDP のエンコード | `text` / `json` / `fixed` の送信側とリスナー側の組み合わせ |
| ツリーの構造 | 入れ子のフォルダの移動・削除・永続化 |

#### 観点 G: 異常系の境界（Error Boundary）

実インフラでしか起こせないエラー。

| チェック項目 | 例 |
| --- | --- |
| 到達不能な相手 | 閉じたポートのブローカー / 閉じた `httptest.Server` / 到達不能な UDP 宛先 |
| 応答しない相手 | 接続は受け付けるが応答しないブローカー（`blackholeBroker`）・HTTP サーバー |
| 通信中の切断 | 受信中にサーバーが接続を切る / ブローカーとの接続が失われる（`mqtt:connection-lost`） |
| 使用中のポート | 他のソケットが握っているポートで `StartListen` |
| ファイル I/O の失敗 | 送信中に選択ファイルが消える・変更される / 保存先に書けない |

#### 観点 H: 時間の境界（Time Boundary）

| チェック項目 | 例 |
| --- | --- |
| タイムアウト | 遅いサーバーと短い `TimeoutSec` / MQTT の接続タイムアウト（`newMQTTHandlerWithConfig`） |
| キャンセルのタイミング | `CancelRequest` を送信前 / 応答待ち / 本文受信中 / 完了後に呼ぶ |
| 非同期の受信 | `Send` 直後の `StopListen` / `Publish` 直後の `Unsubscribe` |
| メッセージの順序 | 連続した Publish / Send の受信順序（保証があるものだけ） |
| 待ち方 | `time.Sleep` に頼り、遅い環境で不安定になるテスト |

#### 観点 I: 並行性の境界（Concurrency Boundary）

goroutine を使う箇所（HTTP の実行中リクエストとキャンセル、MQTT の接続試行と受信コールバック、UDP のリスナー）は必ずチェックする。競合は Step 0 の手順 3 の結果か、コードの読み取りで判断する。

| チェック項目 | 例 |
| --- | --- |
| 同時送信 | 同じ Handler から並行に `SendRequest` / `Send` / `Publish` |
| キャンセルと送信の競合 | 同じ executionID への `CancelRequest` と `SendRequest` |
| 終了処理と操作の競合 | `Shutdown` 中に別 goroutine から `Connect` / `StartListen` / `SendRequest` |
| 状態マップへの並行アクセス | 並行に `Connect` / `Disconnect`、`StartListen` / `StopListen` |
| 保存・破棄の競合（HTTP） | 同じ executionID への `SaveResponseBody` と `DiscardResponseBody` |
| ストレージへの並行書き込み | 並行に `SaveProfile` / `SaveTarget` / `AddRequest` |

#### 観点 J: 環境の境界（Environment Boundary）

| チェック項目 | 例 |
| --- | --- |
| OS 別の経路 | `*_windows.go` にだけある振る舞い（送信中の選択ファイルへの書き込み・削除の禁止など）の統合テストがあるか |
| アドレスの形式 | IPv6 のループバック（`::1`）を宛先・リッスン先にした場合 |
| 再バインド | `StopListen` 後に同じポートで `StartListen` |
| ストレージの場所 | 保存先ディレクトリが途中で消えた場合 |

#### 観点 K: 保存データの復旧（Recovery Policy）

チェック項目は、CLAUDE.md の復旧方針表の各セル（分類 × 破損時 / 退避失敗時 / 破損以外の読み込み失敗。必須データはコレクション・プロファイル・ターゲットのそれぞれ）と、退避先の衝突（`<path>.corrupt` があれば `<path>.corrupt.<unixnano>`）。各セルに対応する統合テストがあるかを、Handler から見た結果（起動を続けられるか、何が見えるか、以後の保存が行われるか）とディスクに残るもの（`.corrupt` への退避、元ファイルが残るか）で確かめる。

- 権限やロックで「読めない」状態は Windows で安定して作れないことがある。既存テストの代用方法（例: `TestHTTP_UnloadableRootIsNotOverwritten` のコメント）を確かめてから不足と判断する。

#### 観点 L: イベント発行（Event Emission）

フロントエンドはイベントで状態を更新するので、発行漏れや余計な発行は画面の不整合になる。

| チェック項目 | 例 |
| --- | --- |
| 発行されるべきイベント | `mqtt:connected` / `mqtt:disconnected` / `mqtt:connection-lost` / `mqtt:connection-failed` / `mqtt:message` / `udp:message` |
| イベントの中身 | connectionId・トピック・ペイロード・QoS・送信元アドレスなどまで検証しているか |
| 発行されないこと | `Disconnect` / `Unsubscribe` / `StopListen` / `Shutdown` の後にイベントが来ないこと（`assertSilent` / `noMessage`） |
| 失敗時のイベント | 接続失敗で `mqtt:connection-failed` だけが出て、`mqtt:connected` が出ないこと |

#### 観点 M: RPC の信頼境界（Trust Boundary）

Wails の RPC は WebView 上の JS から誰でも呼べる。RPC 引数を信頼しない設計（`docs/http-local-file-access-hardening.md` など）が、層を貫いても守られているか。

| チェック項目 | 例 |
| --- | --- |
| ストア外を指す ID | `traversalIDs` を ID に使った `SaveProfile` / `SaveTarget` / コレクション操作で、ストアの外にファイルができないこと（`assertNoFilesOutside`） |
| RPC 引数のパスを読まない | HTTP の body に生のパスを入れても読まれず、ダイアログ由来のトークンだけが使われること |
| 許可リスト外のパス（OpenAPI） | ダイアログを通していないパスへの `ReadFile` / `WriteFile` が拒否されること |
| 保存元の指定 | `SaveResponseBody` が未追跡の executionID を拒否すること |
| RPC に出ている内部用メソッド | フロントエンドが呼ぶ想定のない公開メソッド（例: `UDPHandler.Shutdown`。JS から呼ぶと全リスナーが止まる）。RPC 面から外すべきかも「主なリスク」に書く |

## 出力形式

`docs/backend-integration-test-review.md` に以下の構成で出力する（既存なら上書き）。

```markdown
# バックエンド統合テストレビュー

生成日時: YYYY-MM-DD
対象: internal/integration/ の統合テスト（HTTP / MQTT / UDP / OpenAPI）
実行結果: 統合テスト 成功 / 失敗（失敗時は出力を下に記載） / `-race` 実行済み / 未実行（理由）

---

## サマリー

| ドメイン | テスト関数数 | RPC カバー数 / 全 RPC 数 | Handler の文カバレッジ | 不足ケース数 | 優先度(高/中/低) |
| --- | --- | --- | --- | --- | --- |
| HTTP | N | N / N | N% | N | 高 |
| MQTT | N | N / N | N% | N | 中 |
| UDP | N | N / N | N% | N | 低 |
| OpenAPI | N | N / N | N% | N | 低 |

> **RPC カバー数**: 対象 Handler の公開メソッドのうち、統合テストから呼ばれ、結果（戻り値・副作用・イベント）がアサーションで検証されているものの数。
> Handler を通さず Service で検証しているもの（現状は OpenAPI）は `0 / 7（Service 経由 N）` のように別に書き、文カバレッジの 0% には「統合テストが Handler を通らないため」と注記する。
> **Handler の文カバレッジ**: Step 0 の `go tool cover -func` の値。取れなければ `-` とし、理由を書く。

---

## ドメイン別詳細

### HTTP (`http_test.go`)

#### 対象メソッド（Handler の公開メソッド）

| メソッド名 | ファイル:行 | テスト済み? | テストケース名 |
| --- | --- | --- | --- |
| CreateCollection | http_handler.go:XX | 済 | TestHTTP_CollectionCRUD |
| SaveResponseBody | http_handler.go:XX | 未テスト | - |

> **テスト済みの選択肢**: `済` / `部分的`（一部の分岐のみ）/ `未テスト`

#### 不足テストケース

##### [観点 A] ゴールデンパスの完全性

- **対象メソッド**: `SendRequest` / `SaveResponseBody`
- **不足内容**: 切り詰められたレスポンス本文を保存する流れが統合テストで一度も通っていない
- **根拠コード**: `internal/adapters/http_handler.go:XX`（Step 0 で 0%）
- **実インフラで確かめる理由**: 一時ファイルの生成・リース・保存先への移動が実ファイル I/O をまたぐため
- **推奨テスト名**: `TestHTTP_SaveResponseBody_TruncatedBody`
- **実装ガイド**: `http_test.go` に追加する（`//go:build integration` / `package integration`）。`newHTTPHandlerWithDialog` で組み立て、保存先を返すよう `fileDialog` の `SaveFile` を拡張する
- **優先度**: 高

---

### MQTT (`mqtt_test.go`)

...（同形式）

### UDP (`udp_test.go`)

...（同形式）

### OpenAPI (`openapi_test.go`)

...（同形式）

---

## 全体評価

### 観点別カバレッジ

> **カバー率**: 各観点のチェック項目のうち、既存テストで少なくとも 1 ケース確認できるものの割合（該当しない項目は分母から除外）。Step 0 の文カバレッジとは別の指標。

- **[A] ゴールデンパスの完全性**: N% カバー済み（不足N件）
- **[B] 再起動をまたぐ引き継ぎ**: N% カバー済み（不足N件）
- **[C] リソースリークと終了処理**: N% カバー済み（不足N件）
- **[D] 入力値の境界**: N% カバー済み（不足N件）
- **[E] 状態遷移の境界**: N% カバー済み（不足N件）
- **[F] 組み合わせの境界**: N% カバー済み（不足N件）
- **[G] 異常系の境界**: N% カバー済み（不足N件）
- **[H] 時間の境界**: N% カバー済み（不足N件）
- **[I] 並行性の境界**: N% カバー済み（不足N件）
- **[J] 環境の境界**: N% カバー済み（不足N件）
- **[K] 保存データの復旧**: N% カバー済み（不足N件）
- **[L] イベント発行**: N% カバー済み（不足N件）
- **[M] RPC の信頼境界**: N% カバー済み（不足N件）

### CI で検証されない範囲

- Windows 専用の経路: （該当するテストと、ローカルでしか確かめられない旨）
- データ競合: （`-race` の実行結果、または未実行である旨）

### 最優先で追加すべきテスト TOP5

1. ...（根拠: どのコードパスが無防備か、どんな障害が起きうるか）
2. ...
3. ...
4. ...
5. ...

### 総合評価

**信頼度**: 高 / 中 / 低
**主なリスク**: （未テストのコードパスで起きうる実際の障害シナリオ）
**推奨アクション**: （具体的な次のステップ）
```

## 制約事項

- **コードを根拠にする**: Handler / Application / Infrastructure の実際の分岐・型・エラー条件とファイル名・行番号を根拠にし、一般論での提案はしない
- **存在しない分岐を「不足」にしない**: コードにない分岐や組み合わせ（例: 認証方式の同時指定、呼び出し側が指定する MQTT の接続 ID）や、決定的に再現できない状況（他プロセスによるポートの先取りなど）は提案しない
- **統合テスト固有の価値に絞る**: 単体テストの担当範囲は「単体テストで対応済み」として不足扱いにしない。実ネットワーク・実ストレージ・実プロトコル・層の貫通でのみ現れるリスクを高優先にする
- **「プロジェクト固有情報」の注意点を守る**: 終了処理の所在、dialog を渡さないヘルパーでダイアログを使う RPC を呼ばないこと、OpenAPI の 0% の扱い、CI の条件
- **CLAUDE.md とコードの食い違い**: 観点 K の判定で見つかったら、それ自体をレポートに書く
- **build tag を明記**: 実装ガイドには追加先のファイルと `//go:build integration` / `package integration` を書く
- **リポジトリを汚さない**: 一時ファイルはリポジトリの外に出す。出力してよいのは `docs/backend-integration-test-review.md` だけ
