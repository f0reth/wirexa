---
name: bug-investigation
description: 潜在的なバグ（クラッシュ・データ破損・競合・リーク・誤動作）を静的に調査し、レポートを出力する
argument-hint: "[backend|frontend] [http|mqtt|udp|openapi]"
disable-model-invocation: true
---

このプロジェクト（Wails + Go + SolidJS）を対象に、潜在的なバグを調査してください。

## 実行手順

**推測・記憶・想像による判断は禁止。** 実際のコードを開いて確認してから判断してください。

1. 「対象範囲の決定」に従って調査範囲と出力先を決める
2. 「lint の実行」に従って `task lint` を実行し、結果を控える
3. 範囲内の調査対象を実際に読む（テスト `*_test.go`・`*.test.ts` は、候補の挙動を守っている・再現しているテストを探すときに読む）
4. 「調査観点」に基づいてバグ候補を洗い出す
5. 各候補を「前提となる設計」と照合し、該当するものは「対応不要」に回す
6. 残った候補を「候補ごとに確認すること」に従って裏付けし、到達経路を確かめきれないものは「要確認」に回す
7. 決めた出力先に結果を出力する

## 対象範囲の決定

全体を 1 回で読むと後半の読みが浅くなるため、`$ARGUMENTS` で範囲を絞れる。

| `$ARGUMENTS` | 調査範囲 | 出力先 |
|---|---|---|
| 空 | 下記「調査対象」のすべて | `docs/bug-investigation.md` |
| `backend` | 「バックエンド」の表だけ | `docs/bug-investigation-backend.md` |
| `frontend` | 「フロントエンド」の表だけ | `docs/bug-investigation-frontend.md` |
| `http`・`mqtt`・`udp`・`openapi` のいずれか | 両側のうち、そのプロトコルのパス（下記） | `docs/bug-investigation-<proto>.md` |
| `backend http` のような側とプロトコルの組み合わせ | 指定した側の、そのプロトコルのパス | `docs/bug-investigation-<側>-<proto>.md` |

- プロトコル指定時の「そのプロトコルのパス」は次のとおり。
  - バックエンド: `internal/{domain,application,infrastructure}/<proto>/`・`internal/adapters/<proto>_handler.go`。http・openapi ではファイルダイアログを共用する `internal/adapters/file_dialog.go` も含める
  - フロントエンド: `{domain,application,infrastructure}/<proto>/`・`presentation/components/<proto>/`・`presentation/providers/<proto>-provider.tsx`・下表のプロトコル専用ファイル

    | プロトコル | `presentation/components/sidebar/` | その他 |
    |---|---|---|
    | http | `collection-*.tsx`・`tree-item-node.tsx`・`use-tree-drag-drop.ts`・`drag-state.ts`・`tree-ui-context.tsx`・`rename-input.tsx`・`use-long-press-drag.ts` | `presentation/constants/http.ts` |
    | mqtt | `broker-tree.tsx`・`profile-list.tsx` | 無し |
    | udp | `target-tree.tsx`・`profile-list.tsx` | 無し |
    | openapi | `openapi-file-*.tsx`・`use-long-press-drag.ts` | 無し |

    `profile-list.tsx`（mqtt・udp）・`use-long-press-drag.ts`（http・openapi）・バックエンドの `file_dialog.go`（http・openapi）は 2 つのプロトコルで共有しているため、どちらを指定しても範囲に含める。修正案を出すときは、もう一方のプロトコルへの影響も確認する。
- 範囲外のファイルは、候補の到達経路（呼び出し元・呼び出し先）の確認のために読む。範囲外のファイル自体のバグはレポートに載せない。範囲外になるのは次のファイル。
  - バックエンドの共通部分: `internal/domain/*.go`・`internal/infrastructure/*.go`・`application/store/`・`internal/adapters/log_handler.go`・`app.go`・`main.go`
  - フロントの共通部分: `shared/`・`config/`・`components/ui/`・`application/{shared,ui}/`・`application/logger.ts`・`presentation/components/shared/`・`presentation/utils/`・`sidebar/index.tsx`・`sidebar/protocol-switcher.tsx` など
- 上記以外の値が渡されたら、調査せずに受け付ける値を示して終了する。

## 調査対象

**バックエンド（`internal/`・リポジトリルート）**

| 層 | 調査パス |
|---|---|
| Domain | `internal/domain/*.go`・`internal/domain/{http,mqtt,udp,openapi}/` |
| Application | `internal/application/{http,mqtt,udp,openapi}/`・`internal/application/store/` |
| Infrastructure | `internal/infrastructure/*.go`・`internal/infrastructure/{http,mqtt,udp,openapi}/` |
| Adapters | `internal/adapters/*_handler.go`・`internal/adapters/file_dialog.go` |
| 合成ルート | `app.go`・`main.go`（初期化・終了処理・状態フラグ） |

**フロントエンド（`frontend/src/`）**

| 層 | 調査パス |
|---|---|
| Domain | `domain/{http,mqtt,udp,openapi,ui}/` |
| Application | `application/{http,mqtt,udp,openapi,shared,ui}/`・`application/logger.ts` |
| Infrastructure | `infrastructure/{http,mqtt,udp,openapi,storage,logger,app}/` |
| Presentation | `presentation/`（`components/`・`providers/`・`constants/`・`utils/`）・`App.tsx`・`index.tsx` |
| 汎用 UI | `components/ui/` |
| Shared | `shared/`・`config/` |

**調査対象外**: `frontend/wailsjs/` と `frontend/src/shared/wails-events.ts`（生成物）、`internal/testutil/`、`internal/integration/`、`frontend/e2e/`、`tools/`（`gen-events`・`e2e-broker` などの開発用ツール）。ただし、これらも候補の到達経路の確認や、観点 C2（fake backend とのずれ）の比較対象としては読む。

## lint の実行

機械的に検出できるものは lint に任せ、手作業の調査は lint で取れないものに集中する。

1. `frontend/dist/index.html` が無ければ、CI と同じく `mkdir -p frontend/dist && touch frontend/dist/index.html` で作る（`go vet` と golangci-lint が `//go:embed` で失敗するため）
2. `task lint` を実行する（`go vet`・golangci-lint（integration タグ付きも含む）・Biome・`tsc -b`）
3. 指摘があれば、範囲内のものを「lint の指摘」に載せる。ツールが無いなど環境の理由で実行できなければ、その旨を「調査対象外・未確認」に書いて調査を続ける

次の観点は `.golangci.yml`・`tsconfig` の設定で既に検出されるので、手作業では探さない。

| 観点 | 検出している設定 |
|---|---|
| error を無視する・`_` に捨てる | `errcheck`（`check-blank: true`） |
| カンマ ok なしの型アサーション | `errcheck`（`check-type-assertions: true`） |
| HTTP レスポンスの Body の close 漏れ | `bodyclose` |
| `context.WithCancel` / `WithTimeout` の cancel 漏れ | `govet`（`enable-all` の lostcancel） |
| TS の暗黙の any・null 非許容型への null 代入 | `tsconfig` の `strict: true` |

`errorlint` は無効なので、`%w` の付け忘れと `errors.Is` / `errors.As` を使わない比較は手作業で調べる（観点 G2）。`noUncheckedIndexedAccess` も無効なので、`arr[0]` の型に `undefined` が含まれない（観点 F1）。

## 前提となる設計（バグとして報告しないこと）

### 依存方向（厳守）

AGENTS.md の「Architecture」（依存方向・入出力ポートの置き場所・合成ルート・import 制限）と「ドメイン型・RPC・永続化の境界」に従う。修正案もこの依存方向を守る。

### 意図的な設計

以下は意図的な設計で、それ自体はバグではない。一般的なチェックリストに当てはめると誤検出になるので、見つけたら「対応不要と判断した箇所」に理由つきで載せる。この前提の範囲を外れる使い方（下表の「バグになり得るのは」）は調査する。

| 設計 | 理由 | バグになり得るのは |
|---|---|---|
| HTTP のタイムアウトを `http.Client.Timeout` ではなく context で表現している（`internal/infrastructure/http/net_client.go` の `Do`） | キャンセルと同じ経路に一本化するため。`buildHTTPClient` の `&http.Client{Transport: ...}` にタイムアウトが無いのは正しい | context が `Do` まで伝わっていない、`resolveTimeout` の値域の扱い |
| ハンドラは `NewApp()` で空のまま作り、`initialize()` で `SetupXxxHandler` により注入する二段階初期化 | Wails が `main.go` でバインドするため。初期化に失敗したら `runtime.Quit` し、`shutdown` は `ready` で守っている | 初期化途中で失敗したときに一部だけ注入された状態で動く経路、`ready` を見ずにサービスを触る経路。ハンドラの全メソッドに nil チェックを求めない |
| `_ = x.Close() //nolint:errcheck // best-effort ...` | 失敗しても続行してよい後始末 | 理由と実態が合わない（書き込み系の Close・Rename の失敗を捨てていて、データ損失を見逃す）もの |
| 破損した設定ファイルを退避してスキップする・空で始める | AGENTS.md「設定データの分類と復旧方針」の表（`internal/application/store/recovery.go` にも同じ表がある） | 表と実装が食い違う箇所（観点 G8） |
| stored DTO の json タグ・`omitempty` の有無 | 保存形式の基準は `internal/infrastructure/*/testdata/*.golden.json` | 保存・復元で値が変わる（ゼロ値と未設定を区別できない等）。修正に保存形式の変更が要るなら「修正の影響範囲」に書く |
| フロントの `application/<proto>/` の `createXxxState` がモジュール外の関数として `createSignal` / `createEffect` / `onCleanup` を呼ぶ | Provider（`presentation/providers/*.tsx`）の本体から同期的に呼ばれ、owner の下で動く | Provider の本体以外（`await` の後、イベントハンドラ、`setTimeout` の中など）から呼ばれる経路 |
| Wails イベントの購読は `EventsOn` の戻り値（解除関数）を `onCleanup` で呼んで解除する | `EventsOff` はそのイベント名のリスナーを全部外すため使わない | 解除関数を捨てている、`onCleanup` を owner の外で呼んでいる。修正案で `EventsOff` を勧めない |
| JSX 内やアクセサ関数内の `props.x ?? default`、`splitProps` 後の JSX スプレッド `{...rest}` | どちらもリアクティブのまま。SolidJS の `<For>` に key は無く、要素の参照で追跡する | コンポーネント本体で `const x = props.x` のように一度だけ読む、JS のオブジェクトスプレッド `{ ...props }`・分割代入 |
| フロントとバックの入力検証の重複（例: `frontend/src/domain/udp/types.ts` と `internal/domain/udp/encoding.go`） | 正はバックエンドで、フロントは UX のための先回り検証 | 両者の判定が食い違い、フロントが通したものをバックが拒否する（またはその逆） |
| HTTP の `FileReference.Token`・OpenAPI の `checkGranted` による許可リスト | RPC で受け取ったパスを検証なしに読み書きしないため（`docs/http-local-file-access-hardening.md`） | 許可リストを通らずにファイルへ届く経路（観点 G9） |

## 調査観点

各観点の ID はレポートの「観点」列に使う。範囲に関係しない観点はスキップし、「調査対象外・未確認」にスキップした旨を書く。

### Go バックエンド

- **G1 nil・ゼロ値**: optional なポインタフィールド（例: stored DTO の `*storedRequest`・`*storedFileReference`）の未チェックのデリファレンス、ゼロ値の struct に含まれる nil マップへの書き込み、範囲外インデックス（UI から来る位置・インデックスをクランプしているか。例: `domain.InsertAt`）。RPC の戻り値やイベントのペイロードに入る nil スライスは JSON で `null` になるため、フロントが配列を前提にしていれば観点 C1 として扱う
- **G2 エラー処理（lint で取れないもの）**: `fmt.Errorf` の `%w` 付け忘れで、呼び出し側の `errors.Is` / `errors.As`（`ErrCorruptData`・`NotFoundError`・`ValidationError` など）が効かなくなる箇所、`==` や文字列での比較、`if err != nil` の後に処理が続く経路、ログだけ出して成功として返す箇所
- **G3 並行安全性**: サービスが持つ共有状態（MQTT の接続マップと `opMu`・`stateMu`、HTTP の `pending`、UDP のリスナー）のロック範囲、ロックの取得順序（デッドロック）、goroutine の終了条件（`receiveLoop`・接続試行・`Shutdown` 内の待機）、`WaitGroup` の `Add` と `Done` の対応、チャネルの二重 close、Stop・Shutdown の後のイベント発行。`app.go` の `App` のフィールドが Wails のコールバック（`startup`・`beforeClose`・`shutdown`・`onSecondInstanceLaunch`）と RPC（`ConfirmQuit`）の両方から触られていないか
- **G4 リソースリーク**: UDP ソケットの close（Listen の失敗・Stop・StopAll の各経路）、HTTP の一時ファイル（`http-sessions`）と `ResponseBodyStore` の `Begin` / `Finish` の対応、ループ内の `time.After`、開いたファイルの close
- **G5 型・データ変換**: 整数の変換（QoS・ポート番号・ミリ秒から `time.Duration` への変換のオーバーフロー）、ペイロードのエンコーディング（`internal/domain/udp/encoding.go` の hex・base64 など）、不正な UTF-8 のバイト列を文字列として JSON で渡す箇所
- **G6 ロジック・境界条件・状態**: 状態機械（MQTT の接続状態、UDP の受信状態）で矛盾した状態に入れる経路、ゼロ値が「未設定」と「意図した 0」を区別できない設定値、off-by-one、条件式の誤り
- **G7 context・キャンセル**: RPC から infrastructure まで context が伝わっているか、キャンセル後に結果を書き戻さないか、`Shutdown` のタイムアウト後も動き続ける処理
- **G8 永続化と復旧方針**: 復旧方針の表と実装の食い違い。具体的には次を見る
  - 破損（`domain.ErrCorruptData`）と読み込み失敗（I/O エラー）の区別（`infrastructure.ReadJSONFile`）
  - 退避先（`infrastructure.QuarantineFile`）
  - 「読めなかった」を「存在しない」と同一視していないか（`CollectionRepository.Exists`）
  - 単一ファイルのポリシー（`store.LoadSingleFile`・`LoadWindowState`）
  - `infrastructure.AtomicWriteFile` を経由しない書き込み
  - 同じファイルへの並行した読み書き
  - 変換関数（`fromStoredXxx`）で不正な値を検証せずにドメインへ入れる箇所

  保存漏れのフィールドは `*_RoundTripKeepsEveryField` が検出するので、意図して保存しないフィールド（例: `persistedCollection`）の扱いが正しいかを見る
- **G9 ファイルアクセス**: RPC で受け取ったパス・トークンを許可リストを通さずに使う経路、パスの結合によるディレクトリ外への到達、一時ファイル名の衝突
- **G10 起動・終了処理**: `beforeClose` → `app:before-close` → フロントの判断 → `ConfirmQuit` → `shutdown` の流れで、終了できなくなる経路・二重に終了処理が走る経路、`shutdown` 内の各サービスの停止順序（HTTP を止めてから一時ファイルを回収する、など）

### TypeScript フロントエンド

- **F1 null・undefined**: Go の nil スライス・nil ポインタに由来する `null` を infrastructure の変換（`infrastructure/<proto>/client.ts` など）で吸収しているか（吸収している箇所は `grep -rn '?? \[\]' frontend/src/infrastructure` などで調査のたびに確かめる）、`arr[0]` のような空配列へのアクセス（`noUncheckedIndexedAccess` が無効なので型で守られていない）、非 null アサーション `!`
- **F2 リアクティビティ**: `createXxxState` や `onCleanup`・`createEffect` を owner の外（`await` の後・イベントハンドラ内）で呼ぶ経路、`createEffect` 内で自分の依存を書き換える無限ループ、条件分岐で依存が追跡されなくなる effect、`createMemo` 内の副作用、`untrack` / `batch` の誤用
- **F3 非同期**: 処理されない Promise の reject（`void` で投げっぱなし、`.catch` のない呼び出し）、応答の到着順が入れ替わって古い結果で状態を上書きする経路（アクティブな接続・文書・コレクションを切り替えた後に前の応答が届く、など）、Provider の破棄後に状態を更新する経路、`onMount(async ...)` 内の処理順序の前提
- **F4 型安全性**: Wails イベントのペイロードに対する `data as {...}` のキャスト（例: `application/mqtt/connections.ts`）と Go 側のペイロード型（`internal/domain/` の型と `internal/domain/events.go` のイベント）の一致、生成型からドメイン型への変換でユニオン型の値を検証しているか、ユニオン型の `switch` の網羅性
- **F5 クリーンアップ漏れ**: イベント購読の解除関数、`setTimeout`・`setInterval`・`requestAnimationFrame`、`window` / `document` への `addEventListener`（ドラッグ&ドロップ・長押し・リサイズ）、`ResizeObserver` などを `onCleanup` で解除しているか
- **F6 ロジック・境界条件**: `if (!value)` が `0`（ポート・QoS）・`""` を意図せず弾く箇所、store の更新で参照を共有したまま書き換える箇所、`<For>`（参照で追跡）と `<Index>`（位置で追跡）の選び違い（毎回オブジェクトを作り直して入力中の要素が作り直される、など）、`<Show>` の fallback 漏れで空白になる状態
- **F7 Props**: コンポーネント本体で props を一度だけ読んでリアクティビティを失う箇所、props の分割代入・JS のオブジェクトスプレッド、props の書き換え

### 共通

- **C1 Go と TS の境界**: Go が実際に返す・発行する値（nil スライス、ユニオンに無い文字列値、エラー文字列）と、フロントの型・分岐の前提の食い違い。生成バインディングに現れないイベントのペイロードは特に確認する
- **C2 fake backend とのずれ**: バインド API の実装と `frontend/e2e/fake-backend/install.ts` の挙動の食い違い（予約コレクション `__root__` の扱い、エラーの有無、並び順など）。製品のバグではなく e2e が実際の挙動を検証できていない問題として、深刻度は「低」とする

## 深刻度

- **高**: 実行時のクラッシュ（panic・未処理の例外で画面が壊れる）、ユーザーデータの破損・消失、データ競合、許可リストを迂回したファイルアクセス、アプリを終了できなくなる
- **中**: 特定の入力・操作で機能が正しく動かない、エラーが握りつぶされて原因を追えない、リソースが解放されず長時間の使用で問題になる
- **低**: 現状は到達しないが将来バグになりやすい作り、一部のエッジケースでの意図しない表示、観点 C2

## 候補ごとに確認すること

各候補について次を調べ、「出力形式」の同名の項目に書く。

- **到達経路**: 問題の箇所に実際に到達する呼び出し経路（RPC・イベント・UI 操作から）。呼び出し元を範囲外まで辿って確かめる。到達しないと分かったものは「対応不要」、辿りきれないものは「要確認」に回す
- **再現条件**: どの操作・入力・タイミングで起きるか
- **修正案**: 依存方向を守った最小限の修正
- **修正の影響範囲**: 呼び出し元・共有状態・他の層への影響。次に当たるものは必ず明記する
  - **保存形式の変更**（`storedXxx` DTO の json タグ・構造。golden ファイルの更新が要る）
  - **設定データの復旧方針の変更**（AGENTS.md と `internal/application/store/recovery.go` の表を両方直す）
  - **RPC のシグネチャ・ドメイン型の json タグ・イベント名の変更**
  - **バインド対象のハンドラメソッドの引数・戻り値やドメイン型のフィールドを、ポインタから値へ、または値からポインタへ変えること**（Wails v2 は `*T` と `T` から同じ TS 型を生成するため `frontend/wailsjs` の差分には出ないが、nil が `null` として渡るかどうかが変わる）
  - **ファイルアクセスの方式の変更**
- **挙動を守っている・再現するテスト**: その箇所を検証している既存テストと、バグを再現するテストを足すならどこに書くか（無ければ「無し（先に再現テストを追加）」）
- **検証コマンド**: 常に `task format` → `task lint` → `task test`。加えて、並行処理に関わるなら `task go:test:race`（Windows では cgo と gcc が要る。gcc の無い環境では「ローカルでは実行できないため CI の結果で確認する」と書き添える）、`internal/integration/` が検証する振る舞いに関わるなら `task go:test:integration`、UI の振る舞いに関わるなら `task frontend:test:e2e`
- **付随作業**:
  - バインド対象の Go 構造体・ハンドラメソッドに触れるなら `task wails:generate` を実行する
  - `internal/domain/events.go` のイベント定数に触れるなら `task go:generate:events` を実行する。イベントを追加・移動する場合は `tools/gen-events/main.go` のリストも直す
  - バインド API の実装やイベントの発行を変えるなら、`frontend/e2e/fake-backend/` への影響の有無を書く

## 出力形式

出力先に以下の構成で Markdown を出力する。ファイルが既に存在する場合は上書きする。見出しには調査範囲を書く。行番号は必須で、「〜あたり」とは書かない。

````
# バグ調査レポート（対象: 全体 / backend / frontend / http など）

生成日: YYYY-MM-DD

## サマリー

| 深刻度 | 件数 |
|--------|------|
| 高     | N    |
| 中     | N    |
| 低     | N    |
| 要確認 | N    |

**最優先で対処すべき問題**: （高深刻度の簡潔な説明。無ければ「無し」）

## 問題一覧

| # | 深刻度 | 観点 | ファイル | 概要 |
|---|--------|------|----------|------|
| 1 | 高     | G3   | internal/application/udp/listener_service.go:NN | ... |

## 問題の詳細

### #1 [深刻度: 高] タイトル（G3）
- **該当箇所**: `ファイルパス:行番号`
- **コード抜粋**:
  ```go
  ...
  ```
- **問題の説明**: 何が起きるか
- **到達経路**: RPC / イベント / UI 操作からの経路
- **再現条件**: どの操作・入力・タイミングで起きるか
- **修正案**: どう直すか（コード例）
- **修正の影響範囲**: 呼び出し元・共有状態・他の層への影響（保存形式・復旧方針・RPC などに当たれば明記）
- **挙動を守っている・再現するテスト**: `xxx_test.go` など（無ければ「無し（先に再現テストを追加）」）
- **検証コマンド**: `task test` など
- **付随作業**: `task wails:generate`・fake-backend の追従など（無ければ「無し」）

## 要確認（到達経路を確かめきれなかったもの）

| # | 観点 | ファイル:行 | 懸念 | 確かめられなかった点 |
|---|------|-------------|------|----------------------|

## lint の指摘

`task lint` の結果（範囲内のもの）。指摘が無ければ「無し」、実行できなければ理由。

## 対応不要と判断した箇所（理由つき）

| ファイル:行 | 観点 | 判断根拠 |
|-------------|------|----------|
| internal/infrastructure/http/net_client.go:NN | G7 | タイムアウトは context で表現する設計（前提となる設計） |

## 調査対象外・未確認

- スキップした観点とその理由、読みきれなかったファイル、実行できなかったコマンド

## 推奨アクション

1. （最優先: 高深刻度の修正）
2. （次点: 中深刻度の修正）
3. （長期: 低深刻度・要確認の調査）
````
