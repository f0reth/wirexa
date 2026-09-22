# 変更計画書: HTTP リクエストの永続 ID と実行 ID を分離する

対象: [backend-architecture-review.md](./backend-architecture-review.md) の「4. HTTP リクエストの永続 ID と実行 ID が混同されている」

## 概要

`HTTPRequestService` は実行中リクエストのキャンセル関数と、切り詰めたレスポンスの一時ファイルを
`HTTPRequest.ID`（保存済みリクエストの永続 ID）で管理している。同じ保存済みリクエストを並行送信すると
キャンセル登録と一時ファイルのキーが衝突するため、現在の実装は「同じ ID の送信中リクエストがあれば拒否する」
(`ErrExecutionInProgress`) ことで衝突を回避している。

その結果、frontend は送信のたびに `generateId()` で採番した値を `HTTPRequest.id` に詰めて送り、
保存済みリクエストの ID を送信時に捨てている（`frontend/src/application/http/request.ts:192,206`）。
つまり「永続 ID と実行 ID の混同」は回避されているだけで、RPC の型の上では依然として同じフィールドが
2 つの意味を兼ねている。backend から見ると `req.ID` が保存済みリクエストを指すのかその場限りの
実行 ID なのかを区別できず、ログにも実行 ID しか出ない。

本変更では execution ID を `HTTPRequest` から切り離し、RPC の独立した引数にする。
`HTTPRequest.ID` は保存済みリクエストの識別だけに戻す。あわせて、レビューが指摘した
「application のルート context を `HTTPRequestService` に注入する」も本変更の範囲に含める。

### 本変更で達成される状態

- 同じ保存済みリクエストを何本並行送信しても、それぞれ独立にキャンセル・保存・破棄できる。
- キャンセルが送信の登録より先に backend へ届いても取りこぼさず、その実行はネットワークへ出ない。
- backend のログに永続 ID（`request_id`）と実行 ID（`execution_id`）の両方が出る。
- 実行ごとの context がサービス所有のルート context（アプリの context の子）の子になり、
  `Shutdown` がそのルートを cancel することで、shutdown と context の階層が結び付く。

### 既に実装済みで本変更の対象外の部分

レビュー執筆時点から進んだ結果、以下は対応済みである。本計画では触れない。

- 一時ファイルの execution ID による追跡（`ResponseStore` の `Begin` / `Spill` / `AcquireSave` / `Discard`）
- shutdown 時に実行中の全 HTTP リクエストをキャンセルして完了を待つ処理（`HTTPRequestService.Shutdown`）
- 一時ファイルパスを RPC に出さない lease 方式

## 変更対象ファイル

### バックエンド

| ファイル                                        | 層             | 変更種別 | 変更内容                                                                                                                                  |
| ----------------------------------------------- | -------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/domain/http/port.go`                   | Domain         | 変更     | `HTTPTransport.Do(ctx, executionID, req)` と `RequestUseCase.SendRequest(executionID, req)` へシグネチャ変更。`CancelRequest(executionID)` のコメントを実行 ID 基準に修正 |
| `internal/application/http/request_service.go`    | Application    | 変更     | execution ID を引数で受け取り検証する。登録前に届いたキャンセルを保持して送信開始時に消費する。注入された context から cancel 可能なルートを派生して保持し、実行ごとの context をその子にする。`Shutdown` はルートを cancel する。ログに `request_id` / `execution_id` を出す。内部採番が無くなるため `uuid` の import を外す |
| `internal/infrastructure/http/net_client.go`      | Infrastructure | 変更     | `Do` に `executionID` を追加し、`responses.Begin` / `Finish` / `Spill` へ `req.ID` ではなく `executionID` を渡す。コメント修正                |
| `internal/adapters/http_handler.go`               | Adapters       | 変更     | `SendRequest(executionID string, req httpdomain.HTTPRequest)` へ変更。コメントを「`req.ID` は保存済みリクエストの ID」に修正                 |
| `app.go`                                          | 合成ルート     | 変更     | `httpapp.NewHTTPRequestService(ctx, a.netClient, logger)` へ引数追加                                                                       |

### バックエンドのテスト

| ファイル                                              | 系統      | 変更種別 | 変更内容                                                                                             |
| ----------------------------------------------------- | --------- | -------- | ---------------------------------------------------------------------------------------------------- |
| `internal/application/http/request_service_test.go`     | Go ユニット | 変更・追加 | fake transport の `Do` 追従。永続 ID 衝突・execution ID 重複・execution ID 検証・登録前キャンセル・ルート context のテストを追加 |
| `internal/application/http/file_contents_test.go`       | Go ユニット | 変更     | `SendRequest` 呼び出しに execution ID を追加                                                          |
| `internal/infrastructure/http/net_client_test.go`        | Go ユニット | 変更     | `Do` 呼び出しに execution ID を追加                                                                   |
| `internal/infrastructure/http/net_client_file_test.go`   | Go ユニット | 変更     | 同上                                                                                                  |
| `internal/infrastructure/http/net_client_cleanup_test.go`| Go ユニット | 変更     | 同上。`ErrResponseBusy` のテストは同じ execution ID を再利用する意図を明示                              |
| `internal/integration/http_test.go`                     | Go 統合    | 変更・追加 | `NewHTTPRequestService` に context を追加（69 行目）。全 `SendRequest` 呼び出し（22 箇所）に execution ID を追加。`TestHTTP_CancelRequest` を実行 ID 基準へ。同一永続 ID の 2 並行実行を個別にキャンセルする統合テストと、送信前キャンセルでサーバーに 1 件も届かない統合テストを追加 |

### フロントエンド

| ファイル                                        | 層             | 変更種別 | 変更内容                                                                                                   |
| ----------------------------------------------- | -------------- | -------- | ----------------------------------------------------------------------------------------------------------- |
| `frontend/src/application/http/request.ts`       | Application    | 変更     | `RequestApi.sendRequest(executionId, req)` / `cancelRequest(executionId)` へ変更。送信時の `id` に `activeRequestId()`（未保存なら空文字）を載せる |
| `frontend/src/infrastructure/http/client.ts`     | Infrastructure | 変更     | `sendRequest(executionId, req)` として生成バインディングへ素通しする                                        |
| `frontend/src/application/http/request.test.ts`  | フロント ユニット | 変更・追加 | モック API の追従。送信の `id` が保存済みリクエスト ID になり、execution ID とは別であることを検証          |
| `frontend/src/infrastructure/http/client.test.ts`| フロント ユニット | 変更     | `SendRequest` 呼び出し引数の追従                                                                            |
| `frontend/wailsjs/go/adapters/HTTPHandler.js`    | 生成物         | 再生成   | `task wails:generate`                                                                                        |
| `frontend/wailsjs/go/adapters/HTTPHandler.d.ts`  | 生成物         | 再生成   | 同上                                                                                                         |

### 偽バックエンドと e2e

| ファイル                                          | 系統   | 変更種別 | 変更内容                                                                                          |
| ------------------------------------------------- | ------ | -------- | ------------------------------------------------------------------------------------------------- |
| `frontend/e2e/fake-backend/install.ts`             | UI e2e | 変更     | `SendRequest(executionId, req)` へ変更。`inFlight` のキーを `req.id` から `executionId` に変える。execution ID を Go と同じ規則（空・64 文字超・`[A-Za-z0-9_-]` 以外）で拒否する |
| `frontend/e2e/ui/http/response-save.spec.ts`       | UI e2e | 変更     | `sentExecutionId` を `calls[n][0]`（第 1 引数の文字列）から読むように変更                          |
| `frontend/e2e/ui/http/request-file.spec.ts`        | UI e2e | 変更     | `lastSent` を `calls[last][1]`（第 2 引数のリクエスト）から読むように変更                          |

## 実装方針

### 1. execution ID を RPC の独立した引数にする

```go
// internal/domain/http/port.go
type RequestUseCase interface {
	SendRequest(executionID string, req HTTPRequest) (HTTPResponse, error)
	CancelRequest(executionID string)
}

type HTTPTransport interface {
	Do(ctx context.Context, executionID string, req HTTPRequest) (HTTPResponse, error)
}
```

execution ID は frontend が送信ごとに採番して渡す。backend が採番して返す形にしないのは、
`SendRequest` がレスポンス受信後にしか戻らず、実行中のキャンセルに使う ID を frontend が
持てないためである。backend 採番にするには「送信開始イベント」か「採番専用 RPC」が必要になり、
往復とイベント配線が増える割に、衝突の検出は結局 backend 側の重複チェックで行うことになる。

frontend が値を決める以上、backend は自衛する。`HTTPRequestService.SendRequest` の入口で次を検証する。

- 空文字は `ValidationError{Field: "executionID"}` で拒否する。
  引数の唯一の役割が実行の相関付けである以上、空の execution ID はキャンセルも保存も破棄もできない
  実行を作るだけで、呼び出し側のバグである。現行実装のように内部で採番して黙って続行すると、
  「キャンセルできないリクエスト」がレビュー指摘のまま残る。追跡できない実行を作らないことで、
  shutdown 時の待機対象にも漏れが出ない。
- 長さ 64 文字以下、`[A-Za-z0-9_-]` のみを許可する。RPC 由来の文字列が
  `cancels` マップと `ResponseStore.entries` マップのキーになるため、キー長を有界にする。
  ファイルパスには連結されない（一時ファイル名は `os.CreateTemp` が生成する）ので
  トラバーサルの経路ではないが、UUID 相当の形式を要求しておくことで意図しない値を早期に弾く。
- 同じ execution ID が実行中なら `ErrExecutionInProgress`（既存）。

`HTTPRequest.ID` は backend の実行制御から完全に外れる。`req.ID` を読むのはログの
`request_id` フィールドだけになり、保存済みリクエストの識別という本来の意味に戻る。

依存方向は維持される。ポートの定義は `domain` にあり、`application`（`HTTPRequestService`）と
`infrastructure`（`NetClient`）がそれを実装し、`adapters` は `domain` のインターフェース越しに呼ぶ。
adapters に DTO は追加しない。`executionID` は名前付き文字列型にせず素の `string` で受け取るため、
Wails のバインディング生成にも影響しない。

### 2. キャンセルの順序保証（登録前に届いたキャンセルを取りこぼさない）

`SendRequest` と `CancelRequest` は独立した RPC で、Wails は呼び出しごとに goroutine を起こすため、
webview からの送信順序は goroutine の実行順序を保証しない。`SendRequest` が `s.cancels` へ
キャンセル関数を登録するまでの窓で `CancelRequest` が走ると、現在の実装は「未知の ID は何もしない」
ため、そのキャンセルは黙って捨てられ、リクエストは完走またはタイムアウトまで走り続ける。

これは本変更で新しく生じる問題ではなく、frontend が既に送信ごとの採番値を `req.ID` に詰めている
現行実装にそのまま存在する。ただし本変更の主題が「キャンセルが意図した実行へ確実に届くこと」である以上、
永続 ID との分離と同じ単位で閉じる。POST/PUT/DELETE のような状態変更リクエストで
「キャンセルしたのに送信された」が起きるため、優先度は低くない。

frontend 側では窓が実際に開いている。`loading()` は `setInFlight` の時点で真になり、
Send ボタンが Cancel ボタンへ切り替わるのは `api.sendRequest` を await する前である
（`frontend/src/application/http/request.ts:202`）。

#### 採用する方式: pending cancel（短命な墓標）

`HTTPRequestService` に、実行中でない execution ID へ向いたキャンセルを短時間だけ記録する
マップを持たせ、`SendRequest` が登録と同じ mutex 区間で消費する。

```go
type HTTPRequestService struct {
	// ...
	cancels map[string]context.CancelFunc
	// pending は登録前に届いたキャンセル。SendRequest が同じロック区間で消費する。
	pending map[string]time.Time
	now     func() time.Time // テストで時刻を差し替える (ResponseStore と同じ方式)
}

const (
	// pendingCancelTTL は登録前キャンセルを保持する時間。RPC の到着順の揺れだけを吸収する。
	pendingCancelTTL = 5 * time.Second
	// maxPendingCancels は保持件数の上限。超えたら最古から捨てる。
	maxPendingCancels = 64
)
```

- `CancelRequest(executionID)`: execution ID を `SendRequest` と同じ規則で検証し、不正なら何もしない
  （検証を通らない値を記録できないことで、マップのキーは常に 64 文字以内の既知文字集合に収まる）。
  `cancels` にあればこれまでどおり即座にキャンセルする。無ければ期限切れを掃除したうえで
  `pending[executionID] = now()` を記録する。上限に達していたら最古のエントリを捨てる。
  shutdown 済み（`closed`）なら記録しない。
- `SendRequest(executionID, req)`: 重複チェックの直後、**同じロック区間で** `pending` を見る。
  エントリがあれば削除し、`cancels` へは登録せず、**transport を呼ばずに**
  `fmt.Errorf("failed to send request: %w", context.Canceled)` を返す。

transport を呼んだうえでキャンセル済み context に気づかせる（＝自然に失敗させる）方式は採らない。
`NetClient.Do` は `client.Do` より前に `ResponseStore.Begin` と送信ボディの構築を行い、
file ボディでは token を解決して**ディスクからファイルを読む**ため、
context が既にキャンセル済みでも副作用が先に走ってしまう。
短絡させることで「送信前にキャンセルしたリクエストはネットワークにもディスクにも触れない」
という、テストで断定できる保証になる。

エラー文言を `context.Canceled` の wrap に揃えるのは、実行中キャンセルの既存経路
（`failed to send request: Get "...": context canceled`）と同じ形にして、
frontend に新しい分岐を作らないため。新しい sentinel error は追加しない。

#### キャンセルの意味論（実装前に確定させる）

| キャンセルが届くタイミング       | 振る舞い                                                             |
| -------------------------------- | -------------------------------------------------------------------- |
| 送信の登録より前（TTL 内）       | 送信は開始されない。transport は呼ばれず、キャンセルエラーを返す      |
| 実行中                           | 実行中の context をキャンセルする（現行どおり）                       |
| 完了後                           | 墓標が記録され、TTL 経過で消える。既に返ったレスポンスには影響しない  |
| 未知の ID（送信が来ない）        | 墓標が TTL または件数上限で回収される。副作用なし                     |
| 不正な形式の ID                  | 何もしない。記録もしない                                              |
| shutdown 後                      | 何もしない。実行中のものは `Shutdown` が一括でキャンセルする          |

execution ID は送信ごとに UUID を採番するため、墓標が意図せず別の送信に当たることはない。
TTL 内に同じ execution ID を意図的に再利用した場合はその送信がキャンセル済みとして扱われるが、
これは呼び出し側の ID 再利用が誤りであることの表明として受け入れる。

### 3. ルート context の注入

```go
// internal/application/http/request_service.go
func NewHTTPRequestService(root context.Context, transport domain.HTTPTransport, logger cmn.Logger) *HTTPRequestService
```

`app.go` の `initialize(ctx)` が受け取る Wails の startup context をそのまま渡す。
合成ルートでの注入なので新しい配線パターンは不要で、サービス追加でもないため
`SetupXxxHandler` の二段階配線に変更は入らない（`SetupHTTPHandler` の引数は現状のまま）。

**注意: Wails の startup context は cancel されない。** Wails v2.12.0 の `CreateApp` は
`context.Background()` に `context.WithValue` を重ねるだけで、アプリ終了時にも cancel しない
（`internal/app/app_production.go` / `app_dev.go` で確認済み）。したがって受け取った context を
そのまま親にするだけでは「shutdown で実行が終わる」階層にはならない。

そこでサービスは受け取った context から cancel 可能なルートを派生して自分で所有する。

```go
type HTTPRequestService struct {
	// ...
	root context.Context    // 実行ごとの context の親。Shutdown で cancel される
	stop context.CancelFunc
}

func NewHTTPRequestService(parent context.Context, ...) *HTTPRequestService {
	root, stop := context.WithCancel(parent)
	// ...
}
```

- 各送信は `context.WithCancel(s.root)` で派生させる（現在は `context.Background()`）。
- `Shutdown(timeout)` は `closed = true` にしたうえで `s.stop()` を呼ぶ。これで実行中の全 context が
  cancel されるため、現在の `s.cancels` を回して個別に cancel するループは `s.stop()` に置き換える。
  `wg.Wait()` による待機と「待ち切ったか」の戻り値は現状どおり残す。
- 親（アプリの context）が将来 cancel されるようになった場合も、そのまま全実行へ伝播する。
- `Shutdown` では `pending` も破棄する。

`app.go` 側は引数を 1 つ足すだけで、cancel 関数を合成ルートで保持する必要はない。

### 4. frontend 側

`createRequestState.sendRequest()` は現在 `id: sendId` を渡している。これを次のようにする。

```ts
const executionId = generateId();
const res = await api.sendRequest(executionId, {
  id: activeRequestId() ?? "", // 保存済みリクエストの ID。未保存なら空
  ...
});
```

`inFlight` が保持するのは execution ID のまま、`CurrentResponse.executionId` も現状のままで、
意味と名前が一致する状態になる。未保存（新規タブ）のリクエストは `id` が空文字になるが、
backend は `req.ID` を実行制御に使わないため空でも問題ない。

`infrastructure/http/client.ts` は引数を増やして素通しするだけで、変換ロジックは変えない。

## コード生成

- [x] `task wails:generate` — `HTTPHandler.SendRequest` のシグネチャが変わるため必須。
      `frontend/wailsjs/go/adapters/HTTPHandler.js` と `.d.ts` が更新される
      （CI が `git diff --exit-code frontend/wailsjs` で検査する）
- [ ] `task go:generate:events` — 不要。イベントの追加・変更は無い

## テスト方針

### Go ユニット（`task go:test`）

`internal/application/http/request_service_test.go` に追加する。

- 同じ永続 ID（`req.ID`）のリクエストを異なる execution ID で並行送信すると両方成功し、
  一方をキャンセルしても他方は完走する（レビューが指摘した本体の回帰テスト）
- 同じ execution ID の並行送信は `ErrExecutionInProgress` で拒否される
- execution ID が空、長すぎる、不正文字を含む場合は `ValidationError` を返し、transport を呼ばない
- 注入した親 context をキャンセルすると実行中リクエストの context もキャンセルされる
- `Shutdown` が実行中リクエストの context をキャンセルし、終了を待って `true` を返す（ループを `s.stop()` へ置き換えた後の既存の振る舞いの維持）
- transport に渡る `req.ID` が呼び出し側の指定どおりで、execution ID に置き換えられていない

キャンセルの順序保証（Codex の指摘に対する回帰テスト）。goroutine を使わず、
`CancelRequest` → `SendRequest` の順に**逐次**呼ぶことで競合を決定的に再現する。

- `CancelRequest("exec-1")` の後に `SendRequest("exec-1", req)` を呼ぶと、
  transport の `Do` が **1 度も呼ばれず**、`context.Canceled` を含むエラーが返る
- 墓標は 1 回で消費される。同じ execution ID で再送すると 2 回目は通常どおり送信される
- 注入した時計を `pendingCancelTTL` 以上進めた後の `SendRequest` は通常どおり送信される
- `maxPendingCancels` を超える `CancelRequest` を積むと最古から捨てられ、保持件数が上限を超えない
- 不正な形式・空の execution ID への `CancelRequest` は墓標を作らない（その後の送信が成功する）
- 登録後に届いたキャンセルは従来どおり実行中の context をキャンセルする（既存の振る舞いの維持）
- `Shutdown` 後の `CancelRequest` は何も記録しない

既存の fake transport は `Do(ctx, executionID, req)` へ追従させ、
渡された execution ID を記録して上記の検証に使う。

`internal/infrastructure/http/net_client_cleanup_test.go` の一時ファイル系テストは、
`req.ID` ではなく引数の execution ID で追跡されることを確認する形に読み替える
（同じ `HTTPRequest` を異なる execution ID で 2 回送ると一時ファイルが両方追跡され、
一方の `Discard` が他方を消さないことを追加で検証する）。

### Go 統合（`task go:test:integration`）

`internal/integration/http_test.go`：

- 既存の全 `SendRequest` 呼び出しに execution ID を追加する（機械的変更、22 箇所）。
  `NewHTTPRequestService` の呼び出しには `context.Background()` を渡す
- `TestHTTP_CancelRequest` を「同じ `HTTPRequest`（同一 `ID`）を 2 本並行実行し、
  片方の execution ID だけをキャンセルすると、その 1 本だけが中断して他方は 2xx で完走する」
  へ拡張する。これがレビューの「ユーザーが意図したリクエストと異なる実行がキャンセルされる」
  の直接の回帰テストになる
- 送信前キャンセルの end-to-end 確認を追加する。`httptest.Server` にヒットカウンタを持たせ、
  `h.CancelRequest(execID)` → `h.SendRequest(execID, req)` の順に呼び、
  サーバーへのリクエストが 0 件であることを検証する。ユニットテストが
  「transport を呼ばない」ことを見るのに対し、こちらは「ネットワークに出ない」ことを見る

### フロント ユニット（`task frontend:test`）

- `request.test.ts`: モック API を `sendRequest(executionId, req)` へ追従。
  送信時の `req.id` が `activeRequestId()` と一致し、`executionId` とは別の値であること、
  未保存状態では `req.id` が空文字であることを検証する。
  `saveResponseBody` / `discardResponseBody` に渡るのが送信時の execution ID であること（既存）は維持
- `client.test.ts`: `Handler.SendRequest` が `(executionId, wireRequest)` の順で呼ばれることを検証

### UI e2e（`task frontend:test:e2e`）

バインド API のシグネチャが変わるため `frontend/e2e/fake-backend/install.ts` を同じ形に追従させる。
`inFlight` のキーを execution ID に変え、不正な execution ID は Go と同じ規則でエラーにする。
`response-save.spec.ts` と `request-file.spec.ts` は引数位置の変更に追従する。

pending cancel は偽バックエンドには実装しない。偽バックエンドは単一スレッドの JS で、
`SendRequest` が `inFlight.set` を同期的に済ませてから制御を返すため、
Go 側で問題になる「登録前にキャンセルが割り込む」順序がそもそも発生しない。
UI e2e で再現できない振る舞いを模倣しても、偽物側だけで完結する分岐が増える。
この race は Go のユニット・統合テストで押さえる。

### フルスタック e2e（`task frontend:test:e2e:fullstack`、Windows/ローカル）

シナリオの追加はしないが、生成バインディング経由の送信・キャンセル・レスポンス保存が
通ることの確認として既存の `frontend/e2e/integration/http/http.spec.ts` を実行する。

## 副作用・注意事項

- **RPC のシグネチャが変わる破壊的変更である。** `SendRequest` は引数が 1 つ増える。
  frontend・生成バインディング・偽バックエンドを同一コミット群で揃えないとビルドが通らない。
- **空の execution ID を拒否するようになる。** 現行は空なら backend が内部採番して続行していた。
  RPC を直接叩く経路（WebView 上の任意の JavaScript）が空文字を渡すと
  `ValidationError` になるが、正規の frontend は常に採番して渡すため UI の振る舞いは変わらない。
- **同じ保存済みリクエストの並行送信が許可される。** 現在は `ErrExecutionInProgress` で
  拒否されていた組み合わせが成功するようになる。ただし frontend は元から送信ごとに
  異なる ID を渡していたため、UI から見た振る舞いの変化は無い。
  一時ファイルの件数・総容量の上限（`maxResponseFiles = 8`、2 GiB）と TTL は従来どおり効く。
- **送信前に届いたキャンセルが有効になる。** これまでは黙って無視され、リクエストは送信されていた。
  今後は「キャンセルしたのに送信された」が起きなくなる一方、ユーザーから見ると
  ごく短い窓での Cancel クリックがエラー表示（`failed to send request: context canceled`）を
  伴うようになる。実行中キャンセルと同じ文言・同じ表示経路であり、UI の追加変更は不要。
- **`pending` マップの容量は有界。** キーは検証済み（64 文字・`[A-Za-z0-9_-]`）で、
  件数上限 64・TTL 5 秒のため、RPC を直接叩いて `CancelRequest` を連打されても
  数 KB を超えて増えることはない。
- **ログの項目が増える。** `request_id`（保存済みリクエスト ID、未保存なら空）と
  `execution_id` を出す。ログファイルの互換性を壊す変更ではないが、URL と同様に
  ユーザーデータ由来の値が記録される点は既存と同じ扱いとする。
- レビューの項目 4 のうち、一時ファイルの execution ID 追跡と shutdown 時のキャンセルは
  既に実装済みのため本変更では触らない。項目 1（ファイルアクセス）・3（コレクションの
  トランザクション性）とはファイルが重ならない。

## Git運用

- **ブランチ名**: `refactor/http-execution-id`
- **コミット分割方針**（層の依存方向に沿って下から積む。各コミットでビルドが通る単位にはせず、
  ポート変更から実装追従までを連続した並びにする）
  1. `refactor(domain): HTTP の execution ID をポートの引数に切り出す`
     — `port.go` のシグネチャ変更
  2. `refactor(application): execution ID を検証してルート context の子で実行する`
     — `request_service.go` と既存ユニットテストの追従
  3. `refactor(infrastructure): 一時ファイルの追跡キーを引数の execution ID に変える`
     — `net_client.go` と infrastructure のテスト追従
  4. `refactor(adapters): SendRequest に execution ID 引数を追加する`
     — `http_handler.go` と `app.go` の配線
  5. `chore: Wails バインディングを再生成する`
     — `task wails:generate` の生成物のみ
  6. `refactor(frontend): 送信 ID と保存済みリクエスト ID を分けて渡す`
     — `request.ts` / `client.ts` とフロントのユニットテスト
  7. `test(e2e): 偽バックエンドを execution ID 引数に追従させる`
     — `fake-backend` と UI e2e スペック
  8. `test: 同一リクエストの並行実行を個別にキャンセルできることを検証する`
     — Go ユニット・統合の回帰テスト追加
  9. `fix(application): 送信登録より前に届いたキャンセルを取りこぼさない`
     — pending cancel の追加と、その決定的な回帰テスト（ユニット・統合）。
     execution ID の分離とは独立に読める修正なので単独のコミットに分け、
     分離が終わった土台の上に積む
- **完了後**: `task format` → `task lint` → `task test` → `task frontend:test:e2e`
  （必要に応じて `task go:test:integration`）がすべて通ることを確認 → main へマージ
