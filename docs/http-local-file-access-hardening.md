# HTTP Local File Access Hardening

作成日: 2026-09-09

関連文書: [Backend Architecture Review](./backend-architecture-review.md#1-http-ファイル送信のローカルファイルアクセスが制限されていない)

## 概要

HTTP 機能では、Wails RPC から受け取ったローカルファイルパスを backend が直接読み込んでいる。また、大きな HTTP レスポンスを保存する際は、backend の一時ファイルパスを frontend へ返し、frontend から同じパスを `SaveResponseBody` へ渡している。

この構造では、WebView 上で XSS、依存パッケージ侵害、または不正な RPC 呼び出しが発生した場合に、backend の権限で任意のローカルファイルを読み取って外部へ送信できる。`SaveResponseBody` は指定された保存元をコピー後に削除するため、任意ファイルの読み取りと削除にも利用できる。

本書では、RPC 境界から生のローカルパスを排除し、backend が発行・管理する不透明な参照だけを受理する設計を定める。

## 対象

次のローカルファイルアクセスを対象とする。

- HTTP request の file body
- multipart/form-data の file 行
- 切り詰められた HTTP response の一時ファイル保存
- 上記に含まれる token、実行 ID、一時ファイルのライフサイクル

OpenAPI のファイル編集、保存先ダイアログでユーザーが選択した出力先、インポート／エクスポート以外の永続ストアは対象外とする。ただし、OpenAPI のパス許可リストが持つ制約は本設計の比較材料として扱う。

## 現状の問題

### Request file

HTTP file body は `RequestBody.Contents["file"]`、multipart file 行は `FormRow.FilePath` に生のパスを保持する。どちらも UI で直接編集でき、送信時に `os.ReadFile` へ渡される。

- file body の読み込み: [`internal/infrastructure/http/net_client.go`](../internal/infrastructure/http/net_client.go#L164)
- multipart file 行の読み込み: [`internal/infrastructure/http/form_body.go`](../internal/infrastructure/http/form_body.go#L40)
- file body のパス入力: [`frontend/src/presentation/components/http/request-editor.tsx`](../frontend/src/presentation/components/http/request-editor.tsx#L150)
- multipart file 行のパス入力: [`frontend/src/presentation/components/http/form-row-editor.tsx`](../frontend/src/presentation/components/http/form-row-editor.tsx#L148)

ファイルパスは保存済みリクエストにも含まれる。保存済みパスを無条件に許可すると、RPC から任意のパスをリクエストへ保存し、その値を後から正規の許可として扱わせることができる。

### Response temporary file

切り詰められたレスポンスの一時ファイルは、`requestID -> tempFilePath` として backend で生成される。しかし、レスポンス返却時に追跡表から削除され、生の一時ファイルパスが frontend へ渡される。

- 一時ファイルの追跡解除: [`internal/infrastructure/http/net_client.go`](../internal/infrastructure/http/net_client.go#L62)
- frontend へのパス返却: [`internal/adapters/http_handler.go`](../internal/adapters/http_handler.go#L56)
- 指定されたパスのコピーと削除: [`internal/adapters/http_handler.go`](../internal/adapters/http_handler.go#L73)

このため `SaveResponseBody` は、Wirexa が生成した一時ファイルと呼び出し側が指定した任意ファイルを区別できない。また、追跡解除後の一時ファイルは通常終了時の `Cleanup` でも回収できず、次回起動時の glob 掃除に依存する。

## セキュリティ要件

1. frontend から backend へ送る RPC 引数に、読み取り許可の根拠となる実パスを含めない。ファイルダイアログの初期位置 hint として渡すパス文字列だけは例外とし、読み取り経路へ到達させない。
2. backend は、OS ファイルダイアログを通して選択された request file だけを読み取る。token の発行源は「ダイアログの戻り値」だけとし、frontend 由来の文字列（入力欄、ペースト、drag & drop）はいずれも許可の根拠にしない。
3. frontend が任意の文字列を token として渡しても、未登録・期限切れならファイルを開かない。
4. 保存済みリクエスト内の旧 `filePath` を、ファイル選択の証拠として扱わない。
5. `SaveResponseBody` は Wirexa が生成して追跡中の一時ファイルだけを保存元にできる。
6. request file token と response 一時ファイルの有効期間、失効、回収を backend が管理する。
7. 実パス、token、一時ファイルパスを通常ログおよびエラーメッセージへ出力しない。
8. frontend が破棄 RPC を呼ばない場合でも、一時ファイルの件数と総容量が backend の上限を超えない。
9. 同じ execution ID に対する送信、保存、破棄、終了処理を直列化し、検査後の差し替えを許さない。

## 推奨設計

### 1. Request file を session-scoped token で参照する

`HTTPHandler.OpenFilePicker` は生のパスではなく、次のような選択結果を返す。

```go
type SelectedFile struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
}

func (h *HTTPHandler) OpenFilePicker(hint string) (SelectedFile, error)
```

`hint` はユーザーが入力欄へ打ち込んだ、またはペースト・drag & drop した文字列で、ダイアログの初期位置を決めるためだけに使用する（詳細は §4）。token の発行はダイアログの戻り値からのみ行い、`hint` は許可の根拠にしない。

backend はダイアログで選択されたパスを内部 registry に登録する。

```text
file token -> 実パス、basename、Content-Type、登録時刻
```

token は `crypto/rand` で生成した 16 byte 以上の乱数を base64url または hex で符号化する。UUID を使用する場合も、乱数強度を曖昧に「128 bit 相当」と扱わず、採用する UUID variant の実効ランダム bit 数を確認する。token の発行経路は backend 内のファイルダイアログ処理だけに限定し、RPC からパスと token の組を登録できる API は設けない。`hint` を受け取る `OpenFilePicker` も例外ではなく、`hint` が指すファイルを無条件に登録してはならない。

送信リクエストでは file body と multipart file 行のどちらも token を渡す。backend は token を registry で解決し、登録済みの場合だけファイルを開く。アクセス判定は実際の読み込み直前に行い、インフラ層へ未検証のパスを渡さない。

ファイル名と Content-Type は表示用情報であり、アクセス許可の判定には使わない。送信時の filename と自動 Content-Type は、frontend が返送した値ではなく registry の情報を正とする。ユーザーが明示した multipart part の Content-Type だけは、従来どおり上書き値として扱う。

### 2. token の有効期間と失効

初期対応では file token を現在のアプリセッションだけで有効とする。発行済み token は同一セッション内の保存済みリクエスト再実行に利用でき、次の場合に失効する。

- アプリ終了時
- registry の明示的な破棄時
- 将来、明示的な TTL または参照管理付き eviction を導入した場合の期限切れ・追い出し時

選択解除のたびに即座に token を失効させると、同じ token を参照する別の保存済みリクエストまで壊す可能性がある。このため初期実装では発行済み token をセッション終了まで保持する。ただし、悪意ある frontend による無制限な registry 増加を避けるため、backend に最大 256 エントリの上限を設ける。初期実装では参照状態を誤判定して有効な token を追い出すより、上限到達後の新規選択を明示的に拒否する。メモリには token とメタデータだけを保持し、ファイル内容やファイルハンドルは保持しない。

未知、空、期限切れの token は、ファイルを開く前に `file access denied: select the file again` 相当の明示的なエラーとする。存在しないファイル、権限変更など、登録後に発生した通常のファイル I/O エラーとは区別する。

### 3. 保存済みリクエストと旧データの扱い

runtime 上のファイル選択情報と永続化するリクエスト表現を分離する。永続データには表示用 basename と「再選択が必要」であることだけを保存し、有効な token と実パスは保存しない。

現状の `CollectionService` は domain オブジェクトを cache に保持し、同じオブジェクトを `JSONStore[Collection]` で直列化している。また frontend の autosave は編集中の body 全体を `UpdateRequest` へ渡す。この構造のまま runtime 型へ token を追加すると token も保存され、保存前に同じオブジェクトから token を消すと同一セッション内の再送が壊れる。

このため初期実装では、次の 3 表現を明示的に分ける。

- Wails RPC DTO: `FileReference{Token, Name, ContentType, NeedsReselect}` を含み、実パスを持たない。
- runtime model: RPC DTO と同等の参照情報を保持し、送信時に token を registry で解決する。
- persistence DTO: `StoredFileReference{Name, NeedsReselect}` だけを保持し、token と実パスを表現できない。

collection repository は `StoredCollection` を保存する専用実装へ変更し、runtime model との変換を repository 境界で行う。現在の `JSONStore[httpdomain.Collection]` をそのまま使用しない。保存用コピーを作る場合は、ポインタ、slice、map を共有しない deep copy とし、runtime cache の token を変更しない。

runtime から persistence DTO へ変換するときは、選択中の token が有効でも token を破棄し、basename がある file reference の `NeedsReselect` を必ず `true` にする。逆変換では token を空にする。これにより、runtime の `NeedsReselect=false` だけが保存され、再起動後に token なしの選択済み状態へ復元される不整合を防ぐ。

`AddRequest`、`UpdateRequest`、collection 移動・名称変更に伴う collection 全体保存のすべてが同じ変換を通るようにする。frontend から渡された token を永続データへコピーできる例外経路を設けない。

既存の `RequestBody.Contents["file"]` および `FormRow.FilePath` にパスが存在する場合は、次のように移行する。

1. パスから basename を表示用に取り出す。旧データの OS と現在の OS が異なる場合に備え、`/` と `\` の両方を区切りとして扱う migration 専用関数を使用する。
2. パスを token registry へ登録しない。
3. runtime 上では token 未選択状態として読み込む。
4. UI にファイルの再選択が必要であることを表示する。
5. cache と RPC 応答からはロード直後に生のパスを除去する。ディスク上の旧 JSON は次回の正常な保存時に persistence DTO で上書きする。

§4 の hint は永続化しない。旧パスをそのまま hint として保存すれば再起動後のダイアログを目的の場所で開けるが、hint は RPC 応答として frontend へ戻るため、実パスが WebView 上の任意の JS に開示される。許可への昇格は起きないものの、要件 1 の「実パスを RPC 境界へ出さない」という性質が失われる。初期実装では basename だけを永続化し、再起動後は既定位置でダイアログを開く。開示範囲を再評価したうえで hint の永続化を選ぶ余地は残す。

保存済みパスを起動時に自動許可してはならない。HTTP collection は RPC から追加・更新できるため、自動許可は「不正な RPC で任意パスを保存し、その後に正規の権限として読み込ませる」経路になる。

token を永続化する registry は、再起動後も再選択なしで送信できる利点がある一方、永続的なファイルアクセス権、孤立 token の回収、collection 更新との整合性管理が必要になる。初期対応では採用しない。

### 4. パス入力欄はダイアログの hint として残す

パスの直接入力とペーストは、既存ユーザーにとって `Browse...` より速い指定手段である。これを廃止せずに残すため、入力欄の意味を「アクセス許可」から「ダイアログの初期位置 hint」へ降格する。

backend から見ると、ユーザーがキーボードで打った文字列と、XSS した JS が埋め込んだ文字列は RPC 引数として区別できない。したがって入力された文字列そのものを許可の根拠にはできない。一方、その文字列を**ファイルダイアログの初期位置としてのみ**使い、token はダイアログの戻り値から発行するなら、許可源はネイティブダイアログのままで、信頼境界は変化しない。

UI の挙動は次のとおり。

- 入力欄は編集可能なままとし、入力・ペーストしても token は発行されない。この状態を「未確定」として明示する（`Not confirmed` 相当のバッジと、送信不可であること）。
- 入力欄での Enter、または `Browse...` で `OpenFilePicker(hint)` を呼ぶ。hint が空なら従来どおり既定位置でダイアログを開く。
- ダイアログで確定された結果だけが token・basename・Content-Type を持つ「確定済み」状態になる。以降 UI が保持・表示するのは basename と確定状態であり、実パスは保持しない。
- 選択済みファイルの解除と、multipart part の明示的な Content-Type 編集は従来どおり。

hint の受け渡しには次の制約を課す。

- hint は `os.ReadFile` を含む読み取り経路へ渡さない。用途は `runtime.OpenDialogOptions` の `DefaultDirectory` と `DefaultFilename` に限定する。
- backend で `filepath.Clean` と長さ上限を適用してからダイアログへ渡す。ディレクトリ部分が存在しない場合は hint を捨てて既定位置で開き、エラーにしない。
- Wails は存在しないディレクトリに対して `default directory '%s' does not exist` を返し、メッセージに実パスが含まれる（`pkg/runtime/dialog.go`）。このエラーを `%w` で RPC へ返してはならない（要件 7）。hint の妥当性は backend 側で事前に判定し、Wails のエラーは分類済みメッセージへ変換する。
- 確定済み状態から入力欄が編集された場合、その file reference は未確定へ戻し、以前の token を送信に使わない。

再起動後または旧データの読み込み後は、以前の basename と `Reselect file` を表示する。旧パスは表示用 basename にのみ変換し、hint の初期値としても実パスを frontend へ渡さない（§3）。未選択の file body を空ボディとして黙って送るか、送信を拒否するかは既存挙動と分離して判断し、少なくとも旧パスが存在する状態では再選択エラーを返す。

#### drag & drop

drag & drop も同じ hint 経路に載せる。ドロップされたパスを入力欄へ反映し、確定は常にダイアログで行う。

Wails v2.12 の `runtime.OnFileDrop` は許可源にできない。macOS / Linux ではネイティブ層が `DD:x:y:paths` という文字列を `window.WailsInvoke` と同一のメッセージチャネルへ送っており（`internal/frontend/desktop/darwin/WailsWebView.m`、`internal/frontend/desktop/linux/window.c`、`internal/frontend/dispatcher/dispatcher.go` の `case 'D'`）、送信元の検証がない。このため WebView 上の JS が `window.WailsInvoke("DD:0:0:/etc/passwd")` を実行するだけで、Go 側コールバックが本物の `[]string` として任意パスを受け取る。ドロップされたパスの信頼度は RPC 引数と同等である。

hint としてのみ使う限り、この偽装が成立しても影響はダイアログの初期位置が変わることに留まる。したがって drag & drop の採用は次の条件付きとする。

- 受け取ったパスは hint 専用とし、token を発行しない。
- `options.App` で `DragAndDrop.EnableFileDrop` を有効にする必要がある。有効化は WebView から到達できる Go 側コールバックを 1 つ増やすことを意味するため、必要になった段階で導入する。
- `pkg/runtime.OnFileDrop` の wrapper は、引数の個数が 3 でない場合に callback を呼んだあと `return` せずに `optionalData[0]` へ添字アクセスする。イベントは `go listener.callback(...)` で別 goroutine から呼ばれ、`ProcessMessage` の `recover` が届かないため、JS からの `EventsEmit("wails:file-drop")` でプロセスが落ちる。この wrapper は使用せず、`runtime.EventsOn(ctx, "wails:file-drop", ...)` を直接購読して、引数の個数と型を自前で検査し `recover` を置く。

この制約は Wails 側の実装に依存するため、Wails を更新した際は `dispatcher` と `draganddrop` の実装を再確認する。

### 5. Response 保存を execution ID で行う

frontend は既に送信ごとに `crypto.randomUUID()` で execution ID を生成している。この ID を、一時ファイル保存にも使用する。ただし execution ID は frontend が決める識別子であり、推測困難性や一意性をセキュリティ境界として信頼しない。

```go
func (h *HTTPHandler) SaveResponseBody(executionID string) error
```

backend は `SendRequest` の受理時に execution ID を予約し、一時ファイルを frontend へ渡す際にも追跡表から取り除かない。追跡情報は単なる `sync.Map` の値ではなく、mutex で状態遷移を管理する registry に保持する。

```text
execution ID -> state、temp file path、Content-Type、作成時刻、size、上限到達状態
```

状態は少なくとも次のように遷移する。

```text
running -> ready -> saving -> removed
             ^         |
             +---------+  保存キャンセルまたはコピー失敗
```

- `running`: request を実行中。まだ一時ファイルは公開可能でない。
- `ready`: 保存または破棄が可能。
- `saving`: 保存ダイアログまたはコピー処理中。破棄、同じ ID の送信、TTL 回収を拒否する。
- `removed`: 保存または破棄が完了。再保存は未追跡 ID と同様に拒否する。

同じ execution ID が `running`、`ready`、`saving` のいずれかに存在する間は、新しい `SendRequest` を拒否する。旧エントリを暗黙に置換しない。レスポンスが切り詰められなかった場合または送信が失敗した場合は、終了時に `running` の予約を解放する。

`HTTPResponse.TempFilePath` は廃止する。frontend は送信に使用した execution ID とレスポンスを関連付け、保存時にその ID だけを渡す。保存ダイアログの拡張子と filter は、呼び出し側の値ではなく追跡情報の Content-Type から決定する。

`SaveResponseBody` は次の順序で処理する。

1. registry の lock 下で execution ID が `ready` であることを確認し、`saving` へ変更して保存用 lease を取得する。
2. 保存ダイアログを開く。
3. キャンセルなら lock 下で `ready` へ戻し、一時ファイルを保持して正常終了する。
4. 選択された保存先へ一時ファイルをコピーする。
5. コピー成功後に一時ファイルを削除し、lock 下で追跡表から除去する。
6. コピーまたは削除に失敗した場合は、元ファイルが残っていることを確認して `ready` へ戻し、再試行できる状態でエラーを返す。元ファイルを失った場合はエントリも除去する。

未追跡の execution ID、保存済み execution ID、`running` または `saving` 状態の ID は、保存ダイアログを開く前に拒否する。execution ID は frontend が生成する値であるため、それ自体を信頼するのではなく、backend の追跡表の状態と保存用 lease をアクセス条件とする。

### 6. Response 一時ファイルの回収

一時ファイルは次の契機で回収する。

- 保存成功時
- frontend がレスポンスを破棄するときに呼ぶ `DiscardResponseBody(executionID)`
- アプリの正常終了時
- 次回起動時の stale temp file sweep
- backend の TTL による回収時

保存ダイアログのキャンセル時は、ユーザーが再度保存できるように保持する。複数の HTTP request が同時に実行されても、execution ID ごとに独立して追跡する。`DiscardResponseBody` は `ready` のエントリだけを破棄でき、`saving` は拒否する。`Cleanup` は新規操作を停止して実行中 request をキャンセルし、bounded wait の後に `saving` でない残存エントリを回収する。待機上限までに終了しない操作のファイルは強制的に競合削除せず、専用 session directory を次回起動時 sweep の対象として残す。

frontend は信頼境界の外側にあるため、`DiscardResponseBody` が呼ばれることを容量管理の前提にしない。初期実装から backend に次の制限を設ける。

- 追跡可能な response 一時ファイル数: 8
- 追跡済み一時ファイルの合計容量: 2 GiB
- 同時に spill できる response 数: 2
- `ready` 状態の TTL: 60 分

値は設定ではなく安全側の内部定数から開始し、実利用データに基づいて変更する。容量は spill 開始前に 1 response の絶対上限分を予約し、書き込み完了時に実サイズへ精算することで、複数 request が同時に上限をすり抜けないようにする。上限到達時は既存ファイルを無断で消すのではなく、新しい spill を停止して明示的なエラーを返す。TTL 回収は `ready` だけを対象とし、`running` と `saving` を削除しない。

一時ファイルは可能であれば flat な OS temp 直下ではなく、権限 `0700` の Wirexa 専用 session directory 内へ作成する。起動時 sweep は Wirexa が作成した session directory だけを対象にする。

## Public API・型の変更案

実装時は少なくとも次の変更が必要になる。

| 現在 | 変更後 |
| --- | --- |
| `OpenFilePicker() (string, error)` | `OpenFilePicker(hint string) (SelectedFile, error)` |
| file body のパス文字列 | runtime file reference の token。UI の入力欄は hint として存続 |
| `FormRow.FilePath` | runtime file reference の token と表示用 basename。UI の入力欄は hint として存続 |
| `GuessFormPartContentType(path)` | 選択結果の Content-Type を使用。必要なら token を引数にする |
| `HTTPResponse.TempFilePath` | 削除。frontend が execution ID と response を関連付ける |
| `SaveResponseBody(tempFilePath, contentType)` | `SaveResponseBody(executionID)` |
| なし | `DiscardResponseBody(executionID)` |

Go の RPC DTO、runtime model、persistence DTO、frontend domain 型、Wails 生成 bindings、fake backend を同じ変更単位で更新する。永続化については前節の 3 表現への分離を採用し、runtime token を保存前後の任意の sanitization に依存させない。

## エラーとログの秘匿

`os.PathError` などの OS エラーを `%w` でそのまま RPC 境界または通常ログへ渡すと、registry 内部の実パスや一時ファイルパスが `error()` 文字列に含まれる。同様に、Wails の `OpenFileDialog` は不正な `DefaultDirectory` に対して `default directory '%s' does not exist` を返し、hint として渡した実パスがメッセージに含まれる。file registry と response temp registry の外側へ返すエラーは、次の安定した分類へ変換する。

- `file access denied`: token が空、未知、失効済み
- `selected file unavailable`: 選択後の削除、権限変更、読み取り失敗
- `failed to open file picker`: ダイアログの起動失敗。hint の不正はここへ含めず、hint を捨てて既定位置で開き直す
- `response body unavailable`: execution ID が未知、期限切れ、保存済み
- `response body busy`: 保存、破棄、または同じ ID の操作が競合
- `response storage limit exceeded`: 件数、総容量、同時 spill 上限への到達
- `failed to save response`: 保存先への copy、flush、close、または元ファイル削除の失敗

これらのメッセージへ token、入力された execution ID、実パス、一時ファイルパス、および frontend から渡された hint を連結しない。通常ログにも同じ値を出さず、必要な診断情報は操作種別、状態、サイズ、OS エラーの分類または errno に限定する。保存先パスについても通常ログへは出さない。

## テスト可能な境界

現状の `HTTPHandler` は Wails の `OpenFileDialog` と `SaveFileDialog` を直接呼び出している。ダイアログを開く前の拒否、キャンセル、copy/remove 失敗を deterministic な unit test にするため、次の依存を interface として注入する。

```go
type FileDialog interface {
	OpenFile(ctx context.Context, options OpenDialogOptions) (string, error)
	SaveFile(ctx context.Context, options SaveDialogOptions) (string, error)
}

type ResponseFileStore interface {
	AcquireSave(executionID string) (ResponseFileLease, error)
	Discard(executionID string) error
	Cleanup() error
}
```

production adapter は Wails runtime と OS filesystem を使用し、unit test は fake dialog、隔離した temp directory、失敗を注入できる file operation を使用する。registry の状態遷移と容量管理は Wails context を必要としないサービスとしてテストする。

## Symlink・パス差し替え対策

session token は「任意のパスを RPC から指定できる」問題を解消するが、選択後から送信時にファイルを開くまでの間に、同じパスの対象が symlink や別ファイルへ差し替えられる競合は残る。`filepath.Clean` や文字列 allowlist だけではこの問題を解決できない。

より強い対策は、ファイル選択直後にファイルを開き、そのハンドルを token に関連付けて保持することである。ただし、次のデメリットがある。

- 選択ファイル数に応じた file descriptor 消費
- Windows における削除、移動、上書きの阻害
- ファイルが atomic replace された場合に、ユーザーが期待する最新版ではなく旧 inode を読む可能性
- ハンドルの失効、上限、終了時 close の管理
- アプリ再起動をまたいで復元できない

このため初期対応では token に実パスを関連付け、送信時に開く方式とする。ファイルハンドル保持は、ローカルの別プロセスまたは攻撃者によるパス差し替えまで脅威モデルに含める第2段階の hardening とする。実装前に Windows、macOS、Linux それぞれのファイル置換動作を検証する。

## メリットとデメリット

### メリット

- WebView から任意のローカルパスを指定して送信する経路を遮断できる。
- response 保存 API を任意ファイルの読み取り・削除に転用できなくなる。
- 実パスと一時ファイルパスが RPC payload に現れなくなる。
- 許可、失効、cleanup の責任が backend に集約される。
- response 一時ファイルを保存時まで追跡でき、通常終了時にも確実に回収できる。
- token を用途別に分けることで、request upload 用参照を response 保存へ流用できない。
- パスの直接入力とコピー＆ペーストによる指定手段を維持できる。

### デメリット

- 既存の保存済み file request は、アプリ再起動後にファイルの再選択が必要になる。
- パスの直接入力、コピー＆ペースト、drag & drop はダイアログの hint へ降格し、指定ごとにネイティブダイアログでの確定操作が 1 回増える。
- 入力欄が「確定済み」と「未確定」の 2 状態を持つため、UI と状態遷移が複雑になる。
- backend registry、runtime／永続型の分離、UI 状態、Wails bindings の変更が必要になる。
- token の失効、孤立した一時ファイル、複数同時 request を含むライフサイクルテストが増える。
- session token を取得できる悪意ある JavaScript は、そのセッションで既にユーザーが選択したファイルにはアクセスできる。token 化は WebView 内スクリプト同士を隔離する仕組みではなく、アクセス範囲をユーザー選択済みファイルへ限定する仕組みである。
- 初期段階では symlink／パス差し替えの TOCTOU リスクが残る。

セキュリティ上の影響が任意ファイルの外部送信および削除であることを考えると、互換性や実装コストよりも対策のメリットが大きい。

## 段階的な導入順序

1. file dialog、response file store、file operation のテスト用 interface と、排他的な registry の土台を追加する。この段階では公開 RPC の挙動を変えない。
2. `SaveResponseBody` を execution ID ベースへ変更し、状態遷移、容量上限、TTL、`DiscardResponseBody`、UI の破棄処理を同じ変更単位で導入する。
3. Wails RPC DTO、runtime model、persistence DTO を分離し、旧パスを basename と再選択状態へ変換する migration を追加する。
4. file body について、request file registry、`SelectedFile`、token 解決、`OpenFilePicker(hint)` と入力欄の hint 化を同じ変更単位で導入する。
5. multipart file 行について、token 解決、入力欄の hint 化、明示 Content-Type の扱いを同じ変更単位で導入する。
6. 旧 `filePath` と `Contents["file"]` を公開 RPC 型および frontend 型から削除し、legacy path を受理する互換経路が残っていないことを確認する。
7. drag & drop を hint 源として導入する（任意）。`DragAndDrop.EnableFileDrop` の有効化、`wails:file-drop` の自前購読と引数検査、hint 専用であることのテストを含む。
8. symlink／パス差し替え対策として、ファイルハンドル保持または OS 固有の安全な open 方法を別途評価する。

response 保存を先行させるのは、既存の request 永続形式に影響せず、任意ファイルの読み取り・削除を小さい変更範囲で解消できるためである。

手順 2、4、5 はそれぞれ backend、Wails bindings、frontend、fake backend、テストまでを含む原子的な変更単位とする。backend が token 専用なのに UI が path を送る状態や、互換性のために backend が token と raw path の両方を受理する中間状態をリリースしない。

## テスト計画

### Backend unit tests

- ダイアログを通さず作成した任意 token を拒否する。
- 空、未知、期限切れ、改ざん済み token を拒否する。
- `OpenFilePicker(hint)` の hint が token を発行せず、ダイアログがキャンセルされた場合に何も登録されない。
- hint がダイアログの初期位置以外へ渡らず、読み取り経路へ到達しない。
- 存在しないディレクトリ、空文字、極端に長い文字列を hint に渡しても、エラーにせず既定位置でダイアログを開く。
- hint を含むエラーとログに実パスが出力されない。
- drag & drop で受け取ったパスが token を発行せず、hint としてのみ扱われる。
- `wails:file-drop` を不正な引数個数・型で受け取ってもパニックせず、hint を更新しない。
- 登録済み token が file body と multipart file 行の両方で使用できる。
- filename と Content-Type のアクセス判断に frontend の値を使用しない。
- registry の並行読み書きで race が発生しない。
- request file registry が 256 エントリを超えて増えず、上限到達時に新規選択を拒否する。
- 未追跡 execution ID では保存ダイアログを開かない。
- 保存成功後に一時ファイルと追跡エントリを削除する。
- 保存キャンセル時は一時ファイルと追跡エントリを保持する。
- copy、remove、discard の失敗時に追跡状態が不整合にならない。
- 複数の execution ID が互いの一時ファイルを参照できない。
- 同一 execution ID の並行送信を拒否し、他の request の cancel entry を上書きしない。
- `SaveResponseBody` のダイアログ表示中は同じ ID の送信、破棄、TTL 回収を拒否する。
- 保存、破棄、TTL、cleanup を並行実行しても二重削除、別ファイル保存、孤立ファイルが発生しない。
- 追跡ファイル数、合計 2 GiB、同時 spill 数の各上限を並行 request で超えられない。
- `DiscardResponseBody` が一度も呼ばれなくても TTL または容量制限で使用量が有界になる。
- `Cleanup` と stale sweep が残存ファイルを回収する。
- 返却エラーと logger の記録に token、実パス、一時ファイルパスが含まれない。

### Migration tests

- file body の旧パスを自動許可せず、basename と再選択状態へ変換する。
- multipart の旧 `FilePath` を自動許可しない。
- 不正・空・Windows／Unix 形式の旧パスを読み込んでも、そのパスへアクセスしない。
- 移行後の保存データに生のパスと session token が残らない。
- `AddRequest`、`UpdateRequest`、名称変更、移動による collection 保存のどの経路でも token が永続化されない。
- 保存用 deep copy または DTO 変換が runtime cache の有効な token を消さない。
- migration 後の `GetCollections` と `GetRootItems` が旧パスを frontend へ返さない。

### Frontend unit and E2E tests

- file body と multipart file 行でパスを直接入力・ペーストでき、入力しただけでは未確定状態のまま送信できない。
- 入力欄の値が `OpenFilePicker` へ hint として渡り、ダイアログ確定後に確定済み状態へ移行する。
- 確定済みの入力欄を編集すると未確定へ戻り、以前の token が送信に使われない。
- drag & drop されたファイルが入力欄へ反映され、ダイアログ確定まで送信できない。
- 選択結果の basename、Content-Type、再選択状態を正しく表示する。
- 再起動相当の reload 後に、保存済み file request が再選択を要求する。
- file を再選択すると同一セッション内で繰り返し送信できる。
- response 保存で execution ID だけを backend へ渡す。
- response の置換、request 切り替え、破棄時に `DiscardResponseBody` を呼ぶ。
- 保存キャンセル後に再度保存でき、保存成功後は再保存できない。
- 複数 request の応答順が入れ替わっても、表示中 response と execution ID の対応が崩れない。
- backend が容量上限または TTL で response を回収した場合に、保存不可の状態と再送案内を表示する。

### Verification commands

実装後は Taskfile 経由で次を実行する。

```sh
task generate
task test
task test:go -- -race
task test:go:integration
task test:e2e
task generate:check
task check
```

実 backend の変更を含むため `task test:e2e:fullstack` も実行する。OS ファイルダイアログを E2E から自動操作できない環境では、少なくとも選択、キャンセル、保存、再保存拒否、response 切り替え時の破棄を各対応 OS で手動 smoke test する。hint については、パスをペーストしたときにダイアログが目的の場所で開くこと、存在しないパスでも既定位置で開くことを各 OS で確認する。drag & drop を導入する場合は、ドロップが入力欄を更新するだけで送信可能状態にならないことも併せて確認する。

## 受け入れ条件

- RPC 引数へ任意の絶対パスを渡しても、HTTP request を通じてそのファイルを読み取れない。
- file body と multipart file 行は、現在のセッションでダイアログ選択された token だけを受理する。
- 入力欄、ペースト、drag & drop 由来のパスが、ダイアログでの確定なしに許可へ昇格しない。
- 旧パスまたは保存済みパスが暗黙に許可へ昇格しない。
- `SaveResponseBody` の RPC から保存元パスを指定できない。
- response の一時ファイルは backend の追跡外へ出ず、保存、破棄、終了のいずれかで回収される。
- frontend が破棄を行わなくても、一時ファイル数、合計容量、同時 spill 数が backend の上限を超えない。
- 同じ execution ID の送信、保存、破棄を並行実行しても、別 response の保存、二重削除、孤立ファイルが発生しない。
- frontend domain 型と Wails bindings に request の実パスおよび response の一時ファイルパスが公開されない。
- collection の全保存経路で session token と旧パスが永続化されず、runtime token は正常な autosave 後も利用できる。
- 通常ログと frontend へ返すエラーに実パス、token、一時ファイルパスが含まれない。
- 既存の text、JSON、form-urlencoded、file を含まない multipart request の送信挙動が変わらない。
- 初期対応後に残る symlink／パス差し替えリスクが、既知の残存リスクとして追跡される。

## 採用しない案

### パス文字列の allowlist だけを追加する

既存構造への変更は小さいが、生のパスが RPC 境界に残り、保存済みパスの再許可、パス情報の露出、symlink 差し替えの問題が残る。token 化への暫定措置としてのみ有効で、最終設計にはしない。

### drag & drop されたパスをそのまま許可源にする

`runtime.OnFileDrop` で受け取ったパスをダイアログの戻り値と同格に扱えば、ドロップだけで送信できる。しかし Wails v2.12 では、ネイティブ層が `DD:x:y:paths` を `window.WailsInvoke` と同じメッセージチャネルへ流し、dispatcher が送信元を検証しない。WebView 上の JS が `window.WailsInvoke("DD:0:0:<任意パス>")` を送れば同じコールバックが呼ばれるため、ドロップされたパスの信頼度は RPC 引数と変わらず、本設計の要件 2 を満たさない。§4 のとおり hint 源としてのみ採用する。

### 入力されたパスをネイティブ確認ダイアログで承認する

`runtime.MessageDialog` にパスを表示し、ユーザーが OK した場合に token を発行する案。ペーストだけで完結する点は hint 方式より速いが、ユーザーが承認するのは「自分が辿ったファイル」ではなく「WebView が提示した文字列」になる。確認ダイアログの形骸化、悪意ある JS によるダイアログ連打、長大パスの省略表示や Unicode の RTL override・homoglyph による表示偽装への対策が別途必要になり、「ユーザーが選択したファイルだけ」という不変条件が「ユーザーが OK を押した文字列だけ」へ弱まる。hint 方式で同等の入力手段を維持できるため採用しない。

### 保存済み request のパスを起動時に自動許可する

RPC から保存された任意パスを後から許可へ昇格できるため採用しない。OpenAPI recents の seed は、追加経路が backend のダイアログ処理に限定されているため成立しており、RPC から更新可能な HTTP collection にはそのまま適用できない。

### token と実パスの対応を初期段階から永続化する

再選択を減らせる一方で、永続的なアクセス権の失効、registry と collection の整合性、孤立 token の回収を新たに解決する必要がある。初期対応の安全性と単純性を優先するため採用しない。

### 選択ファイルを常にアプリ管理領域へコピーする

パス差し替えを避けられるが、大容量 request file のディスク使用量、コピー時間、原本更新との乖離、cleanup が問題になる。必要になった場合は、ファイルハンドル保持と合わせて別途比較する。
