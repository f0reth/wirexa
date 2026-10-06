---
name: refactor-assistant
description: 既存機能を壊さずにできるリファクタリング・可読性向上・潜在バグ修正の候補を調査し、レポートを出力する
argument-hint: "[backend|frontend] [http|mqtt|udp|openapi|common]"
disable-model-invocation: true
---

このプロジェクト（Wails + Go + SolidJS）を対象に、既存機能を壊さずに改善できる箇所を調査してください。調べるのは次の 3 種類です。

- **リファクタリング**: 重複の共通化、責務の整理、不使用コードの削除など、構造の改善
- **可読性**: 挙動も構造も変えずに、読み手が意図を取り違えにくくする改善
- **潜在バグ**: 今は表に出ていないが、特定の入力・操作順・タイミング、または今後の変更で不具合になるコードの修正

指定された引数: 「$ARGUMENTS」（かぎ括弧の中が空なら引数なし）

## 実行手順

**推測・記憶・想像による判断は禁止。** 実際のコードを開いて確認してから判断してください。

1. 「対象範囲の決定」に従って調査範囲と出力先を決める
2. 範囲内の調査対象を実際に読む（テストは「候補ごとに確認すること」の「挙動を守っているテスト」を探すときに読む）
3. 調査観点に基づいて改善候補（リファクタリング・可読性・潜在バグ）を洗い出す
4. 各候補を「プロジェクトの規約・意図的な設計」と照合し、該当するものは「対応不要」に回す
5. 決めた出力先に結果を出力する

## 対象範囲の決定

全体（テストを除いて Go 約 8,000 行・TS 約 11,500 行）を 1 回で読むと後半の読みが浅くなるため、引数で範囲を絞れる。

| 引数 | 調査範囲 | 出力先 |
|---|---|---|
| 無し | 下記「調査対象」のすべて | `docs/refactor-report.md` |
| `backend` | 「バックエンド」の表だけ | `docs/refactor-report-backend.md` |
| `frontend` | 「フロントエンド」の表だけ | `docs/refactor-report-frontend.md` |
| `http`・`mqtt`・`udp`・`openapi` のいずれか | 両側の、そのプロトコルのパス | `docs/refactor-report-<proto>.md` |
| `common` | 両側の共通部分 | `docs/refactor-report-common.md` |
| 側とプロトコルまたは `common` の組み合わせ（`backend http`・`frontend common` など。順不同） | 指定した側の、そのプロトコルのパスまたは共通部分 | `docs/refactor-report-<側>-<proto>.md`・`docs/refactor-report-<側>-common.md`（順序によらず側を先に書く） |

上記以外の値（`common http` のようなプロトコルと `common` の組み合わせを含む）が渡されたら、調査せずに受け付ける値を示して終了する。

### プロトコルのパス

- バックエンド: `internal/{domain,application,infrastructure}/<proto>/`・`internal/adapters/<proto>_handler.go`
- フロントエンド: `{domain,application,infrastructure}/<proto>/`・`presentation/components/<proto>/`・`presentation/providers/<proto>-provider.tsx`・下表のプロトコル専用ファイル

  | プロトコル | `presentation/components/sidebar/` | その他 |
  |---|---|---|
  | http | `collection-*.tsx`・`tree-item-node.tsx`・`use-tree-drag-drop.ts`・`drag-state.ts`・`tree-ui-context.tsx`・`rename-input.tsx`・`use-long-press-drag.ts` | `presentation/constants/http.ts` |
  | mqtt | `broker-tree.tsx`・`profile-list.tsx` | 無し |
  | udp | `target-tree.tsx`・`profile-list.tsx` | 無し |
  | openapi | `openapi-file-*.tsx`・`use-long-press-drag.ts` | 無し |

  `profile-list.tsx`（mqtt・udp）と `use-long-press-drag.ts`（http・openapi）は 2 つのプロトコルで共有しているため、どちらを指定しても範囲に含める。改善候補を出すときは、もう一方のプロトコルへの影響も確認する。

### 共通部分

どのプロトコルにも属さない次のファイル。プロトコル指定時は範囲外になり、`common` 指定時はこれだけが範囲になる。

- バックエンド: `internal/domain/*.go`・`internal/infrastructure/*.go`・`internal/application/store/`・`internal/adapters/file_dialog.go`（http・openapi で共用）・`internal/adapters/log_handler.go`・`app.go`・`main.go`
- フロントエンド: `shared/`・`config/`・`components/ui/`・`domain/ui/`・`application/{shared,ui}/`・`application/logger.ts`・`infrastructure/{logger,app}/`・`presentation/components/shared/`・`presentation/utils/`・`sidebar/index.tsx`・`sidebar/protocol-switcher.tsx`・`App.tsx`・`index.tsx`

### `infrastructure/storage/local-storage.ts`

http・mqtt・udp の保存実装とテーマの保存を 1 ファイルに持つため、部分ごとに範囲を分ける。

- http・mqtt・udp の保存実装は、それぞれのプロトコルの範囲。プロトコル指定時は、指定したプロトコルの保存実装と、共通の `loadFromStorage`・`saveToStorage`・`removeFromStorage` の使い方だけを候補の対象にする
- テーマの保存（`createThemeStorage`）と、`loadFromStorage`・`saveToStorage`・`removeFromStorage`・形の検査関数（`isString` など）の定義は共通部分
- openapi は保存実装を持たないので、openapi 指定時は範囲に含めない
- 保存実装のほとんどは `domain/<proto>/ports.ts` のポートの実装だが、http の `createActiveRequestStorage` だけはポートが無く、`http-provider.tsx` が戻り値の型を推論でそのまま使っている

### 範囲外のファイルの扱い

- 範囲外のファイル（プロトコル指定時は共通部分と指定していないプロトコルのパス、`common` 指定時はすべてのプロトコルのパス）は、候補の影響範囲の確認や「既に共通化されたものを再実装していないか」の確認のために読む。範囲外のファイル自体の改善候補はレポートに載せない。
- `common` 指定時に共通部分の公開シンボルを変える・消す候補を出すときは、4 プロトコルすべての呼び出し元を確認する。
- プロトコル間の共通化候補は、範囲外のプロトコルの対応箇所を比較対象として読んだうえで載せてよい。`common` 指定時は、共通部分どうしの重複と共通部分そのものの改善だけを載せ、「プロトコル側の重複を共通部分へ移す」候補は載せない（プロトコル指定または側だけの指定で調べる）。

## 調査対象

**バックエンド（`internal/`・リポジトリルート）**

| 層 | 調査パス |
|---|---|
| Domain | `internal/domain/*.go`・`internal/domain/{http,mqtt,udp,openapi}/` |
| Application | `internal/application/{http,mqtt,udp,openapi}/`・`internal/application/store/` |
| Infrastructure | `internal/infrastructure/*.go`・`internal/infrastructure/{http,mqtt,udp,openapi}/` |
| Adapters | `internal/adapters/*_handler.go`・`internal/adapters/file_dialog.go` |
| 合成ルート | `app.go`・`main.go` |

**フロントエンド（`frontend/src/`）**

| 層 | 調査パス |
|---|---|
| Domain | `domain/{http,mqtt,udp,openapi,ui}/` |
| Application | `application/{http,mqtt,udp,openapi,shared,ui}/`・`application/logger.ts` |
| Infrastructure | `infrastructure/{http,mqtt,udp,openapi,storage,logger,app}/` |
| Presentation | `presentation/`（`components/`・`providers/`・`constants/`・`utils/`）・`App.tsx`・`index.tsx`（エントリポイント） |
| 汎用 UI | `components/ui/` |
| Shared | `shared/`・`config/` |

**調査対象外**（改善候補は出さないが、候補の影響範囲としては確認する）: 生成物（`frontend/wailsjs/`・`frontend/src/shared/wails-events.ts`）、`internal/testutil/`、`internal/integration/`、`frontend/e2e/`、`tools/`（`gen-events`・`e2e-broker` などの開発用ツール）、`frontend/src/` の CSS（`*.module.css`・`index.css`。上記の行数にも含めていない）。

## 前提となる設計（違反する提案をしないこと）

### 依存方向（厳守）

AGENTS.md の「Architecture」（依存方向・入出力ポートの置き場所・合成ルート・import 制限）と「ドメイン型・RPC・永続化の境界」に従う。加えて次を前提とする。

- openapi は保存用のポートを持たないため `domain/openapi/ports.ts` は無い。
- infrastructure → application の既存 import 2 か所の import 先である `application/openapi/ports.ts`（`ParseResult`）と `application/logger.ts`（`Logger`）は、application に置かれたポートとして現行の設計で扱う。domain への移動などは提案せず、見つけたら「対応不要と判断した箇所」に理由つきで載せる。
- フロントの `application/` が solid-js の signal / store で状態を持つのは現行の設計であり、それ自体は問題にしない。

### 意図的な重複（共通化・削除を提案しない）

以下は同期漏れを防ぐため、または境界を分けるための**意図的な重複**。見つけたら「対応不要と判断した箇所」に理由つきで載せる。

| 重複 | 理由 |
|---|---|
| フロント `domain/<proto>/types.ts` と Go のドメイン型 | Go の型が RPC の配線型の正で、フロント側はユニオン型・型ガードで意味付けする層。生成型→フロント型の変換は infrastructure で行う。生成型を application / presentation から直接使う案は規約違反（変換の書き方については表の下を参照） |
| infrastructure の `storedXxx` DTO と Go のドメイン型 | 保存形式を RPC の json タグから独立させるため（AGENTS.md「ドメイン型・RPC・永続化の境界」）。`testutil.AssertNoTypesFrom` は struct 型だけを検査するが、現状の stored DTO に名前付きの基本型を使う箇所は無いので、使用箇所を探さなくてよい。統合・埋め込みの提案は禁止 |
| adapters に RPC 用 DTO が無いこと | ドメインと同じ形の DTO を adapters に新設する提案は禁止（ドメインに対応物の無いアダプタ固有の入力型は可） |
| フロントとバックの入力検証（例: `frontend/src/domain/udp/types.ts` の数値検証と `internal/domain/udp/encoding.go`） | 正はバックエンド。フロントは送信前の先回り検証で、UX のために持つ |
| フロントのエラー文言定数（例: `RESPONSE_UNAVAILABLE_ERROR` と `ErrResponseUnavailable`） | RPC 越しにはエラー文字列しか渡らないため |
| 名前付き文字列型を RPC のメソッド引数で `string` として受けて内部変換している箇所 | Wails がメソッド引数の名前付き型を生成できない制約への回避策 |

入力検証とエラー文言定数は、削除ではなく「ずれをどう防ぐか」（テスト・コメントでの対応付け）なら提案してよい。

型のミラー自体は意図的な重複だが、生成型→フロント型の変換の書き方の不統一（検証なしのキャスト、Go 側の項目追加で受信・送信のどちらかで黙って落ちる列挙）は改善候補にしてよい。現状の書き方はプロトコルで違う。

- **http**（`client.ts`）: スプレッドで素通ししてユニオン型だけを型ガードで絞り込むのが基本。`Collection`・`TreeItem`・`KeyValuePair`・`RequestAuth`・`FileReference` はフィールドを列挙し、`getSidebarLayout` は列挙したうえで `kind` を検証なしに `as` でキャストしている
- **udp**（`client.ts`）: スプレッドで素通し。`UdpListenSession` の `encoding` は型ガード（`isPayloadEncoding`）を使わず `as` でキャストしている。イベント（`onMessage`）は `EventsOn` のペイロードを検証なしで `UdpReceivedMessage` 型のコールバックへ渡している
- **mqtt**（`client.ts`）: 受信方向は戻り値を `as` でキャストするだけで検証しない（`saveProfile`・`deleteProfile` はキャストも無く生成型のまま返す）。送信方向は `connect` と `saveProfile` がフィールドを列挙して渡している（http・udp は `createFrom` で素通しするので、Go 側に項目を足すと mqtt だけ送信時に黙って落ちる）。イベントのペイロードは `infrastructure/mqtt/events.ts` を `unknown` のまま通り、`application/mqtt/connections.ts` が `as` でキャストしている
- **openapi**: `client.ts` は無い。`infrastructure/openapi/file-io.ts` がファイル I/O と最近使ったファイルの RPC を受け持ってフィールドを列挙し、`parser.ts` が仕様の解析を受け持つ

## 調査観点

### リファクタリング

**Go バックエンド**

- 重複コードの共通化（4 プロトコル間の類似処理。共通の置き場所になり得る `internal/infrastructure/*.go`・`internal/domain/*.go`・`internal/application/store/` に既にあるものを再実装していないか）
- エラーハンドリングの不統一（`fmt.Errorf` の `%w` 付け忘れ、センチネルエラー・`NotFoundError`・`ValidationError` を使うべき所で文字列だけのエラーを返している、`errors.Is`/`errors.As` ではなく文字列比較している、など。`fmt.Errorf("...: %w")` とカスタムエラー型の併用自体は Go の通常の書き方なので問題にしない）
- インターフェース・ポートの不必要な肥大化
- 不要なポインタ渡し・値渡しの混在（RPC とイベントの境界の内側、つまり application・infrastructure の内部に限る。境界にかかるものは「禁止事項」参照）

**TypeScript フロントエンド**

- `application/` 層での状態管理の重複（4 プロトコル間の類似パターン）
- `presentation/` コンポーネントへのロジック漏れ（application に移せる業務ロジック）
- 型定義の重複（フロント内部での重複定義。Go 型のミラーは「意図的な重複」参照）
- Solid.js のリアクティビティを正しく活用していない箇所

**共通**

- 同じロジックがバックエンド・フロントエンド両方に実装されている箇所（意図的な重複かどうかを必ず判定する）
- 不使用コード（dead code）。リンターが既に検出するものは探さない（Go は golangci-lint の `unused`、TS は `noUnusedLocals`・`noUnusedParameters`）。探すのはリンターが拾えないもの: export されているがどこからも参照されていないシンボル、テストからしか使われていない公開シンボル、常に同じ値で呼ばれる引数や到達しない分岐
- 肥大化したファイル・関数の責務単位での分割（目安はテストを除いて 500 行を超えるファイル。例: `internal/application/mqtt/service.go`・`internal/application/http/collection_service.go`・`frontend/src/application/mqtt/connections.ts`）。提案するのは、公開シンボルと挙動を変えずに同じパッケージ・同じ層の中でファイルや関数を分ける案だけ。構造を作り直す案は「禁止事項」の大規模リライトにあたる

### 可読性

挙動も公開シンボルの意味も変えずに、読み手が意図を取り違えにくくする改善を探す。整形やリンターが既に指摘するもの（gofmt・golangci-lint・biome）は探さない。好みの違いにとどまる書き換えも載せず、「どう読み違えるおそれがあるか」を説明できるものだけを候補にする。

- 実際の役割とずれた名前（処理内容が変わったのに古い名前のまま、同じ概念に複数の名前、別の概念に同じ名前）
- コードと食い違っているコメント・doc コメント、理由が書かれていない非自明な処理（回避策・順序への依存・意図的に握りつぶしているエラー）。コメントは日本語で書く
- 深い入れ子や長い条件式（早期リターン、条件に名前を付けた変数・関数への切り出しで読みやすくなるもの）
- 意味の分からない数値・文字列リテラル（定数にして名前を付けられるもの）
- 呼び出し側から意味が読み取れない引数（`bool` の並び、同じ型の引数が続いて順序を取り違えやすいもの）
- 1 つの関数に複数の抽象度の処理が混ざっていて、流れを追うために細部まで読む必要がある箇所

### 潜在バグ

今は不具合として表に出ていないが、特定の入力・操作順・タイミング、または今後の変更で不具合になる箇所を探す。**「どういう条件で、何が起きるか」をコードから具体的に示せるものだけ**を候補にする（「念のため」の防御的なコードの追加は載せない）。既に起きている不具合を見つけた場合も、この種別で載せる。

**Go バックエンド**

- 並行処理: ロックなしで共有される状態、ロックを持ったままの I/O・イベント発行・コールバック呼び出し、終了しない goroutine、close 済みチャネルへの送信、`context` のキャンセルを見ない待ち
- リソースの解放漏れ（エラー経路で `Close`・`Unlock`・購読解除が呼ばれない、ループ内の `defer`）
- 握りつぶしているエラー、エラー時にも後続の処理が走って中途半端な状態を保存・通知する箇所
- nil・ゼロ値・空スライス・空文字列を想定していない箇所（nil マップへの書き込み、範囲外の添字、ゼロ除算）
- スライス・マップを呼び出し元と共有したまま返す・保持する箇所（呼び出し元の変更が内部状態に波及する）
- 複数の手順からなる更新が途中で失敗したときに、メモリ上の状態と保存データが食い違う箇所
- 型・列挙値を足したときに黙って素通りする分岐（`default` の無い `switch`、列挙値の一覧を手で持っている箇所）

**TypeScript フロントエンド**

- 非同期処理の競合: `await` の後で、古い応答が新しい状態を上書きする、既に削除・切り替え済みの対象へ書き込む
- 後始末の漏れ（`onCleanup` の無いイベント購読・タイマー・`EventsOn`、解除関数を捨てている箇所）
- 捕捉していない Promise の拒否、失敗をユーザーにも記録にも出さずに終わる箇所
- リアクティビティの取りこぼし（props の分割代入、追跡範囲の外での signal 読み取り、`await` の後での読み取り）と、effect どうしが互いに書き換え合う構造
- 検証なしの `as` キャスト・非 null アサーション（`!`）で、実行時の値が型と食い違いうる箇所（生成型→フロント型の変換は「意図的な重複」の表の下を参照）
- ユニオン型に値を足したときに黙って素通りする分岐（網羅性を検査していない `switch`・`if` の連鎖）
- localStorage から読んだ値など、外から来たデータの形を検査せずに使っている箇所

**共通**

- フロントとバックで同じ前提（上限値・予約名・既定値・エラー文言）を別々に持っていて、片方だけ変えるとずれる箇所。意図的な重複にあたるものは、削除ではなく「ずれをどう防ぐか」を提案する
- `frontend/e2e/fake-backend/` と Go の実装で挙動が食い違っている箇所（食い違いは Go 側の候補の付随作業として書く。fake-backend 自体の改善候補は載せない）

## 禁止事項

- 大規模リライト
- フレームワーク変更
- 挙動変更。このプロジェクトでは以下も挙動変更として扱う:
  - **保存形式の変更**（`storedXxx` DTO の json タグ・構造。基準は `internal/infrastructure/*/testdata/*.golden.json`）
  - **設定データの復旧方針の変更**（破損・読み込み失敗時の扱い。AGENTS.md と `internal/application/store/recovery.go` の表）
  - **RPC のシグネチャ・ドメイン型の json タグ・イベント名の変更**
  - **バインド対象のハンドラメソッドの引数・戻り値やドメイン型のフィールドを、ポインタから値へ、または値からポインタへ変えること**。Wails v2 は `*T` と `T` から同じ TS 型を生成するため、`frontend/wailsjs` の差分には出ない。それでも nil が `null` として渡るかどうかが変わる
  - **ファイルアクセスの方式の変更**（HTTP の `FileReference.Token`、OpenAPI の `checkGranted` による許可リスト。RPC で受け取ったパスを検証なしに読み書きしない。`docs/http-local-file-access-hardening.md`）
  - **潜在バグの修正だけは、不具合が起きる条件での挙動を変えてよい。** ただし正常系の挙動と、上に挙げた 5 項目（保存形式・復旧方針・RPC とイベントの形・ポインタと値の別・ファイルアクセスの方式）は変えない。これらを変えないと直せない場合は、改善候補にせず「対応不要と判断した箇所」に「修正には仕様の判断が必要」として、発生条件と一緒に載せる
- クリーンアーキテクチャの破壊（上記「依存方向」に反する提案）
- 生成物を手で編集する提案

## 候補ごとに確認すること

各候補について次を調べ、「出力形式」の同名の項目に書く。

- **挙動を守っているテスト**: その箇所を検証している既存テスト。ユニットテスト（`*_test.go`・`*.test.ts`）に加えて、`internal/integration/` と `frontend/e2e/ui/**/*.spec.ts` も探す。`presentation/` と `components/ui/` はユニットテストがほとんど無く、UI の振る舞いは e2e の spec が守っている（例: `http/tree-drag-drop.spec.ts`・`http/sidebar-operations.spec.ts`）。どれにも無いときだけ「無し」とする。潜在バグの候補では、既存テストが正常系しか見ていないのが普通なので、発生条件を再現するテストを先に追加する前提で、追加先のテストファイルと確認する内容を書く
- **発生条件と影響**（潜在バグの候補だけ）: どの入力・操作順・タイミング、またはどんな変更をしたときに、何が起きるか。該当する行を追って確かめた内容だけを書き、確かめられなかった部分はそう明記する。修正で挙動が変わる範囲（修正前と修正後）も書く
- **検証コマンド**: 常に `task format` → `task lint` → `task test`。加えて、並行処理に触れるなら `task go:test:race`、`internal/integration/` が検証する振る舞いに関わるなら `task go:test:integration`、UI の振る舞いに関わるなら `task frontend:test:e2e`
- **付随作業**:
  - バインド対象の Go 構造体・ハンドラメソッドに触れるなら `task wails:generate` を実行する
  - `internal/domain/events.go` のイベント定数に触れるなら `task go:generate:events` を実行する。イベントを追加・移動する場合は `tools/gen-events/main.go` のリストも直す
  - バインド API の実装やイベントの発行を動かすなら、`frontend/e2e/fake-backend/` への影響の有無を書く（fake-backend も `WailsEvents` を参照している）
  - コンポーネントの削除・分割・移動を提案するなら、対応する `*.module.css` の扱いを書く

## 出力形式

出力先に以下の構成で Markdown を出力してください。見出しには調査範囲を書く。

優先度は、潜在バグなら発生しやすさと影響の大きさ（データの消失・破損、固まる、誤った結果を黙って表示する、は高）、リファクタリングと可読性なら、今後の変更で誤りを生みやすい箇所ほど高くする。一覧は優先度の高い順に並べ、同じ優先度の中では潜在バグを先に書く。

```
# リファクタリング調査レポート（対象: 全体 / backend / frontend / http / common など）

## 改善候補一覧

| 優先度 | 種別 | ファイル | 内容 |
|--------|------|----------|------|
| 高     | 潜在バグ / リファクタリング / 可読性 | ... | ... |

## 改善候補の詳細

### [タイトル]
- **種別**: リファクタリング / 可読性 / 潜在バグ
- **該当箇所**: `ファイルパス:行番号`
- **現状**: 何が問題か
- **発生条件と影響**: （潜在バグだけ）どういう条件で何が起きるか、修正で挙動が変わる範囲
- **改善案**: どう直すか
- **期待効果**: 可読性向上 / 重複削減 / 不具合の予防 / パフォーマンス改善 など
- **挙動を守っているテスト**: `xxx_test.go`・`xxx.test.ts`・`e2e/ui/.../xxx.spec.ts` など（無ければ「無し（先にテストを追加）」）
- **検証コマンド**: `task test` など
- **付随作業**: `task wails:generate`・fake-backend の追従など（無ければ「無し」）

## 対応不要と判断した箇所（理由つき）
```
