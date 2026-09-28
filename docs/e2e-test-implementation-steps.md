# E2E テスト追加の実装ステップ

元資料: [`e2e-test-review.md`](./e2e-test-review.md)（以下「レビュー」）
作成日: 2026-09-28

レビューの不足ケース・構造問題・ロードマップを、1 ステップ = 1 PR（またはコミット群）で完結する単位に分けた。各ステップは単独でマージでき、マージ後も `task frontend:ci` / `task frontend:test:e2e` が通る状態を保つ。

---

## 分け方の方針

- **基盤の変更とテストの追加を同じステップに入れる。** 偽バックエンドの seed やイベント注入口は、それを使う最初のテストと一緒に入れる（使われない拡張だけを先に入れない）。
- **UI モード（CI で走る）を先に、フルスタック（Windows ローカルのみ）を後に。** ただし優先度「高」でインフラの準備が大きいもの（MQTT ブローカー）は独立させ、前倒しできるようにしている。
- **本体コードの修正を含むステップは明記する。** テストだけのステップと混ぜず、コミットを分ける（例: タブ ID の一意化）。
- **既存テストの一括書き換え（セレクタ整理）は最後。** 途中のステップで追加するテストは、最初からレビューの「Playwright 実装ガイド」に従って書き、必要なヘルパーはその時点で `e2e/fixtures/app.ts` に足す。
- 偽バックエンドを拡張するときは、Go 側の該当実装（レビューの根拠コード欄）と挙動を合わせる。合わせられない場合は `install.ts` にその旨をコメントで残す。

## 依存関係

```
Step 1  フルスタックの後始末 ─────────────────────────────┐
Step 2  HTTP 送信値                                         │
Step 3  OpenAPI 未保存確認（切り替え）                       │
Step 4  HTTP ツリー操作 ── Step 5  HTTP ツリー D&D ─────────┼─ Step 12 フルスタック永続化
Step 6  UDP UI①（偽BE検証） ── Step 7  UDP UI②（送信）      │
Step 8  偽BE イベント注入口 ─┬─ Step 10 MQTT UI②（受信）    │
Step 9  MQTT UI①（未接続） ──┘                               │
Step 11 フルスタック MQTT（独立。前倒し可） ◄───────────────┘
Step 13 フルスタック エラー表示・UDP 上限・echo
Step 14 HTTP レスポンス表示ほか
Step 15 OpenAPI 保存・D&D・最近のファイル
Step 16 共通 UI・アクセシビリティ（本体修正あり）
Step 17 セレクタ整理（既存テストの書き換え）
```

- Step 5 → Step 12: マウス方式 D&D のヘルパーを流用する
- Step 8 → Step 10: `emit` を使って MQTT メッセージを流す
- Step 16 → Step 17: UDP タブ・タブ ID の本体修正後に位置依存セレクタを外す
- それ以外のステップは互いに独立で、順番を入れ替えてよい

## TOP5 との対応

| レビューの TOP5 | ステップ |
| --- | --- |
| 1. MQTT の接続・購読・受信・Publish | Step 9, 10, 11 |
| 2. HTTP ツリーの D&D と `sidebar_layout.json` | Step 5, 12 |
| 3. OpenAPI の未保存確認（切り替え時・終了時） | Step 3, 8 |
| 4. UI モードの UDP spec 新設 | Step 6, 7 |
| 5. headers / params / auth / settings の送信反映 | Step 2（ワイヤ形式は Step 13） |

---

## Step 1: フルスタックの後始末と前提確認を直す

**観点**: [L]（状態汚染・実質無検証・前提条件・timeout）　**モード**: 両方　**本体修正**: なし

既存テストの信頼性に関わる小さな修正をまとめて先に片付ける。以降のフルスタックのステップはこの後始末を前提にする。

- `e2e/fixtures/integration.ts` に UDP リスナーの掃除を追加し、`e2e/integration/udp/udp.spec.ts` の `afterEach` で `window.go.adapters.UDPHandler.GetListeners()` → `StopListen` を呼ぶ（末尾の Stop クリックに頼らない）
- `e2e/integration/global-setup.ts` の待機条件を `document.title` から `getByRole("button", { name: "MQTT" })` の可視化、または `window.go.adapters` の存在に変える
- `backend-integration.spec.ts` の M-1（`:37-53`）を、事前に作った項目が一覧に出ることの検証に書き換える（無理なら削除）
- `backend-integration.spec.ts:9-10` の「dragTo はネイティブ DnD イベントを発火しない」というコメントを事実に合わせて直す
- `e2e/ui/openapi/openapi.spec.ts:41-82` の `timeout: 3000` を削除する

**完了条件**: `task frontend:test:e2e` が通る。`task frontend:test:e2e:fullstack` で UDP の 1 本を意図的に失敗させても後続の UDP テストが落ちない。

---

## Step 2: HTTP の入力値が送信内容に反映されることを検証する

**観点**: [I] 送信値（高）、[E] CancelRequest の ID（低）、[F] URL 以外の復元（中）　**モード**: UI　**本体修正**: なし

偽バックエンドの拡張は不要。`fake.args("SendRequest")`（`request-file.spec.ts:28-31` が先例）で書ける。

- 新規 `e2e/ui/http/request-send.spec.ts`
  - `headers, query params and bearer token are sent with the request`
  - method / Body / Settings（Timeout・Max Response Body・Proxy・TLS・Redirect）が `SendRequest` の引数に載る
- `e2e/ui/common/async-loading.spec.ts` の `clicking cancel aborts the in-progress request` に、送信時と同じ execution ID で `CancelRequest` が呼ばれることの検証を追加
- `e2e/ui/common/request-persistence.spec.ts` に `method, headers and body are restored after reload` を追加（`fake.waitForCalls("UpdateRequest")` で自動保存を待つ）

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 3: OpenAPI の未保存確認（切り替え時）を検証する

**観点**: [K] 未保存確認ダイアログ（高）　**モード**: UI　**本体修正**: なし

New → 入力 → New をもう一度、の操作で UI モードのまま書ける。

- `e2e/ui/openapi/openapi.spec.ts`（またはファイルを分けて `unsaved-changes.spec.ts`）
  - `creating a new document with unsaved text asks to save`
  - `Cancel keeps the current document`
  - `Discard replaces the document`
  - Save & continue は `SaveFileAs` の seed が要るので、保存先を返す seed（例: `saveFileAsPath`）をここで偽バックエンドに足すか、Step 15 に回す

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 4: HTTP ツリー操作（D&D 以外）を検証する

**観点**: [C] フォルダ／リクエストの削除・リネーム（高）、ダイアログ Cancel・フォルダ開閉・New Request・フォルダ内追加（中）、[F] フォルダ展開状態の永続化（低）　**モード**: UI　**本体修正**: なし

- `e2e/ui/common/sidebar-operations.spec.ts`（大きくなるなら `e2e/ui/http/tree.spec.ts` に分ける）
  - `can delete a request after confirming` / フォルダの削除
  - `can rename a request by double-click` / フォルダのリネーム / blur での確定
  - `cancelling the delete dialog keeps the item`
  - `expanding a folder reveals its children`（既存の "folders" テストが実際はコレクションをトグルしている点も直す）
  - `New Request from the add menu creates a root-level request`
  - `can add a request inside a folder` / フォルダ内へのフォルダ追加
  - `expanded folders are restored after reload`
- 必要に応じて `e2e/fixtures/app.ts` にフォルダ・リクエスト行を取るヘルパーを足す

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 5: HTTP ツリーの D&D を検証する

**観点**: [C] HTTP ツリーの D&D（高）　**モード**: UI　**本体修正**: なし

- `e2e/fixtures/app.ts` にマウス方式の D&D ヘルパーを追加（`mouse.move(src)` → `down()` → 6px 以上 `move()` → `data-drop-zone` の中心へ `move()` → `up()`。座標は `boundingBox()` から取る）。Step 12 でも使う
- 新規 `e2e/ui/http/tree-drag-drop.spec.ts`
  - `dragging a request onto another collection moves it`
  - `dragging a collection reorders the sidebar`
  - コレクション内のアイテムをルートへ出す
  - フォルダへのドロップ
  - いずれも UI 表示に加えて `fake.snapshot` / `fake.args("MoveItem" 等)` でバックエンド側の状態を確かめる

**完了条件**: `task frontend:test:e2e` が通る。CI で 3 回連続して通る（D&D はフレークしやすいので確認する）。

---

## Step 6: UI モードの UDP spec を新設する（ターゲット管理）

**観点**: [J] UI モードの UDP 一式（高）、[D] 範囲外ポートの保存（高）、[C] UDP ターゲット並び替え（低）、[F] ターゲット順の永続化（低）　**モード**: UI　**本体修正**: なし

- `e2e/fake-backend/install.ts`
  - `SaveTarget` に Go の `UDPTarget.Validate`（`internal/domain/udp/types.go:79-87`）と同じ検証を入れる
  - `StartListen` に `listener_service.go:41-53` と同じポート範囲検証を入れる
  - `e2e/fake-backend/types.ts` の `FakeSeed` に `startListenError` を追加（Step 7 で使う）
- 新規 `e2e/ui/udp/targets.spec.ts`
  - ターゲット作成・`editing a udp target updates its host:port in the sidebar`・削除（Cancel も含む）
  - `saving a udp target with port 0 shows an error and keeps the dialog open`
  - `udp targets can be reordered by drag and drop`（HTML5 DnD。`locator.dragTo()` を使う）と、リロード後の順序
- 既存の `form-validation.spec.ts:48`（ポートの `validity` だけを見るテスト）はこの spec へ移すか、残して重複を許すかを決める

**完了条件**: `task frontend:test:e2e` が通る。偽バックエンドの変更で既存テストが落ちない。

---

## Step 7: UI モードの UDP spec を新設する（送信・リスン）

**観点**: [J] 送信内容・エンコーディング（高）、固定長フィールド（中）、[E] 送信中・リスン開始中の無効化（低）、エラー表示　**モード**: UI　**本体修正**: なし

- `e2e/fake-backend/install.ts` に `Send` / `StartListen` の遅延 seed を追加（例: `udpSendDelayMs`、`startListenDelayMs`）
- 新規 `e2e/ui/udp/send.spec.ts`
  - `json encoding sends the json payload`、text / fixed の切り替え（`fake.args("Send")`）
  - `fixed-length fields show byte counts and flag invalid hex`
  - `fixed-length fields are sent in order with the chosen endianness`
  - 送信中は "Sending..." で無効
- 新規 `e2e/ui/udp/listen.spec.ts`
  - `udp Start is disabled while a session is listening`
  - seed `startListenError` でフォーム内にエラーが出る

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 8: 偽バックエンドにイベント注入口を作る

**観点**: [K] ウィンドウを閉じるときの未保存確認（高）、[J] 受信ログの Clear・Copy（低）、ロードマップ 6　**モード**: UI　**本体修正**: なし

- `e2e/fake-backend/install.ts:580-584` の `EventsOn` / `EventsOff` を実装し、`window.__wirexaFake.emit(name, data)` を追加
- `e2e/fixtures/ui.ts` の `fake` に `emit(name, data)` を追加（イベント名は `src/shared/wails-events.ts` の定数を使う）
- 使うテスト
  - `e2e/ui/openapi/unsaved-changes.spec.ts`: `before-close with unsaved changes shows the dialog and does not quit on Cancel`、Discard で `ConfirmQuit` が呼ばれる、未編集なら即 `ConfirmQuit`
  - `e2e/ui/udp/listen.spec.ts`: `udp:message` を流して受信ログの表示・Clear・Copy payload

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 9: MQTT の UI モード（未接続で書けるもの）

**観点**: [E] エラートースト（高）、[H] Publish の未接続時無効化（高）・複数ブローカーの切り替え（中）・プロファイル編集・プリセット（中）、[D] ポート 0 / 65536・空白ホスト（中）、[F] 最後のプロファイル復元・プリセットの永続化（中）、[C] プリセットの並び替え（低）　**モード**: UI　**本体修正**: あり（実装中に見つかった不具合。コミットを分けた）

偽バックエンドの `Connect` は必ず失敗するので、その性質をそのまま使う。

- 実装中に見つけて直した本体の不具合
  - リロード後に最後に選んだブローカーが復元されない。`connections.ts` の保存用 `createEffect` が起動直後（アクティブな接続が無い状態）に走り、`restore()` が読む前に `mqtt:lastActiveProfileId` を消していた
  - ブローカーを切り替えても接続バーの scheme / host / port が前のブローカーのまま。`broker-manager.tsx` の `Show` が切り替えで作り直されず、入力欄の signal が初回の値のままだった。その状態で編集すると、切り替え先のプロファイルの URL を前のブローカーのホストで上書き保存していた（ダイアログで編集した場合も同じ）

- `e2e/ui/mqtt/mqtt.spec.ts`（大きくなるなら `profiles.spec.ts` / `publish.spec.ts` に分ける）
  - `failed mqtt connect shows an error toast`（E2E で初めてトースト `role="alert"` を検証する）
  - `broker dialog rejects port 0 and 65536 and a blank host`
  - プロファイル編集（Edit broker）
  - seed `mqttProfiles` で 2 件用意し、選択の切り替えと `last active broker is selected again after reload`
  - `publish button is disabled while offline`
  - プリセットの追加・選択・改名・削除・並び替えと `publish presets survive a reload`
  - 空状態 "No brokers yet" / "No active connection..."

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 10: MQTT の UI モード（接続済み・受信）

**観点**: [H] 購読 → 受信 → 表示（高）・ミュート・フィルタ・Clear・詳細・Auto（低〜中）、[D] 5000 件上限（中）　**モード**: UI　**本体修正**: なし　**前提**: Step 8

- `e2e/fake-backend/install.ts` に `Connect` を成功させる seed（例: `mqttConnect: "ok"`）を追加。接続状態がイベント経由で決まるなら `mqtt:connected` も発火する。`Subscribe` / `Unsubscribe` / `Publish` の呼び出しを記録する
- 新規 `e2e/ui/mqtt/messages.spec.ts`
  - 購読の追加・削除・ミュート（`fake.args("Subscribe")`）
  - `mqtt:message` を emit → Messages に表示 → 詳細 → Copy payload
  - `muted subscription hides its messages`
  - トピックフィルタ・Clear
  - `with Auto enabled the newest message is scrolled into view`（Auto が既定 OFF であることも確かめる）
  - `mqtt message list drops the oldest message when the 5001st arrives`（フレームをまたいで届ける）
  - `mqtt messages beyond 5000 within one frame are dropped from the buffer`（1 フレーム内にまとめて届ける）
  - Publish で `fake.args("Publish")` にトピック・QoS・Retain・ペイロードが載る。トピック空・非 retain の空ペイロードでは送らない

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 11: フルスタックに MQTT ブローカーと spec を用意する

**観点**: [H] 接続・切断（高）・購読と受信・Publish のループバック（高）・購読の独立（中）・Broker Topics スキャン（低）、[D] 不正トピック（中）、[M] MQTT イベントの UI 反映（高）、ロードマップ 3　**モード**: フルスタック　**本体修正**: なし（ブローカー起動用の `tools/` コマンドを足す場合を除く）

他のステップと独立しているので、TOP5 の 1 位として前倒ししてよい。

- **最初に決めること**: ブローカーの実装
  - A. mochi-mqtt を起動する `tools/e2e-broker/` を作り、`playwright.integration.config.ts` の webServer に足す（Go 側の統合テストと実装が揃う。依存追加なし）
  - B. Node の組み込みブローカー（aedes など）を `e2e/fixtures/mqtt-broker.ts` にする（`http-server.ts` と同じ形。依存が増える）
  - **A を採用した。** アプリ以外のクライアントからの publish は、同じプロセスが開く操作用 HTTP（`POST /publish` → mochi のインラインクライアント）で行う。ポートと呼び出しは `e2e/fixtures/mqtt-broker.ts` にまとめた
- `e2e/fixtures/integration.ts` に MQTT 接続の掃除（`afterEach` で `Disconnect`）を追加
- 新規 `e2e/integration/mqtt/mqtt.spec.ts`
  - `connects to a local broker and shows Connected` / `disconnect returns the tab to Disconnected` / Save & Connect
  - `published message on a subscribed topic appears in Messages`
  - `publishing to a subscribed topic loops back into Messages`
  - `subscribing to 'a/#/b' shows a validation error toast`
  - `switching brokers keeps each broker's subscriptions separate`
  - Broker Topics のスキャン
  - 余力があれば 5000 件上限（Step 10 で UI 側は検証済みなので低優先）。見送った

**完了条件**: `task frontend:test:e2e:fullstack` が Windows ローカルで通り、2 回続けて実行しても状態が漏れない。

---

## Step 12: フルスタックでディスクへの永続化を検証する

**観点**: [M] ディスクへの保存（高）・`sidebar_layout.json`（高）、ロードマップ 5　**モード**: フルスタック（一部 UI）　**本体修正**: なし　**前提**: Step 5

- `e2e/fixtures/integration.ts` に `%TEMP%/wirexa-e2e-appdata/Wirexa/` 配下の JSON を読むヘルパーを追加
- `backend-integration.spec.ts` の M-2〜M-4 に JSON の直接確認を足す（`created collection is written under APPDATA/Wirexa/collections` など）。期待する形は各パッケージの `testdata/*.golden.json` に合わせる
- `reordering collections persists sidebar_layout.json`（Step 5 の D&D ヘルパーを使う）
- 既存の M-8（ブローカーの並び替え）は `mqtt:profileOrder` のテストとして改名し、UI モードの観点 F（`e2e/ui/mqtt/`）へ移す

**完了条件**: `task frontend:test:e2e:fullstack` と `task frontend:test:e2e` が通る。

---

## Step 13: フルスタックでバックエンドのエラー表示と UDP の上限を検証する

**観点**: [M] バックエンドのエラーが UI に出る（高）、[J] 500 件上限で最古を捨てる・新しい順（中）・リスン失敗（中）、[I] ワイヤ形式での送信値、ロードマップ 2　**モード**: フルスタック　**本体修正**: なし

- `e2e/integration/udp/udp.spec.ts`
  - `backend validation error for udp target is shown as a toast`
  - `listening on a port already in use shows an error`（Node 側でポートを塞ぐ）
  - `udp log shows newest first and drops msg-0 at the cap`（既存の 500 件テストを拡張）
- `e2e/fixtures/http-server.ts` の `/echo` が受信ヘッダー・クエリを返すようにし、`e2e/integration/http/http.spec.ts` に headers / params / auth がワイヤに載ることの検証を追加

**完了条件**: `task frontend:test:e2e:fullstack` が通る。

---

## Step 14: HTTP のレスポンス表示と残りの HTTP フロー

**観点**: [I] Timing・4xx/5xx・空ボディ・画像/バイナリ（中）・Show truncated body（低）・Copy body の UI 化（中）・Doc タブ・レスポンスパネル切替・Max Response Body（低）、[D] JSON レスポンス内の HTML（中）、[E] 自動保存失敗バナー（中）・送信中の切り替え（中）　**モード**: UI　**本体修正**: なし

- `e2e/fake-backend/install.ts` に `updateRequestError` の seed を追加
- 新規 `e2e/ui/http/response-viewer.spec.ts`（seed `httpResponse`）
  - `404 response is shown with destructive badge`、空ボディ、Timing タブ
  - `binary response shows hex view and save button`、画像表示
  - `show truncated body reveals the partial body with a warning`
  - `json response containing html is rendered as text`
  - `copy button writes response body to clipboard`（`integration/http/http.spec.ts:216` から移す、または複製）
- `e2e/ui/http/http.spec.ts` などに追加
  - `auto-save failure shows a dismissible banner`
  - `response of a request sent before switching is not shown`（切り詰めボディなら `DiscardResponseBody` が呼ばれる）
  - Doc タブ、レスポンスパネルの表示切替

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 15: OpenAPI の保存・ファイル D&D・最近のファイル

**観点**: [K] 保存・Ctrl+S・dirty 表示・プレビュー切替（中）・ファイル D&D（中）・最近のファイル（低）、[C] OpenAPI ファイルの並び替え（低）　**モード**: UI　**本体修正**: なし

- `e2e/fake-backend/install.ts` に `SaveFileAs` が返すパス（Step 3 で入れていなければ）、`GetRecents` / `ReadFile` の seed を追加
- `e2e/ui/openapi/` に追加
  - `Ctrl+S on an untitled doc opens save-as and clears the dirty marker`、Save ボタン、プレビュー表示の切り替え
  - Step 3 で保留した Save & continue
  - `dropping a yaml file opens it as an untitled document`（`DataTransfer` に `File` を入れて `drop` を dispatch）
  - `removing a recent file clears the editor when it was active`、最近のファイルの選択・並び替え（マウス方式。Step 5 のヘルパーを流用）
  - 空状態 "No files opened yet"

**完了条件**: `task frontend:test:e2e` が通る。

---

## Step 16: 共通 UI とアクセシビリティ（本体修正を含む）

**観点**: [A] 空状態・UDP/OpenAPI パネルの表示（中・低）、[B] keep-alive・サイドバー開閉（中）、[G] ダイアログの Escape / Tab 循環（中）・タブ ID の重複（低）・キーボードでの選択（低）、[L] 位置依存セレクタの原因　**モード**: UI　**本体修正**: あり（コミットを分ける）

- テストのみのコミット
  - `http form input survives a round trip through another protocol`
  - `clicking the active protocol toggles the sidebar`
  - `each protocol shows its empty state on first launch`、`udp and openapi panels become visible when selected`
  - `Escape closes the confirm dialog without deleting`、`Tab cycles focus inside the broker dialog`
  - `Enter on a focused request selects it`（ブローカー・ターゲット・プリセットも）
  - エラートーストの Dismiss
- 本体修正のコミット（`fix(frontend): ...`）
  - `tabs.tsx:24` のタブ ID を一意にする（リクエスト側とレスポンス側の衝突解消）と `tab ids are unique on the http panel`
  - UDP の Send / Listen タブを `role="tab"` にする（または送信ボタンに `data-testid`）
  - ユニットテスト（`task frontend:test`）への影響を確認する

**完了条件**: `task frontend:ci`、`task frontend:test`、`task frontend:test:e2e` が通る。

---

## Step 17: 既存テストのセレクタを整理する

**観点**: [L] ハードコードされたセレクタ（39 箇所）・位置依存のセレクタ（17 箇所）　**モード**: 両方　**本体修正**: なし　**前提**: Step 16

振る舞いを変えないリファクタリングだけのステップ。

- `e2e/fixtures/app.ts` に `editor()`（CodeMirror の内部クラス依存をここに集約）、リクエスト編集エリア／レスポンスエリアでスコープしたタブ取得を追加
- 置き換え
  - `locator("#tabpanel-*")` → パネルでスコープした `getByRole("tabpanel")`
  - `locator("span").filter({ hasText: /^New Collection$/ })` → `app.collection(name).getByText(name)`
  - `locator("#broker-*")` → `getByLabel`
  - `locator('[aria-label="Add"]')` → `getByRole("button", { name: "Add" })`
  - `locator('input[type="checkbox"]')` → `getByRole("checkbox")`
  - `.cm-*` の直接参照 → `app.editor()`
  - `integration/udp/udp.spec.ts` の `.last()`、`integration/http/http.spec.ts:107,122` の `.nth(1)` → Step 16 の修正を使ってスコープ指定
- `rg '\.nth\(|\.last\(\)|locator\("[#.]' frontend/e2e` で残りを確認し、残すものは理由をコメントに書く

**完了条件**: `task frontend:ci`、`task frontend:test:e2e`、`task frontend:test:e2e:fullstack` が通る。

---

## 対象外・保留

| 項目 | レビューの観点 | 理由 |
| --- | --- | --- |
| 空白のみの UDP ホスト（`"   "`）を拒否するか | [D]（低） | Go の `UDPTarget.Validate` は空文字だけを拒否する。仕様を決めてから、必要なら Go の検証を直したうえでテストを書く |
| ウィンドウ操作・ネイティブダイアログ・ウィンドウ状態の保存 | [M] 前提 | フルスタックのスイートは WebView2 のウィンドウを操作しないため検証範囲外 |
| `os.UserCacheDir()`（HTTP セッション領域）の隔離、`WIREXA_E2E_REUSE=1` 時の `APPDATA` | [M] 前提 | テスト追加ではなく config の改善。必要になった時点で別途対応する |
| コレクション内のアイテムをサイドバーの任意の位置へ出す | [C]（中） | Step 5 で判明した本体の不具合。`tree-item-node.tsx` の `isNoOp` がサイドバーゾーンでも `di.sourceIndex`（親の中での位置）と比べるため、その番号の前後 2 つのゾーンが隠れて落とせない。本体を直してからテストを足す（Step 5 のテストは隠れない位置に落としている） |

## 進捗

- [x] Step 1: フルスタックの後始末と前提確認
- [x] Step 2: HTTP 送信値
- [x] Step 3: OpenAPI 未保存確認（切り替え時）
- [x] Step 4: HTTP ツリー操作
- [x] Step 5: HTTP ツリー D&D
- [x] Step 6: UDP UI（ターゲット管理）
- [x] Step 7: UDP UI（送信・リスン）
- [x] Step 8: 偽バックエンドのイベント注入口
- [x] Step 9: MQTT UI（未接続）
- [x] Step 10: MQTT UI（接続済み・受信）
- [x] Step 11: フルスタック MQTT
- [x] Step 12: フルスタック永続化
- [ ] Step 13: フルスタック エラー表示・UDP 上限・echo
- [ ] Step 14: HTTP レスポンス表示ほか
- [ ] Step 15: OpenAPI 保存・D&D・最近のファイル
- [ ] Step 16: 共通 UI・アクセシビリティ
- [ ] Step 17: セレクタ整理
