# E2E テストレビュー

生成日時: 2026-10-04
対象: MQTT（`presentation/components/mqtt/`、`sidebar/broker-tree.tsx`・`profile-list.tsx`、`presentation/providers/mqtt-provider.tsx`、`application/mqtt/`、`internal/adapters/mqtt_handler.go`）。観点 A〜G・L〜N は MQTT の範囲だけを見た。
スイート: UI e2e（`ui/mqtt/` の spec 4 本・テスト 86 件、ほかに `ui/common/` の MQTT 関連 11 件）/ フルスタック e2e（`integration/mqtt/mqtt.spec.ts` 18 件、ほかに `integration/common/backend-integration.spec.ts` の MQTT 分 4 件）

テストは実行していない。本体コード・テスト基盤・spec を静的に読んだ結果である。

---

## ユーザーフロー一覧

パスは `frontend/src/` からの相対。spec は `frontend/e2e/` からの相対。

| フロー | コンポーネント（ファイル:行） | UI e2e | フルスタック |
| --- | --- | --- | --- |
| 起動時の空状態（`No brokers yet`・`No active connection…`） | `presentation/components/mqtt/index.tsx:17-25`、`sidebar/profile-list.tsx:138-140` | 済（ui/mqtt/profiles.spec.ts:20、ui/common/initial-state.spec.ts:33） | 対象外 |
| 起動時の読み込み（`GetProfiles` → `GetConnections` → 最後のブローカーを選ぶ） | `presentation/providers/mqtt-provider.tsx:118-134`、`application/mqtt/connections.ts:239-307` | 済（ui/mqtt/profiles.spec.ts:625, 964, 988, 1026） | 済（integration/mqtt/mqtt.spec.ts:144） |
| ブローカーの新規作成（New Broker → Save） | `sidebar/broker-tree.tsx:72-77`、`mqtt/broker-settings-dialog.tsx:61-64` | 済（ui/mqtt/mqtt.spec.ts:10） | 済（integration/common/backend-integration.spec.ts:157） |
| ダイアログの入力検証（名前・ホスト・ポート） | `application/mqtt/profile-validation.ts:16-36`、`mqtt/broker-settings-dialog.tsx:48-54` | 済（ui/mqtt/mqtt.spec.ts:36, 52、ui/mqtt/profiles.spec.ts:35, 69） | 対象外 |
| ダイアログでの編集（スキーム・クライアント ID・認証・TLS を含む） | `mqtt/broker-settings-dialog.tsx:100-185` | 済（ui/mqtt/profiles.spec.ts:89, 128, 144, 163, 224, 268） | 部分的（integration/common/backend-integration.spec.ts:181。名前とホストの変更だけ。スキーム・クライアント ID・認証・TLS は見ていない） |
| ダイアログを閉じる（Cancel・Escape・背景クリック）とフォーカストラップ | `mqtt/broker-settings-dialog.tsx:71-95` | 済（ui/common/keyboard-accessibility.spec.ts:75, 108） | 対象外 |
| 接続バーでの URL 編集（入力のたびに保存、無効な値は保存しない） | `mqtt/broker-manager.tsx:57-79`、`application/mqtt/connections.ts:320-335` | 済（ui/mqtt/profiles.spec.ts:163, 520, 599, 846） | 対象外 |
| ブローカーの選択（クリック・Enter）と複数ブローカーの切り替え | `sidebar/broker-tree.tsx:52-60`、`sidebar/profile-list.tsx:85-92` | 部分的（ui/mqtt/profiles.spec.ts:814, 925、ui/common/keyboard-accessibility.spec.ts:171。接続中のブローカー 2 つの切り替えは無い） | 済（integration/mqtt/mqtt.spec.ts:348） |
| ブローカーの並び替え | `sidebar/profile-list.tsx:46-48, 84`、`application/mqtt/profiles.ts:86-88` | 済（ui/mqtt/mqtt.spec.ts:215） | 対象外 |
| ブローカーの削除（確認・Cancel・接続中・失敗） | `sidebar/broker-tree.tsx:93-101`、`sidebar/profile-list.tsx:144-158` | 部分的（ui/mqtt/mqtt.spec.ts:81、ui/mqtt/profiles.spec.ts:687, 873, 908。確立待ちのブローカーの削除は無い） | 部分的（integration/common/backend-integration.spec.ts:181。未接続のブローカーの削除とファイルの消去だけ。接続中の削除は見ていない。Cancel・失敗は対象外） |
| Connect / Disconnect | `mqtt/broker-manager.tsx:146-181`、`application/mqtt/connections.ts:407-496` | 済（ui/mqtt/messages.spec.ts:730、ui/mqtt/profiles.spec.ts:1061） | 済（integration/mqtt/mqtt.spec.ts:51, 72） |
| Save & Connect（新規・接続中の編集） | `sidebar/broker-tree.tsx:79-90` | 済（ui/mqtt/profiles.spec.ts:224, 424, 743） | 部分的（integration/mqtt/mqtt.spec.ts:88。新規だけ。接続中の編集からの張り直しは見ていない） |
| 確立待ちの表示と中止（`Connecting…`）、二重 Connect の防止 | `mqtt/broker-manager.tsx:52-53, 163-170`、`application/mqtt/connections.ts:370-405` | 済（ui/mqtt/profiles.spec.ts:334, 366, 1090, 1124） | 対象外 |
| 接続失敗・拒否・切断失敗の通知 | `application/mqtt/connections.ts:209-218, 396-401, 411-417` | 済（ui/mqtt/profiles.spec.ts:717, 785, 1061） | 済（integration/mqtt/mqtt.spec.ts:106。届かない宛先への接続失敗だけ。RPC の拒否と切断の失敗は対象外） |
| 接続が生きている間の編集のロック | `sidebar/broker-tree.tsx:63-68`、`mqtt/broker-settings-dialog.tsx:187-212` | 済（ui/mqtt/profiles.spec.ts:334, 395, 424, 459） | 対象外 |
| 自動再接続（`mqtt:connection-lost` → `mqtt:connected`）と中止 | `application/mqtt/connections.ts:182-207` | 済（ui/mqtt/messages.spec.ts:823、ui/mqtt/profiles.spec.ts:489） | 部分的（integration/mqtt/mqtt.spec.ts:430。自動復旧と接続 ID の維持だけ。中止は見ていない） |
| 購読（Subscribe ボタン・Enter・QoS・重複・空白） | `mqtt/panels/subscriptions-panel.tsx:42-65`、`application/mqtt/subscriptions.ts:77-122` | 済（ui/mqtt/messages.spec.ts:74, 113, 142, 157、ui/mqtt/mqtt.spec.ts:123, 137, 160） | 部分的（integration/mqtt/mqtt.spec.ts:175, 214。ボタンでの QoS 0 の購読と不正なフィルターだけ。QoS 1・2 の購読は見ていない。Enter・重複・空白は対象外） |
| 購読解除 | `mqtt/panels/subscriptions-panel.tsx:103-111`、`application/mqtt/subscriptions.ts:124-159` | 部分的（ui/mqtt/messages.spec.ts:74, 936。切断後の解除は無い） | 未テスト |
| ミュート / 共有購読 / 外された購読の通知 | `application/mqtt/subscriptions.ts:47-74, 227-236`、`application/mqtt/message-buffer.ts:99-118` | 済（ui/mqtt/messages.spec.ts:267, 306, 336, 989） | 部分的（integration/mqtt/mqtt.spec.ts:490。張り直しで外された購読の通知だけ。共有購読は見ていない。ミュートは対象外） |
| メッセージの受信・一覧・詳細・コピー・バイナリ | `mqtt/panels/messages-panel.tsx:109-163`、`mqtt/panels/message-detail.tsx:22-72` | 済（ui/mqtt/messages.spec.ts:192, 240, 485, 520） | 済（integration/mqtt/mqtt.spec.ts:175, 296。コピーは対象外） |
| トピックでの絞り込み・Clear | `mqtt/panels/messages-panel.tsx:60-91`、`application/mqtt/messages.ts:14-57, 84-88, 122-130` | 済（ui/mqtt/messages.spec.ts:360, 387, 430, 860） | 対象外 |
| Auto（最新メッセージへの追従） | `mqtt/panels/messages-panel.tsx:47-53`、`application/mqtt/messages.ts:93-107` | 済（ui/mqtt/messages.spec.ts:546, 575） | 対象外 |
| メッセージ上限（`MQTT_MAX_MESSAGES` = 5000、1 フレームのバッファ 5000） | `application/mqtt/message-buffer.ts:11, 122-125, 131-136` | 済（ui/mqtt/messages.spec.ts:620, 643） | 対象外 |
| パブリッシュ（トピック・QoS・retain・ペイロード、未接続では無効） | `mqtt/publish-tab.tsx:179-229`、`application/mqtt/publish.ts:17-39` | 済（ui/mqtt/messages.spec.ts:667, 705、ui/mqtt/publish.spec.ts:48） | 済（integration/mqtt/mqtt.spec.ts:195, 235, 261） |
| プリセット（追加・選択・改名・削除・並び替え・復元） | `mqtt/publish-tab.tsx:28-177`、`application/mqtt/presets.ts:14-93` | 済（ui/mqtt/publish.spec.ts:59, 109, 162, 179, 222、ui/mqtt/mqtt.spec.ts:185） | 対象外 |
| トピックスキャン（開始・停止・一覧からの購読・上限 `MQTT_MAX_TOPICS` = 500） | `mqtt/panels/broker-topics-panel.tsx:38-97`、`application/mqtt/subscriptions.ts:161-225`、`application/mqtt/message-buffer.ts:55-87` | 部分的（ui/mqtt/mqtt.spec.ts:245, 307, 338, 371, 406。再スキャンで一覧が消えることは見ていない） | 済（integration/mqtt/mqtt.spec.ts:389, 464。上限は対象外） |
| Subscribe / Publish タブ | `mqtt/tab-bar.tsx:12-24`、`mqtt/index.tsx:29-47` | 済（ui/mqtt/mqtt.spec.ts:100、ui/common/keyboard-accessibility.spec.ts:267） | 対象外 |
| 別のプロトコルを表示している間の受信 | `application/mqtt/connections.ts:163`（リスナーは provider に常駐） | 済（ui/mqtt/messages.spec.ts:900、ui/common/protocol-switching.spec.ts:56） | 対象外 |
| リロード後の復元（接続・購読・スキャンの停止・確立待ち） | `application/mqtt/connections.ts:239-307` | 済（ui/mqtt/messages.spec.ts:782、ui/mqtt/profiles.spec.ts:334） | 部分的（integration/mqtt/mqtt.spec.ts:144。確立済みの接続と購読の維持だけ。スキャンの停止と確立待ちは見ていない） |

> 各列の値: `済（ファイル名:行番号）` / `部分的（ファイル名:行番号）` / `未テスト` / `対象外`（そのスイートで検証する意味が無い）。括弧の中に範囲を書いた行は、そのスイートが見ているのはその範囲だけである。

---

## 既存テストの概要

| spec | テスト数 | 検証している内容（要約） |
| --- | --- | --- |
| ui/mqtt/mqtt.spec.ts | 15 | ブローカーの作成と削除（`SaveProfile`・`DeleteProfile` の引数）、ダイアログの名前・ポート・ホストの検証、Subscribe / Publish タブの切り替え、未接続での購読の無効化、QoS の選択、retain、並び順の `mqtt:profileOrder` への保存と復元。トピックスキャン（開始・停止・`mqtt:scan-topic`・`mqtt:scan-stopped`・一覧からの購読・500 件の上限・未接続での失敗・開始中の停止）。 |
| ui/mqtt/profiles.spec.ts | 36 | 空状態、ダイアログの検証（ポート 0・65535・65536、空白のホストと名前）、編集（ダイアログと接続バー、スキーム変更時の既定ポート、認証・TLS が `SaveProfile`・`Connect` に届く）、接続が生きている間の編集のロックと `Connecting…` での中止、Save & Connect での張り直し、接続バーの検証、保存・削除・接続・切断の失敗、起動時の `GetProfiles`・`GetConnections` の失敗、最後に選んだブローカーの復元、プロファイルが消えた接続の復元、応答を待つ間の切り替えと二重 Connect。 |
| ui/mqtt/messages.spec.ts | 28 | 接続済みでの購読と解除（`Subscribe`・`Unsubscribe` の引数）、不正なフィルターのエラー、Enter での購読、重複と空白、受信 → 一覧 → 詳細 → コピー、バイナリの hex 表示、ミュート、共有購読（`$share/`・`$queue/`）、外された購読の通知、トピックでの絞り込みと Clear、行のレイアウト、Auto、5000 件の上限（一覧とバッファ）、Publish の引数と検証エラー、切断と再接続での購読の張り直し、リロード後の復元、自動再接続、別プロトコル表示中の受信、解除の失敗、張り直しで拒否された購読、Unicode とマークアップ。 |
| ui/mqtt/publish.spec.ts | 7 | 未接続での Publish の無効化、プリセットの追加・選択・改名・削除、並び替えと `mqtt:presets` の復元、Enter / Space での選択、Escape での改名の取り消しと空の名前、選択中のプリセットを消したあとの入力、壊れた `mqtt:*` の保存値。 |
| ui/common/（MQTT 関連） | 11 | 起動時に MQTT が選ばれている（initial-state.spec.ts:3）、HTTP へ切り替えると MQTT パネルが隠れる（:11）、空状態（:33）、プロトコルを順に切り替えたときの MQTT ボタンの `aria-pressed`（protocol-switching.spec.ts:3）、サイドバーの見出し `Brokers`（:40）、ブローカーダイアログの Escape・背景クリック（keyboard-accessibility.spec.ts:75）とフォーカストラップ（:108）、Enter でのブローカー選択（:171）、タブのロールと id の重複（:267）、プロトコル切り替え後も MQTT パネルが残る（protocol-switching.spec.ts:56, 72）。 |
| integration/mqtt/mqtt.spec.ts | 18 | 実ブローカーへの接続・切断・Save & Connect、届かない宛先での失敗、TLS で使えないスキームの拒否、リロード後の接続と購読の維持、受信・ループバック・retained・バイナリ、不正なフィルターとワイルドカード付きトピックの検証エラー、手動の再接続での張り直し、ブローカーごとの購読の独立、スキャンと一覧からの購読、ブローカーによる切断からの自動復旧、スキャンの停止、張り直しで拒否された購読。 |
| integration/common/backend-integration.spec.ts（MQTT 分） | 4 | M-1: バインディングで保存したプロファイルが一覧に出る（:52）。M-3: リロード後も残る（:149）、`mqtt-profiles/<id>.json` の形が golden と一致する（:157）、編集は同じファイルを書き換え、削除でファイルが消える（:181）。 |

---

## サマリー

| 観点 | チェック項目数 | 済 | 部分的 | 不足 |
| --- | --- | --- | --- | --- |
| [A] 初期状態 | 3 | 2 | 1 | 0 |
| [B] プロトコル切り替え | 2 | 2 | 0 | 0 |
| [C] サイドバー操作 | 5 | 4 | 1 | 0 |
| [D] 入力・バリデーション・境界値 | 6 | 5 | 1 | 0 |
| [E] 非同期操作 | 5 | 4 | 1 | 0 |
| [F] 画面側の永続化 | 4 | 4 | 0 | 0 |
| [G] キーボード操作・アクセシビリティ | 4 | 4 | 0 | 0 |
| [H] MQTT 固有フロー | 9 | 6 | 3 | 0 |
| [I] HTTP 固有フロー | 0 | — | — | — |
| [J] UDP 固有フロー | 0 | — | — | — |
| [K] OpenAPI 固有フロー | 0 | — | — | — |
| [L] テスト構造 | 9 | 6 | 2 | 1 |
| [M] バックエンド統合 | 5 | 3 | 2 | 0 |
| [N] 偽バックエンドの忠実さ | 6 | 3 | 2 | 1 |

> チェック項目数は、レビューの手順（`.claude/skills/e2e-test-review/SKILL.md`）にある各観点の表の行数。該当なしの行は数えない。下の観点別詳細の箇条書きの数とは一致しない。観点 L では「済」は問題が見つからなかったことを表す。

---

## 観点別詳細

### [A] 初期状態

テーマの初期状態と復元は該当なし（プロトコルに依らないので `common` の範囲）。

#### テスト済み

- `ui/common/initial-state.spec.ts:3` — 起動時に MQTT のボタンが `aria-pressed="true"`
- `ui/mqtt/profiles.spec.ts:20`、`ui/common/initial-state.spec.ts:33` — `No brokers yet` と `No active connection…`、タブが出ない
- `ui/mqtt/profiles.spec.ts:631` — `GetProfiles` の失敗でトーストが 1 つ出る、`mqtt:lastActiveProfileId` を消さない、`GetConnections` を呼ばない、初期プリセットは作る
- `ui/mqtt/profiles.spec.ts:964` — `GetConnections` の失敗では通知せず、全ブローカーを未接続で並べる
- `ui/mqtt/profiles.spec.ts:988, 1026` — 無くなったブローカーの ID は保存値から消す、プロファイルが消えた接続は接続済みのタブとして復元する
- `ui/mqtt/publish.spec.ts:66` — 初回起動で `no name` のプリセットが 1 件できて選ばれている

#### 不足

- **不足内容**: `GetProfiles` に失敗した起動のあとでブローカーを作ったときの `mqtt:profileOrder`。読み込みに失敗すると一覧は空のままで、そこへ 1 件保存すると並び順が「新しい 1 件だけ」で上書きされ、次の起動で既存のブローカーの並びが ID 順に戻る（コードを読んだ限りの挙動。実行しては確かめていない）。既存の spec:631 が守っているのは `mqtt:lastActiveProfileId` だけ。
- **根拠コード**: `application/mqtt/profiles.ts:40-47`（`commit` は一覧の ID をそのまま保存する）、`:70-74`（保存結果を一覧に足して `commit`）、`application/shared/order.ts:1-11`（並び順に無い ID は末尾）
- **スイート**: UI e2e
- **推奨テスト名**: `test("saving a broker after a failed load keeps the stored broker order", ...)`
- **追加先**: `ui/mqtt/profiles.spec.ts` の `when loading brokers fails on startup`
- **優先度**: 中（失うのは並び順だけ。Go の `GetProfiles` はエラーを返さないので、起きるのは Wails のランタイムが不調なときに限られる。テストを書くと今の実装では落ちるはずなので、どう直すかを先に決める必要がある）

### [B] プロトコル切り替え

選択中のプロトコルの再クリック（サイドバーの開閉）は該当なし（`common` の範囲）。

#### テスト済み

- `ui/common/protocol-switching.spec.ts:40` — MQTT ではサイドバーの見出しが `Brokers`
- `ui/common/protocol-switching.spec.ts:56, 72` — 別のプロトコルへ切り替えても MQTT パネルは `display: none` で残る
- `ui/mqtt/messages.spec.ts:900` — HTTP を表示している間に届いたメッセージが、戻ったあと一覧に出る。接続・購読・入力中のトピックも残り、`Connect` は増えない

#### 不足

なし。

### [C] サイドバー操作

コレクション・フォルダの開閉と `__root__` は該当なし（HTTP のツリーだけの機能）。サイドバーそのものの開閉は観点 B のとおり `common` の範囲。リネームはブローカーではダイアログでの編集に当たる（インラインのリネームは実装に無い）。

#### テスト済み

- `ui/mqtt/mqtt.spec.ts:10` — 追加。ID は空で送り、作ったブローカーが選ばれる
- `ui/mqtt/profiles.spec.ts:89, 144` — ダイアログでの名前の変更と Cancel
- `ui/mqtt/mqtt.spec.ts:81`、`ui/mqtt/profiles.spec.ts:908` — 削除の確認と Cancel。最後の 1 件を消すと空状態に戻る
- `ui/mqtt/profiles.spec.ts:873` — 接続中のブローカーを消すと `Disconnect` を呼び、残ったブローカーが選ばれる
- `ui/mqtt/profiles.spec.ts:925`、`ui/common/keyboard-accessibility.spec.ts:171` — クリックと Enter での選択。Edit ボタンは選択を変えない
- `ui/mqtt/mqtt.spec.ts:215` — 並び替え

#### 不足

- **不足内容**: 確立待ち（`Connecting…`）のブローカーを削除したとき、バックエンドの接続を切ること。接続中（Connected）の削除は spec:873 が見ているが、確立待ちは `connected` が偽のまま `Disconnect` を送る別の分岐を通る。送らないと、画面から見えない接続がバックエンドに残る。
- **根拠コード**: `application/mqtt/connections.ts:529-541`（`conn.type === "online"` なら未確立でも `disconnect` を送り、失敗しても通知しない）、`sidebar/broker-tree.tsx:93-101`
- **スイート**: UI e2e（`seed.mqttConnect: "pending"`）
- **推奨テスト名**: `test("deleting a broker whose connection is pending disconnects it", ...)`
- **追加先**: `ui/mqtt/profiles.spec.ts` の `editing a broker whose connection is pending`
- **優先度**: 中（確立待ちのタブを閉じると `disconnect` を送ることは、単体テスト `application/mqtt/connections.test.ts:1096` が見ている。e2e で足すのは、サイドバーの削除からそこへつながること）

- **不足内容**: 選択していないブローカーを削除しても、選択中のブローカーとその接続バーが変わらないこと。既存の削除のテストはどれも選択中のブローカー（または唯一のブローカー）を消している。
- **根拠コード**: `application/mqtt/connections.ts:547-550`（選択中の接続を閉じたときだけ選び直す）
- **スイート**: UI e2e
- **推奨テスト名**: `test("deleting another broker keeps the selected broker", ...)`
- **追加先**: `ui/mqtt/profiles.spec.ts` の `broker rows`
- **優先度**: 低

### [D] 入力・バリデーション・境界値

不正な URL、key-value エディタ、UDP の固定長フィールドは該当なし（HTTP・UDP の機能）。

#### テスト済み

- `ui/mqtt/mqtt.spec.ts:36`、`ui/mqtt/profiles.spec.ts:69` — 名前が空・空白だけのとき Save と Save & Connect が無効
- `ui/mqtt/profiles.spec.ts:35` — ポート `0`・`65536` は無効、`65535` は有効。ホストが空・空白だけなら無効
- `ui/mqtt/mqtt.spec.ts:52` — `1e3`・`1883.0` のポートと `::1` のホストを通さない
- `ui/mqtt/profiles.spec.ts:520` — 接続バーは無効な値を保存せず、Connect も押せない
- `ui/mqtt/messages.spec.ts:113`、`integration/mqtt/mqtt.spec.ts:214` — 不正なトピックフィルター（`a/#/b`・`a+/b`）のエラーがトーストに出る
- `ui/mqtt/messages.spec.ts:157` — 前後の空白を除いて購読する。空白だけ・購読済みは送らない
- `ui/mqtt/messages.spec.ts:667` — トピックが空、retain なしで空白だけのペイロードは送らない。retain 付きの空ペイロードは送る
- `ui/mqtt/messages.spec.ts:620, 643` — `MQTT_MAX_MESSAGES`（5000）ちょうどと +1。1 フレームのバッファの上限
- `ui/mqtt/mqtt.spec.ts:338` — `MQTT_MAX_TOPICS`（500）+1
- `ui/mqtt/messages.spec.ts:1048` — 日本語・絵文字・`<script>`・`<img onerror>` を文字として表示する
- `ui/mqtt/publish.spec.ts:179` — プリセットの名前は空・空白だけでは確定しない

#### 不足

- **不足内容**: トピックの長さの上限（65535 バイト）と NUL 文字の拒否。購読・パブリッシュの検証エラーは `#`・`+` の位置とワイルドカードだけを見ていて、この 2 つを通す e2e はどちらのスイートにも無い。
- **根拠コード**: `internal/domain/mqtt/topic.go:50-62`、`fake-backend/install.ts:624-632`（偽バックエンドも同じ検証を持つ）
- **スイート**: UI e2e
- **推奨テスト名**: `test("a topic longer than 65535 bytes or containing NUL shows the backend error", ...)`
- **追加先**: `ui/mqtt/messages.spec.ts`（`subscribing to an invalid filter shows the backend error` の近く）
- **優先度**: 低（Go の単体テスト `internal/domain/mqtt/topic_test.go:11, 36, 83` が見ている）

### [E] 非同期操作

#### テスト済み

- `ui/mqtt/profiles.spec.ts:334, 366` — 確立待ちの間は `Connecting…` を出して入力欄を無効にする。押すと `Disconnect` で中止する
- `ui/mqtt/profiles.spec.ts:1124` — 応答の前に Connect を 2 回押しても `Connect` は 1 回
- `ui/mqtt/profiles.spec.ts:1090` — 結果を待つ間に別のブローカーへ切り替えても、結果は元のブローカーに反映される
- `ui/mqtt/profiles.spec.ts:578, 599` — 保存の失敗。ダイアログは開いたまま、接続バーからの失敗が続いてもトーストは 1 つ
- `ui/mqtt/profiles.spec.ts:687, 717, 785, 1061` — 削除・接続・接続拒否・切断の失敗
- `ui/mqtt/messages.spec.ts:823` — `mqtt:connection-lost` が続いてもトーストは 1 つ
- `ui/mqtt/messages.spec.ts:936, 989` — 解除の失敗、張り直しで拒否された購読
- `ui/mqtt/mqtt.spec.ts:371, 406` — スキャンの開始の失敗、開始中の停止では通知しない

#### 不足

- **不足内容**: 切断したあとで購読の行を外したときの挙動。切断後のタブは `online` のままなので `Unsubscribe` を送り、バックエンドは `connection not found` を返すが、未接続なので通知せずに行を外す。そのあと Connect しても、外した購読は張り直さない。「切断 → 購読を整理 → 再接続」は普通に通る操作だが、通知が出ないことも、張り直さないことも見ていない。
- **根拠コード**: `application/mqtt/subscriptions.ts:132-158`（`connected` が偽なら失敗を通知せず、行は必ず外す）、`application/mqtt/connections.ts:477-482`（張り直すのは残っている行だけ）
- **スイート**: UI e2e
- **推奨テスト名**: `test("a subscription removed while disconnected shows no error and is not re-subscribed on Connect", ...)`
- **追加先**: `ui/mqtt/messages.spec.ts`（`disconnect disables subscribe and publish…` の近く）
- **優先度**: 中（未接続のときに通知せず行を外すことは、単体テスト `application/mqtt/subscriptions.test.ts:259` が見ている。e2e で足すのは、そのあとの Connect まで通したときの張り直し）

- **不足内容**: 再接続時に購読の RPC そのものが失敗した場合（`Failed to re-subscribe to <topic>` のトーストと、`GetConnections` と突き合わせて行を外す処理）。spec:989 が通るのは「確立前に受け付けて、確立時に外される」経路で、この経路ではないことを spec 自身が確かめている（:1011-1013）。
- **根拠コード**: `application/mqtt/connections.ts:483-491, 501-527`
- **スイート**: UI e2e（偽バックエンドの拡張が必要: 確立済みになってから購読が届く順序を作れない。観点 N を参照）
- **推奨テスト名**: `test("a re-subscribe that fails on a live connection removes the row and keeps the rest", ...)`
- **追加先**: `ui/mqtt/messages.spec.ts` の `a subscription rejected by the broker`
- **優先度**: 低（`application/mqtt/connections.test.ts:929` が単体テストで見ている）

### [F] 画面側の永続化

`app:theme`・`wirexa:http:*`・`udp:targetOrder` は該当なし。

#### テスト済み

- `ui/mqtt/profiles.spec.ts:814` — `mqtt:lastActiveProfileId` の保存値と、リロード後の復元
- `ui/mqtt/publish.spec.ts:109` — `mqtt:presets` の保存値（並び順・retain を含む）と復元。リロード後はどのプリセットも選ばれていない
- `ui/mqtt/mqtt.spec.ts:215` — `mqtt:profileOrder` の保存値と復元
- `ui/mqtt/publish.spec.ts:279` — 3 つのキーの壊れた値（形の違う要素、配列でない値、JSON でない文字列）で既定値に戻り、例外もトーストも出ない

#### 不足

なし（選択中のブローカーを消したときに `mqtt:lastActiveProfileId` が切り替わることは見ていないが、無いブローカーの ID を持ち続けないことを spec:988 が見ているので挙げない）。

### [G] キーボード操作・アクセシビリティ

ショートカットは該当なし（MQTT には無い）。

#### テスト済み

- `ui/common/keyboard-accessibility.spec.ts:108` — ダイアログを開くと名前の欄にフォーカスがあり、Tab / Shift+Tab がダイアログの中で回る
- `ui/common/keyboard-accessibility.spec.ts:75` — Escape と背景クリックで保存せずに閉じる
- `ui/common/keyboard-accessibility.spec.ts:171` — Enter でブローカーを選ぶ（Space は同じ `ProfileList` を UDP のターゲットで見ている: :182）
- `ui/mqtt/messages.spec.ts:142`、`ui/mqtt/mqtt.spec.ts:137` — トピック欄の Enter で購読する。未接続では送らない
- `ui/mqtt/publish.spec.ts:162, 179` — プリセットの Enter / Space での選択、改名の Enter と Escape
- `ui/common/keyboard-accessibility.spec.ts:267` — `tab`・`tabpanel` のロールとアクセシブル名、パネル内の id の重複なし

#### 不足

なし。

補足（テストの不足ではない）: 接続バーとダイアログのスキーム・ホスト・ポートの欄はラベルを持たない（`mqtt/broker-manager.tsx:106-136`、`mqtt/broker-settings-dialog.tsx:111-141`）。このためページオブジェクトは placeholder と `xpath=preceding-sibling::select` で取っている（`fixtures/app.ts:597-615`）。`aria-label` を付ければ `getByLabel` に置き換えられる。

### [H] MQTT 固有フロー

#### テスト済み

- プロファイル: `ui/mqtt/mqtt.spec.ts:10, 81`、`ui/mqtt/profiles.spec.ts:89, 128, 163, 224, 268, 814, 846`
- 接続・切断: `ui/mqtt/messages.spec.ts:730`、`ui/mqtt/profiles.spec.ts:334, 424, 717, 743, 785, 1061`、`integration/mqtt/mqtt.spec.ts:51, 72, 88, 106`
- 購読: `ui/mqtt/messages.spec.ts:74, 267, 306, 336, 989`、`integration/mqtt/mqtt.spec.ts:175, 490`
- メッセージ一覧: `ui/mqtt/messages.spec.ts:192, 240, 360, 387, 430, 485, 520, 546, 575, 860`
- メッセージ上限: `ui/mqtt/messages.spec.ts:620, 643`
- パブリッシュ: `ui/mqtt/messages.spec.ts:667, 705`、`ui/mqtt/publish.spec.ts:48`、`integration/mqtt/mqtt.spec.ts:195, 235, 261`
- プリセット: `ui/mqtt/publish.spec.ts:59, 109, 162, 179, 222, 279`
- トピックスキャン: `ui/mqtt/mqtt.spec.ts:245, 307, 338, 371, 406`、`integration/mqtt/mqtt.spec.ts:389, 464`
- 再接続・切り替え: `ui/mqtt/messages.spec.ts:730, 782, 823`、`integration/mqtt/mqtt.spec.ts:144, 317, 348, 430`

#### 不足

- **不足内容**: 接続中のブローカー 2 つの切り替えが UI e2e に無い。購読・メッセージ・Auto・スキャンの状態は接続ごとに持つが、トピックの絞り込みはパネルで 1 つの値で、切り替え先の選択肢に無ければ解除される。選択していない接続に届いたメッセージも、その接続の一覧に溜まる。いずれも `fake.emit` に接続 ID を渡せば UI e2e で書ける。今あるのはフルスタックの 1 件（CI では動かない）だけで、絞り込みの解除は見ていない。
- **根拠コード**: `application/mqtt/messages.ts:66-88`（一覧は選択中の接続のもの。絞り込みは 1 つの signal で、選択肢から消えたら解除する）、`application/mqtt/message-buffer.ts:92-127`（接続 ID ごとに振り分ける）、`sidebar/broker-tree.tsx:52-56`
- **スイート**: UI e2e
- **推奨テスト名**: `test("each connected broker keeps its own subscriptions and messages, and switching resets a topic filter the other broker lacks", ...)`
- **追加先**: `ui/mqtt/messages.spec.ts`（`seed.mqttProfiles` に 2 件）
- **優先度**: 中（部品ごとには単体テストがある: 接続 ID ごとの振り分けは `application/mqtt/connections.test.ts:421`、選択中の接続だけを読み書きすることは `application/mqtt/messages.test.ts:309`、選択肢から消えた絞り込みの解除は `:449`。e2e で足すのは、サイドバーでの切り替えを通してこれらがつながること）

- **不足内容**: 切断後の購読の行の削除と再接続（観点 E の 1 件目と同じ。ここでは数えない）。

- **不足内容**: スキャンをもう一度始めると、前回の一覧を消してから集め直すこと。spec:291 は「止めても一覧は残る」を見て、そのあと Scan を押しているが（:294）、押した直後に一覧が空（`Scanning...`）になることは見ていない。停止と行き違いで届いた `mqtt:scan-topic` を捨てることも見ていない。
- **根拠コード**: `application/mqtt/subscriptions.ts:179-185`（開始時に `brokerTopics` を空にする）、`application/mqtt/message-buffer.ts:63-64`（スキャン中でなければ捨てる）、`mqtt/panels/broker-topics-panel.tsx:75-78`
- **スイート**: UI e2e
- **推奨テスト名**: `test("starting a scan again clears the previous topics, and topics arriving after Stop are ignored", ...)`
- **追加先**: `ui/mqtt/mqtt.spec.ts` の `broker topics scan`
- **優先度**: 低

- **不足内容**: Auto を OFF に戻すと追従が止まること。ON にする側だけを見ている（spec:546, 575）。
- **根拠コード**: `mqtt/panels/messages-panel.tsx:47-53, 77`、`application/mqtt/messages.ts:93-96`
- **スイート**: UI e2e
- **推奨テスト名**: `test("turning Auto off stops following new messages", ...)`
- **追加先**: `ui/mqtt/messages.spec.ts`
- **優先度**: 低

### [I] HTTP 固有フロー

該当なし（対象は MQTT）。

### [J] UDP 固有フロー

該当なし（対象は MQTT）。

### [K] OpenAPI 固有フロー

該当なし（対象は MQTT）。

### [L] テスト構造

#### 問題なし

- 固定 sleep: `ui/mqtt/`・`integration/mqtt/` に `page.waitForTimeout` は無い。フレームを待つ箇所は `requestAnimationFrame` を待つヘルパー（`ui/mqtt/messages.spec.ts:63-70`）
- ロケーター: spec に直接書かれた CSS は `ui/mqtt/messages.spec.ts:1075`（要素として解釈されていないことの確認）、`ui/common/keyboard-accessibility.spec.ts:119, 125-127, 130`（フォーカスのある要素と、フォーカスできる要素の数。理由のコメントあり）、`:296-297`（パネル内の `[id]` を集めて重複を見る）だけで、どれも role では表せない
- `test.only`・`test.skip`・`test.fixme`: 無い
- 個別の `timeout`: `integration/mqtt/mqtt.spec.ts:121, 447, 508` の `RECONNECT_TIMEOUT`（:447, 508 は paho の自動再接続を待つ。:40-41 に理由あり。:121 は届かない宛先への最初の接続の失敗を待っていて、定数の名前とコメントに合わない）と `ui/mqtt/messages.spec.ts:618` の `test.slow()`（5000 件の描画。理由あり）
- URL・ポートの直書き: 実際に接続するブローカーのポートは `fixtures/mqtt-broker.ts:6-15` の定数。`integration/common/backend-integration.spec.ts:63` の `tcp://127.0.0.1:1883` と `ui/mqtt/profiles.spec.ts:299, 308` の `http://127.0.0.1:1883` は直書きだが、前者は保存するだけの値、後者は検証で弾かれるか偽バックエンドに渡す値で、どちらも接続には使わない
- フルスタックの後始末: `integration/mqtt/mqtt.spec.ts:43-47` が `disconnectMqttConnections` と `allowBrokerFilters` を呼ぶ。retained メッセージは `finally` で消している（:289-292, 340-343, 420-423, 458-460）。ブローカー名とトピックはテストごとに一意

#### 不足

- **不足内容**: ヘルパーの重複。`App` に寄せられるものが spec ごとに書かれている。
  - Publish フォームのロケーター: `ui/mqtt/publish.spec.ts:17-26` の `publishForm` と同じものが `ui/mqtt/messages.spec.ts:674-677, 712-715`、`integration/mqtt/mqtt.spec.ts:204-207, 243-246, 270-275` にある
  - トピックの絞り込み（`getByRole("combobox", { name: "Filter by topic" })`）: `ui/mqtt/messages.spec.ts:380, 404, 446, 509, 591, 885`
  - New Broker を押してダイアログを取る: `ui/mqtt/mqtt.spec.ts:40-42, 56-58`、`ui/mqtt/profiles.spec.ts:39-40, 70-71, 229-230, 582-583, 748-749`、`ui/common/keyboard-accessibility.spec.ts:82, 109`
  - Subscribe / Publish タブを開く（`getByRole("tab", { name: "Publish" }).click()`）: 3 つの spec で 10 か所以上
  - 空状態の文言: `ui/mqtt/mqtt.spec.ts:5-6`、`ui/mqtt/profiles.spec.ts:25-27, 820-822, 997-999`、`ui/common/initial-state.spec.ts:40`
  - `integration/mqtt/mqtt.spec.ts:34-38` の `switchBroker` は `App.selectBroker`（`fixtures/app.ts:565-575`）から待ちを除いただけ
- **根拠コード**: `fixtures/app.ts:506-805`（MQTT の節にこれらの操作が無い）
- **スイート**: 両方
- **推奨**: `App` に `mqttPublishForm()`・`mqttTopicFilter`・`openNewBrokerDialog()`・`openMqttTab(name)`・`mqttEmptyState` を足す。`selectBroker` は `connected` を見ずに選ぶだけの形も取れるようにして `switchBroker` を消す
- **優先度**: 中（文言やロールが変わると 3 つの spec を直すことになる）

- **不足内容**: スイートの選び方。接続中のブローカー 2 つの切り替えはフルスタックにだけある（`integration/mqtt/mqtt.spec.ts:348`）。画面の状態の分離は UI e2e で書ける（観点 H の 1 件目）。フルスタックの側は「実ブローカーから届く分が接続ごとに分かれる」ことの確認として残してよい。
- **優先度**: 中（観点 H に集約）

- **不足内容**: `ui/mqtt/profiles.spec.ts:293` の `Connect rejects a scheme that cannot be used with TLS` は画面を通さずバインディングを直接呼ぶので、UI e2e では偽バックエンドの実装（`fake-backend/install.ts:659-667`）だけを確かめている。フルスタックの同名のテスト（`integration/mqtt/mqtt.spec.ts:130`）との対で文言のずれを検出する役目はあるので、消す必要は無い。バインディングを直接呼ぶ理由は両方の spec のコメントに既にある（`ui/mqtt/profiles.spec.ts:292`、`integration/mqtt/mqtt.spec.ts:129`）。足すとすれば、もう一方のスイートの同名のテストと対になっていることの一言だけ。
- **優先度**: 低

- **不足内容**: 「届かないこと」の確認が弱い箇所。`integration/mqtt/mqtt.spec.ts:183-192` は購読したトピックへ publish したあとで購読していないトピックへ publish し、件数が 1 であることを見るが、2 件目が誤って届く場合でも、届く前に検証が通りうる。同じ spec の :254-256 のように、購読していないトピックを先に送り、あとから送った目印が届いた時点で件数を見る順にするとよい。ただし、順を直しても見えるのは画面に出ないことまでである。画面は購読の行に一致しないメッセージを捨てる（`application/mqtt/message-buffer.ts:104-106`）ので、Go やブローカーが余分に配信していても件数は変わらない。
- **優先度**: 低

- **不足内容**: `pageErrors` が拾うのは、fixture の `goto` が終わったあとの例外だけ（`fixtures/ui.ts:101-113, 123-127`。fixture のコメントにもある）。`reload` してから見るテスト（`ui/mqtt/profiles.spec.ts:631, 988, 1026`、`ui/mqtt/publish.spec.ts:279`）は起動時の例外も拾うが、`ui/mqtt/profiles.spec.ts:964` は `reload` せずに `expect(pageErrors).toEqual([])` を見ているので、`GetConnections` に失敗した起動そのものの例外は拾えていないことがある。
- **推奨**: `page` の fixture で `goto` の前に `pageerror` を購読する。または :964 を `reload` してから見る形にする
- **優先度**: 低

### [M] バックエンド統合

M-2・M-4・M-8・M-9・M-10 は該当なし（HTTP・UDP・OpenAPI の保存データとファイルアクセス。MQTT の RPC はファイルパスを受け取らない）。

#### テスト済み

- M-1: `integration/common/backend-integration.spec.ts:52` — バインディングで保存したプロファイルが、リロード後の `GetProfiles` で一覧に出る
- M-3: `integration/common/backend-integration.spec.ts:149, 157, 181` — リロード後も残る、`mqtt-profiles/<id>.json` の形が `testdata/profile.golden.json` と一致する、編集は同じファイルを書き換える、削除でファイルが消える
- M-5: `integration/mqtt/mqtt.spec.ts:130, 214, 235` — Go の検証エラー（TLS で使えないスキーム、`#` の位置、publish のワイルドカード）の文言
- M-6: `integration/mqtt/mqtt.spec.ts:51, 106, 175, 195, 261, 296, 317, 430` — 実接続、接続失敗、受信、ループバック、retained、バイナリ、再接続
- M-7: `integration/mqtt/mqtt.spec.ts` — `mqtt:connected`（:51）、`mqtt:connection-failed`（:106）、`mqtt:message`（:175）、`mqtt:scan-topic`（:389）、`mqtt:connection-lost`（:430）、`mqtt:scan-stopped`（:464）、`mqtt:subscription-dropped`（:490）。`internal/domain/events.go:8-15` の 8 種類のうち 7 種類は、イベントが届かなければ通らない検証がある。残る `mqtt:disconnected` は、発火する操作（:72）はあるが届いたことは見ていない（不足を参照）
- Go 側のメモリに残る状態: `integration/mqtt/mqtt.spec.ts:144` — リロード後も同じ接続 ID で復元し、購読にメッセージが届く

補足: ここでの「リロード」はページの読み直しで、Go のプロセスは動いたままである。リロード後に一覧へ出ることが示すのは Go のメモリにある値で、ディスクから読み直すこと（アプリの再起動）ではない。ディスクへの書き込みは :157, 181 がファイルを直接読んで見ている。保存したファイルを全フィールド読み戻せることは、Go の単体テスト `internal/infrastructure/mqtt/profile_repository_test.go:25` が見ている。

#### 不足

- **不足内容**: 購読を解除したあと、ブローカーの側でも購読が外れていること（M-6）。フルスタックには購読の行を外すテストが 1 件も無い。UI e2e は `Unsubscribe` の引数を見ているが、偽バックエンドにブローカーは無い。画面の件数では確かめられない。行を外した時点で画面はそのトピックを捨て（`application/mqtt/message-buffer.ts:104-106`）、Go も振り分け先を外して捨てる（`internal/infrastructure/mqtt/paho_client.go:119-134, 292-306`）ので、ブローカーに購読が残っていても画面には出ない。
- **根拠コード**: `internal/application/mqtt/subscription.go:170-191`、`application/mqtt/subscriptions.ts:124-159`
- **スイート**: フルスタック（`tools/e2e-broker` に、クライアントの購読を返す操作用の口が要る。今あるのは `/publish`・`/disconnect-clients`・`/deny` だけ: `tools/e2e-broker/main.go:143-201`）
- **推奨テスト名**: `test("unsubscribing removes the subscription from the broker and keeps the other one", ...)`
- **追加先**: `integration/mqtt/mqtt.spec.ts`（行を外したあと、ブローカーの購読の一覧と `GetConnections` の `subscriptions` に残した 1 件だけがあることを見る）
- **優先度**: 中（Go の統合テスト `internal/integration/mqtt_test.go:441` の `TestMQTT_Unsubscribe` がサービスの単位では見ている）

- **不足内容**: `mqtt:disconnected` が画面に届くこと（M-7）。`integration/mqtt/mqtt.spec.ts:72` は画面の Disconnect を押すが、画面は RPC の応答のあと自分でタブを未接続にする（`application/mqtt/connections.ts:407-419`）ので、イベントが届かなくても通る。
- **根拠コード**: `internal/application/mqtt/service.go:255`、`application/mqtt/connections.ts:192-195`
- **スイート**: フルスタック
- **推奨テスト名**: `test("a connection closed from outside the page turns the tab Disconnected", ...)`
- **追加先**: `integration/mqtt/mqtt.spec.ts`（画面を通さずバインディングの `Disconnect` を直接呼び、画面が Disconnected になることを見る）
- **優先度**: 低（発火は Go の統合テスト `internal/integration/mqtt_test.go:1587` が、受けたあとの状態は単体テストが見ている）

- **不足内容**: TLS と WebSocket での実接続。e2e ブローカーは TCP のリスナーしか持たない（`tools/e2e-broker/main.go:109`）ので、`mqtts://`・`ssl://`・`ws://`・`wss://` で実際につながることはどのスイートも見ていない。:130 の「TLS で使えないスキームの拒否」は接続の前の検証で、ハンドシェイクの確認ではない。Go の側も、スキームの変換の単体テスト（`internal/infrastructure/mqtt/paho_client_test.go:281-289`）と検証の統合テスト（`internal/integration/mqtt_test.go:1697`）までである。
- **スイート**: フルスタック（`tools/e2e-broker` に TLS と WebSocket のリスナーが要る）
- **優先度**: 低

- **不足内容**: リロード時にスキャンを止めること。UI e2e は `StopTopicScan` を呼ぶことを見ている（`ui/mqtt/messages.spec.ts:782`）が、実バックエンドで `scanning` が偽になることは見ていない。
- **根拠コード**: `application/mqtt/connections.ts:284-295`、`internal/application/mqtt/topic_scan.go:177-191`
- **スイート**: フルスタック
- **推奨テスト名**: `test("reloading the page stops a running topic scan", ...)`
- **追加先**: `integration/mqtt/mqtt.spec.ts`
- **優先度**: 低

- **不足内容**: ユーザー名・パスワード・クライアント ID が実ブローカーに届くこと。UI e2e は `Connect` の引数まで見ている（`ui/mqtt/profiles.spec.ts:224`）。フルスタックの e2e ブローカーは認証を常に通す（`tools/e2e-broker/main.go:59`）ので、今のままでは確かめられない。
- **スイート**: フルスタック（`tools/e2e-broker` に、接続したクライアントの ID とユーザー名を返す操作用の口が要る）
- **優先度**: 低

### [N] 偽バックエンドの忠実さ

#### テスト済み・一致しているもの

- RPC の網羅: `internal/adapters/mqtt_handler.go:42-95` の 11 メソッドがすべて `fake-backend/install.ts:709-897` にある
- 検証の順序と文言: トピック・QoS の検証が接続の検索より先（`internal/application/mqtt/subscription.go:55-62, 71-78, 170-174` と `install.ts:824-826, 845-846, 892-894`）。`invalid topic: …`・`invalid broker URL: …`・`connection not found: <id>`・`profile not found: <id>` の形も同じ（`internal/domain/errors.go:23-25, 33-38`）
- 並び順: `GetProfiles` は ID の昇順（`internal/application/store/cached_store.go:62-75` と `install.ts:713`）、購読は購読した順で、購読済みのトピックは QoS だけを上書き（`subscription.go:15-20` と `install.ts:832-835`）
- 未知の ID の `SaveProfile`・`DeleteProfile` は失敗する（`cached_store.go:85-90, 103-105` と `install.ts:720-723, 737-739`）
- イベントの種類: `Connect` の結果は `mqtt:connected` / `mqtt:connection-failed`、確立時に拒否された購読は `mqtt:connected` のあとで `mqtt:subscription-dropped`、`Disconnect` は `mqtt:disconnected`。`Connect` の応答とイベントの順序は一致していない（不足の 2 件目）
- 型の追従: `fake-backend/types.ts:10-13` は domain 型、`Connect` の引数は生成された `mqttdomain.ConnectionConfig`（`install.ts:30, 763`）
- 同じシナリオが両方のスイートにあるもの: 不正なフィルター（`ui/mqtt/messages.spec.ts:113` / `integration/mqtt/mqtt.spec.ts:214`）、ワイルドカード付きの publish（:705 / :235）、TLS で使えないスキーム（`ui/mqtt/profiles.spec.ts:293` / :130）、接続失敗（:717 / :106）、張り直しで拒否された購読（`ui/mqtt/messages.spec.ts:989` / :490）、スキャン（`ui/mqtt/mqtt.spec.ts:245, 307` / :389, 464）、バイナリ（`ui/mqtt/messages.spec.ts:240` / :296）、リロード後の復元（:782 / :144）

#### 不足

- **不足内容**: 購読解除の失敗の種類。Go は応答を確認できなかった場合（`ErrAckTimeout`）だけ購読を外し、それ以外の失敗では購読を残してエラーを返す。偽バックエンドの `unsubscribeError` は前者だけを模す。画面はどちらでも行を外すので、後者ではバックエンドに購読が残ったまま行だけが消え、リロードすると行が戻る。この食い違いが意図したものかどうかを確かめるテストがどちらのスイートにも無い。
- **根拠コード**: `internal/application/mqtt/subscription.go:178-188`、`fake-backend/install.ts:842-853`、`application/mqtt/subscriptions.ts:155-158`
- **スイート**: UI e2e（偽バックエンドの拡張が必要: `unsubscribeError` に「購読を残して失敗する」種類を足す）
- **推奨テスト名**: `test("an unsubscribe that fails without removing the subscription …", ...)`（期待する挙動を決めてから名前を付ける）
- **優先度**: 中

- **不足内容**: `Connect` の応答より先に届く接続状態のイベント。Go は `Connect` が返る前に接続の goroutine を始めるので、`mqtt:connected` などが応答より先に届きうる。画面はそのための処理を持つが、偽バックエンドは必ず応答のあと（`setTimeout`）に発火するので、UI e2e はこの順序を一度も通らない。
- **根拠コード**: `application/mqtt/connections.ts:128-160`、`internal/application/mqtt/service.go:139, 180-182`、`fake-backend/install.ts:782-801`
- **スイート**: UI e2e（偽バックエンドの拡張が必要: 結果のイベントを応答の前に出す `FakeSeed` の指定）
- **優先度**: 低（`application/mqtt/connections.test.ts:1359` などの単体テストが見ている）

- **不足内容**: 注入口の不足（観点 E から）。確立済みの接続に対して再接続の購読が届く順序を作れないので、`Failed to re-subscribe` の経路を UI e2e で通せない。購読の RPC の遅延（または結果のイベントを購読より先に出す指定）が要る。
- **根拠コード**: `application/mqtt/connections.ts:483-491`、`fake-backend/install.ts:821-837`
- **優先度**: 低

- **不足内容**: 未確立の接続への `Publish`。Go は paho のエラーを `failed to publish: …` で返すが、偽バックエンドは成功を返す。画面は未接続のとき Publish を無効にするので、画面からは通らない。
- **根拠コード**: `internal/application/mqtt/subscription.go:62-67`、`fake-backend/install.ts:883-896`、`mqtt/publish-tab.tsx:182-185, 222`
- **優先度**: 低（記録のみ。自動再接続中の publish は `internal/integration/mqtt_test.go:1607` が見ている）

---

## 優先して追加すべきテスト

該当なし（優先度 高 は無い）。必ず通るフロー（作成・接続・購読・受信・パブリッシュ・切断・削除）は UI e2e とフルスタックの両方にあり、データを失う挙動で未テストのものは見つからなかった。

---

## 総合評価

**主なリスク**:

- 接続中のブローカーを 2 つ使う場面は、CI で動く e2e が無い。application 層の単体テスト（CI で動く）は接続ごとの振り分けと選択中の接続の読み書きを見ているが、サイドバーでの切り替えを通した画面の挙動は、ローカルでフルスタックを回すまで確かめられない。
- 切断中の操作（購読の行の削除、確立待ちのブローカーの削除）は、通知を出さずにバックエンドへ RPC を送る分岐で、どちらも e2e では未テスト（application 層の単体テストはある）。画面からの配線が壊れると「外したはずの購読が再接続で戻る」「見えない接続が残る」になる。
- 購読解除が失敗したときの画面とバックエンドの食い違い（観点 N の 1 件目）は、テストの前に期待する挙動を決める必要がある。`GetProfiles` に失敗した起動のあとの並び順（観点 A）も同じ。

**推奨アクション**:

1. `ui/mqtt/messages.spec.ts` に、接続中のブローカー 2 つの切り替え（購読・メッセージの分離と絞り込みの解除）と、切断後に外した購読を再接続で張り直さないことの 2 件を足す。
2. `ui/mqtt/profiles.spec.ts` に、確立待ちのブローカーの削除で `Disconnect` を呼ぶことを足す。
3. `integration/mqtt/mqtt.spec.ts` に、購読解除のあとブローカーの側でも購読が外れていることを足す。画面の件数では確かめられないので、先に `tools/e2e-broker` へ購読を返す操作用の口を足す。
4. 購読解除の失敗（購読が残る場合）と、読み込み失敗後の `mqtt:profileOrder` について、期待する挙動を決める。決めたら `FakeSeed.unsubscribeError` を拡張してテストにする。
5. `fixtures/app.ts` に Publish フォーム・トピックの絞り込み・New Broker のダイアログ・タブの切り替えを足し、3 つの spec の重複を消す。新しいテストを足す前にやると、足す側も短くなる。
