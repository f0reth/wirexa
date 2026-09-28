# フロントエンド E2E テストレビュー

生成日時: 2026-09-28
対象: frontend/e2e/ 配下の E2E テスト（Playwright）
実行モード: **フロントエンドのみ（dev:e2e）とフルスタックの 2 系統が併存**

| 設定ファイル | モード | テスト置き場 | webServer | 実行環境 |
| --- | --- | --- | --- | --- |
| `frontend/playwright.config.ts` | フロントエンドのみ | `e2e/ui/**` (15 ファイル) | `bun run dev:e2e`（`vite.e2e.config.ts` が `e2e/fake-backend/install.ts` を注入し `window.go` / `window.runtime` を差し替え） | CI（`.github/workflows/ci.yml:119-144`）・ローカル |
| `frontend/playwright.integration.config.ts` | フルスタック | `e2e/integration/**` (4 spec) | `bun run build && vite preview` + `wails dev -s -noreload -nogorebuild`（`APPDATA` を `%TEMP%/wirexa-e2e-appdata` に隔離） | Windows ローカルのみ（CI 対象外） |

> 偽バックエンドの制約（以降の「モード」判定の前提）
>
> - `window.runtime.EventsOn` は何もしない（`e2e/fake-backend/install.ts:580-584`）。**`mqtt:message`・`udp:message`・`app:before-close` などバックエンドからのイベントを UI モードでは一切注入できない。**
> - `MQTTHandler.Connect` は常に `connection refused` で失敗する（`install.ts:545-547`）。MQTT のオンライン状態は UI モードでは作れない。
> - `UDPHandler.SaveTarget` / `StartListen` は Go 側の検証（`internal/domain/udp/types.go:79-87`、`internal/application/udp/listener_service.go:41-53`）を再現していない（`install.ts:477-505`）。
> - `OpenAPIHandler.OpenFilePicker` は常に `""`、`GetRecents` は常に `[]`（`install.ts:557-563`）。最近使ったファイル一覧を UI モードで作れない。

---

## Step 1: ユーザーフロー一覧

本体コードから抽出したユーザーフロー（85 件）。「済」は現行 E2E がそのフローの主経路を操作・検証しているもの。

### 共通

| # | フロー | コンポーネント（ファイル:行） | テスト済み? |
| --- | --- | --- | --- |
| 1 | アプリ起動 → MQTT パネルが初期表示される | `App.tsx:65`, `protocol-switcher.tsx:41` | 済（initial-state.spec.ts:3） |
| 2 | プロトコル切り替え（mqtt/http/udp/openapi） | `App.tsx:69-76,119-134` | 済（protocol-switching.spec.ts:3） |
| 3 | 選択中プロトコルを再クリック → サイドバー開閉（`aria-pressed` も false になる） | `App.tsx:70-71,40-62`, `protocol-switcher.tsx:41` | 未テスト |
| 4 | 訪問済みパネルの keep-alive（切り替え往復で入力状態が残る） | `App.tsx:78-85,121` | 部分的（protocol-switching.spec.ts:56 は display のみ） |
| 5 | テーマ切り替えと永続化 | `protocol-switcher.tsx:48-68`, `App.tsx:87-93`, `local-storage.ts:14,79-80` | 済（theme-persistence.spec.ts:5,29） |
| 6 | エラートーストの表示・Dismiss | `components/ui/toast.tsx:26-48` | 未テスト |
| 7 | サイドバー／パネルのリサイズ | `App.tsx:111-117`, `components/ui/resizable.tsx` | 未テスト |

### HTTP — サイドバー

| # | フロー | コンポーネント（ファイル:行） | テスト済み? |
| --- | --- | --- | --- |
| 8 | Add メニュー → New Collection | `collection-tree.tsx:70-77,226-235` | 済（sidebar-operations.spec.ts:7） |
| 9 | Add メニュー → New Request（ルート直下 `__root__`） | `collection-tree.tsx:51-68,219-225` | 未テスト |
| 10 | コレクションへフォルダ／リクエスト追加 | `collection-node.tsx:119-138` | 済（sidebar-operations.spec.ts:64,77） |
| 11 | フォルダ内へのフォルダ／リクエスト追加 | `tree-item-node.tsx:187-206` | 未テスト |
| 12 | コレクションのリネーム（ダブルクリック → Enter / Escape） | `collection-node.tsx:99-115`, `rename-input.tsx:24-27` | 済（sidebar-operations.spec.ts:12,28） |
| 13 | フォルダ／リクエストのリネーム | `tree-item-node.tsx:167-183,313-329` | 部分的（作成直後の名前確定のみ `fixtures/app.ts:90-97`） |
| 14 | リネームを blur で確定 | `rename-input.tsx:28` | 未テスト |
| 15 | コレクション削除（確認ダイアログ） | `collection-node.tsx:139-150`, `collection-tree.tsx:296-315` | 済（sidebar-operations.spec.ts:46） |
| 16 | フォルダ／リクエスト削除 | `tree-item-node.tsx:207-223,332-348` | 未テスト |
| 17 | 削除ダイアログの Cancel | `confirm-dialog.tsx:41`, `collection-tree.tsx:311` | 未テスト |
| 18 | リクエスト選択 → エディタへ読み込み | `tree-item-node.tsx:286-296`, `collection-tree.tsx:195-197` | 済（request-persistence.spec.ts:47, integration/http/http.spec.ts:188） |
| 19 | キーボード（Enter/Space）でリクエスト選択 | `tree-item-node.tsx:297-300` | 未テスト |
| 20 | コレクションの開閉 | `collection-node.tsx:78-88` | 済（sidebar-operations.spec.ts:77） |
| 21 | フォルダの開閉と展開状態の永続化 | `tree-item-node.tsx:146-155`, `local-storage.ts:16,105-106` | 未テスト |
| 22 | ツリーの D&D（アイテム移動・コレクション並び替え・ルートへの移動） | `use-long-press-drag.ts:1-2,15`, `use-tree-drag-drop.ts:45,104,166`, `collection-tree.tsx:132-169` | 未テスト |
| 23 | 空状態 "No collections yet" | `collection-tree.tsx:289-291` | 未テスト |

### HTTP — リクエスト編集・送信

| # | フロー | コンポーネント（ファイル:行） | テスト済み? |
| --- | --- | --- | --- |
| 24 | メソッド選択 | `request-bar.tsx:55-75` | 済（http.spec.ts:13） |
| 25 | URL 入力と送信ボタンの有効／無効 | `request-bar.tsx:17-24,42,97-104` | 済（form-validation.spec.ts:12-44） |
| 26 | URL 欄で Enter → 送信 | `request-bar.tsx:82-84` | 済（keyboard-accessibility.spec.ts:45） |
| 27 | 送信中 → Cancel | `request-bar.tsx:87-95`, `application/http/request.ts:270-274` | 済（async-loading.spec.ts:30） |
| 28 | Params / Headers の行追加・削除・有効切替 | `key-value-editor.tsx:27-75` | 済（form-validation.spec.ts:87,110,133） |
| 29 | 編集した headers / params / auth / settings が SendRequest に渡る | `application/http/request.ts:224-236` | 未テスト |
| 30 | Body 種別の切り替え | `request-editor.tsx:96-148` | 済（http.spec.ts:43-266） |
| 31 | form-data の行種別と行ごとの Content-Type | `form-row-editor.tsx:99` | 済（http.spec.ts:173-250） |
| 32 | ファイル参照（Browse で確定／入力のみは未確定） | `file-reference-input.tsx`, `request.ts:213-218` | 済（request-file.spec.ts:36-198） |
| 33 | Auth 種別の切り替え | `request-editor.tsx:159-206` | 済（http.spec.ts:271-314） |
| 34 | Settings（Timeout / Max Response Body / Proxy / TLS / Redirect） | `request-settings-panel.tsx:26-143` | 部分的（http.spec.ts:318-357。Max Response Body 未、送信値未検証） |
| 35 | Doc タブ（Markdown） | `request-editor.tsx:218-220`, `doc-editor.tsx:15-27` | 未テスト |
| 36 | レスポンスパネルの表示切替 | `request-bar.tsx:107-119`, `http/index.tsx:15-22,43-67` | 未テスト |
| 37 | 自動保存と保存失敗バナー | `request.ts:338-363,406-432`, `http/index.tsx:30-41` | 部分的（自動保存は request-persistence.spec.ts:21、バナー未） |
| 38 | 送信中に別リクエストへ切り替え → 応答を表示せず破棄 | `request.ts:197-204` | 未テスト |

### HTTP — レスポンス

| # | フロー | コンポーネント（ファイル:行） | テスト済み? |
| --- | --- | --- | --- |
| 39 | ステータス・ボディ表示 | `response-viewer.tsx:142-163` | 済（async-loading.spec.ts:49, integration/http/http.spec.ts:90） |
| 40 | エラー表示 | `response-viewer.tsx:126-140` | 済（async-loading.spec.ts:65） |
| 41 | Headers タブ（複数値ヘッダー含む） | `response-viewer.tsx:296-317` | 済（integration/http/http.spec.ts:98,113） |
| 42 | Timing タブ | `response-viewer.tsx:319-334` | 未テスト |
| 43 | Copy body | `response-viewer.tsx:83-105`, `copy-button.ts:35` | 済（integration/http/http.spec.ts:216。フルスタックのみ） |
| 44 | 切り詰めボディ（Show truncated body / Save body to file） | `response-viewer.tsx:177-243` | 部分的（response-save.spec.ts:25-84。Show truncated body 未） |
| 45 | 画像／バイナリ（hex）表示 | `response-viewer.tsx:246-267` | 未テスト |

### MQTT

| # | フロー | コンポーネント（ファイル:行） | テスト済み? |
| --- | --- | --- | --- |
| 46 | ブローカープロファイル作成 | `broker-tree.tsx:63-67`, `broker-settings-dialog.tsx:203-205` | 済（mqtt.spec.ts:7） |
| 47 | プロファイル入力検証 | `profile-validation.ts:16-33`, `broker-settings-dialog.tsx:48-54` | 済（mqtt.spec.ts:12,27） |
| 48 | プロファイル編集（Edit broker） | `profile-list.tsx:101-112`, `broker-tree.tsx:102` | 未テスト |
| 49 | プロファイル削除 | `profile-list.tsx:113-127,144-158`, `broker-tree.tsx:80-84` | 済（mqtt.spec.ts:55） |
| 50 | プロファイル選択・接続タブの切り替え | `broker-tree.tsx:52-60` | 部分的（作成直後の自動選択のみ） |
| 51 | Save & Connect / Connect / Disconnect | `broker-settings-dialog.tsx:194-202`, `broker-manager.tsx:116-136` | 未テスト |
| 52 | 未接続時のブローカー URL インライン編集 | `broker-manager.tsx:31-53,76-109` | 未テスト |
| 53 | Subscribe / Publish タブ切り替え | `tab-bar.tsx` | 済（mqtt.spec.ts:70） |
| 54 | 購読の追加・削除・ミュート | `subscriptions-panel.tsx:41-107`, `application/mqtt/subscriptions.ts:30-96` | 部分的（mqtt.spec.ts:93 はオフライン時の無効化のみ） |
| 55 | Broker Topics のスキャン | `broker-topics-panel.tsx:55-90` | 未テスト |
| 56 | メッセージ一覧・トピックフィルタ・Auto・Clear | `messages-panel.tsx:62-116` | 未テスト |
| 57 | メッセージ詳細と Copy payload | `message-detail.tsx:11-76` | 未テスト |
| 58 | Publish（トピック／QoS／Retain／送信） | `publish-tab.tsx:166-224` | 部分的（mqtt.spec.ts:134 は Retain のみ） |
| 59 | Publish プリセットの追加・選択・改名・削除・並び替え | `publish-tab.tsx:27-164` | 部分的（Retained バッジのみ mqtt.spec.ts:147） |
| 60 | 最後にアクティブだったプロファイルの復元 | `application/mqtt/connections.ts:352-371`, `local-storage.ts:11` | 未テスト |
| 61 | 空状態（"No brokers yet" / "No active connection..."） | `profile-list.tsx:138-140`, `mqtt/index.tsx:16-24` | 未テスト |

### UDP

| # | フロー | コンポーネント（ファイル:行） | テスト済み? |
| --- | --- | --- | --- |
| 62 | ターゲット作成 | `target-tree.tsx:34-112,123-127` | 済（integration/udp/udp.spec.ts:45。UI モードでは未） |
| 63 | ターゲット編集 | `target-tree.tsx:138` | 未テスト |
| 64 | ターゲット削除 | `target-tree.tsx:139-141` | 未テスト |
| 65 | ターゲット並び替えと順序の永続化 | `profile-list.tsx:46-48`, `application/udp/targets.ts:28-33`, `local-storage.ts:17` | 未テスト |
| 66 | ターゲット選択 → 送信フォームへ読み込み | `target-tree.tsx:137`, `application/udp/send.ts:129` | 済（integration/udp/udp.spec.ts:16-25） |
| 67 | テキスト送信 | `send-form.tsx:261-269` | 済（integration/udp/udp.spec.ts:40） |
| 68 | エンコーディング（text/json/fixed）切り替え | `send-form.tsx:81-97,242-259` | 未テスト |
| 69 | 固定長フィールド編集（追加・型・長さ・値検証・削除・エンディアン） | `send-form.tsx:100-240` | 未テスト（Go 統合テストのみ） |
| 70 | Send / Listen タブ | `udp/index.tsx:27-56` | 済（integration/udp/udp.spec.ts:27-32） |
| 71 | リスン開始・停止 | `listen-form.tsx:60-69,82-90` | 済（integration/udp/udp.spec.ts:63） |
| 72 | リスン失敗時のエラー表示（範囲外・使用中ポート） | `listen-form.tsx:71-73`, `listener_service.go:41-53` | 未テスト |
| 73 | 受信ログ・Clear・Copy payload | `message-log.tsx:16-48` | 部分的（integration/udp/udp.spec.ts:85。Clear / Copy 未） |
| 74 | 受信上限（500 件） | `application/udp/receive.ts:30` | 部分的（integration/udp/udp.spec.ts:104。件数のみ） |

### OpenAPI

| # | フロー | コンポーネント（ファイル:行） | テスト済み? |
| --- | --- | --- | --- |
| 75 | New（paste a spec）で無題文書 | `openapi-file-tree.tsx:75-83`, `openapi-provider.tsx:168-173` | 済（integration/openapi/openapi.spec.ts:19） |
| 76 | Open File（ネイティブダイアログ） | `openapi-file-tree.tsx:84-92`, `openapi-provider.tsx:114-127` | 未テスト |
| 77 | 最近のファイルの選択・削除・並び替え | `openapi-file-tree.tsx:24-61`, `openapi-file-node.tsx:52-74` | 未テスト |
| 78 | 編集 → プレビュー反映 | `editor-panel.tsx:30-40`, `preview-panel.tsx:40-54` | 済（ui/openapi/openapi.spec.ts:30） |
| 79 | パースエラーの表示 | `editor-panel.tsx:28` | 済（ui/openapi/openapi.spec.ts:69） |
| 80 | プレビュー表示の切り替え | `openapi/index.tsx:80-94` | 未テスト |
| 81 | 保存（ボタン / Ctrl+S）と dirty 表示 `*` | `openapi/index.tsx:22-39,70-79` | 未テスト |
| 82 | 未保存確認ダイアログ（Save & continue / Discard / Cancel） | `openapi-provider.tsx:82-112,222-236` | 未テスト |
| 83 | ファイルの D&D で開く | `openapi/index.tsx:43-54` | 未テスト |
| 84 | ウィンドウを閉じるときの未保存確認（`app:before-close` → `ConfirmQuit`） | `openapi-provider.tsx:187-199`, `infrastructure/app/lifecycle.ts:11-21`, `app.go:225` | 未テスト |
| 85 | 未許可パスの読み込み拒否 | `internal/adapters/openapi_handler.go:98` | 済（integration/openapi/openapi.spec.ts:53） |

集計: 済 35 / 部分的 11 / 未テスト 39

---

## Step 2: 既存テストカバレッジ

### フロントエンドのみ（`e2e/ui`, 偽バックエンド）

| テストファイル | describe / test | 検証内容 | 使用ロケーター |
| --- | --- | --- | --- |
| common/smoke.spec.ts:3 | app launches and renders | タイトルが /Wirexa/ | `toHaveTitle` |
| common/initial-state.spec.ts:3 | mqtt button is active by default | MQTT の `aria-pressed=true` | `getByRole("button")` |
| common/initial-state.spec.ts:11 | switching to http shows http panel | http-panel が flex、mqtt-panel が none | `getByTestId("*-panel")`, `toHaveCSS` |
| common/initial-state.spec.ts:21 | dark theme is restored from localStorage | `app:theme` を仕込んでリロード → ボタン名 | `page.evaluate`, `getByRole` |
| common/protocol-switching.spec.ts:3 | can switch between all protocols in order | 4 プロトコルの `aria-pressed` | `getByRole("button")` |
| common/protocol-switching.spec.ts:40 | sidebar content changes | 見出し Brokers/Collections/Targets が消える | `getByText` |
| common/protocol-switching.spec.ts:56 | returning to mqtt preserves mqtt panel in dom | display の往復 | `getByTestId`, `toHaveCSS` |
| common/protocol-switching.spec.ts:72 | previous protocol panel is hidden | 直前パネルが none | `getByTestId`, `toHaveCSS` |
| common/sidebar-operations.spec.ts:7-89 | 作成 / リネーム Enter・Escape / 削除ダイアログ / リクエスト選択 / 開閉 | ツリー表示・`aria-current` | `getByTestId("rename-input")`, `locator("span").filter`, `getByRole("dialog")` |
| common/form-validation.spec.ts:12-176 | URL 空・不正で Send 無効、UDP ポートの min/max、KV 行追加削除・有効切替、空白リネーム、日本語名 | `toBeDisabled`, `validity.valid`, 行数 | `getByPlaceholder`, `locator("#tabpanel-*")`, `locator('input[type="checkbox"]')` |
| common/async-loading.spec.ts:16-77 | 送信中 Cancel 表示・キャンセル・成功表示・接続エラー表示 | `Sending request...`、200、`response-error` | seed `httpResponseDelayMs` / `httpError`, `getByTestId` |
| common/keyboard-accessibility.spec.ts:6-52 | aria-label、Tab 移動、Enter 送信 | `toBeFocused`、Cancel 表示 | `keyboard.press` |
| common/request-persistence.spec.ts:21,47 | 選択中リクエストと URL がリロード後に復元 | `aria-current`、URL 値 | seed `collections`, `fake.waitForCalls("UpdateRequest")` |
| common/theme-persistence.spec.ts:5,29 | テーマ切替 → リロード | ボタン名 | `getByRole` |
| http/http.spec.ts:13-398 | メソッド、Body 種別、form 行、Auth、Settings、Headers | 表示・入力値 | `getByTestId("method-select"/"form-kind-select")`, `locator("#tabpanel-*")`, `.cm-editor` |
| http/request-file.spec.ts:36-198 | 未確定パスは送れない、Browse で token 付き送信、再選択表示 | `fake.args("SendRequest")`, `fake.calls` | seed `pickedFile` |
| http/response-save.spec.ts:25-84 | 切り詰めボディ保存は execution ID のみ、再送で破棄、回収済みの案内 | `fake.args("SaveResponseBody"/"DiscardResponseBody")` | seed `httpResponse`, `saveResponseError` |
| http/root-collection.spec.ts:32 | `__root__` の削除・リネーム拒否（偽バックエンドの回帰） | バインディング直接呼び出し | `page.evaluate`, `fake.snapshot` |
| mqtt/mqtt.spec.ts:7-151 | プロファイル作成・検証・削除、タブ、オフライン時 Subscribe 無効、QoS、Retain | `toBeDisabled`, `aria-selected` | `locator("#broker-*")`, `getByTestId("qos-select")` |
| openapi/openapi.spec.ts:30-83 | 編集 → プレビュー、クリア、パースエラー | "No valid OpenAPI spec"、lint マーク | `locator(".cm-editor"/".cm-content"/".cm-lint*")`, `document.execCommand` |

### フルスタック（`e2e/integration`, 実 Go バックエンド）

| テストファイル | describe / test | 検証内容 | 使用ロケーター |
| --- | --- | --- | --- |
| integration/global-setup.ts:8 | （globalSetup） | タイトルが /Wirexa/ になるまで待つ | `waitForFunction` |
| common/backend-integration.spec.ts:37-53 | M-1: GetCollections / GetBrokerProfiles / GetTargets | サイドバー見出しの表示のみ | `app.switchTo` |
| common/backend-integration.spec.ts:61 | M-2: collection persists after page reload | リロード後もコレクションがある | `app.collection` |
| common/backend-integration.spec.ts:76 | M-3: broker profile persists after page reload | 同上（ブローカー） | `locator('[role="button"]').filter` |
| common/backend-integration.spec.ts:90 | M-4: udp target persists after page reload | 同上（ターゲット） | `getByText` |
| common/backend-integration.spec.ts:107 | M-8: broker order persists after reorder and reload | HTML5 DnD を dispatch、リロード後の順序 | `dispatchEvent("dragstart"...)` |
| http/http.spec.ts:77-236 | 実サーバーへの送信・ステータス・ボディ・ヘッダー（複数値）、multipart、保存と再読み込み、Copy body | Node の `http.createServer` | `getByTestId("response-body")`, `expect.poll(savedUrl)` |
| openapi/openapi.spec.ts:19,53 | 無題文書へのペースト → プレビュー、未許可パスの ReadFile 拒否 | プレビュー表示、reject | clipboard, `page.evaluate` |
| udp/udp.spec.ts:40-156 | 送信、リスン開始停止、受信表示、500 件上限、最新メッセージ表示 | Node の `dgram` で送受信 | `getByText("Received (N)")`, `getByRole("button").last()` |

---

## サマリー

> **チェック項目数の数え方**: 各観点テーブルの行数を「チェック項目数」とする。カバー率 = (済 + 部分的 × 0.5) ÷ 項目数。観点 L の「済」は「問題なし」を意味する。

| 観点 | チェック項目数 | 済 | 部分的 | 不足 | カバー率 |
| --- | --- | --- | --- | --- | --- |
| [A] ページ表示・初期状態 | 4 | 2 | 1 | 1 | 63% |
| [B] プロトコル切り替え | 4 | 3 | 1 | 0 | 88% |
| [C] サイドバー操作 | 6 | 2 | 4 | 0 | 67% |
| [D] フォーム入力・バリデーション・境界値 | 9 | 4 | 4 | 1 | 67% |
| [E] 非同期操作・ローディング状態 | 5 | 3 | 2 | 0 | 80% |
| [F] テーマ・設定の永続化 | 3 | 1 | 2 | 0 | 67% |
| [G] キーボード操作・アクセシビリティ | 4 | 4 | 0 | 0 | 100% |
| [H] MQTTプロトコル固有フロー | 7 | 1 | 1 | 5 | 21% |
| [I] HTTPプロトコル固有フロー | 9 | 8 | 1 | 0 | 94% |
| [J] UDPプロトコル固有フロー | 5 | 3 | 2 | 0 | 80% |
| [K] OpenAPIプロトコル固有フロー | 3 | 2 | 1 | 0 | 83% |
| [L] テスト構造の整合性 | 8 | 4 | 2 | 2 | 63% |
| [M] バックエンド統合テスト | 8 | 3 | 3 | 2 | 56% |
| **合計** | **75** | **40** | **24** | **11** | **69%** |

チェック表ベースでは 69% だが、チェック表は既存テストが狙った項目に寄っている。Step 1 のフロー単位では 48%（後述）で、こちらが実態に近い。

---

## 観点別詳細

### [A] ページ表示・初期状態

#### テスト済みケース

- `initial-state.spec.ts:3` — 起動直後に MQTT ボタンが `aria-pressed=true`
- `initial-state.spec.ts:21` — `app:theme="dark"` を仕込んでリロードするとダーク表示
- （部分的）`initial-state.spec.ts:11` — http-panel の `display: flex` は見ているが、udp-panel / openapi-panel が flex になることは見ていない（`protocol-switching.spec.ts:72` は直前パネルが none になることだけを見る）

#### 不足ケース

- **不足内容**: 各サイドバー・パネルの空状態表示
- **根拠コード**: `collection-tree.tsx:289-291` "No collections yet"、`profile-list.tsx:138-140`（"No brokers yet" は `broker-tree.tsx:98`、"No targets yet" は `target-tree.tsx:134` から渡る）、`openapi-file-tree.tsx:126-128` "No files opened yet"、`mqtt/index.tsx:16-24` "No active connection..."、`udp/index.tsx:17-25` "ターゲットを選択してください"。`async-loading.spec.ts:73-75` は "Send a request to see the response" が**消える**ことだけを見ている
- **推奨テスト名**: `test("each protocol shows its empty state on first launch", ...)`
- **モード**: フロントエンドのみで検証可能
- **優先度**: 中

- **不足内容**: UDP / OpenAPI パネルが `display: flex` になること
- **根拠コード**: `App.tsx:122-129`
- **推奨テスト名**: `test("udp and openapi panels become visible when selected", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 低（`switchTo` がサイドバー見出しで間接的に担保している）

### [B] プロトコル切り替え

#### テスト済みケース

- `protocol-switching.spec.ts:3` — MQTT→HTTP→UDP→OpenAPI の `aria-pressed`
- `protocol-switching.spec.ts:40` — サイドバー見出しの入れ替わり
- `protocol-switching.spec.ts:72` — 直前のパネルが `display: none`
- （部分的）`protocol-switching.spec.ts:56` — 往復後の display は見ているが、**状態が保持されること**（keep-alive の目的）は見ていない

#### 不足ケース

- **不足内容**: HTTP で URL を入力 → MQTT → HTTP に戻っても URL・タブ選択が残る
- **根拠コード**: `App.tsx:78-85,121` — 一度訪れたパネルは `Show` で残し `display` だけ切り替える。アンマウントされる実装に変わると入力が消えるが、現テストは display しか見ないので検出できない
- **推奨テスト名**: `test("http form input survives a round trip through another protocol", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中

- **不足内容**: 選択中のプロトコルボタンを再クリックするとサイドバーが閉じ、もう一度で開く
- **根拠コード**: `App.tsx:69-72`（同一プロトコルなら `setSidebarOpen(prev => !prev)`）、`App.tsx:40-62`（`SidebarCollapseController`）、`protocol-switcher.tsx:41`（閉じると `aria-pressed=false`）
- **推奨テスト名**: `test("clicking the active protocol toggles the sidebar", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中（表の外だが、既存 UI で未テストの操作）

### [C] サイドバー操作

#### テスト済みケース

- `sidebar-operations.spec.ts:7` / `:64` / `:77` — コレクション・リクエスト・フォルダの追加
- `sidebar-operations.spec.ts:12,28` — コレクションのリネーム（Enter 確定 / Escape 取消）
- `sidebar-operations.spec.ts:46` — コレクション削除（ダイアログ → Delete）
- `sidebar-operations.spec.ts:64` — リクエスト選択で `aria-current`
- `mqtt.spec.ts:55` — ブローカー削除
- `backend-integration.spec.ts:107` — ブローカーの D&D 並び替え（フルスタックのみ）

#### 不足ケース

- **不足内容**: HTTP ツリーの D&D（リクエストを別コレクション／フォルダへ移動、コレクションの並び替え、コレクション内アイテムをルートへ出す）
- **根拠コード**: `use-long-press-drag.ts:1-2,15`（250ms 長押しまたは 5px 移動で開始するマウス方式。HTML5 DnD ではない）、`use-tree-drag-drop.ts:45,104,166`（`elementFromPoint` でドロップ先を判定）、`collection-tree.tsx:132-169`（`moveSidebarEntry` / `moveItemToSidebar` / `moveItem`）。偽バックエンドは `MoveItem` / `MoveSidebarEntry` / `MoveItemToSidebar` を実装済み（`install.ts:380-441`）
- **推奨テスト名**: `test("dragging a request onto another collection moves it", ...)`、`test("dragging a collection reorders the sidebar", ...)`
- **モード**: フロントエンドのみで検証可能（`page.mouse.down()` → `move()` → `up()`。`dragTo()` は mousedown 直後に移動するので 5px しきい値は越えられる）
- **優先度**: 高（アイテムの所属が変わる操作で、壊れるとデータが意図しない場所へ移る）

- **不足内容**: フォルダ／リクエストの削除とリネーム
- **根拠コード**: `tree-item-node.tsx:207-223`（Delete folder）、`:332-348`（Delete request）、`:167-183,313-329`（ダブルクリックでリネーム）
- **推奨テスト名**: `test("can delete a request after confirming", ...)`、`test("can rename a request by double-click", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 高

- **不足内容**: 削除ダイアログの Cancel で何も消えない
- **根拠コード**: `collection-tree.tsx:311`、`profile-list.tsx:154`、`confirm-dialog.tsx:41`
- **推奨テスト名**: `test("cancelling the delete dialog keeps the item", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中

- **不足内容**: フォルダ自体の開閉（テスト名は "folders" だが実際はコレクションをトグルしている）
- **根拠コード**: `sidebar-operations.spec.ts:77-89` はコレクション行をクリックしている。フォルダは既定で閉じている（`tree-item-node.tsx:100` `isExpanded(id, false)`）ので、フォルダを開いて子リクエストが見える流れは未検証
- **推奨テスト名**: `test("expanding a folder reveals its children", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中

- **不足内容**: Add メニュー → New Request（ルート直下）と、フォルダ内へのフォルダ／リクエスト追加
- **根拠コード**: `collection-tree.tsx:51-68,219-225`、`tree-item-node.tsx:187-206`
- **推奨テスト名**: `test("New Request from the add menu creates a root-level request", ...)`、`test("can add a request inside a folder", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中

- **不足内容**: UDP ターゲット・MQTT プリセット・OpenAPI ファイルの並び替え
- **根拠コード**: `profile-list.tsx:46-48`（`createListReorder`、HTML5 DnD）、`publish-tab.tsx:37-38`、`openapi-file-tree.tsx:24-61`（マウス方式）
- **推奨テスト名**: `test("udp targets can be reordered by drag and drop", ...)`
- **モード**: UDP・プリセットはフロントエンドのみ（`backend-integration.spec.ts:13-19` の `dragRowOnto` を流用できる）。OpenAPI は偽バックエンドの `GetRecents` が常に空なので、偽バックエンドの拡張かフルスタックが要る
- **優先度**: 低

### [D] フォーム入力・バリデーション・境界値

#### テスト済みケース

- `form-validation.spec.ts:12,36` — URL 空欄で Send 無効
- `form-validation.spec.ts:27` — `not-a-url` で Send 無効
- `form-validation.spec.ts:87,110` — KV エディタの行追加・削除
- `form-validation.spec.ts:133` — KV 行の有効チェックボックス
- `mqtt.spec.ts:12,27` — ブローカー名空欄、`1e3` / `1883.0` / `::1` の拒否
- （部分的）`form-validation.spec.ts:150` — 空白のみのリネームは元の名前に戻る
- （部分的）`form-validation.spec.ts:48` — UDP ターゲットのポート入力の `min`/`max` 属性と `validity`
- （部分的）`integration/udp/udp.spec.ts:104` — UDP 500 件上限
- （部分的）`form-validation.spec.ts:170` — コレクション名に日本語

#### 不足ケース

- **不足内容**: 範囲外ポートで UDP ターゲットを**保存したとき**の挙動
- **根拠コード**: `target-tree.tsx:44-52` はダイアログ側で検証せず `Number(port)` をそのまま保存する。Go 側は `UDPTarget.Validate`（`internal/domain/udp/types.go:79-87`）で拒否し、`targets.ts:40-45` が通知して、`target-tree.tsx:123-127` がダイアログを開いたままにする。偽バックエンドの `SaveTarget`（`install.ts:477-485`）は検証しないので、UI モードでは 0 番ポートのターゲットが保存できてしまい、実アプリと挙動が違う。既存テストは `validity.valid` を見るだけで Save を押していない
- **推奨テスト名**: `test("saving a udp target with port 0 shows an error and keeps the dialog open", ...)`
- **モード**: フルスタックで検証するか、偽バックエンドの `SaveTarget` に Go と同じ検証を足してフロントエンドのみで検証する
- **優先度**: 高（偽バックエンドと実バックエンドの挙動が食い違っている）

- **不足内容**: MQTT ブローカーのポート `0` / `65536`、ホスト `"   "`
- **根拠コード**: `profile-validation.ts:16-25`（`^\d+$` かつ 1〜65535、ホストは空白を拒否）。既存テスト `mqtt.spec.ts:27` は指数表記・小数・IPv6 だけを見ている
- **推奨テスト名**: `test("broker dialog rejects port 0 and 65536 and a blank host", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中

- **不足内容**: MQTT の不正トピック（空、中間の `#`、`+` の混在）で購読・Publish したときのエラー表示
- **根拠コード**: フロントエンドの `domain/mqtt/topic.ts` はマッチングだけで検証しない。検証は Go の `ValidateTopicFilter` / `ValidateTopicName`（`internal/domain/mqtt/topic.go:20,33`、呼び出しは `internal/application/mqtt/service.go:282,298`）で、失敗すると `subscriptions.ts:55` が "Failed to subscribe to ..." を通知する
- **推奨テスト名**: `test("subscribing to 'a/#/b' shows a validation error toast", ...)`
- **モード**: フルスタック（接続済みブローカーが必要）
- **優先度**: 中

- **不足内容**: `MQTT_MAX_MESSAGES = 5000` / `MQTT_MAX_TOPICS = 500` の上限
- **根拠コード**: `config/limits.ts:1-2`、`application/mqtt/connections.ts:213-231`（トピックは先頭から、メッセージは古い順に捨てる）。ただしメッセージには上限が 2 段ある。表示一覧は上限を超えると最古を捨てる（`connections.ts:224-230`）が、その手前の受信バッファ（`connections.ts:244-246`）は 1 描画フレーム内で 5,000 件に達すると**後続の新着を捨てる**（古い方ではなく新しい方が落ちる）。UDP 上限 500（`limits.ts:3`、`receive.ts:30`）のテストは件数表示 "Received (500)" しか見ておらず、**捨てられたのが最古の `msg-0` であること**を確かめていない
- **推奨テスト名**: `test("mqtt message list drops the oldest message when the 5001st arrives", ...)`（フレームをまたいで届ける）、`test("mqtt messages beyond 5000 within one frame are dropped from the buffer", ...)`（1 フレーム内にまとめて届ける）、`test("udp log keeps the newest 500 and drops msg-0", ...)`
- **モード**: フルスタック（ブローカー／UDP 送信）。偽バックエンドにイベント注入口を足せばフロントエンドのみでも可
- **優先度**: 中

- **不足内容**: `<script>` / `<img onerror>` を含む名前・レスポンスが文字列として表示される
- **根拠コード**: `response-viewer.tsx:268-273` は JSON レスポンスを `innerHTML` で描画する（`json-highlight.ts:1-3,26-31` でエスケープ済み）。エスケープが外れる回帰を検出するテストが無い
- **推奨テスト名**: `test("json response containing html is rendered as text", ...)`
- **モード**: フロントエンドのみ（seed `httpResponse: { body: '{"x":"<img src=x onerror=...>"}', contentType: "application/json" }`）
- **優先度**: 中

- **不足内容**: 空白のみのトピック・ホスト
- **根拠コード**: `subscriptions.ts:31-32`（trim して空なら無視）。UDP ターゲットの Go 検証は `t.Host == ""` しか見ない（`types.go:80`）ため、`"   "` のホストは保存される。意図した挙動かどうかをテストで固定しておくとよい
- **推奨テスト名**: `test("whitespace-only udp host is rejected", ...)`（仕様を決めてから）
- **モード**: フルスタック
- **優先度**: 低

### [E] 非同期操作・ローディング状態

#### テスト済みケース

- `async-loading.spec.ts:16` — 送信中は Send が Cancel に置き換わる（二重送信防止）
- `async-loading.spec.ts:30` — Cancel で "Sending request..." が消える
- `async-loading.spec.ts:49` — 成功後に 200 とボディ
- （部分的）`async-loading.spec.ts:65` — HTTP の接続エラー表示

#### 不足ケース

- **不足内容**: エラートースト（`role="alert"`）の表示
- **根拠コード**: `toast.tsx:26-48`。E2E 全体で一度もトーストを検証していない。UI モードで出せるもの: MQTT の Connect 失敗（偽バックエンドの `Connect` は必ず失敗する `install.ts:545-547` → `connections.ts:486` "Failed to reconnect"）。UDP の Start 失敗はトーストではなくフォーム内の `<p>` に出る（`receive.ts:47-51` → `listen-form.tsx:71-73`）うえ、偽バックエンドの `StartListen`（`install.ts:493-504`）が拒否するのは未知の encoding だけで、UI の選択肢（`PAYLOAD_ENCODINGS`、`listen-form.tsx:55`）からは指定できない。現状の UI 操作では出せないので、偽バックエンドに失敗の seed（例: `startListenError`）を足す必要がある
- **推奨テスト名**: `test("failed mqtt connect shows an error toast", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 高（失敗をユーザーに伝える唯一の経路）

- **不足内容**: 自動保存失敗バナー "Save failed: ..." と ✕ で閉じる操作
- **根拠コード**: `http/index.tsx:30-41`、`request.ts:354-362`
- **推奨テスト名**: `test("auto-save failure shows a dismissible banner", ...)`
- **モード**: フロントエンドのみ（偽バックエンドに `updateRequestError` の seed を足す）
- **優先度**: 中

- **不足内容**: 送信中に別のリクエストへ切り替えると、戻ってきた応答を表示しない
- **根拠コード**: `request.ts:197-204`（`sentView !== view` なら破棄、切り詰めボディは `DiscardResponseBody`）
- **推奨テスト名**: `test("response of a request sent before switching is not shown", ...)`
- **モード**: フロントエンドのみ（seed `httpResponseDelayMs` を短めにする）
- **優先度**: 中

- **不足内容**: UDP の送信中・リスン開始中の無効化
- **根拠コード**: `send-form.tsx:263,268`（"Sending..." で disabled）、`listen-form.tsx:61,68`（"Starting..."、セッションがあれば Start 無効）
- **推奨テスト名**: `test("udp Start is disabled while a session is listening", ...)`
- **モード**: フロントエンドのみ（偽バックエンドに遅延 seed を足す）
- **優先度**: 低

- **不足内容**: Cancel が送信時と同じ execution ID で `CancelRequest` を呼ぶこと
- **根拠コード**: `request.ts:270-274`、`install.ts:448-450`
- **推奨テスト名**: 既存の `clicking cancel aborts the in-progress request` に `fake.args("CancelRequest")` の検証を足す
- **モード**: フロントエンドのみ
- **優先度**: 低

### [F] テーマ・設定の永続化

#### テスト済みケース

- `theme-persistence.spec.ts:5,29` — テーマ切替 → リロードで維持
- （部分的）`request-persistence.spec.ts:21` — 選択中の HTTP リクエストがリロード後に復元
- （部分的）`request-persistence.spec.ts:47` — URL が自動保存されて復元（method / headers / body / auth は見ていない）

#### 不足ケース

- **不足内容**: MQTT の最後にアクティブだったプロファイルの復元
- **根拠コード**: `connections.ts:352-371`、`local-storage.ts:11`（`mqtt:lastActiveProfileId`）
- **推奨テスト名**: `test("last active broker is selected again after reload", ...)`
- **モード**: フロントエンドのみ（seed `mqttProfiles` で 2 件用意）
- **優先度**: 中

- **不足内容**: MQTT Publish プリセットの永続化
- **根拠コード**: `local-storage.ts:12,63-73`（`mqtt:presets`）、`mqtt-provider.tsx:154-156`（空なら 1 件作る）
- **推奨テスト名**: `test("publish presets survive a reload", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中

- **不足内容**: フォルダ展開状態と UDP ターゲット順の永続化
- **根拠コード**: `local-storage.ts:16-17,105-106,119-120`
- **推奨テスト名**: `test("expanded folders are restored after reload", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 低

- **不足内容**: URL 以外（method / headers / body / auth / settings）の自動保存と復元
- **根拠コード**: `request.ts:338-353` はリクエスト全体を `UpdateRequest` に送る
- **推奨テスト名**: `test("method, headers and body are restored after reload", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中

### [G] キーボード操作・アクセシビリティ

#### テスト済みケース

- `keyboard-accessibility.spec.ts:18,29` — Tab で URL 欄から Send へ
- `keyboard-accessibility.spec.ts:45` — Enter で送信
- `sidebar-operations.spec.ts:28` — リネーム中の Escape
- `keyboard-accessibility.spec.ts:6` — プロトコルボタンの `aria-label`（accessible name での取得に加え、`toHaveAttribute("aria-label", label)` で属性そのものも確かめている）

#### 不足ケース

- **不足内容**: ダイアログの Escape で閉じる・Tab がダイアログ内で循環する
- **根拠コード**: `focus-trap.ts:25-43`（Escape で `onEscape`、Tab / Shift+Tab を循環）。ConfirmDialog・ブローカー設定・UDP ターゲットのすべてが使う
- **推奨テスト名**: `test("Escape closes the confirm dialog without deleting", ...)`、`test("Tab cycles focus inside the broker dialog", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 中

- **不足内容**: `role="tab"` の ID 重複
- **根拠コード**: `tabs.tsx:24` が `id="tab-${value}"` を出す。HTTP パネルではリクエスト側（`request-editor.tsx` の Headers / Body）とレスポンス側（`response-viewer.tsx:13-17` の Body / Headers）が同じ ID を持ち、`tabpanel` の `aria-labelledby`（`tabs.tsx:49`）が一意に決まらない。なお `integration/http/http.spec.ts:107,122` が `getByRole("tab", { name: "Headers" }).nth(1)` と位置で選んでいる直接の理由は ID 重複ではなく、リクエスト側とレスポンス側に同じ名前 "Headers" のタブがあること（role と name で 2 件一致する）。ID を一意にしてもこの `.nth(1)` は残るので、パネルでスコープして選ぶ（[L]・実装ガイド参照）
- **推奨テスト名**: `test("tab ids are unique on the http panel", ...)`（修正と合わせて）
- **モード**: フロントエンドのみ
- **優先度**: 低

- **不足内容**: キーボードでのツリー／リスト選択
- **根拠コード**: `tree-item-node.tsx:297-300`、`profile-list.tsx:89-92`、`publish-tab.tsx:91-97`（Enter / Space で選択）。`integration/udp/udp.spec.ts:16-25` は回避策として focus + Enter を使っているが、それ自体をアクセシビリティとして検証していない
- **推奨テスト名**: `test("Enter on a focused request selects it", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 低

### [H] MQTTプロトコル固有フロー

> モード注記: 偽バックエンドは `Connect` を必ず失敗させ（`install.ts:545-547`）、`EventsOn` が no-op（`install.ts:580-584`）。フロントエンドのみモードではオンライン状態も受信も作れない。フルスタック側には **MQTT の spec が 1 本も無い**（`e2e/integration/` は common / http / openapi / udp のみ）。

#### テスト済みケース

- `mqtt.spec.ts:110` — QoS 0/1/2 の選択（購読側のみ）
- （部分的）`mqtt.spec.ts:93` — 未接続時は Subscribe ボタンが無効

#### 不足ケース

- **不足内容**: ブローカーへの接続・切断
- **根拠コード**: `broker-manager.tsx:116-136`（Connect / Disconnect）、`broker-settings-dialog.tsx:194-202`（Save & Connect）、`connections.ts:397-500`
- **推奨テスト名**: `test("connects to a local broker and shows Connected", ...)`、`test("disconnect returns the tab to Disconnected", ...)`
- **モード**: フルスタック（テスト用ブローカーが必要）。接続**失敗**のトーストだけならフロントエンドのみで検証可能
- **優先度**: 高（アプリの既定プロトコルの主経路で、どちらのモードでも一度も通っていない）

- **不足内容**: 購読 → 受信 → Messages パネル表示 → 詳細表示
- **根拠コード**: `subscriptions.ts:30-65`、`connections.ts:244-252`（`mqtt:message` を rAF でまとめて反映）、`connections.ts:193-199`（ミュートされていない購読に一致したものだけ表示）、`messages-panel.tsx:138-175`、`message-detail.tsx:11-76`
- **推奨テスト名**: `test("published message on a subscribed topic appears in Messages", ...)`、`test("muted subscription hides its messages", ...)`
- **モード**: フルスタック
- **優先度**: 高

- **不足内容**: Publish
- **根拠コード**: `publish-tab.tsx:174-180`（トピック空・非 retain で空ペイロードは送らない）、`publish-tab.tsx:217-220`（未接続時は無効）、`mqtt-provider.tsx:159-170`
- **推奨テスト名**: `test("publishing to a subscribed topic loops back into Messages", ...)`、`test("publish button is disabled while offline", ...)`
- **モード**: 送信はフルスタック、未接続時の無効化はフロントエンドのみ
- **優先度**: 高

- **不足内容**: 複数ブローカーの切り替えと状態の独立
- **根拠コード**: `broker-tree.tsx:52-60`（既存接続があれば切り替え、なければオフラインタブを作る）、`connections.ts:384-395`
- **推奨テスト名**: `test("switching brokers keeps each broker's subscriptions separate", ...)`
- **モード**: 選択の切り替えはフロントエンドのみ（seed `mqttProfiles`）。購読の独立はフルスタック
- **優先度**: 中

- **不足内容**: メッセージ上限 5000 件
- **根拠コード**: `limits.ts:1`、`connections.ts:224-231`、`connections.ts:245`（バッファ自体も 5000 で打ち切る）
- **推奨テスト名**: `test("mqtt message list is capped at 5000", ...)`
- **モード**: フルスタック
- **優先度**: 中

- **不足内容**: 新着メッセージへのオートスクロール
- **根拠コード**: `messages-panel.tsx:62-68`（`autoFollow` のときだけ末尾へ `scrollToIndex`）、`messages-panel.tsx:96-107`（Auto ボタン）。`autoFollow` の既定値は false（`connections.ts:83,116`）なので、**Auto を ON にしたときだけ**追従する仕様であることも併せて検証する。リストは仮想化されている（`messages-panel.tsx:50-60`）ため、末尾要素の DOM 存在で判定できる
- **推奨テスト名**: `test("with Auto enabled the newest message is scrolled into view", ...)`
- **モード**: フルスタック（偽バックエンドにイベント注入口を足せばフロントエンドのみでも可）
- **優先度**: 低

- **不足内容**（表外）: プロファイル編集、Broker Topics スキャン、トピックフィルタ、Clear、プリセットの追加・改名・削除
- **根拠コード**: `profile-list.tsx:101-112`、`broker-topics-panel.tsx:55-90`、`messages-panel.tsx:85-116`、`publish-tab.tsx:27-164`
- **モード**: 編集・プリセットはフロントエンドのみ、スキャン・フィルタはフルスタック
- **優先度**: 中（編集・プリセット）／低（その他）

### [I] HTTPプロトコル固有フロー

#### テスト済みケース

- `http.spec.ts:13` — メソッド選択
- `integration/http/http.spec.ts:77,90` — 実サーバーへの送信とステータス・ボディ表示
- `http.spec.ts:43-266` — Body 種別切替と form 行
- `integration/http/http.spec.ts:98,113` — レスポンスヘッダー（複数値含む）
- `integration/http/http.spec.ts:188`、`request-persistence.spec.ts:47` — 保存と再読み込み
- `http.spec.ts:271-314` — Basic / Bearer
- `http.spec.ts:318-357` — Timeout / Proxy / TLS / Redirect の入力 UI
- `integration/http/http.spec.ts:216` — Copy body
- （部分的）`http.spec.ts:361,381` — ヘッダーは入力できることだけを見ていて、送信内容に含まれることは見ていない

#### 不足ケース

- **不足内容**: 入力した headers / params / auth / settings / method が実際の送信内容になる
- **根拠コード**: `request.ts:224-236`。偽バックエンドは `fake.args("SendRequest")` で第 2 引数を取れる（`request-file.spec.ts:28-31` に先例あり）。フルスタックでは `/echo`（`integration/http/http.spec.ts:19-27`）がヘッダーを返せば検証できる
- **推奨テスト名**: `test("headers, query params and bearer token are sent with the request", ...)`
- **モード**: フロントエンドのみ（fake.args）＋フルスタック（echo でワイヤ形式）
- **優先度**: 高（入力 UI だけ見ていて、送信に反映されることが無検証）

- **不足内容**: レスポンスの Timing タブ、4xx/5xx の表示、空ボディ、画像・バイナリ
- **根拠コード**: `response-viewer.tsx:319-334`（Timing）、`:24-28`（`statusVariant`）、`:166-175`（"No response body"）、`:246-267`（画像・hex）
- **推奨テスト名**: `test("404 response is shown with destructive badge", ...)`、`test("binary response shows hex view and save button", ...)`
- **モード**: フロントエンドのみ（seed `httpResponse`）
- **優先度**: 中

- **不足内容**: 切り詰めボディの "Show truncated body"
- **根拠コード**: `response-viewer.tsx:196-200,224-243`
- **推奨テスト名**: `test("show truncated body reveals the partial body with a warning", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 低

- **不足内容**: Copy body を UI モードでも検証する
- **根拠コード**: `copy-button.ts:35` は `navigator.clipboard` を使うので Wails に依存しない。現状フルスタックだけにあり CI で走らない
- **推奨テスト名**: 既存 `copy button writes response body to clipboard` を `e2e/ui/http` へ移す（または複製する）
- **モード**: フロントエンドのみ
- **優先度**: 中

- **不足内容**: Doc タブ、レスポンスパネルの表示切替、Max Response Body 設定
- **根拠コード**: `request-editor.tsx:218-220`、`request-bar.tsx:107-119`、`request-settings-panel.tsx:46-69`
- **モード**: フロントエンドのみ
- **優先度**: 低

### [J] UDPプロトコル固有フロー

> モード注記: **`e2e/ui/` に UDP の spec が 1 本も無い**。UDP の E2E はすべてフルスタック（CI 対象外）で、CI 上で UDP を触るのは `form-validation.spec.ts:48` のポート属性チェックだけ。

#### テスト済みケース

- `integration/udp/udp.spec.ts:40` — 送信（Node 側で受信内容まで確認）
- `integration/udp/udp.spec.ts:63` — リスン開始・停止
- `integration/udp/udp.spec.ts:85` — 受信メッセージの表示
- （部分的）`integration/udp/udp.spec.ts:104` — 500 件上限（件数のみ）
- （部分的）`integration/udp/udp.spec.ts:136` — 最新メッセージが見える（並び順は未検証）

#### 不足ケース

- **不足内容**: UI モードでの UDP 一式（ターゲット作成・編集・削除、送信内容、エンコーディング切替）
- **根拠コード**: `target-tree.tsx:34-171`、`send-form.tsx:81-97,261-269`。偽バックエンドの `Send` は引数を記録する（`install.ts:511-516`）
- **推奨テスト名**: `test("editing a udp target updates its host:port in the sidebar", ...)`、`test("json encoding sends the json payload", ...)`
- **モード**: フロントエンドのみ
- **優先度**: 高（CI 上で UDP の主要フローが一度も通らない）

- **不足内容**: 固定長フィールドの編集（追加、型変更によるバイト数、hex 不正時の赤枠、削除、エンディアン）
- **根拠コード**: `send-form.tsx:100-240`、`application/udp/field-validation.ts:85`（invalid-hex）、`field-display.ts:42-43`
- **推奨テスト名**: `test("fixed-length fields show byte counts and flag invalid hex", ...)`、`test("fixed-length fields are sent in order with the chosen endianness", ...)`
- **モード**: フロントエンドのみ（`fake.args("Send")`）。ワイヤ上のバイト列はフルスタック
- **優先度**: 中

- **不足内容**: 500 件上限で**最古が**捨てられること、新しい順に並ぶこと
- **根拠コード**: `receive.ts:30`（`[msg, ...prev].slice(0, 500)` = 新しい順、末尾の最古を捨てる）。UDP には MQTT のようなオートスクロールは無く、**先頭に最新を積む**のが仕様。既存テストは `getByText(newest)` の可視性だけなので、最新を末尾に積む実装でも（件数が少なければ）通る
- **推奨テスト名**: `test("udp log shows newest first and drops msg-0 at the cap", ...)`
- **モード**: フルスタック
- **優先度**: 中

- **不足内容**: リスン失敗（使用中ポート・範囲外）のエラー表示
- **根拠コード**: `listen-form.tsx:71-73`、`listener_service.go:41-53`
- **推奨テスト名**: `test("listening on a port already in use shows an error", ...)`
- **モード**: フルスタック
- **優先度**: 中

- **不足内容**: 受信ログの Clear・Copy payload
- **根拠コード**: `message-log.tsx:17-19,36-48`
- **モード**: フルスタック（偽バックエンドにイベント注入口を足せばフロントエンドのみ）
- **優先度**: 低

### [K] OpenAPIプロトコル固有フロー

#### テスト済みケース

- `ui/openapi/openapi.spec.ts:30,48` — 編集 → プレビュー反映、クリアで空表示
- `ui/openapi/openapi.spec.ts:69` — 不正 YAML で lint マーク
- （部分的）`integration/openapi/openapi.spec.ts:19` — 無題文書へのペースト。ダイアログ経由のファイル読み込みはネイティブダイアログのため E2E から操作できない

#### 不足ケース

- **不足内容**: 未保存確認ダイアログ（Save & continue / Discard changes / Cancel）
- **根拠コード**: `openapi-provider.tsx:82-89`（無題文書は非空なら未保存扱い）、`:102-112`（`guardSwitch`）、`:222-236`
- **推奨テスト名**: `test("creating a new document with unsaved text asks to save", ...)`、`test("Cancel keeps the current document", ...)`、`test("Discard replaces the document", ...)`
- **モード**: フロントエンドのみ（New → 入力 → New をもう一度）
- **優先度**: 高（データ消失を防ぐ仕組みで、未検証）

- **不足内容**: ウィンドウを閉じるときの未保存確認
- **根拠コード**: `openapi-provider.tsx:187-199`、`infrastructure/app/lifecycle.ts:11-21`（`app:before-close` を受けて `ConfirmQuit`）
- **推奨テスト名**: `test("before-close with unsaved changes shows the dialog and does not quit on Cancel", ...)`
- **モード**: 偽バックエンドの `EventsOn` を実装して `app:before-close` を発火し、`fake.calls("ConfirmQuit")` を見る（フロントエンドのみ）。フルスタックではウィンドウを閉じる操作を Playwright から出せない
- **優先度**: 高

- **不足内容**: 保存（Save ボタン / Ctrl+S）、dirty 表示 `*`、プレビュー表示切替
- **根拠コード**: `openapi/index.tsx:22-31`（Ctrl+S）、`:34-39`（dirty で `*`）、`:70-79`、`:80-94`
- **推奨テスト名**: `test("Ctrl+S on an untitled doc opens save-as and clears the dirty marker", ...)`
- **モード**: フロントエンドのみ（偽バックエンドの `SaveFileAs` がパスを返すよう seed を足す）
- **優先度**: 中

- **不足内容**: ファイルの D&D で開く
- **根拠コード**: `openapi/index.tsx:43-54`（HTML5 drop → `openDroppedFile`、`openapi-provider.tsx:176-184`）
- **推奨テスト名**: `test("dropping a yaml file opens it as an untitled document", ...)`
- **モード**: フロントエンドのみ（`DataTransfer` に `File` を入れて `dispatchEvent("drop")`）
- **優先度**: 中

- **不足内容**: 最近のファイルの選択・削除・並び替え
- **根拠コード**: `openapi-file-node.tsx:52-74`、`openapi-file-tree.tsx:24-61`、`application/openapi/files.ts:25-37`
- **推奨テスト名**: `test("removing a recent file clears the editor when it was active", ...)`
- **モード**: 偽バックエンドの `GetRecents` / `ReadFile` を seed 可能にすればフロントエンドのみ
- **優先度**: 低

### [L] テスト構造の整合性

#### 問題なしの項目

- `waitForTimeout` — 使用ゼロ。デバウンス待ちは `fake.waitForCalls`（`fixtures/ui.ts:41-45`）と `expect.poll`（`integration/http/http.spec.ts:201`）に置き換わっている
- `test.skip` / `test.fixme` — 無し
- `test.only` — 無し。両 config に `forbidOnly: !!process.env.CI`
- baseURL — 両 config が `use.baseURL` を持ち、テストは `page.goto("/")` のみ

#### 不足・問題ケース

- **問題**: ハードコードされたセレクタ（39 箇所）
- **根拠コード**: `locator("#tabpanel-*")`（http.spec.ts 他 13 箇所。`tabs.tsx:48` の実装 ID に依存）、`locator(".cm-editor" / ".cm-content" / ".cm-lint*")`（CodeMirror の内部クラス）、`locator("span").filter({ hasText: /^New Collection$/ })`（sidebar-operations.spec.ts:18,31、form-validation.spec.ts:157）、`locator("#broker-host")`（mqtt.spec.ts:35-38。`getByLabel` で取れる）、`locator('[aria-label="Add"]')`（fixtures/app.ts:67。`getByRole("button", { name: "Add" })` で足りる）、`locator('input[type="checkbox"]')`（form-validation.spec.ts:138）
- **推奨**: `#tabpanel-*` は `getByRole("tabpanel")`、リネームのダブルクリック対象は `app.collection(name).getByText(name)`、`#broker-*` は `getByLabel`。CodeMirror は内部クラス依存が避けられないので `fixtures/app.ts` に `editor()` として 1 か所へ寄せる
- **優先度**: 中

- **問題**: 位置依存のセレクタ（`.nth()` / `.last()` 17 箇所）
- **根拠コード**: `integration/udp/udp.spec.ts:49-51` の `getByRole("button", { name: "Send" }).last()` はタブボタン "Send"（`udp/index.tsx:28-37`）と送信ボタン（`send-form.tsx:261-269`）が同名のため。`integration/http/http.spec.ts:107,122` の `.nth(1)` はリクエスト側とレスポンス側に同名の "Headers" タブがあるため（`response-viewer.tsx:13-17`。[G] 参照）
- **推奨**: UDP のタブを `role="tab"` にするか送信ボタンを `data-testid` で取る
- **優先度**: 低

- **問題**: フルスタックでの状態汚染（UDP リスナーの漏れ）
- **根拠コード**: `integration/udp/udp.spec.ts:85-100,104-132,136-157` は末尾で Stop を押すが `try/finally` も `afterEach` も無い。リスナーは Go 側に残り、次のテストのページ読み込み時に `receive.ts:36-41,69` が復元し、`listen-form.tsx:61` がセッションありで Start を無効にする。1 本のアサーション失敗が後続の UDP テストを連鎖的に落とす。バックエンドのコレクション・ブローカーも 1 回の実行中は蓄積する（名前をテストごとに変えて回避しているが、同名で再試行すると `app.collection(name)` が曖昧になる）
- **推奨**: `afterEach` で `window.go.adapters.UDPHandler.GetListeners()` → `StopListen` を呼んで掃除する
- **優先度**: 高

- **問題**: 実質アサーションの無いテスト
- **根拠コード**: `backend-integration.spec.ts:37-53`（M-1 の 3 本）が確かめているのは見出しの表示だけ。HTTP と UDP の 2 本はテスト本体に `expect` が無く `switchTo` 内の見出し確認のみ、MQTT の 1 本（`:44-49`）は "Brokers" の `toBeVisible()` を 1 つ持つ。見出し "Collections" / "Brokers" / "Targets" は静的文字列（`collection-tree.tsx:205`、`profile-list.tsx:53`）で、バインディングが失敗しても表示されるため、テスト名が主張する「バインディングが正常動作」を検証できていない。ほかに `smoke.spec.ts:3`（`index.html:6` の静的タイトル）
- **推奨**: M-1 は seed 済みデータ（フルスタックでは事前に作った項目）が一覧に出ることを検証するか削除する
- **優先度**: 中

- **問題**（部分的）: フルスタックの前提条件確認が HTML の配信までしか見ていない
- **根拠コード**: `global-setup.ts:14-17` は `document.title` が /Wirexa/ になるのを待つが、タイトルは `index.html:6` の静的値なので HTML が返った時点で成立する。コメントの「アプリが完全に表示されるまで」とずれている
- **推奨**: `getByRole("button", { name: "MQTT" })` の可視化、または `window.go.adapters` の存在を待つ
- **優先度**: 低

- **問題**（表外）: expect の個別 timeout
- **根拠コード**: `ui/openapi/openapi.spec.ts:41-82` の `timeout: 3000` は config の既定 10 秒（`playwright.config.ts:13`「個々の expect に timeout を撒かずここに集約する」）より**短く**、CI の遅いランナーでフレークの原因になる
- **推奨**: 削除して既定値に任せる
- **優先度**: 低

### [M] バックエンド統合テスト

> **前提**: フルスタック用の `playwright.integration.config.ts` は既にあり、`wails dev` と `vite preview` を webServer として起動し、`APPDATA` を一時ディレクトリへ隔離している（`playwright.integration.config.ts:22-37,72-101`）。ただし **CI では走らない**（Windows / ローカル専用）。
>
> 隔離の範囲には次の制限がある。
>
> - 設定するのは `APPDATA` だけ（`playwright.integration.config.ts:88`）。`os.UserConfigDir()` 配下の設定データとログは隔離されるが、HTTP セッション領域（`app.go:110-114` の `os.UserCacheDir()`、Windows では `%LOCALAPPDATA%`）は実ユーザーの領域を使う
> - `WIREXA_E2E_REUSE=1`（`playwright.integration.config.ts:38-39`）で既存の `wails dev` を使い回すと、そのプロセスに新しい `APPDATA` は渡らない。開始時の固定パス削除（`:32-36`）は走るが、アプリはその場所を見ていないので「毎回まっさらな状態から始まる」は成り立たない
>
> また、ここでの「フルスタック」は Playwright が起動した Chromium を Wails の開発 URL（`playwright.integration.config.ts:56,66-71`、`global-setup.ts:9-14`）へつなぐ形で、実バックエンドのバインディングとイベントは通るが、`main.go` が作るデスクトップウィンドウ（WebView2）自体は操作しない。ウィンドウ操作・ネイティブダイアログ・ウィンドウ状態の保存はこのスイートの検証範囲外。

#### テスト済みケース

- 各フルスタック spec — バインディング呼び出しは `CreateCollection`・`SaveProfile`・`SaveTarget`・`SendRequest`・`Send`・`StartListen` などの経由で実際に通っている（M-1 の専用テストは実質無検証、[L] 参照）
- `integration/http/http.spec.ts:77-236` — 実 HTTP サーバーへの送信とレスポンス表示（`internal/infrastructure/http/net_client.go`）
- `integration/udp/udp.spec.ts:85` — `udp:message` イベントの受信と UI 反映（`internal/infrastructure/wails_emitter.go`）
- （部分的）`backend-integration.spec.ts:61,76,90` — コレクション・ブローカー・ターゲットの「永続化」

#### 不足ケース

- **不足内容**: コレクション・ブローカー・ターゲットが**ディスクに**保存されること
- **根拠コード**: M-2〜M-4 は `page.reload()` しかしない。リロードで再取得するのは Go プロセスのメモリ上の状態で、アプリは再起動していない。保存がエラーを返す壊れ方なら、キャッシュの更新は保存成功後なので（`internal/application/store/cached_store.go:83-88`、`internal/application/http/collection_service.go:219-224`）作成自体が失敗しテストも落ちる。一方、書き込みは成功するが内容が誤っている（保存形式の変化、フィールドの欠落など）場合や、ディスクからの再読込（`JSONStore` の Load）が壊れた場合はリロードでは検出できない（対象は `internal/infrastructure/http/collection_repository.go`、`internal/infrastructure/mqtt/profile_repository.go`、`internal/infrastructure/udp/target_repository.go`、いずれも `JSONStore` 経由の atomic write）
- **推奨テスト名**: `test("created collection is written under APPDATA/Wirexa/collections", ...)`
- **モード**: フルスタック。アプリ再起動は webServer の都合で難しいので、テストから `%TEMP%/wirexa-e2e-appdata/Wirexa/` 配下の JSON を `fs` で直接読んで確認する
- **優先度**: 高

- **不足内容**: サイドバーレイアウト（`sidebar_layout.json`）の永続化
- **根拠コード**: M-8（`backend-integration.spec.ts:103-134`）は「Sidebar layout persistence」と名乗っているが、並び替えているのは**ブローカー**で、その順序は localStorage の `mqtt:profileOrder` に保存される（`application/mqtt/profiles.ts:64-68`、`local-storage.ts:13,110-113`）。バックエンドの `internal/infrastructure/http/sidebar_layout_repository.go`（HTTP コレクションとルート直下アイテムの並び）は一度も E2E を通っていない
- **推奨テスト名**: `test("reordering collections persists sidebar_layout.json", ...)`。既存 M-8 は `mqtt:profileOrder` のテストとして改名し、観点 F へ移す
- **モード**: フルスタック（D&D はマウス方式なので `page.mouse` を使う。[C] 参照）
- **優先度**: 高

- **不足内容**: バックエンドのエラーが UI に表示される
- **根拠コード**: `internal/domain/errors.go` の `ValidationError` / `NotFoundError`。UI に出る経路の例: UDP ターゲットの範囲外ポート（`internal/domain/udp/types.go:83-85` → `targets.ts:40-45` のトースト）、使用中ポートでのリスン（`listener_service.go:51-53` → `listen-form.tsx:71-73`）。既存の `response-error` 検証（`integration/http/http.spec.ts:164`）はフロントエンドで生成するエラー（`request.ts:213-218`）で、バックエンド由来ではない。OpenAPI の拒否（`integration/openapi/openapi.spec.ts:53`）もバインディングの reject を見るだけで UI 表示は見ていない
- **推奨テスト名**: `test("backend validation error for udp target is shown as a toast", ...)`
- **モード**: フルスタック
- **優先度**: 高

- **不足内容**: MQTT のイベント（`mqtt:connected` / `mqtt:disconnected` / `mqtt:connection-lost` / `mqtt:connection-failed` / `mqtt:message`）
- **根拠コード**: `internal/domain/events.go:10-14`、`connections.ts:244-300`（`mqtt:disconnected` は接続状態を切断にし、`mqtt:connection-failed` は "MQTT connection failed" を通知して状態を戻す）。Go 側の配信は `internal/integration/mqtt_test.go`（`TestMQTT_LifecycleEvents` など）が CI で検証済みなので、不足しているのは UI への反映
- **推奨テスト名**: [H] の接続・受信テストで兼ねる
- **モード**: フルスタック
- **優先度**: 高（[H] と重複）

---

## 最優先で追加すべきテスト TOP5

1. **MQTT の接続・購読・受信・Publish（観点 H / M）** — フルスタックに MQTT の spec が無く、UI モードの偽バックエンドは接続を必ず失敗させる。既定で開くプロトコルの主経路がどちらのモードでも一度も通っていない
   - 根拠: `e2e/integration/` に mqtt ディレクトリが無い、`install.ts:545-547`、`broker-manager.tsx:116-136`、`connections.ts:244-252`
2. **HTTP ツリーの D&D とバックエンドの sidebar_layout 永続化（観点 C / M）** — M-8 が検証しているのは localStorage のブローカー順で、`sidebar_layout.json` と `MoveItem` 系は未検証。アイテムが別のコレクションへ移る操作なので、壊れたときの被害が大きい
   - 根拠: `use-tree-drag-drop.ts`、`collection-tree.tsx:132-169`、`profiles.ts:64-68`
3. **OpenAPI の未保存確認（切り替え時・終了時）（観点 K）** — データ消失を防ぐ仕組みが未検証。切り替え時の確認は UI モードで今すぐ書ける。終了時の確認は偽バックエンドの `EventsOn` 実装が要る
   - 根拠: `openapi-provider.tsx:82-112,187-199,222-236`
4. **UI モードの UDP spec 新設（観点 J / D）** — CI で UDP の主要フローが一度も通らない。あわせて偽バックエンドの `SaveTarget` / `StartListen` に Go と同じ検証を入れ、範囲外ポートでのエラー表示を検証する
   - 根拠: `e2e/ui/` に udp ディレクトリが無い、`install.ts:477-505` と `internal/domain/udp/types.go:79-87` の食い違い
5. **入力した headers / params / auth / settings が送信に反映されること（観点 I）** — 入力 UI のテストは多いが、`SendRequest` に届く値を 1 つも見ていない。`fake.args("SendRequest")` で安く書ける
   - 根拠: `request.ts:224-236`、`http.spec.ts:361-398`

---

## フルスタックE2E移行ロードマップ

フルスタックの土台（config・HTTP サーバー・UDP ソケット・APPDATA 隔離）はできている。残りは次のとおり。

1. **Playwright 設定の追加** — 済（`playwright.integration.config.ts`）
2. **テスト用 HTTP サーバーのセットアップ** — 済（`e2e/fixtures/http-server.ts`、ポート 0 で TOCTOU なし）。`/echo` を拡張し、受信ヘッダーも返すと [I] の送信値検証に使える
3. **テスト用 MQTT ブローカーのセットアップ** — 未。選択肢は 2 つ
   - Go の mochi-mqtt（`internal/integration/mqtt_test.go` と `internal/infrastructure/mqtt/paho_client_test.go` で既に依存している）を起動する小さな `tools/` コマンドを作り、webServer として立てる。Go 側と同じブローカー実装で揃う
   - Node の組み込みブローカー（aedes など）を `e2e/fixtures/mqtt-broker.ts` にする。依存が増えるが、`http-server.ts` と同じ形で書ける
4. **データリセット機構** — 実行単位では済（`APPDATA` を実行開始時に削除して作り直す。ただし `WIREXA_E2E_REUSE=1` で既存サーバーを使い回すときは効かず、`os.UserCacheDir()` の HTTP セッション領域は対象外。[M] の前提参照）。テスト単位では未。少なくとも UDP リスナーは `afterEach` で `StopListen` する（[L] 参照）
5. **ディスク永続化の検証** — 未。テストから `%TEMP%/wirexa-e2e-appdata/Wirexa/` 配下の JSON を読み、`storedXxx` の形（各パッケージの `testdata/*.golden.json`）で保存されていることを確かめる
6. **偽バックエンドのイベント注入口**（フルスタック移行と並行）— `install.ts:580-584` の `EventsOn` を実装し、`window.__wirexaFake.emit(name, data)` を生やす。`mqtt:message`・`udp:message`・`app:before-close` を UI モードで流せるようになり、[H] の上限・オートスクロール、[J] の Clear / Copy、[K] の終了確認を CI で検証できる

---

## 総合評価

**カバレッジ概算**: 48%（ユーザーフロー 85 件中、済 35 件＋部分的 11 件を半分として 40.5 件）

**信頼度**: 中

HTTP のリクエスト編集と送信は UI・フルスタックの両方でよく押さえられており、偽バックエンドの seed / `fake.args` / `waitForCalls` による検証スタイルも堅い。一方で MQTT（既定プロトコル）の接続系、UDP の UI モード、OpenAPI の保存と未保存確認、ツリーの D&D は未検証。

**主なリスク**:

- MQTT の接続・購読・受信の UI 連携が壊れても、フロントエンドの E2E（UI・フルスタックとも）は検出しない。バックエンド単体の接続・購読・Publish・受信は Go 統合テスト（`internal/integration/mqtt_test.go:357` ほか、CI の `test-go-integration` ジョブ）が実ブローカーで検証している
- HTTP ツリーの D&D の回帰でリクエストが別のコレクションに移る、または消える
- OpenAPI の未保存確認が効かなくなり、編集内容を失う
- 偽バックエンドが Go の検証を再現していない箇所（UDP ターゲット・リスン）で、UI モードのテストが実アプリと違う挙動を前提に通ってしまう
- UDP のフルスタックテストが 1 本落ちると、残ったリスナーが後続を連鎖的に落とす

**推奨アクション**:

1. 偽バックエンドを拡張する: `EventsOn` / `emit` の実装、`Connect` 成功の seed、`SaveTarget` / `StartListen` の検証、`GetRecents` / `SaveFileAs` の seed
2. UI モードで書けるものから追加する: OpenAPI の未保存確認、HTTP ツリーの D&D、送信値の検証、UDP spec、エラートースト
3. フルスタックに MQTT ブローカーを用意し、`e2e/integration/mqtt/mqtt.spec.ts` を新設する
4. フルスタックの M-2〜M-4 に JSON ファイルの直接確認を足し、M-8 を HTTP の `sidebar_layout.json` の検証に置き換える
5. [L] の構造問題を直す: UDP リスナーの `afterEach` 掃除、M-1 の実質無検証テスト、`#tabpanel-*` などのセレクタ、openapi.spec.ts の短い timeout

---

## Playwright 実装ガイド（不足ケース追加時の参考）

このアプリ固有の実装上の注意点。

### セレクタの指定方針

- プロトコル切り替えボタン: `page.getByRole("button", { name: "MQTT", exact: true })`（`fixtures/app.ts:14-16` の `app.protocolButton`）。`exact` が無いと "MQTT" が他のボタン名に部分一致しうる
- リネーム入力: `page.getByTestId("rename-input")`（`rename-input.tsx:14`。`app.renameInput`）。`getByRole("textbox")` はリクエスト編集欄とも一致するので使わない
- ConfirmDialog の確認ボタン: 既定のラベルは **"Delete"**（`confirm-dialog.tsx:58`、`app.confirmDelete()`）。OpenAPI の未保存確認は "Save & continue" / "Discard changes" / "Cancel"（`openapi-provider.tsx:224-233`）
- サイドバーの行アクション（Add folder / Add request / Delete collection など）はホバーで出る。`app.collection(name).hover()` のあと `getByRole("button", { name })` で押す（`fixtures/app.ts:84-87`）
- ブローカー／ターゲット行は `role="button"` の `div`（`profile-list.tsx:76-93`）。行のクリックはアクションボタンに吸われることがあるので、`integration/udp/udp.spec.ts:16-25` のように focus + Enter で選ぶと安定する
- HTTP パネルの "Headers" / "Body" タブはリクエスト側とレスポンス側で同名。`page.locator("#tabpanel-body")` ではなく、対象のパネル（リクエスト編集エリア）でスコープしてから `getByRole("tab")` を使う

### ドラッグ&ドロップ

- ブローカー・ターゲット・プリセット: ネイティブ HTML5 DnD（`list-reorder.tsx:14-56`）。`locator.dragTo()` は Chromium でネイティブ DnD イベントを発火するので、そのまま使える（ドロップ位置はターゲット中央。上下半分で挿入位置が決まる `list-reorder.tsx:32-37` に合わせるなら `targetPosition` で指定する）。既存の `backend-integration.spec.ts:13-19` の `dragRowOnto` は共有 `DataTransfer` で dragstart → dragover → drop → dragend を手動 dispatch する方式で、`clientY` が 0 になるため常にターゲットの直前に挿入される。同ファイル 9-10 行目のコメント「dragTo はネイティブ DnD イベントを発火しない」は事実と異なる
- HTTP ツリー・OpenAPI ファイル: マウスイベント方式（`use-long-press-drag.ts`）。`page.mouse.move(src)` → `down()` → 6px 以上 `move()` → ドロップ先の挿入ゾーン（`data-drop-zone` 属性、`tree-item-node.tsx:14-18`）へ `move()` → `up()`。ドロップ先は `elementFromPoint` で決まるので、要素の中心座標を `boundingBox()` から取る

### 非同期待機

- Wails はバインディング呼び出しに HTTP API を使わないため、`page.waitForResponse()` はバインディング呼び出しの待機に使えない（フロントエンドのみ・フルスタック両モードとも）
- フロントエンドのみモードでは、バインディングの完了を `fake.waitForCalls("UpdateRequest")`（`fixtures/ui.ts:41-45`）で待つ。自動保存のデバウンス（`request.ts:409` の 500ms）を固定 sleep で待たない
- フルスタックモードでは、バックエンドの状態を `page.evaluate` で `window.go.adapters.*` から読んで `expect.poll` で待つ（`integration/http/http.spec.ts:49-69,201`）
- バックエンドからのデータは `window.runtime.EventsOn` 経由でフロントエンドに届き、UI が更新される。Playwright からはその DOM 変化を `await expect(locator).toBeVisible()` / `toHaveText()` 等で待つ。MQTT メッセージは rAF でまとめて反映される（`connections.ts:248-251`）ので、送信直後の同期チェックはしない
- フルスタックで実サーバーの受信を確かめるときは、ブラウザ側ではなく Node 側のフィクスチャ（`udp-server.ts` の `firstMessage` など）で待つ

### テスト間のリセット（フロントエンドのみモード）

- リセットは不要。偽バックエンドの状態は sessionStorage にあり、Playwright はテストごとに新しいコンテキストを作る（`install.ts:1-7`）
- `localStorage` は `fixtures/ui.ts:63-75` がコンテキストの初回ロードでだけ消す。`page.reload()` で復元を見るテストはそのまま書ける
- 初期データは UI で作らず `test.use({ seed: {...} })` で仕込む（`request-persistence.spec.ts:5-11`）

### テスト間のリセット（フルスタックモード）

- データディレクトリは実行開始時に作り直される（`playwright.integration.config.ts:32-36`）。テスト間では共有されるので、名前をテストごとに一意にする
- Go 側に残る状態（UDP リスナー、MQTT 接続）はページのリロードでは消えず、次のテストで復元される（`receive.ts:36-41`、`connections.ts:316-361`）。`afterEach` で `StopListen` / `Disconnect` を呼んで掃除する
- バックエンド全体をリセットする RPC は無い。必要になったら、テスト専用バインディングを本番に足すのではなく、フィクスチャから `window.go.adapters.*` の既存 API（`DeleteCollection` など）を呼んで消す
