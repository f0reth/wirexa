---
name: e2e-test-review
description: E2Eテスト（Playwright）の網羅性をユーザーフロー・UI操作・境界条件・バックエンド統合・偽バックエンドの忠実さの観点で分析し、Markdownレポートを出力する
argument-hint: "[http|mqtt|udp|openapi|common|<コンポーネントのパス>]"
disable-model-invocation: true
---

フロントエンド（`frontend/e2e/`）の E2E テストケースが十分かどうかを分析し、レポートを出力する（既にあれば上書き）。リポジトリに書き出してよいのはレポートだけで、テストや本体コードは書き換えない。

指定された引数: 「$ARGUMENTS」（かぎ括弧の中が空なら引数なし）

## 前提: 2 つのスイート

このアプリの E2E は 2 つのスイートに分かれていて、どちらも既に多数の spec がある。「どちらで検証しているか」「どちらで検証すべきか」を常に区別すること。

| | UI e2e | フルスタック e2e |
| --- | --- | --- |
| spec | `frontend/e2e/ui/**/*.spec.ts` | `frontend/e2e/integration/**/*.spec.ts` |
| 設定 | `playwright.config.ts` | `playwright.integration.config.ts` |
| 実行 | `task frontend:test:e2e` | `task frontend:test:e2e:fullstack` |
| バックエンド | `e2e/fake-backend/install.ts` が `window.go` / `window.runtime` を差し替える（Go は動かない） | 実物（`wails dev`）。MQTT は `tools/e2e-broker`、HTTP / UDP の相手は `e2e/fixtures/http-server.ts`・`udp-server.ts` |
| 状態の隔離 | ページのメモリだけ。テスト間の掃除は不要 | `APPDATA` を一時ディレクトリへ向ける。実行中は同じ保存先を使い回すので前のテストのデータが残る |
| 並列 | 完全並列 | 直列（アプリは 1 インスタンス） |
| CI | 実行する | 実行しない（Windows・ローカル専用） |

どちらのスイートで検証するかは次で決める。

- **UI e2e で足りる**: 画面の表示・操作・状態遷移、RPC に渡した引数（`fake.args`）、RPC の失敗・遅延時の表示（`FakeSeed` の `*Error`・`*DelayMs`）、バックエンドからのイベントへの反応（`fake.emit` / `fake.emitAll`）、localStorage の復元（`page.reload()`）。CI で回るので、UI e2e で書けるものは UI e2e を優先する。
- **フルスタックでしか確かめられない**: Go の実処理の結果（実際に送られた HTTP リクエスト・MQTT/UDP の実通信）、ディスク上の保存形式、Go 側の検証エラーの文言、RPC 引数を信頼しない設計（ファイルアクセスの許可リストなど）、Go 側のメモリに残る状態（接続・リスナー）の復元。
- 偽バックエンドに無い機能（`FakeSeed` に注入口が無い失敗など）が必要な場合は「偽バックエンドの拡張が必要」と書く。フルスタックへ回す理由にはしない。

## 対象スコープと出力先

| `$ARGUMENTS` | 対象 | 出力先 |
| --- | --- | --- |
| 空 | すべて | `docs/e2e-test-review.md` |
| `mqtt`・`http`・`udp`・`openapi` のいずれか | そのプロトコルのフロー（下記のパス）と観点 H〜K の該当分。共通の観点 A〜G・L〜N もそのプロトコルの範囲だけ見る | `docs/e2e-test-review-<proto>.md` |
| `common` | プロトコルに依らない部分（`App.tsx`、`presentation/components/sidebar/index.tsx`・`protocol-switcher.tsx`、`components/ui/`（確認ダイアログ・フォーカストラップ・トースト・一覧の並び替え）、`application/ui/`（テーマ・通知）、`e2e/ui/common/`、`e2e/integration/common/`、`e2e/fixtures/`、`e2e/fake-backend/`）。観点 H〜K は対象外（終了時の確認は実装が `openapi-provider.tsx` にあるので openapi で見る） | `docs/e2e-test-review-common.md` |
| コンポーネントのパス（例: `presentation/components/sidebar`） | そのコンポーネントの操作だけ | `docs/e2e-test-review-<パスの末尾>.md` |

プロトコルごとのパス（テストはどれも `e2e/ui/<proto>/` と `e2e/integration/<proto>/`）:

| | 本体（`frontend/src/`） | バックエンド |
| --- | --- | --- |
| mqtt | `presentation/components/mqtt/`、`sidebar/broker-tree.tsx`・`profile-list.tsx`、`presentation/providers/mqtt-provider.tsx`、`application/mqtt/` | `internal/adapters/mqtt_handler.go` |
| http | `presentation/components/http/`、`sidebar/collection-tree.tsx`・`collection-node.tsx`・`tree-item-node.tsx`・`tree-ui-context.tsx`・`rename-input.tsx`・`use-tree-drag-drop.ts`・`use-long-press-drag.ts`・`drag-state.ts`、`presentation/providers/http-provider.tsx`、`application/http/` | `internal/adapters/http_handler.go` |
| udp | `presentation/components/udp/`、`sidebar/target-tree.tsx`・`profile-list.tsx`、`presentation/providers/udp-provider.tsx`、`application/udp/` | `internal/adapters/udp_handler.go` |
| openapi | `presentation/components/openapi/`、`sidebar/openapi-file-tree.tsx`・`openapi-file-node.tsx`・`use-long-press-drag.ts`、`presentation/providers/openapi-provider.tsx`、`application/openapi/`、`infrastructure/app/lifecycle.ts`（終了時の確認） | `internal/adapters/openapi_handler.go` |

`e2e/ui/common/` にはプロトコル固有のテストも入っている（`keyboard-accessibility` の HTTP・MQTT のテスト、`initial-state` の各プロトコルの空状態など）。プロトコルを絞った場合も、そのプロトコルに関わる `common/` のテストは読むこと。

## 分析手順

### Step 1: ユーザーフローの把握（本体コードを先に読む）

1. `frontend/src/App.tsx` と `presentation/components/sidebar/protocol-switcher.tsx` — 全体レイアウト、プロトコル切り替え、サイドバーの開閉、テーマ
2. `presentation/components/sidebar/` — 追加・削除・リネーム・選択・並び替え・開閉
3. 各プロトコルのコンポーネントと `presentation/providers/` — 主要な操作と、状態がどこで管理されているか
4. `infrastructure/storage/local-storage.ts` — localStorage に保存しているキー（リロードで復元されるもの）
5. `config/limits.ts` — 上限値の定数（レポートには実際の値を使う）
6. `internal/adapters/` のハンドラ — 公開している RPC。`internal/domain/events.go` — バックエンドが発火するイベント
7. `infrastructure/app/lifecycle.ts` と `openapi-provider.tsx` — 終了時の確認（`app:before-close` → `ConfirmQuit`）

列挙したフローをレポートの「ユーザーフロー一覧」に書く。

### Step 2: テスト基盤と既存テストの把握

テスト基盤を先に読む。spec はこれらを通して画面を操作するので、読まないと spec の内容を誤解する。

- `e2e/fixtures/app.ts` — ページオブジェクト `App`。ロケーターと操作はここに集約されている
- `e2e/fixtures/ui.ts`・`e2e/fake-backend/types.ts` — UI e2e の test の入口（`app`・`fake`・`seed`）。`fake`（`calls`・`args`・`waitForCalls`・`emit`・`emitAll`・`snapshot`）と `FakeSeed`（初期データ、失敗・遅延の注入）
- `e2e/fake-backend/install.ts` — 偽バックエンドの実装
- `e2e/fixtures/integration.ts`・`e2e/integration/global-setup.ts`・各 fixture — フルスタックの test の入口（`app` と保存データの読み取り・後始末ヘルパー）

そのうえで対象の spec をすべて読み、「何を検証しているか」を spec ファイル単位で要約する。テスト 1 件ごとの表は作らない（件数が多く、読み手の役に立たない）。

### Step 3: 観点ごとに不足を検出

**Step 1 で把握した本体コードを根拠に**不足しているテストケースを検出する。一般論ではなく、コードのファイル名と行番号を根拠に書く。観点の表は「見るべき点」であり、機能の一覧ではない。表に無くても、コードにある操作でテストが無ければ該当する観点で報告する。

観点が対象に該当しない場合は「該当なし（理由）」と書く。

---

#### 観点 A: 初期状態（Initial Render）

| チェック項目 | このアプリでの例 |
| --- | --- |
| 起動時の初期プロトコル | MQTT のボタンが `aria-pressed` になっている |
| 各プロトコルの空状態 | `No brokers yet`・`No collections yet`・`No targets yet`・`No files opened yet` など |
| テーマの初期状態と復元 | `app:theme` に保存されたテーマが起動時に適用される |
| 起動時の読み込み失敗 | `GetSidebarLayout`・`GetProfiles`・`GetTargets` が失敗したときの表示と、その後の操作で保存済みデータを壊さないこと（`FakeSeed` の `getSidebarLayoutError`・`getProfilesError`・`getTargetsError`）。`GetCollections`・`GetRecents` は Go 側がエラーを返さず、`GetRecents` の失敗は画面に出ない（`console.error` だけ）ので、テストが無くても不足として報告しない |

#### 観点 B: プロトコル切り替え（Protocol Navigation）

| チェック項目 | このアプリでの例 |
| --- | --- |
| 4 プロトコルの切り替えとサイドバーの内容 | 見出しが `Brokers` / `Collections` / `Targets` / `OpenAPI Files` に変わる |
| 切り替え後の状態の保持 | 一度開いたパネルは `display: none` で残り、入力内容が戻ってくる（`visited`） |
| 選択中のプロトコルを再クリック | サイドバーが閉じる・開く。別のプロトコルを選ぶと開き直す |

#### 観点 C: サイドバー操作（Sidebar Interaction）

| チェック項目 | このアプリでの例 |
| --- | --- |
| 追加 | コレクション・フォルダ・リクエスト（ルート直下を含む）、ブローカー、ターゲット、OpenAPI の新規文書 |
| リネーム | ダブルクリック → 入力 → Enter / blur で確定、Escape で取り消し |
| 削除と確認ダイアログ | Delete で消える、Cancel / Escape で残る、子を持つフォルダの削除 |
| 選択 | クリック・キーボードで選び、対応する編集パネルが開く |
| 並び替え・移動 | ツリー（コレクション間・フォルダ内・ルートへの移動）、ブローカー・ターゲット・最近使ったファイルの並び替え |
| 開閉 | コレクション・フォルダの開閉と、その復元（`wirexa:http:expandedFolders`） |
| 予約コレクション `__root__` | 画面に出ない・削除やリネームができない |

#### 観点 D: 入力・バリデーション・境界値（Form Input, Validation & Boundary）

| チェック項目 | このアプリでの例 |
| --- | --- |
| 必須項目が空のときの送信ブロック | URL 空欄で Send が無効、名前が空のブローカーは Save が無効 |
| 空白だけの入力 | リネームで空白だけを確定したとき、ホストが空白のとき |
| ポート番号の範囲（1〜65535） | MQTT ブローカー、UDP ターゲット・送信先・待ち受けポートに `0`・`65535`・`65536` |
| 不正な URL | `not-a-url` |
| 不正な MQTT トピックフィルター | 検証はバックエンド側。画面はエラーをトーストで出す |
| `config/limits.ts` の上限 | `MQTT_MAX_MESSAGES`・`MQTT_MAX_TOPICS`・`UDP_MAX_MESSAGES` の境界（上限ちょうど・上限 +1） |
| key-value エディタ | 行の追加・削除・有効/無効、Form Data の種別（Text / File / JSON） |
| UDP の固定長フィールド | 不正な hex、バイト数の表示、エンディアン |
| 特殊文字・Unicode | 名前やトピックに日本語・絵文字・`<script>` |

#### 観点 E: 非同期操作（Async & Loading State）

| チェック項目 | このアプリでの例 |
| --- | --- |
| 処理中の表示と二重実行の防止 | HTTP は Send が Cancel に替わる。UDP は `Sending...`・`Starting...` で無効化 |
| 取り消し | Cancel で `CancelRequest` が呼ばれ、表示が戻る |
| 成功後の更新 | レスポンス表示、接続状態、一覧への反映 |
| 失敗時の表示 | トースト、フォーム内のエラー、自動保存失敗のバナー。同じ失敗でトーストが重複しないこと |
| 処理中に対象を切り替えたとき | 送信中に別のリクエストへ切り替えたら、古いレスポンスを表示しない |

#### 観点 F: 画面側の永続化（Persistence in localStorage）

`infrastructure/storage/local-storage.ts` のキーごとに、リロード後の復元を検証しているか。バックエンドの保存は観点 M。

| チェック項目 | このアプリでの例 |
| --- | --- |
| テーマ | `app:theme` |
| 選択中のリクエスト | `wirexa:http:activeRequest` |
| フォルダの開閉 | `wirexa:http:expandedFolders` |
| 最後に使ったブローカー | `mqtt:lastActiveProfileId` |
| プリセット | `mqtt:presets` |
| 並び順 | `mqtt:profileOrder`・`udp:targetOrder` |
| 形の壊れた保存値 | 既定値に戻って起動できる |

#### 観点 G: キーボード操作・アクセシビリティ（Keyboard & Accessibility）

| チェック項目 | このアプリでの例 |
| --- | --- |
| Tab でのフォーカス移動 | 入力欄 → ボタン、ダイアログ内のフォーカストラップ |
| Enter / Space での実行・選択 | URL 欄の Enter で送信、行の Enter / Space で選択 |
| Escape での取り消し | リネーム、確認ダイアログ |
| ショートカット | OpenAPI の Ctrl+S |
| ロールとアクセシブル名 | `button`・`tab`・`tabpanel`・`dialog`、id の重複が無いこと |

#### 観点 H: MQTT 固有フロー

| チェック項目 | このアプリでの例 | 検証するスイート |
| --- | --- | --- |
| プロファイルの作成・編集・削除 | ダイアログと接続バーの両方からの編集、複数ブローカーの切り替え | UI |
| 接続・切断 | Connect / Disconnect、`Save & Connect`、接続失敗・拒否 | UI（表示）/ フルスタック（実接続） |
| 購読・購読解除 | Subscriptions への追加、ミュート、共有購読（`$share/`・`$queue/`）、切断された購読の通知 | UI / フルスタック（実受信） |
| メッセージ一覧 | 受信、詳細表示とコピー、トピックでの絞り込み、Clear、Auto スクロール | UI（`fake.emit`） |
| メッセージ上限 | `MQTT_MAX_MESSAGES` を超えたときの破棄 | UI（`fake.emitAll`） |
| パブリッシュ | トピック・QoS・retain・ペイロード、オフライン時は無効 | UI（`fake.args`）/ フルスタック（ループバック） |
| プリセット | 追加・選択・リネーム・削除・並び替え・復元 | UI |
| トピックスキャン | スキャン結果の一覧と、そこからの購読 | UI / フルスタック |
| 再接続・切り替え | 再接続後も購読が続く、ブローカーごとに購読が独立している | フルスタック |

#### 観点 I: HTTP 固有フロー

| チェック項目 | このアプリでの例 | 検証するスイート |
| --- | --- | --- |
| メソッド・URL・送信 | メソッド選択、送信、レスポンス表示 | UI / フルスタック（実送信） |
| ヘッダー・クエリ・認証 | Basic / Bearer の入力欄と、実際に送られる値 | UI（`fake.args`）/ フルスタック（サーバーが受けた値） |
| ボディ | none / JSON / Text / Form Data / Form URL Encoded / File の切り替えと、種別ごとの値の保持 | UI / フルスタック（multipart の中身） |
| ファイル参照 | 手入力のパスは未確定で送れない、Browse で確定、編集でトークンを捨てる、再起動後は再選択を求める | UI / フルスタック（手入力のパスが送られないこと） |
| リクエスト設定 | タイムアウト、プロキシ、TLS 検証、リダイレクト | UI |
| レスポンス表示 | ステータス、Body / Headers / Timing、画像、バイナリ（hex）、空ボディ、HTML を含む JSON、複数値ヘッダー | UI / フルスタック |
| 大きなレスポンス | 打ち切り表示、保存、保存は 1 回だけ、回収済みのときの案内 | UI |
| コピー | レスポンスボディをクリップボードへ | UI |
| 自動保存と復元 | 入力の自動保存、Doc タブ、保存失敗のバナー、リロード後の復元 | UI / フルスタック |
| レスポンスパネルの開閉 | 隠す・戻す、送信時に自動で開く | UI |

#### 観点 J: UDP 固有フロー

| チェック項目 | このアプリでの例 | 検証するスイート |
| --- | --- | --- |
| ターゲットの作成・編集・削除・並び替え | ダイアログ、`host:port` の表示 | UI |
| 送信 | エンコーディング（text / json / 固定長）ごとのペイロード、種別ごとの値の保持 | UI（`fake.args`）/ フルスタック（実送信） |
| 待ち受けの開始・停止 | 開始中の表示、待ち受け中は Start が無効、リロード後の復元 | UI / フルスタック |
| 開始の失敗 | ポート未指定、使用中のポート | UI / フルスタック |
| 受信ログ | 新しい順、Clear、ペイロードのコピー | UI（`fake.emit`）/ フルスタック |
| メッセージ上限 | `UDP_MAX_MESSAGES` を超えたときの破棄 | UI（`fake.emitAll`）/ フルスタック |

#### 観点 K: OpenAPI 固有フロー

| チェック項目 | このアプリでの例 | 検証するスイート |
| --- | --- | --- |
| 編集とプレビュー | 入力がプレビューに反映される、空・不正な YAML/JSON のときの表示、プレビューの開閉 | UI / フルスタック |
| 新規文書・ファイルを開く | New、ファイルダイアログ、ファイルのドロップ | UI（開いた状態から始めるなら `FakeSeed.openApiFiles`。ダイアログで開く経路は偽バックエンドの拡張が必要: OpenAPI の `OpenFilePicker` は常にキャンセルを返す。`FakeSeed.pickedFile` は HTTP 用） |
| 保存 | Save、Ctrl+S、Save As、未保存マークの付き方と消え方 | UI |
| 未保存の確認 | 別の文書を開く・新規作成・ドロップ時の `Save & continue` / `Discard changes` / `Cancel` | UI |
| 終了時の確認 | `app:before-close` を受けたときの確認と `ConfirmQuit` | UI（`fake.emit`） |
| 最近使ったファイル | 選択、履歴からの削除、並び替え、リロード後の順序 | UI |
| ファイルアクセスの許可リスト | ダイアログで選んでいないパスの `ReadFile` / `WriteFile` が拒否される（`docs/http-local-file-access-hardening.md`） | フルスタック |

#### 観点 L: テスト構造（Test Structure Integrity）

| チェック項目 | 見るところ |
| --- | --- |
| 固定 sleep | `page.waitForTimeout`。`expect(...)`、`expect.poll`、`fake.waitForCalls` に置き換える |
| ページオブジェクトを通さないロケーター | spec に直接書かれた CSS クラス・XPath・DOM 構造依存のロケーター。`fixtures/app.ts` の中にあるもの（CodeMirror の `.cm-*`、`data-drop-zone` など、理由がコメントされているもの）は指摘しない |
| ヘルパーの重複 | 複数の spec に同じ操作がコピーされていないか（`App` に寄せる） |
| `test.only`・`test.skip`・`test.fixme` の放置 | |
| アサーションの無いテスト、操作しただけで結果を見ていないテスト | テスト名が言っていることを実際に検証しているか |
| 個別の `timeout` 指定 | 既定値は config の `expect.timeout` にある。上書きには理由があるか |
| URL・ポートの直書き | `baseURL` を使っているか |
| フルスタックの後始末 | Go 側に残る状態（UDP リスナー・MQTT 接続）を `afterEach` で `stopUdpListeners`・`disconnectMqttConnections` を使って止めているか。前のテストのデータが残っていても通る書き方か（件数の完全一致ではなく名前で絞る） |
| スイートの選び方 | UI e2e で書けるのにフルスタックにだけあるテスト（CI で回らない）、逆に偽バックエンドの挙動を確かめているだけのテスト |

#### 観点 M: バックエンド統合（Backend Integration）

フルスタックでしか確かめられないこと。番号は変えない。`e2e/integration/common/backend-integration.spec.ts` のコメントと describe 名（M-1〜M-4・M-8）がこの番号を使っている。M-5〜M-7・M-9・M-10 は spec に番号が書かれていないので、プロトコルごとの spec（`e2e/integration/<proto>/`）を内容で照らし合わせる。

| 番号 | チェック項目 | このアプリでの例 | 根拠ファイル |
| --- | --- | --- | --- |
| M-1 | RPC が実バックエンドに届き、結果が返る | 画面で作ったものが `GetCollections`・`GetProfiles`・`GetTargets` に現れる | `internal/adapters/*_handler.go` |
| M-2 | コレクションの永続化 | リロード後も残る。`collections/*.json` の形が golden と一致する | `internal/infrastructure/http/collection_repository.go` |
| M-3 | MQTT プロファイルの永続化 | 同上（`mqtt-profiles/*.json`） | `internal/infrastructure/mqtt/profile_repository.go` |
| M-4 | UDP ターゲットの永続化 | 同上（`udp-targets/*.json`） | `internal/infrastructure/udp/target_repository.go` |
| M-5 | バックエンドのエラーの表示 | Go 側の検証エラーや I/O エラーが画面に出る | `internal/domain/errors.go` |
| M-6 | 実通信 | HTTP の実送信（サーバーが受けた内容）、MQTT の実接続・送受信、UDP の実送受信 | `internal/infrastructure/http/net_client.go`、`mqtt/paho_client.go`、`udp/net_socket.go` |
| M-7 | イベントの受信 | バックエンドが発火したイベントが画面に反映される | `internal/infrastructure/wails_emitter.go`、`internal/domain/events.go` |
| M-8 | サイドバーレイアウトの永続化 | 並び替えが `sidebar_layout.json` に保存される | `internal/infrastructure/http/sidebar_layout_repository.go` |
| M-9 | OpenAPI の最近使ったファイル | `openapi-recents.json` への保存と復元 | `internal/infrastructure/openapi/recent_repository.go` |
| M-10 | RPC 引数を信頼しない設計 | 許可されていないパス・トークンの拒否 | `internal/infrastructure/http/file_registry.go`、`openapi/file_access.go` |

永続化は「リロード後に残る」と「ディスク上のファイルの形が `testdata/*.golden.json` と一致する」（`readStoredEntities`・`readGolden`・`shapeOf`）の両方で見る。アプリの再起動はこのスイートでは行えない（1 回の実行で `wails dev` を 1 つ起動するだけ）ので、再起動をまたぐ検証が無いことは不足として報告しない。破損ファイルの退避など起動時の復旧は Go の統合テスト（`internal/integration/`）の範囲。

#### 観点 N: 偽バックエンドの忠実さ（Fake Backend Fidelity）

UI e2e は偽バックエンドの挙動を前提に通る。偽バックエンドが Go とずれていると、テストが通っても実物では動かない。

| チェック項目 | 見るところ |
| --- | --- |
| RPC の網羅 | `internal/adapters/` の公開メソッドと `app.go` の `ConfirmQuit`（`main.App`）がすべて `install.ts` にあるか。足りないと UI e2e でその操作は失敗するか、何も起きない |
| 挙動の一致 | 検証の順序と条件（空の名前、ポート範囲、`__root__` の予約など）、戻り値の形、既定値、並び順が Go の application サービスと同じか |
| イベントの一致 | RPC のあとに発火するイベントの種類と順序が Go と同じか（MQTT の接続成功・失敗など） |
| 型の追従 | `fake-backend/types.ts` が domain 型を使っていて、フィールドの追加に追従できているか |
| 忠実さを確かめるテスト | 同じシナリオが UI e2e とフルスタックの両方にあり、ずれを検出できるか（特に検証エラーと予約コレクション） |
| 注入口の不足 | 観点 A・E で必要な失敗・遅延のうち、`FakeSeed` に注入口が無いもの |

ずれを見つけたら、Go 側と `install.ts` 側の両方のファイル名と行番号を書く。

---

## 出力形式

次の構成で出力する。表の例は書式を示すだけで、内容は実際に読んだコードから書くこと。

```markdown
# E2E テストレビュー

生成日時: YYYY-MM-DD
対象: （引数に応じた範囲）
スイート: UI e2e（spec N 本・テスト N 件）/ フルスタック e2e（spec N 本・テスト N 件）

---

## ユーザーフロー一覧

| フロー | コンポーネント（ファイル:行） | UI e2e | フルスタック |
| --- | --- | --- | --- |
| （フロー） | （ファイル:行） | 済（spec:行） | 対象外 |

> 各列の値: `済（ファイル名:行番号）` / `部分的（ファイル名:行番号）` / `未テスト` / `対象外`（そのスイートで検証する意味が無い）

---

## 既存テストの概要

| spec | テスト数 | 検証している内容（要約） |
| --- | --- | --- |
| ui/mqtt/messages.spec.ts | N | （2〜3 行で要約） |

---

## サマリー

| 観点 | チェック項目数 | 済 | 部分的 | 不足 |
| --- | --- | --- | --- | --- |
| [A] 初期状態 | N | N | N | N |
| ...（A〜N） | | | | |

> チェック項目数は各観点の表の行数。該当なしの行は数えない。

---

## 観点別詳細

### [A] 初期状態

#### テスト済み

- `ファイル名:行` — 検証している内容

#### 不足

- **不足内容**: （何が検証されていないか）
- **根拠コード**: `ファイル名:行` — （その挙動を実装している箇所）
- **スイート**: UI e2e / フルスタック / UI e2e（偽バックエンドの拡張が必要: 何を足すか）
- **推奨テスト名**: `test("...", ...)`
- **追加先**: （既存の spec ファイル名。無ければ新しいファイル名）
- **優先度**: 高 / 中 / 低

...（A〜N を同じ形式で）...

---

## 優先して追加すべきテスト

（最大 5 件。優先度 高 が無ければ「該当なし」と書く。件数を埋めるために優先度の低いものを挙げない）

1. **（内容）**（観点 X）— 根拠: `ファイル名:行`、スイート: ...

---

## 総合評価

**主なリスク**: （未テストのフローで起きうるリグレッション）
**推奨アクション**:

1. （具体的な次の手順）
```

## 実装上の注意（推奨テストを書くときに従う）

- **test は fixture から import する**: UI e2e は `../../fixtures/ui`、フルスタックは `../../fixtures/integration`。`page.goto("/")` と localStorage の初期化は fixture が済ませているので、spec では行わない。`beforeEach` で `localStorage.clear()` を呼ぶと、リロード後の復元を見るテストが壊れる。
- **操作は `App`（`fixtures/app.ts`）を通す**: 無い操作は `App` にメソッドを足す前提で書く。
- **UI の文言は英語**（`Send`・`Cancel`・`Delete`・`Save`・`New Broker`・`New Target` など）。ロケーターは `getByRole`・`getByLabel`・`getByPlaceholder`・`getByTestId` を使う。
- **待ち方**: RPC は Wails の IPC なので `page.waitForResponse()` では待てない。HTTP リクエストも Go 側から送るので、フルスタックでもブラウザのネットワークには現れない。画面の変化を `expect(locator)` で待つか、UI e2e なら `fake.waitForCalls("Binding名")` を使う。
- **RPC に渡した値の検証**: UI e2e は `fake.args("Binding名")`、状態は `fake.snapshot()`。フルスタックは相手サーバー（`fixtures/http-server.ts`・`udp-server.ts`）が受けた内容か、ディスク上のファイルで見る。
- **イベント**: UI e2e は `fake.emit(WailsEvents.xxx, payload)`。同じフレームに大量に流すときは `fake.emitAll`。
- **初期データ**: UI e2e は `test.use({ seed: {...} })`。画面操作で作るのは、その操作自体を検証するテストだけにする。
- **ドラッグ**: ツリー（コレクション・最近使ったファイル）は独自のマウス D&D なので `locator.dragTo()` は使えない。`app.dragTreeNode` を使う。ブローカー・ターゲットの一覧と MQTT のプリセット一覧は HTML5 D&D（`components/ui/list-reorder.tsx`）で `app.dragListRowBefore` を使う。
- **クリップボード**: `context.grantPermissions(["clipboard-read", "clipboard-write"])` のあと `navigator.clipboard.readText()` で読む。
- **フルスタックの後始末**: 観点 L の「フルスタックの後始末」のとおりに書く。保存データは名前を一意にして絞り込む。

## 制約事項

- **存在しない UI を「不足」と報告しない**: コードに実装されていない機能はテスト不要。
- **既にあるテストを「不足」と報告しない**: 不足と書く前に、`e2e/ui/common/` と、もう一方のスイートを確認する。同じ内容が単体テスト（`*.test.ts`）で十分に検証されていて E2E で重ねる意味が薄いものは、優先度を 低 にしてその旨を書く。
- **優先度の基準**:
  - 高: ユーザーが必ず通るフロー、またはデータを失う・壊す可能性のある挙動が未テスト
  - 中: 重要な機能だが、別のテストで間接的に通っている、または使用頻度が低い
  - 低: エッジケース、実装の詳細に近い検証、単体テストで検証済みのもの
- **テスト構造の問題は観点 L、バックエンド統合の不足は観点 M、偽バックエンドのずれは観点 N に集約する**。
- **テストは実行しなくてよい**: 静的に読んで分析する。実行して確かめた場合は、どのコマンドを実行したかをレポートに書く。
