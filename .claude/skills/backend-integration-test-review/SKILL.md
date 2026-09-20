---
name: backend-integration-test-review
description: バックエンド（Go）の統合テストケースの網羅性を多角的な観点で分析し、Markdownレポートを出力する
disable-model-invocation: true
---

バックエンド（`internal/integration/`）の統合テストケースが十分かどうかを分析し、`docs/backend-integration-test-review.md` にレポートを出力してください。

## 統合テストの性質（単体テストとの違い）

統合テストは以下の特徴を持つため、単体テストとは異なる観点で分析する。

- **実インフラを使う**: 埋め込みMQTTブローカー・httptest.Server・実UDPソケット・tempディレクトリのJSONストレージを使用
- **DI組み立てを手動で行う**: `newHTTPHandler` / `newMQTTHandler` / `newUDPHandler` がテストごとに組み立て
- **Adapter層を通してテスト**: Handler → Application → Infrastructure の全層を貫通
- **ゴールデンパスと実際のエラー境界**: モックではなく実際のエラーが発生する境界をテストする意義が高い

## 対象スコープ

- 分析対象: `internal/integration/*_test.go`（4ファイル: `helper_test.go` / `http_test.go` / `mqtt_test.go` / `udp_test.go`）
  - `helper_test.go` はヘルパー関数（`freePort` / `freeUDPPort`）の定義のみで、テスト関数自体は含まない
  - `connectBroker` / `waitConnected` は `helper_test.go` ではなく `mqtt_test.go` 内に定義されている
- 対応するAdapter実装: `internal/adapters/`（`http_handler.go` / `mqtt_handler.go` / `udp_handler.go`）
  - **除外**: `log_handler.go` / `openapi_handler.go` / `*_dto.go` は統合テストの対象外
- Application層の実装: `internal/application/http/` / `internal/application/mqtt/` / `internal/application/udp/`
- `$ARGUMENTS` で特定のドメイン（例: `http` / `mqtt` / `udp`）が指定された場合は、Step 1〜3 の全手順においてそのドメインのAdapterファイル・テストファイルのみを対象にする

## 分析手順

### Step 1: Adapterの公開メソッドを把握する

`internal/adapters/` の各ハンドラーファイルを読み、以下を列挙する。レポートの「対象メソッド」テーブルに記載する。

- 公開メソッド名と引数・戻り値の型
- 内部で呼ぶApplicationサービスのメソッド
- エラーを返しうる分岐（Applicationエラー、バリデーション等）
- 状態を持つフィールド（接続マップ、セッションマップ等）

### Step 2: 既存の統合テストを読み取る

`internal/integration/` の各テストファイルを読み、以下を把握する。

- テスト関数名と実際にテストしているシナリオ
- テストで検証しているアサーション（戻り値・エラー有無・副作用）
- ヘルパーの活用状況（`freePort` / `freeUDPPort` は `helper_test.go`、`connectBroker` / `waitConnected` は `mqtt_test.go` に定義）
- クリーンアップ（`t.Cleanup` / `defer`）の有無

### Step 3: 以下の観点で不足を検出

以下の観点ごとに、**実際のAdapterコードを根拠に**不足しているテストケースを検出する。
推測・一般論は禁止。必ずコードの具体的なファイル名と行番号を根拠にすること。

---

#### 観点 A: ゴールデンパスの完全性（Golden Path Coverage）

境界テストよりも先に確認すべき前提。各ドメインの正常系E2Eフローが端から端まで揃っているか。

| チェック項目             | 統合テスト固有の例                                        |
| ------------------------ | --------------------------------------------------------- |
| CRUDの完全サイクル       | Create→Read→Update→Delete が1テストで通し確認されているか |
| 接続〜切断の完全フロー   | Connect→Subscribe→Publish→受信確認→Unsubscribe→Disconnect |
| リスナーの完全フロー     | StartListen→Send→受信確認→StopListen                      |
| 複数リクエストの連続実行 | SendRequest を複数回呼んで毎回正常レスポンスが得られるか  |

---

#### 観点 B: 永続化ラウンドトリップ（Persistence Round-Trip）

書き込み後に新しいHandlerインスタンスで読み直したとき（= アプリ再起動相当）にデータが正しく復元されるか。環境の境界（ストレージ破損）とは別の、**正常な引き継ぎ**の確認。

| チェック項目                      | 統合テスト固有の例                                                                  |
| --------------------------------- | ----------------------------------------------------------------------------------- |
| コレクション/リクエストの再ロード | `newHTTPHandler` を同一tempDirで2回呼び、2回目で1回目の書き込みデータが取得できるか |
| ネスト構造の再ロード              | フォルダ内リクエストが再ロード後も同じ構造を持つか                                  |
| 名前変更後の再ロード              | `RenameCollection` 後に再ロードして名前が保持されているか                           |

---

#### 観点 C: リソースリーク（Resource Leak）

`Shutdown` 後または `t.Cleanup` 後にgoroutine・接続・ファイルディスクリプタがリークしていないか。`go test -race` とは別の観点。

| チェック項目                     | 統合テスト固有の例                                                 |
| -------------------------------- | ------------------------------------------------------------------ |
| `Shutdown` 後のgoroutineリーク   | `runtime.NumGoroutine()` を前後で比較、または `goleak` を使用      |
| `Disconnect` せずにHandlerを破棄 | MQTT接続を切らずにテストが終わったとき、ブローカー側で接続が残るか |
| `StopListen` せずにHandlerを破棄 | UDPリスナーgoroutineがリークしないか                               |
| 大量接続後の`Shutdown`           | 100接続を作成後に`Shutdown`を呼び、全接続が閉じられるか            |

---

#### 観点 D: 入力値の境界（Input Boundary）

統合テストでの注目点: 実際にネットワーク・ストレージ・プロトコルへ渡る値の境界

| チェック項目                 | 統合テスト固有の例                                       |
| ---------------------------- | -------------------------------------------------------- |
| 空文字列の入力               | Broker URL が `""` / コレクション名が `""` / QoSが範囲外 |
| 無効なURL・ホスト形式        | `http://` のみ / ポートなし / スキームなし               |
| 無効なポート番号             | 0 / 65536 / -1                                           |
| ペイロードの極端なサイズ     | 0バイト / 最大UDPパケットサイズ超                        |
| 特殊なトピック文字列（MQTT） | `#` / `+` / 空 / 階層が深いトピック                      |
| 存在しないID                 | 実際のデータなしでUpdate/Delete/Disconnect               |
| HTTPリクエストの各フィールド | Method が空 / 不正なURL / BodyあるがContent-Typeなし     |

---

#### 観点 E: 状態遷移の境界（State Transition Boundary）

統合テストでの注目点: 実際のネットワーク接続・セッション・ライフサイクルの遷移

| チェック項目                       | 統合テスト固有の例                                        |
| ---------------------------------- | --------------------------------------------------------- |
| 接続前の操作                       | `Connect` 前に `Publish` / `Subscribe`                    |
| 切断後の操作                       | `Disconnect` 後に `Publish` / `Subscribe` / `Unsubscribe` |
| 二重接続・二重切断                 | 同じ接続IDに対して `Connect` を2回 / `Disconnect` を2回   |
| `Shutdown` 後の操作                | `Shutdown` 後に `Connect` / `Send`                        |
| リスナー停止後の送信               | `StopListen` 後にそのポートへ `Send`                      |
| コレクション削除後のリクエスト追加 | 削除済みcollectionIDへ `AddRequest`                       |
| リクエストキャンセル後の再送信     | `CancelRequest` 後に同IDで `SendRequest`                  |

---

#### 観点 F: 組み合わせの境界（Combination Boundary）

統合テストでの注目点: 複数のドメインが絡む操作・複数フラグの組み合わせ

| チェック項目                     | 統合テスト固有の例                                      |
| -------------------------------- | ------------------------------------------------------- |
| ヘッダー有効/無効フラグ          | `Enabled: false` のヘッダーがリクエストに含まれないこと |
| クエリパラメータ有効/無効フラグ  | `Enabled: false` のパラメータが除外されること           |
| 認証の組み合わせ                 | BasicAuth + BearerToken を同時に指定                    |
| 複数購読トピックとワイルドカード | 同一接続で `test/#` と `test/specific` を両方購読       |
| QoS 0/1/2 の組み合わせ           | Subscribe QoS 1 に対して Publish QoS 0 での到達保証     |
| UDPエンコード種別                | TextエンコードのリスナーにFixedエンコードで送信した場合 |
| フォルダのネスト                 | フォルダ内フォルダ内リクエストの永続化                  |

---

#### 観点 G: 異常系の境界（Error Boundary）

統合テストでの注目点: 実インフラレベルのエラー（ブローカー障害・ネットワーク不達等）

| チェック項目                         | 統合テスト固有の例                               |
| ------------------------------------ | ------------------------------------------------ |
| 到達不能ブローカーへの接続           | `tcp://127.0.0.1:1` （拒否されるポート）         |
| 到達不能HTTPサーバーへのリクエスト   | 閉じたhttptest.Serverへ送信                      |
| 到達不能UDPホストへの送信            | 存在しないホストへ `Send`                        |
| 既に使用中のポートでStartListen      | 同じポートで2回 `StartListen`                    |
| ブローカー切断後のPublish            | ブローカー停止後に接続を通じてPublish            |
| 存在しない接続IDへの操作             | `Publish("nonexistent-id", ...)`                 |
| JSONストレージの破損（初回起動相当） | リポジトリの作成は正常だがデータが壊れている場合 |
| HTTPタイムアウト                     | 応答しないサーバーへのリクエスト                 |

---

#### 観点 H: 時間の境界（Time Boundary）

統合テストでの注目点: 実際のI/O待機・タイムアウト・非同期処理

| チェック項目                     | 統合テスト固有の例                                  |
| -------------------------------- | --------------------------------------------------- |
| ゼロタイムアウトのHTTPリクエスト | `Timeout: 0` で送信したとき即時完了 or エラー       |
| HTTPリクエストのタイムアウト     | 応答が遅いサーバー + 短いタイムアウト               |
| MQTT接続確立の待機               | `waitConnected` なしに即座に `Subscribe`            |
| MQTTメッセージの順序             | 連続して複数メッセージをPublishしたときの受信順序   |
| `CancelRequest` のタイミング     | 送信直後（接続前）/ 受信中 / 受信完了後             |
| UDPリスナーの非同期受信          | `Send` 直後に `StopListen` したときのメッセージ到達 |

---

#### 観点 I: 並行性の境界（Concurrency Boundary）

統合テストでの注目点: 複数goroutineからの同時操作・`go test -race` での競合

| チェック項目                            | 統合テスト固有の例                              |
| --------------------------------------- | ----------------------------------------------- |
| 同時 `SendRequest`                      | 複数goroutineから同じHandlerで並行送信          |
| 同時 `Publish` と `Subscribe`           | 同じ接続で並行してPublish/Subscribe             |
| `Shutdown` と操作の競合                 | `Shutdown` 中に別goroutineからPublish           |
| 複数リスナーへの同時送信                | 同ポートに複数goroutineから送信                 |
| `CancelRequest` と `SendRequest` の競合 | 送信goroutineとキャンセルgoroutineの同時実行    |
| 接続マップへの並行アクセス              | 複数goroutineで Connect / Disconnect を同時実行 |

---

#### 観点 J: 環境の境界（Environment Boundary）

統合テストでの注目点: tempディレクトリ・ネットワーク環境・OS依存の挙動

| チェック項目                           | 統合テスト固有の例                                      |
| -------------------------------------- | ------------------------------------------------------- |
| ストレージディレクトリが削除された場合 | テスト中にtempDirを手動削除して操作                     |
| ストレージのJSONが不正な場合           | コレクションファイルに `{invalid}` を書いてから読み込み |
| 同一ポートへの再バインド               | `StopListen` 後に同じポートで `StartListen`             |
| IPv6アドレスの扱い                     | `::1` / `[::1]:PORT` 形式のホスト指定                   |
| ポート枯渇                             | `freeUDPPort` が返すポートを先占されている場合          |

---

## 出力形式

`docs/backend-integration-test-review.md` に以下の構成で出力してください。ファイルが既に存在する場合は上書きしてください。

```markdown
# バックエンド統合テストレビュー

生成日時: YYYY-MM-DD
対象: internal/integration/ の統合テスト（HTTP / MQTT / UDP）

---

## サマリー

| ドメイン | テスト関数数 | カバー済みメソッド数 / 全メソッド数 | 不足ケース数 | 優先度(高/中/低) |
| -------- | ------------ | ----------------------------------- | ------------ | ---------------- |
| HTTP     | N            | N / N                               | N            | 高               |
| MQTT     | N            | N / N                               | N            | 中               |
| UDP      | N            | N / N                               | N            | 低               |

> **カバー済みの定義**: 対象メソッドに対して少なくとも1つの統合テストが存在し、正常系または異常系のいずれかがアサーションで検証されていることを「カバー済み」とする。

---

## ドメイン別詳細

### HTTP (`http_test.go`)

#### 対象メソッド（Adapter公開メソッド）

| メソッド名       | ファイル:行        | テスト済み? | テストケース名                        |
| ---------------- | ------------------ | ----------- | ------------------------------------- |
| CreateCollection | http_handler.go:XX | 済          | TestHTTP_CollectionCRUD               |
| SendRequest      | http_handler.go:XX | 部分的      | TestHTTP_SendRequest_2xx, 4xx5xx, ... |

#### 不足テストケース

##### [観点 A] ゴールデンパスの完全性

- **対象メソッド**: `CreateCollection` / `AddRequest` / `SendRequest` / `DeleteCollection`
- **不足内容**: Create→AddRequest→Send→Delete の完全サイクルが1テストで通しテストされていない
- **根拠コード**: `http_handler.go:XX`（各メソッドは個別にテスト済みだが組み合わせがない）
- **推奨テスト名**: `TestHTTP_FullCycleFlow`
- **実装ガイド**: ファイル先頭に `//go:build integration` を付与し `package integration` で実装する
- **優先度**: 高

##### [観点 D] 入力値の境界

- **対象メソッド**: `SendRequest`
- **不足内容**: URL が空文字列 `""` のケースがテストされていない
- **根拠コード**: `http_handler.go:XX` の `if req.URL == ""` 分岐 / または application層での検証
- **推奨テスト名**: `TestHTTP_SendRequest_EmptyURL`
- **実装ガイド**: ファイル先頭に `//go:build integration` を付与し `package integration` で実装する
- **優先度**: 高

---

### MQTT (`mqtt_test.go`)

#### 対象メソッド（Adapter公開メソッド）

...（同形式）

#### 不足テストケース

...（同形式）

---

### UDP (`udp_test.go`)

#### 対象メソッド（Adapter公開メソッド）

...（同形式）

#### 不足テストケース

...（同形式）

---

## 全体評価

### 観点別カバレッジ

> **カバー率の計算方法**: 各観点のチェック項目数のうち、既存テストで少なくとも1ケースが確認できるものの割合。

- **[A] ゴールデンパスの完全性**: N% カバー済み（不足N件）
- **[B] 永続化ラウンドトリップ**: N% カバー済み（不足N件）
- **[C] リソースリーク**: N% カバー済み（不足N件）
- **[D] 入力値の境界**: N% カバー済み（不足N件）
- **[E] 状態遷移の境界**: N% カバー済み（不足N件）
- **[F] 組み合わせの境界**: N% カバー済み（不足N件）
- **[G] 異常系の境界**: N% カバー済み（不足N件）
- **[H] 時間の境界**: N% カバー済み（不足N件）
- **[I] 並行性の境界**: N% カバー済み（不足N件）
- **[J] 環境の境界**: N% カバー済み（不足N件）

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

---

## 制約事項

- **コードを読まずに記載禁止**: 実際のAdapter/Application/Infrastructureコードの分岐・型・エラー条件を根拠にすること。一般論でのテスト追加提案は禁止
- **統合テスト固有の価値に絞る**: 単体テストで既にカバーされている純粋なロジック境界は「単体テストで対応済み」と記載して不足扱いにしない
- **実インフラで検出できる境界を優先**: モックに依存する単体テストでは発見できない、実ネットワーク・実ストレージ・実プロトコルでのみ現れるバグリスクを高優先にする
- **`go test -race` 観点を忘れない**: goroutineを使うHandler（CancelRequest、MQTT受信ループ、UDPリスナー）は並行性の境界（観点I）を必ずチェック
- **存在しない分岐を「不足」と報告しない**: Adapterコードにその分岐がない場合はテスト不要
- **build tag を必ず明記**: 推奨テストには「実装ガイド」として `//go:build integration` と `package integration` の記載が必要であることを添える

---
