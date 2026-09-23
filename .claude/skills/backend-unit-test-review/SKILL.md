---
name: backend-unit-test-review
description: バックエンド（Go）の単体テストケースの網羅性を多角的な観点で分析し、Markdownレポートを出力する
disable-model-invocation: true
argument-hint: "[パッケージパス | ファイルパス | 関数名]"
---

バックエンド（`internal/`）の単体テストケースが十分かを分析し、`docs/backend-unit-test-review.md` にレポートを出力する（既にあれば上書き）。リポジトリに書き出してよいのはこのファイルだけ。

> この Skill に書いたファイル名・関数名・テストダブル名は**執筆時点の例**である。必ず現在のコードで確かめ、食い違えば現在のコードを正とする（例をそのまま事実としてレポートに書かない）。

層構成・依存方向・保存形式・復旧方針は CLAUDE.md を基準にする。CLAUDE.md とコードが食い違っていれば、それ自体をレポートに書く。

## テストの前提

- **外部のテスト・モックライブラリはない**（testify / gomock / go-cmp なし）。アサーションは `t.Fatalf` / `t.Errorf` と `reflect.DeepEqual`。`AssertExpectations` や `EXPECT()` がないことを不足として報告しない。
- テストダブルはテストファイル内の手書き struct で、名前は統一されていない（`mockXxx` / `fakeXxx` / `recordingXxx` / `inMemoryRepo` / `memFiles` / `countingOpener` / `barrierEmitter` など）。接頭辞では探さず、出力ポートなどの interface を満たす struct をテストダブルとして扱う。
  - 振る舞いの差し替え: `doFn func(...)` のような関数フィールド（例: `internal/application/http/request_service_test.go` の `mockTransport`）。未設定時に既定の成功値を返すものがある。
  - 呼び出しの記録: スライスやカウンタのフィールド（例: `mockTransport.execIDs`、`recordingLogger.errors`）。並行に呼ばれるものは `sync.Mutex` で守っている。呼び出し確認は、この記録フィールドをテストが読んで検証しているかで判断する。
- 保存形式のテスト基盤（観点 K の基準）:
  - `*_RoundTripKeepsEveryField`: `testutil.Populate` で全フィールドを埋めた値を保存 → 読み込みして比較する。意図して保存しないフィールドは期待値の補正関数（例: `persistedCollection`）に理由付きで書く。
  - `*_GoldenFormat`: `testdata/*.golden.json` を読めて、書き戻すと同じ JSON になるか。
  - `testutil.AssertNoTypesFrom(t, storedXxx{}, testutil.DomainPkg)`: stored DTO が domain 型を埋め込んでいないか。

## 対象スコープ

- **分析対象**: `internal/**/*_test.go` と同パッケージの本体コード（`internal/adapters/` を含む）。`internal/integration/`（Step 0 の分類にだけ使う）と `internal/testutil/` は除外する。
- テストファイルがないパッケージ・ファイルも「未テスト」として含める。関数・メソッドを持たないもの（型・interface・定数・エラー変数の定義だけ。例: `internal/domain/mqtt`・`internal/domain/openapi`）は含めない。
- **`$ARGUMENTS` の指定があれば** Step 0〜3 すべてをその範囲に絞る。

| 指定形式 | 例 | 動作 |
| --- | --- | --- |
| パッケージパス | `internal/application/http` | そのパッケージのみ |
| ファイルパス | `internal/application/http/request_service.go` | そのファイルのみ |
| 関数名 | `SendRequest` | その関数が定義されているファイルで、その関数に絞る |

  同名の関数・メソッドが複数ある場合（例: `SendRequest` は `HTTPHandler` と `HTTPRequestService` の両方にある）はすべて対象にし、`ファイル:行` と受け手の型で区別して、どれを対象にしたかを冒頭に書く。
- **`$ARGUMENTS` なし（`internal/` 全体）の場合**: 不足テストケースの詳細は優先度 **高・中** だけ書き、低 はサマリーの件数に含めるだけにする。冒頭に「低優先度の詳細はパッケージを指定して再実行すると出力される」と書く。

## 分析手順

### Step 0: 文カバレッジの取得

主観に頼らないよう、最初に実行されていない行を機械的に把握する。プロファイルなどの一時ファイルはリポジトリの外（OS の一時ディレクトリなど）に出す。`//go:embed` は `main.go` にしかないので `frontend/dist` のスタブは不要。

1. `go test -coverprofile=<tmp>/cover.out ./internal/...`（絞り込み指定があればそのパッケージだけ）
2. `go tool cover -func=<tmp>/cover.out` で 0% の関数とカバー率の低い関数を控える。パスは先頭の `github.com/f0reth/Wirexa/` を除き、`internal/adapters/http_handler.go:161` の形にそろえる。`internal/testutil/` は載せない。
3. 0% の関数を分類するため、統合テストのカバレッジを取る（`$ARGUMENTS` の指定に関係なくこのまま実行）:
   `go test -tags integration -coverpkg=./internal/... -coverprofile=<tmp>/integ.cover.out ./internal/integration/...`
   - 外部ネットワークは不要（組み込み mochi ブローカーとループバック）で、Windows でも動く。
   - 失敗したらレポート冒頭に書き、「統合テストで検証済み」の分類は使わずに続ける。
   - サマリーの文カバレッジには合算しない。
4. 並行処理（goroutine・mutex・チャネル）がある対象では `go test -race ./internal/...`（絞り込み指定があればそのパッケージだけ）を実行し、観点 G の根拠にする。
   - `task go:test:race` は `./...` 固定で絞り込めず、対象外のパッケージも含むので使わない。
   - Windows で cgo / gcc がなく実行できなければ冒頭に書き、CI（ubuntu）が通っている前提で読み取りだけで判断する。

テストの失敗・ビルドエラーはレポート冒頭に出力を載せ、読めた範囲で分析を続ける。OS 別ファイル（`*_windows.go` / `*_windows_test.go` など）は実行した OS のものしか計測されず、CI は ubuntu だけで動く点に注意する。

0% の関数は次の分類に当たるかを先に確かめる。当たるものは不足テストケースにせず、0% 関数の一覧で分類名を添えるだけにする。

- **Wails ランタイム依存**: Wails が渡す `ctx` で `github.com/wailsapp/wails/v2/pkg/runtime` を直接呼ぶもの（`adapters/file_dialog.go` の `wailsFileDialog`、`infrastructure/wails_emitter.go`、`infrastructure/window_manager.go` の全関数（`clamp` も `runtime.ScreenGetAll` を呼ぶ））。runtime を呼ばない判定は `window_state.go` の `LoadWindowState` にあり、そちらは通常どおり分析する。
- **委譲のみ**: 引数を入力ポートへそのまま渡し、戻り値をそのまま返すだけの Handler メソッド。型変換だけのもの（例: `UDPHandler.StartListen`）やエラー時に戻り値をゼロ値にそろえるだけのもの（例: `HTTPHandler.SendRequest`）も含む。分岐・エラーの変換・引数の組み立てがあるもの（例: `LogHandler.Log` の `switch entry.Level`）は通常どおり分析する。
- **依存の注入のみ**: 依存をフィールドに代入するだけの関数（`SetupXxxHandler`、`JSONStore.SetLogger` など）。nil チェックや既定値の補完があれば通常どおり分析する。
- **統合テストで検証済み**: 手順 3 で実行されていて、**かつ本物の I/O（実ソケット・実ブローカー・OS 固有の API など）に依存する**もの（例: `infrastructure/udp/net_socket.go`、`infrastructure/mqtt/paho_client.go` の接続まわり）。一覧に統合テストでのカバー率を添える。
  - 本物の I/O なしで単体テストを書ける関数（純粋なロジック、`t.TempDir()` で確かめられるファイル操作。例: `DropFileContents`、`domain/udp` の `Validate`、各リポジトリの `Delete`）は、統合テストで通っていてもこの分類に入れない。
  - 統合テストでも 0% のもの（例: `applyTLSScheme`）は分類しない。統合テストで通らない分岐は観点 A〜K で分析してよい。

未実行の分岐は Step 3 の候補にする。ただしアサーションのない実行もカバー率に含まれるので、検証の有無は Step 2 で判断する。

### Step 1: 本体コードの把握

テストより先に本体コード（`_test.go` 以外）を読み、以下を列挙する（レポートの「対象関数」テーブルに使う）。

- 関数・メソッド一覧（公開・非公開両方）と入出力の型・意味
- 分岐（if/switch/for）と状態遷移
- エラーを返しうる箇所

### Step 2: 既存テストの読み取り

- テスト関数名と実際にテストしているケース、テーブルドリブンテストの入力値の範囲
- 何をテストダブルで差し替え、何を実際に実行しているか（`t.TempDir()` の実ファイルを使っているか）
- アサーションの内容（戻り値の中身まで見ているか。`errors.Is` / `errors.As` で種類まで確かめているか）
- テストダブルの記録フィールドを読んで、引数・回数・順序を確かめているか
- 時間の扱い（注入した `now` を差し替えているか、`time.Sleep` に頼っていないか）

### Step 3: 不足ケースの検出

以下の観点ごとに、**本体コードのファイル名と行番号を根拠に**不足ケースを検出する。一般論は禁止。本体コードにない分岐・組み合わせは不足にしない（表の例は観点の説明用）。対象に該当しない観点（例: 並行処理のないパッケージへの G）は「該当なし（理由）」と書いてスキップする。

#### 観点 A: 正常系の網羅

他の観点より先に確認する前提。

| チェック項目 | 例 |
| --- | --- |
| 最も基本的な入力での成功ケース | 必須フィールドだけ埋めたリクエストで正常レスポンスが得られるか |
| 複数の正常バリエーション | GET/POST/PUT それぞれに正常系があるか |
| 成功時の戻り値の検証 | `err == nil` だけでなく戻り値の中身まで確認しているか |

#### 観点 B: 入力値の境界

| チェック項目 | 例 |
| --- | --- |
| 最小値・最大値 | 0, 1, -1, MaxInt、len=0, len=1 |
| 空文字列 vs 空白のみ vs nil | `""`, `" "`, `nil` |
| Unicode・特殊文字 | `\n`, `\t`, null byte |
| バリデーション境界の前後 | 許可メソッド一覧の直前/直後 |

#### 観点 C: 状態遷移の境界

| チェック項目 | 例 |
| --- | --- |
| 初期状態からの操作 | 未初期化のサービスへの呼び出し |
| 終了状態からの操作 | Close 後に SendRequest、二重クローズ |
| 許可されていない状態遷移 | 接続前に Publish |
| 操作の順序依存 | Create→Update→Delete の中間をスキップ |

#### 観点 D: 組み合わせの境界

| チェック項目 | 例 |
| --- | --- |
| 必須フィールドの欠落 | URL なし、Method なし |
| 判別フィールドと無関係な値の同時指定 | `Auth.Type` が `basic` なのに `Token` もある（`net_client.go` の `switch req.Auth.Type`） |
| 依存関係のあるフィールド | Body があるのに Content-Type なし |
| Enabled フラグの組み合わせ | Header の Enabled 制御 |

#### 観点 E: 異常系の境界

| チェック項目 | 例 |
| --- | --- |
| 依存コンポーネントのエラー | Transport が err を返す |
| エラーの種類による分岐 | `errors.Is` / `errors.As` による sentinel error の判定 |
| エラー後の状態整合性 | リソースリークがないか |
| パニックになりうる箇所 | nil ポインタデリファレンス |

#### 観点 F: 時間の境界

| チェック項目 | 例 |
| --- | --- |
| キャンセル済み context | `cancel(); svc.Do(ctx, ...)` |
| タイムアウト直前・直後 | 100ms 制限に 99ms vs 101ms |
| ゼロ値のタイムアウト | `Timeout: 0` |
| デッドライン超過済み context | `DeadlineExceeded` |
| 注入した時計の活用 | 本体が `now` フィールド（`request_service.go`・`openapi/file_service.go`・`file_registry.go`・`response_store.go`）を持つのに、テストが実時間に依存している |
| `time.Sleep` による待ち合わせ | 固定時間の Sleep で非同期処理を待っている。チャネル・記録フィールドのポーリング・`testing/synctest` で置き換えられないか。ただし実ブローカーや実ソケットを待つ場合は synctest の仮想時計が進まないので候補にしない |

#### 観点 G: 並行性の境界

| チェック項目 | 例 |
| --- | --- |
| 同一リソースへの並行書き込み | 複数 goroutine から同じサービスを呼ぶ |
| goroutine リーク | 長時間ブロッキング後のキャンセル |
| 競合状態 | `go test -race` で落ちるパターン |
| close 後のチャネルへの send | |

#### 観点 H: 環境の境界

| チェック項目 | 例 |
| --- | --- |
| ストレージファイルが存在しない | 初回起動時の空状態 |
| ネットワーク不達 | 到達不能ホストへの接続 |
| OS 別ファイルのテスト | `*_windows.go` にだけある分岐のテストが `*_windows_test.go` にしかなく、CI で実行されない（`file_open_windows.go` / `file_open_other.go` など） |

**保存データの破損・読み込み失敗**は、CLAUDE.md の復旧方針表で判定する。対象ファイルの分類（必須 / best effort / 再生成可能）ごとに、表の各セル（破損 / 退避失敗 / 破損以外の読み込み失敗）と表の下の規則（破損と I/O エラーの区別、`QuarantineFile` の退避先の衝突、`__root__` 作成前の `Exists` 確認、`LoadWindowState`）に対応するテストがあるかを確認する。

#### 観点 I: 戻り値・副作用の検証

| チェック項目 | 例 |
| --- | --- |
| 戻り値の中身が未検証 | `err == nil` のみで `Response.StatusCode` 等が未確認 |
| 副作用の確認漏れ | Create 成功後に実際に保存されたか、イベントが発火したかを見ていない |
| エラー時の内容が未検証 | エラーが返ることだけ確認し、種類やメッセージを見ていない |

#### 観点 J: テストダブルの検証

| チェック項目 | 例 |
| --- | --- |
| 記録フィールドを読んでいない | `execIDs` などを記録しているのにテストが検証していない |
| 呼び出し引数の未検証 | 渡された値（リクエスト内容・ID 等）を確認していない |
| 呼び出し回数・順序の未検証 | `len(記録)` を見ておらず、複数回呼ばれてもパスする |
| 「呼ばれないこと」の未検証 | エラー時・キャンセル時に後続の保存やイベント発火が起きないことを確かめていない |
| 既定の成功値で隠れた失敗 | `doFn` 未設定時の既定値のせいで、本来の経路を通らなくてもパスする |
| テストダブルで隠れた動作 | 下の層を差し替えているため検証できない動作は「テストダブルで未検証」と記載する |

#### 観点 K: 保存形式の境界

対象: リポジトリの stored DTO と変換関数（`toStoredXxx` / `fromStoredXxx` など）。リポジトリを持たないパッケージは「該当なし」。

| チェック項目 | 例 |
| --- | --- |
| 往復テストの有無 | `*_RoundTripKeepsEveryField` がない |
| 保存しないフィールドの理由 | 期待値の補正関数で落としているフィールドに理由がない |
| golden の有無 | `*_GoldenFormat` と `testdata/*.golden.json` がない、または書き戻した JSON との一致を確かめていない |
| domain 型の埋め込み検査 | stored DTO に対する `AssertNoTypesFrom` がない |
| 変換関数の分岐 | `fromStoredXxx` に旧形式の移行・既定値の補完などの分岐があるのに、その入力を与えるテストがない |

## 出力形式

```markdown
# バックエンド単体テストレビュー

生成日時: YYYY-MM-DD
対象: internal/ 配下の単体テスト（integration除く）

---

## サマリー

| パッケージ | テスト関数数 | 文カバレッジ | 不足ケース数 | 優先度(高/中/低) |
| --- | --- | --- | --- | --- |
| application/http | N | N% | N | 高 |

> **文カバレッジ**: Step 0 の値。取れなかった場合は `-` とし、理由を下に書く。
> **優先度**: パッケージ内の不足ケースのうち最も高いもの。

### カバレッジ 0% の関数

`ファイル:行 関数名` で列挙し、Step 0 の分類に当たるものは分類名（統合テストで検証済みはそのカバー率も）、ほかにテスト不要と判断したもの（例: 単純なコンストラクタ）は理由を添える。

`$ARGUMENTS` なしの場合は関数ごとに列挙せず、`ファイル: N 件（うち Wails ランタイム依存 N / 委譲のみ N / 依存の注入のみ N / 統合テストで検証済み N）` とファイル単位でまとめる。

---

## パッケージ別詳細

### `internal/application/http`

#### 対象関数

| 関数名 | ファイル:行 | 分岐数 | テスト済み? |
| --- | --- | --- | --- |
| SendRequest | request_service.go:10 | 3 | 部分的 |

> **テスト済み?**: `済`（全分岐カバー）/ `部分的` / `未テスト`。Step 0 の分類に当たる関数は `未テスト（委譲のみ）` のように分類名を添え、不足ケース数に数えない。

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象関数**: `SendRequest`
- **不足内容**: URL が空文字列 `""` のケースがテストされていない
- **根拠コード**: `request_service.go:18` の `if req.URL == ""` 分岐が未カバー
- **推奨テスト名**: `TestSendRequest_EmptyURL`
- **優先度**: 高

---

## 全体評価

### 観点別カバレッジ

> 各観点のチェック項目のうち、既存テストで 1 ケース以上確認できるものの割合（該当なしの項目は分母から除く）。テストの読み取りによる判断で、文カバレッジとは別の指標。

- **[A] 正常系の網羅**: N% カバー済み（不足N件）
- …（[K] 保存形式の境界 まで同じ形式）

### 最優先で追加すべきテスト TOP5

1. ...（根拠: どのコードパスが無防備か）

### 総合評価

**信頼度**: 高 / 中 / 低
**主なリスク**: （未テストのコードパスで起きうる障害）
**推奨アクション**: （具体的な次のステップ）
```
