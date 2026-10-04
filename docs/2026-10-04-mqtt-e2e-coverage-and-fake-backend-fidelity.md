# 変更計画書: MQTT の E2E テストの拡充と偽バックエンドの忠実さの修正

## 概要

`docs/e2e-test-review-mqtt.md` の不足と指摘を、優先度 中 と 低 の両方について実施する（優先度 高 の不足は無い）。

1. UI e2e に不足しているテストを足す（優先度 中 13 件、優先度 低 22 件と既存テストへのケース追加 2 件）。
2. フルスタック e2e に不足しているテストを足す（優先度 中 4 件、優先度 低 6 件）。
3. 偽バックエンドを直す。`Connect` が TLS フラグを読めていない（`useTLS` を読んでいるが配線形式は `useTls`）。`GetProfiles` の並び順が Go と違う。優先度 低 のテストに要る注入口を足す。
4. フルスタック用のブローカー（`tools/e2e-broker`）に、クライアントの切断と購読の拒否を起こす操作を足す。
5. `App`（`fixtures/app.ts`）に MQTT の操作とロケーターを寄せ、spec の重複と直接のロケーターを置き換える。
6. レビューが「本体側の事項」とした 4 件を直す: アイコンボタンのアクセシブル名、Connect の二重実行、未接続での Enter による購読、`Connect` の応答より先に届いた接続イベントの扱い。

**前提**: `docs/2026-10-04-mqtt-connected-edit-and-connection-bar-validation.md`（以下「接続中編集の計画」）を先に main へマージする。同じファイル（`connections.ts`、`fixtures/app.ts`、`profiles.spec.ts`）に触れ、あちらが `App` に足す接続バーのホスト・ポート欄と編集ダイアログを開く操作をこの計画が使うため。レビューの「既存のブローカーを編集して Save & Connect」は、あちらの計画のテスト（`Save is disabled for a connected broker and Save & Connect reconnects with the edit`）が満たすので、この計画には含めない。

**対象外**: `startTopicScanError` の注入口（レビューは注入口の一覧に挙げているが、開始の失敗は未接続での Scan のテストが今の偽バックエンドのまま通すので、使うテストが無い）。

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| internal/application/store/cached_store.go | Backend application | 変更 | `GetAll` が ID の昇順で返す（今は map の反復順で、呼ぶたびに変わりうる） |
| internal/application/store/cached_store_test.go | テスト | 変更 | `GetAll` の順序のテスト |
| tools/e2e-broker/main.go | テスト基盤（Go） | 変更 | `POST /disconnect-clients`（接続中のクライアントをすべて切る）、`POST /deny?filter=<f>` と `DELETE /deny`（そのフィルターへの購読を ACL で拒否する・解除する）を足す。`auth.AllowHook` を、拒否するフィルターを持つ自前の hook に替える |
| tools/e2e-broker/main_test.go | テスト | 新規 | 拒否の hook と操作用 HTTP のテスト（「テスト方針」） |
| frontend/src/application/mqtt/connections.ts | Frontend application | 変更 | 実行中の `handleConnect`・`handleReconnect` と同じ対象への呼び出しを無視する。タブを作る前（`Connect` の応答待ちと、起動時の `GetConnections` の応答待ち）に届いた接続状態のイベントを、タブを作った時点で反映する |
| frontend/src/application/mqtt/connections.test.ts | テスト | 変更 | 上の 2 件のテスト。接続の結果を待つ間に別のブローカーへ切り替えるテスト |
| frontend/src/application/mqtt/subscriptions.test.ts | テスト | 変更 | `addSubscription` の入力の扱い（重複・空白の除去・空白だけ・成功後に入力欄を消す）のテスト |
| frontend/src/presentation/components/mqtt/panels/subscriptions-panel.tsx | Frontend presentation | 変更 | トピック欄の Enter は `canSubscribe()` が真のときだけ `addSubscription` を呼ぶ。購読の削除ボタンに `aria-label="Remove subscription"` |
| frontend/src/presentation/components/mqtt/publish-tab.tsx | Frontend presentation | 変更 | プリセットの追加ボタンに `aria-label="Add preset"`、削除ボタンに `aria-label="Delete preset"` |
| frontend/e2e/fake-backend/install.ts | テスト基盤 | 変更 | `Connect` の引数を `mqttdomain.ConnectionConfig`（型だけの import）にして `useTls` を読む。`GetProfiles`・`GetTargets` は ID の昇順で返す。`Subscribe` は購読中のトピックの位置を保って QoS を上書きする。下の注入口を実装する |
| frontend/e2e/fake-backend/types.ts | テスト基盤 | 変更 | `FakeSeed` に注入口を足す（「実装方針」の一覧） |
| frontend/e2e/fixtures/app.ts | テスト基盤 | 変更 | MQTT の操作とロケーターを足す（「実装方針」の一覧） |
| frontend/e2e/fixtures/ui.ts | テスト基盤 | 変更 | `pageErrors` fixture（未捕捉の例外のメッセージの配列）を足す |
| frontend/e2e/fixtures/mqtt-broker.ts | テスト基盤 | 変更 | 何も待ち受けていないポートの定数 `mqttUnusedPort`（18832）。`publishFromBroker` がバイト列も受ける。`disconnectBrokerClients()`・`denyBrokerFilter(filter)`・`allowBrokerFilters()` を足す |
| frontend/e2e/ui/mqtt/messages.spec.ts | テスト | 変更 | テストを追加。`connectionId` を fixture にする。`message()` に型を付ける。ヘルパーとロケーターを `App` のものへ置き換える。コメントの誤り（`:492`）を直す |
| frontend/e2e/ui/mqtt/profiles.spec.ts | テスト | 変更 | テストを追加。ロケーターと `pageerror` の収集を置き換える |
| frontend/e2e/ui/mqtt/mqtt.spec.ts | テスト | 変更 | テストを追加。検証の弱いテスト 3 件を直す。見出しコメントを直す。ロケーターを置き換える |
| frontend/e2e/ui/mqtt/publish.spec.ts | テスト | 変更 | テストを追加。追加・削除ボタンを名前で取る（`addPresetButton` の `xpath=..` を無くす） |
| frontend/e2e/ui/common/keyboard-accessibility.spec.ts | テスト | 変更 | テストを追加。`:145-146` の placeholder のロケーターを `App` のものへ置き換える |
| frontend/e2e/integration/mqtt/mqtt.spec.ts | テスト | 変更 | テストを追加。`newBroker`・`subscribe` を `App` のものへ置き換える。`afterEach` で `allowBrokerFilters()` を呼ぶ |
| frontend/e2e/integration/common/backend-integration.spec.ts | テスト | 変更 | M-3 にテストを追加 |

バインド API のシグネチャは変えない。`GetProfiles`・`GetTargets` の戻り値の順序だけが決まるようになる。

## 実装方針

### 1. `GetProfiles` の順序（Go と偽バックエンド）

`CachedStore.GetAll`（`cached_store.go:59-67`）は map を反復して返すので、順序が呼ぶたびに変わりうる。画面は `mqtt:profileOrder` に無いプロファイルを読み込んだ順のまま並べる（`application/shared/order.ts` の `applyOrder` は安定ソート）ので、並び順が保存されていないプロファイル（localStorage を消した直後など）は起動のたびに順序が変わりうる。偽バックエンドは追加した順で返すので、この違いをテストが検出できない。

- Go: `GetAll` で ID を集めて `slices.Sort` し、その順で返す。ID は UUID なので意味のある順ではないが、起動のたびに変わらない。意味のある並びはこれまでどおり localStorage の並び順が決める。
- 偽バックエンド: `GetProfiles` と `GetTargets`（同じ `CachedStore` を使う）は ID の昇順に並べた複製を返す。`db` の中の順序と `snapshot()` は変えない。
- 依存方向: `CachedStore` は application 層で、標準ライブラリの `slices` を使うだけ。ポートも配線（`app.go`）も変えない。

既存の UI e2e への影響: seed は `profile-alpha` / `profile-beta`、`target-a` / `target-b` のように ID が辞書順なので、初回の表示順は変わらない。画面から作ったもの（ID は `profile-<n>`）は保存時に並び順が localStorage に入るので、リロード後も今の順序のまま。`fake.snapshot()` の順序を見ているテスト（`profiles.spec.ts:350-353・449-454`）は影響を受けない。

### 2. 偽バックエンドの `Connect` の引数

`install.ts:712-717` のインラインの型を `mqttdomain.ConnectionConfig`（`wailsjs/go/models` からの型だけの import）に替え、`validateBrokerScheme(config.broker, config.useTls)` にする。生成型を使うので、Go の json タグが変わって再生成されれば tsc が検出する。`e2e/` は biome の `noRestrictedImports` の対象外で、`fixtures/integration.ts` も生成された型を import している。

画面のスキーム（`BROKER_SCHEMES`）はどれも TLS で使えるので、この検証は画面からは通らない。`page.evaluate` で `window.go.adapters.MQTTHandler.Connect` を直接呼ぶテストを、UI e2e とフルスタックの両方に同じ文言で足す。

### 3. 偽バックエンドの注入口（`FakeSeed`）

| 注入口 | 動作 | 使うテスト |
| --- | --- | --- |
| `mqttProfiles` の `clientId`・`username`・`password`・`useTls`（任意） | 省けば今の固定値 | 編集ダイアログの読み込み |
| `mqttConnections`（`ConnectionStatus` の配列） | 起動時からバックエンドにある接続 | プロファイルが削除済みの接続の復元、再接続時に拒否された購読 |
| `getConnectionsError` | `GetConnections` を必ず失敗させる | 起動時の復元の失敗 |
| `unsubscribeError` | `Unsubscribe` が検証のあとで購読を外してから失敗する。Go の応答待ちのタイムアウト（`ErrAckTimeout`。エラーを返すが購読は外す。`subscription.go:181`）に当たる | 購読解除の失敗 |
| `disconnectError` | `Disconnect` を必ず失敗させ、接続は残す。RPC 自体の失敗（`getProfilesError` と同じ扱い）を模す。Go の `Disconnect` は、ある接続に対しては失敗しない（`service.go:232-244`） | 切断の失敗 |
| `subscribeError`（`{ topic, message }`） | ブローカーがそのトピックの購読を拒否する。確立済みの接続への `Subscribe` は検証のあとで失敗する。確立前の `Subscribe` は登録して成功を返し、確立したときに `mqtt:connected` のあとで購読を外して `mqtt:subscription-dropped` を出す（Go の `Subscribe` と `resubscribe`。`subscription.go:78-90・136-155`） | 再接続時に拒否された購読 |
| `startTopicScanDelayMs` | `StartTopicScan` が解決するまでの遅延。待つ間に `StopTopicScan` か `Disconnect` が来たら、`topic scan was stopped` で失敗する（Go の `errScanStopped`） | 開始中の Stop |
| `mqttConnectDelayMs` | `Connect` の RPC が接続 ID を返すまでの遅延 | Connect の二重実行 |
| `mqttConnectResultDelayMs` | 接続 ID を返してから結果のイベントを出すまでの遅延 | 結果を待つ間のブローカーの切り替え |

遅延は既存の `delay()`（`install.ts:528`）を使う。

`mutates`（`install.ts:175-184`）は本体を呼んだ直後に `save()` し、本体が例外を投げると `save()` しない。待ってから状態を変える `StartTopicScan` と、状態を変えてから失敗する `Unsubscribe` は、`StartListen`（`install.ts:562-583`）と同じく `counted` にして自分で `save()` する。`mutates` のままだと、遅延のあとの `scanning` や外した購読が sessionStorage に入らず、リロードで前の状態が戻る。

`StartTopicScan` は Go と同じく、開始中に止められたら失敗させる（開始を待つ間は `scanning` を偽のままにし、`GetConnections` の `Scanning` が稼働中だけ真なのと揃える）。画面は止めた開始の失敗を通知しない（`subscriptions.ts:194-195`）。

**`fake.emit` は偽バックエンドの状態を変えない**（`install.ts:922-932`）。`mqtt:connection-lost`・`mqtt:subscription-dropped`・`mqtt:scan-stopped` を流しても、`mqttConnections` の `connected`・購読・`scanning` は元のまま残る。Go はこれらのイベントと一緒に状態も変える（購読を外す、スキャンを外す）ので、流したあとでリロードすると Go と違う状態が復元される。イベントを流すテストは画面の反応だけを見て、そのあとのリロードと、流したイベントに関わる `fake.snapshot()` の検証はしない。この制約は `__wirexaFake.emit` のコメントに書く。バックエンドの状態まで含めた確認は、フルスタックのテスト（切断・スキャンの停止・購読の拒否）が受け持つ。

### 4. `tools/e2e-broker` の拡張

- `POST /disconnect-clients`: `server.Clients.GetAll()` のうちインラインクライアント以外を `Stop` する。アプリの接続とスキャン用の接続が切れ、Go は `mqtt:connection-lost` と `mqtt:scan-stopped` を出し、paho が自動再接続する。
- `POST /deny?filter=<f>` / `DELETE /deny`: `auth.AllowHook` の代わりに、接続はすべて許可し、登録されたフィルターへの購読だけを `OnACLCheck` で拒否する hook を使う（`internal/integration/mqtt_test.go:979-995` の `denyFilterHook` と同じ考え方。フィルターの集合は mutex で守る）。購読してから拒否を登録して切断すると、再接続時の張り直しが拒否されて `mqtt:subscription-dropped` が出る。
- どちらも既存の `/publish` と同じく、ループバックで待ち受ける認証なしの操作用 HTTP。ファイル先頭の doc コメントの一覧に足す。
- フルスタックの spec は `afterEach` で拒否の登録を消す（ブローカーは 1 回の実行の中で共有される）。

### 5. 本体の修正

**アクセシブル名**: アイコンだけのボタン 3 つに `aria-label` を足す。`Button` は残りの props をそのまま `<button>` に渡す（`components/ui/button.tsx:37-52`）。購読の削除は `Unsubscribe` ではなく `Remove subscription` にする（`getByRole("button", { name: "Subscribe" })` の部分一致に当たらないようにするため）。

**Connect の二重実行**（`connections.ts`）: `handleReconnect` は接続 ID、`handleConnect` はプロファイル ID を実行中の集合に入れ、同じ対象の呼び出しは終わるまで無視する（`finally` で外す）。今は応答を待つ間にもう一度押すと `Connect` が 2 回送られ、先の接続がタブを持たないままバックエンドに残る。

**未接続での Enter**（`subscriptions-panel.tsx:45`）: Enter のハンドラを `e.key === "Enter" && canSubscribe() && addSubscription()` にして、Subscribe ボタンの無効化（`:57`）と揃える。今は Enter だけが `Subscribe("offline-<id>", ...)` を送って `connection not found` のトーストを出す。`addSubscription` は変えない（スキャン結果の一覧からの購読も通るので、判定を足すと影響が広がる）。

**タブを作る前に届いた接続イベント**（`connections.ts`）: Go は `Connect` の return の前に接続の goroutine を始めるので、`mqtt:connected`・`mqtt:connection-failed` が RPC の応答より先に届きうる。今はタブがまだ無いのでイベントを捨て（`updateConnection` はタブが無ければ何もしない。`connections.ts:80-84`）、Connected にならない。起動時の `restore()` が `GetConnections` の応答を待つ間（`connections.ts:183-188`）も同じで、取得のあとに出たイベントが応答より先に届くと、取得時点の古い状態のままタブができる。

- タブを作る RPC の実行中（`handleConnect`・`handleReconnect` の `api.connect` と、`restore()` の `api.getConnections`）にだけ、タブの無い接続 ID の接続状態のイベントを `Map<接続 ID, 確立しているか>` に覚える。対象は `mqtt:connected`（真）と、`mqtt:connection-lost`・`mqtt:connection-failed`・`mqtt:disconnected`（偽）。
- 同じ接続 ID のイベントが続けて届いたら、最後のもので上書きする。`mqtt:connected` だけを覚えると、応答の前に確立して切れた接続（`connected` → `connection-lost`）を Connected と表示してしまう。
- 応答を受けてタブを作るときに、その接続 ID の値があれば `connected` に反映する。`restore()` では `GetConnections` の値より優先する（覚えた値は取得より後に届いた状態）。通知はイベントの受信時に出しているので、ここでは出さない。
- 実行中の RPC が無くなったら Map を空にする（他のウィンドウやタブを閉じた後の接続の ID を持ち続けない）。
- 購読の側が同じ順序の問題を扱っているやり方（`subscriptions.ts:42-44` の `pendingSubscribes`）に合わせる。

**依存方向**: どれも `application/mqtt/` と `presentation/components/mqtt/` の中の変更で、新しい import もポートも無い。状態の書き込みはイベントのリスナーとイベントハンドラから行い、presentation の effect からは書かない。

### 6. `App`（`fixtures/app.ts`）に足すもの

接続中編集の計画が足すもの（接続バーのホスト・ポート欄、編集ダイアログを開く操作）はそのまま使い、足りない分を足す。名前は実装時にあちらの命名に合わせる。

| 追加 | 内容 | 置き換える箇所 |
| --- | --- | --- |
| `brokerHostInput(scope?)` / `brokerPortInput(scope?)` / `brokerSchemeSelect(scope?)` | placeholder（`localhost`・`1883`）とネイティブ select。scope を省くと接続バー、ダイアログを渡すとダイアログの欄。個別のラベルが無いので placeholder で取る理由をコメントする | `mqtt.spec.ts:36-37`、`profiles.spec.ts:41-42・86-94・121・128・133・191・407-408・443`、`keyboard-accessibility.spec.ts:145-146`、`integration/mqtt/mqtt.spec.ts:30-31・71` |
| `brokerDialog(title?)` | `getByRole("dialog", { name })` | 各 spec のダイアログの取得 |
| `brokerRowAction(name, "Edit broker" \| "Delete broker")` | 行を hover してボタンを返す | `profiles.spec.ts:77-79・124-126・141-143・281-283`、`mqtt.spec.ts:61-62` |
| `createBrokerProfile(name, options?)` | `options` に `host`・`port`・`action`（`"Save"` / `"Save & Connect"`）。省けば今と同じ | `integration/mqtt/mqtt.spec.ts:22-34` の `newBroker` |
| `selectBroker(name, { connected: true })` | 接続済みのブローカーを選ぶ。Connect の代わりに Disconnect が出るまで待つ（今の `selectBroker` は Connect を待つので、接続済みのブローカーでは終わらない。`app.ts:528-533`） | seed の `mqttConnections` で接続済みにしたブローカーを選ぶテスト |
| `brokerDisconnectButton` / `mqttStatus(text)` | 接続バーの Disconnect と Connected / Disconnected の表示 | 各 spec の直接のロケーター |
| `subscribeMqtt(topic, qos?)` | Subscriptions パネルから購読して行が出るまで待つ | `messages.spec.ts:61-68`、`integration/mqtt/mqtt.spec.ts:43-48` |
| `mqttTopicInput` / `mqttSubscribeButton` | Subscriptions パネルの入力欄とボタン | 各 spec の直接のロケーター |
| `removeMqttSubscriptionButton(topic)` | 名前 `Remove subscription` で取る | `messages.spec.ts:89` の `.last()` |
| `mqttMessagesAction("Auto" \| "Clear")` | Messages パネルの見出し行のボタン | `messages.spec.ts:290-293・437-440・466-469・518-521・535-538` |
| `mqttMessageRows` | 仮想スクロールの行（`[data-index]`）。DOM 構造に依存する理由をコメントする | `messages.spec.ts:345` |
| `addMqttPresetButton` / `deleteMqttPresetButton(topic)` | 名前 `Add preset` / `Delete preset` で取る | `publish.spec.ts:32-37・108・112` |
| `mqttScanButton` / `mqttStopScanButton` | Broker Topics パネルの Scan / Stop | `mqtt.spec.ts:205-206` |

`mqtt.spec.ts:169-171` の独自の行ロケーターは `app.broker` に置き換える。

### 7. fixture

- `fixtures/ui.ts` に `pageErrors` fixture を足す。使うテストだけが引数に取り、最後に `expect(pageErrors).toEqual([])` で確かめる（自動では検査しない）。`page` fixture に依存するので、収集は最初の `goto` のあとから始まる。`messages.spec.ts:45-49` の `collectPageErrors` と `profiles.spec.ts:187-188・232-233・277-278` のインラインの収集を置き換える。今は操作の途中から収集しているテストもあるので、収集の範囲は少し広がる。
- `messages.spec.ts` のモジュール変数 `connectionId`（`:16`）を、spec の中で `test.extend` した fixture にする（接続して接続 ID を返す）。`beforeEach` は無くす。fixture は自動では動かさず（`auto` にしない）、接続が要るテストが引数に取る。モジュール変数を読んでいた `message()`・`numbered()` は接続 ID を引数で受ける。seed の `mqttConnections` で接続済みにするテストはこの fixture を取らず（取ると `connectBroker` が、接続済みのブローカーには出ない Connect ボタンを待って終わらない）、`selectBroker(name, { connected: true })` で選び、接続 ID は seed に書いた値を使う。
- `messages.spec.ts:25-35` の `message()` の戻り値に `MqttRawMessage & { retained: boolean }` の型を付ける（フロントエンドの domain 型は `retained` を持たないが、Go の `MQTTMessage` は送る）。

### 8. 検証の弱い既存テストの修正（`mqtt.spec.ts`）

- `can create a broker profile`（`:7`）: `fake.args("SaveProfile")` が `id: ""` と既定の URL を持つこと、作ったブローカーが選ばれて接続バー（Connect ボタン）が出ることを足す。
- `can delete a broker profile`（`:56`）: `fake.args("DeleteProfile")` と、空状態（`No brokers yet`、`No active connection...`）に戻ることを足す。
- `can select QoS level 0, 1, and 2`（`:111`）: `toContainText("1")` を `toHaveText` での完全一致に替える。選んだ値が RPC に渡ることは、`messages.spec.ts` の `subscribing and unsubscribing reach the backend`（`:72-94`）が QoS 1 で見ている。そのテストに QoS 2 と、選び直さない QoS 0 の購読を足し、`fake.args("Subscribe")` で 3 つの値を確かめる。ここ（未接続のブローカー）では表示だけを見る。
- 見出しコメントの「観点H-1〜H-6」を、番号を付けない内容の説明に替える。

## 永続化への影響

なし。保存形式（`mqtt-profiles/*.json`、localStorage のキーと値）は変えない。`GetAll` の順序は RPC の戻り値だけに関わり、ファイルの読み書きには関わらない。

## コード生成

不要（バインド対象の Go 構造体・メソッドのシグネチャ、イベントとも変更なし）。

## テスト方針

### Go ユニット

- `cached_store_test.go`: `GetAll` が ID の昇順で返す（3 件以上を逆順に入れて確かめる。何度呼んでも同じ順）。

- `tools/e2e-broker/main_test.go`
  - 拒否の hook: 登録したフィルターへの購読だけを拒否し、publish と他のフィルターは通す。解除すると通る。登録・解除・判定を複数の goroutine から同時に呼ぶ（`task go:test:race` で確かめる）。
  - 操作用 HTTP（`httptest`）: `POST /deny` は `filter` が空なら 400、`DELETE /deny` で登録が空になる。`POST /disconnect-clients` は、クライアントがいなくても 204 を返す。

### Go 統合

追加なし。`internal/integration/` に `GetProfiles`・`GetTargets` の順序を前提にした箇所が無いことを実装時に確かめる。

### フロント ユニット

- `connections.test.ts`
  - `handleReconnect` の実行中にもう一度呼んでも `connect` は 1 回で、タブは 1 つ。`handleConnect` も同じ。失敗したあとは、もう一度呼べる。
  - `connect` が解決する前に `mqtt:connected` が届いても、タブは Connected になる。`mqtt:connection-failed` が先に届いたら未接続のままで、通知は 1 回。
  - `connect` が解決する前に `mqtt:connected`、続けて `mqtt:connection-lost` が届いたら、タブは未接続のまま。
  - `restore()` の `getConnections` が解決する前に `mqtt:connected` が届いたら、未接続で返ってきた接続のタブは Connected になる。`mqtt:connection-lost` が届いたら、接続済みで返ってきた接続のタブは未接続になる。
  - `connect` も `getConnections` も実行していないときに届いた、タブの無い接続 ID のイベントは覚えない。
  - 接続の結果を待つ間に別のブローカーへ切り替えても、`mqtt:connected` は元のブローカーのタブに反映される。
- `subscriptions.test.ts`: 重複するトピックは `subscribe` を呼ばずに入力欄を消す。前後の空白を除いて送る。空白だけなら何もしない。成功したら入力欄を消す。引数でトピックを渡したとき（スキャン結果からの購読）は入力欄を消さない。

### UI e2e（優先度 中）

`ui/mqtt/messages.spec.ts`

| テスト名 | 見ること |
| --- | --- |
| `disconnect disables subscribe and publish, and Connect re-subscribes the kept topics` | 購読してスキャンを始める → Disconnect。Disconnected になり URL の入力欄が戻る、Subscribe と Publish が無効、購読の行は残る、Scan の表示に戻る、`mqttConnections` が空。Connect で新しい接続 ID に `Subscribe` が送られ Connected に戻る |
| `reload restores a connected broker with its subscriptions and stops a running scan` | 購読してスキャン中にリロード。Connected と購読の行が戻る、`StopTopicScan` が接続 ID で呼ばれ Scan の表示で戻る、`scanning` が false |
| `a lost connection shows one toast and returns to Connected when the backend reconnects` | `mqtt:connection-lost` を 2 回流す。トースト `MQTT connection lost` は 1 つ、Disconnected、購読の行は残る。`mqtt:connected` で Connected に戻る。画面の反応だけを見る（流したイベントは偽バックエンドの状態を変えないので、リロードも `snapshot()` の検証もしない） |
| `publishing to a topic with a wildcard shows the backend error` | トピック `devices/#`。トースト `Failed to publish message` に `invalid topic: must not contain wildcards (+ or #)` |
| `a binary payload is listed by its size and shown as hex` | `payloadBase64: true`、`payload: "3q2+7w=="`。一覧は `[binary 4 bytes]`、詳細は `Binary content (4 bytes)` と hex ダンプ（`de ad be ef`） |
| `subscribing to an already subscribed topic sends nothing and clears the field` | 前後に空白のあるトピックは空白を除いて送る。同じトピックをもう一度入れると `Subscribe` の回数が増えず入力欄が空になる。空白だけなら何も送らない |

`ui/mqtt/profiles.spec.ts`

| テスト名 | 見ること |
| --- | --- |
| `deleting a connected broker disconnects it and closes its tab` | 2 件のうち接続中の方を削除。`fake.args("Disconnect")` が接続 ID、`mqttConnections` が空、行が消え、残りのブローカーが選ばれて接続バーにその URL が出る |
| `changing the scheme resets the port to its default and saves the broker URL` | 接続バーで `mqtts` → ポート 8883、`ws` → 9001。`SaveProfile` の最後の引数と一覧の URL。ダイアログでもスキームを変えるとポートが替わり、Save でその URL が保存される |
| `client id, credentials and TLS from the dialog reach SaveProfile and Connect` | 新規のダイアログで全欄を入れて Save & Connect。`SaveProfile` の引数、`Connect` の引数（`profileId` が保存後の ID、`useTls: true`） |
| `edit dialog loads the saved client id, credentials and TLS` | seed に全フィールドを入れたプロファイル。編集ダイアログの各欄に出る |
| `Connect rejects a scheme that cannot be used with TLS` | `page.evaluate` で `Connect({ broker: "http://...", useTls: true, ... })`。`invalid broker URL: scheme cannot be used with TLS` で失敗し、接続が増えない |

`ui/mqtt/mqtt.spec.ts`

| テスト名 | 見ること |
| --- | --- |
| `starting a scan on a disconnected broker shows an error and returns to Scan` | 未接続のブローカーで Scan。トースト `Failed to start topic scan` に `connection not found`、Scan の表示に戻る |
| `a scanned topic can be subscribed from the list` | `mqtt:scan-topic` を流して `+` を押す。`Subscribe(connectionId, topic, 0)`、購読の行、ボタンが `Already subscribed` で無効 |

### UI e2e（優先度 低）

| 観点 | テスト名 | 追加先 | 見ること・使う注入口 |
| --- | --- | --- | --- |
| A | `brokers are still listed as offline when GetConnections fails on startup` | profiles.spec.ts | `getConnectionsError`。ブローカーが一覧に出て、選ぶと Connect が出る。例外もトーストも無い |
| A | `a last active broker that no longer exists leaves the empty state` | profiles.spec.ts | `mqtt:lastActiveProfileId` に無い ID を書いてリロード。どのブローカーも選ばれない |
| A | `a connection whose profile was deleted is restored as a connected tab` | profiles.spec.ts | `mqttConnections` に、プロファイルの無い接続を仕込み、その `profileId` を `mqtt:lastActiveProfileId` に書いてリロード。Connected のタブが選ばれ、接続バーに接続の URL が出る |
| B | `mqtt messages received while another protocol is shown are listed after switching back` | messages.spec.ts | 購読と入力途中のトピックを残して HTTP へ切り替え、`mqtt:message` を流して戻る。Connected・購読の行・入力途中のトピックが残り、メッセージが一覧に出ている |
| C | `cancelling the delete dialog keeps the broker` | profiles.spec.ts | Cancel で行が残り、`DeleteProfile` は 0 回 |
| C | `clicking a broker row selects it and the edit button does not` | profiles.spec.ts | 行（名前の文字）のクリックで接続バーがその URL になる。別の行の Edit ボタンを押しても選択は変わらない |
| D | `broker dialog rejects a whitespace-only name` | profiles.spec.ts | Save と Save & Connect が無効 |
| D | `broker topics list keeps the newest 500 topics` | mqtt.spec.ts | `fake.emitAll` で 501 件。最初のトピックが消え、最後のトピックがある（`ScrollArea` の末尾までスクロールして確かめる） |
| D | `unicode and markup in names, topics and payloads are shown as text` | messages.spec.ts | 日本語・絵文字・`<script>` を含むブローカー名・トピック・ペイロード。文字として表示され、`pageErrors` が空、ダイアログ（`alert`）が開かない |
| D | 既存の `subscribing to an invalid filter shows the backend error` にケースを足す | messages.spec.ts | `a+/b` → `invalid topic: + must occupy an entire level` |
| E | `failed unsubscribe shows an error toast and removes the row` | messages.spec.ts | `unsubscribeError`。トースト `Failed to unsubscribe from <topic>`、行は消える。`mqttConnections` の購読からも消えている |
| E | `failed disconnect shows an error toast and marks the broker disconnected` | profiles.spec.ts | `disconnectError`。トースト `Failed to disconnect`、Disconnected。RPC が届かなかった場合なので、`mqttConnections` には接続が残っている |
| E | `a subscription rejected on reconnect is removed and the rest are kept` | messages.spec.ts | `mqttConnections` に購読 2 件つきの接続を仕込み、片方に `subscribeError`。Disconnect → Connect（購読は確立前に送られる）でトースト `Subscription to <topic> was dropped`、その行だけ消え、`mqttConnections` の購読は残りの 1 件。確立後に同じトピックを購読するとトースト `Failed to subscribe to <topic>` が出て行は増えない |
| E | `stopping a scan that is still starting shows no error` | mqtt.spec.ts | `startTopicScanDelayMs`。Scan → すぐ Stop。トースト無し（止めた開始は `topic scan was stopped` で失敗するが通知しない）、Scan の表示、最後は `scanning` が false |
| E | `a connection result that arrives after switching brokers updates the original broker` | profiles.spec.ts | `mqttConnectResultDelayMs`。Connect を押して別のブローカーへ切り替える。元のブローカーの行が Connected になり、切り替え先は Disconnected のまま |
| E | `pressing Connect twice before the first response leaves one connection` | profiles.spec.ts | `mqttConnectDelayMs`。Connect を 2 回押す。`Connect` は 1 回、`mqttConnections` は 1 件、Connected |
| F | `malformed mqtt values in localStorage fall back to defaults` | publish.spec.ts | 形の壊れた `mqtt:presets`（配列でない／`qos: 3` の要素と正しい要素の混在）・`mqtt:profileOrder`・`mqtt:lastActiveProfileId` を書いてリロード。例外なく起動し、読めるプリセットだけが残る |
| G | `Escape and a backdrop click close the broker dialog without saving` | keyboard-accessibility.spec.ts | どちらも `SaveProfile` は 0 回 |
| G | `Enter in the topic field subscribes` | messages.spec.ts | Enter で `Subscribe` が送られ、行が出て入力欄が空になる |
| G | `Enter in the topic field while disconnected does not add a subscription` | mqtt.spec.ts | 未接続で Enter。`Subscribe` は 0 回、トースト無し、行も増えない（本体の修正後の挙動） |
| G | `Escape while renaming a preset restores its name` | publish.spec.ts | Escape で元の名前に戻る。空の名前で Enter を押しても元の名前のまま。フォーカスした行で Space を押すと選ばれる |
| G | `mqtt subscribe and publish are exposed as tabs with named panels` | keyboard-accessibility.spec.ts | `tabpanel` がタブの名前で引け、id が重複しない |
| H | 既存の `shared subscription shows its messages` にケースを足す | messages.spec.ts | `$queue/sensors/#` でも接頭辞の無いトピックのメッセージが出る |
| H | `removing the filtered subscription resets the topic filter` | messages.spec.ts | Clear で詳細が `Select a message to view details` に戻る。絞り込み中の購読を外すと絞り込みが `All topics` に戻る |
| H | `form edits after deleting the selected preset are not written to another preset` | publish.spec.ts | 選択中のプリセットを消してフォームを編集。残りのプリセット（`mqtt:presets`）は変わらない |

### フルスタック e2e（優先度 中）

`integration/mqtt/mqtt.spec.ts`

| テスト名 | 見ること |
| --- | --- |
| `connecting to an unreachable broker shows a connection failed toast and leaves no connection` | `127.0.0.1:${mqttUnusedPort}` を指すプロファイルで Connect。トースト `MQTT connection failed`、Disconnected のまま、`GetConnections` が空になる（`expect.poll`） |
| `publishing to a topic with a wildcard shows the backend error` | UI e2e と同じ文言。接続は保たれる |
| `reloading the page keeps the connection and still receives subscribed topics` | 購読 → `page.reload()`。Connected と購読の行が戻り、`publishFromBroker` のメッセージが一覧に出る |
| `Connect rejects a scheme that cannot be used with TLS` | UI e2e と同じ呼び出しと文言 |

### フルスタック e2e（優先度 低）

| テスト名 | 追加先 | 見ること |
| --- | --- | --- |
| `editing and deleting a broker updates and removes its file` | integration/common/backend-integration.spec.ts（M-3） | 画面から編集すると同じ `<id>.json` が書き換わり、ファイルの数は増えない。削除でファイルが消える |
| `a retained publish is delivered to a later subscription` | integration/mqtt/mqtt.spec.ts | Publish タブから retain 付き・QoS 1 で送る → あとから購読して受信する。`finally` で retained を消す |
| `a binary message from the broker is listed by its size` | integration/mqtt/mqtt.spec.ts | `publishFromBroker` にバイト列（UTF-8 でない）を渡す。一覧は `[binary N bytes]`、詳細は hex 表示 |
| `a connection dropped by the broker shows the lost toast and recovers` | integration/mqtt/mqtt.spec.ts | 購読 → `disconnectBrokerClients()`。トースト `MQTT connection lost`、自動再接続で Connected に戻り、その後の `publishFromBroker` が届く（購読の成立は retained メッセージで待つ） |
| `a scan stopped by the broker returns to Scan with a toast` | integration/mqtt/mqtt.spec.ts | Scan を押し、`GetConnections` の `scanning` が true になるまで待って（`expect.poll`）から `disconnectBrokerClients()`。トースト `MQTT topic scan stopped`、Scan の表示、`GetConnections` の `scanning` が false。画面の Stop の表示は RPC の完了前に出る（`subscriptions.ts:179-185`）ので、待つ条件にしない。開始中に切ると Go は `mqtt:scan-stopped` を出さず、開始の失敗（`Failed to start topic scan`）になる（`topic_scan.go:222-227`） |
| `a subscription rejected on reconnect is removed and notified` | integration/mqtt/mqtt.spec.ts | 2 件購読 → 片方を `denyBrokerFilter` → `disconnectBrokerClients()`。再接続後にトースト `Subscription to <topic> was dropped`、その行だけ消える |

フルスタックの既存のテスト（Disconnect と再接続、Save & Connect、スキャン結果からの購読）は実通信の確認として残す。

## 副作用・注意事項

- **`GetProfiles` と `GetTargets` が ID の昇順で返るようになる。** 並び順が localStorage に無いプロファイル・ターゲットの表示順が、起動のたびに変わらなくなる。並び順が保存されているものの表示は変わらない。UDP にも及ぶので、`task frontend:test:e2e` で UDP の spec も通ることを確かめる。
- **Connect の応答を待つ間の 2 回目の Connect は無視される。** 応答のあと（確立待ちで Connect ボタンがまだ出ている間）に押した場合は、今と同じく切断してから張り直す。
- **未接続のときのトピック欄の Enter は何もしなくなる**（今は RPC が失敗してトーストが出る）。入力は残る。
- **タブを作る前に届いた接続イベントを反映するようになる。** `Connect` の応答待ちでは実物でまず起きない順序（接続の確立にはネットワークの往復が要る）なので、普段の挙動は変わらない。起動時の `GetConnections` の応答待ちでは、確立待ち・自動再接続中にリロードしたときに起こりうる。
- **再購読の RPC の失敗（`Failed to re-subscribe to <topic>`）は UI e2e では見ない。** Go は確立前の `Subscribe` を登録して成功を返すので、偽バックエンドもそれに合わせ、再接続時の拒否は `mqtt:subscription-dropped` で見る。RPC が失敗する 3 つの分岐は `connections.test.ts:828・854・874` が見ている。
- **`e2e/` は biome の対象外**（`biome.json:14` の `!!**/e2e`）なので、`task format`・`task lint` の biome は e2e のファイルを整形も検査もしない。e2e のファイルを検査するのは `task lint` の中の `tsc -b`（`tsconfig.e2e.json`）だけ。整形は周りのコードに手で合わせる。
- **アクセシブル名の追加で、スクリーンリーダーの読み上げが変わる**（今は名前の無いボタン）。見た目は変わらない。
- **追加するテストが失敗したら、本体の不具合の可能性がある。** レビューは静的に読んだだけで、挙動を実行して確かめていない（未接続での Scan の文言、スキーム変更後の保存、MQTT の `tabpanel` の名前、500 件の上限の表示など）。期待と違う挙動が出たら、テストを挙動に合わせて書き換えず、報告して扱いを決める。
- **`pageErrors` fixture は収集の開始が早くなる。** これまで拾っていなかった例外でテストが落ちたら、同じく報告する。
- **フルスタックの実行時間が延びる。** 接続失敗のテストは閉じたポートへの接続が拒否されるまで（Windows のループバックでは 2 秒ほど）、切断のテストは paho の自動再接続まで待つ。ポート 18832 を別のプロセスが使っていると接続失敗のテストが落ちる。
- **`/disconnect-clients` はブローカーの全クライアントを切る。** フルスタックは 1 つのアプリで順番に実行するので、他のテストには及ばない。
- 接続中編集の計画より先に着手すると、`connections.ts`・`fixtures/app.ts`・`profiles.spec.ts` で衝突する。

## Git運用

- **ブランチ名**: `test/mqtt-e2e-coverage`
- **コミット分割方針**:
  1. `fix(store): GetAll が ID の昇順で返すようにする`（`cached_store.go` とテスト）
  2. `fix(mqtt): プリセットと購読のアイコンボタンにアクセシブル名を付ける`
  3. `fix(mqtt): 実行中の Connect をもう一度送らない`（`connections.ts` とユニットテスト）
  4. `fix(mqtt): 未接続のときはトピック欄の Enter で購読しない`
  5. `fix(mqtt): タブを作る前に届いた接続イベントを反映する`（`connections.ts` とユニットテスト）
  6. `test(mqtt): 購読の入力の扱いとブローカー切り替え中の接続結果のユニットテストを追加する`
  7. `fix(e2e): 偽バックエンドの Connect が useTls を読み、一覧を ID の昇順で返すようにする`
  8. `test(e2e): 偽バックエンドに MQTT の失敗と遅延の注入口を追加する`（`install.ts`・`types.ts`）
  9. `refactor(e2e): MQTT の操作とロケーターを App と fixture に寄せる`（既存 spec の置き換え。テストの内容は変えない）
  10. `test(e2e): MQTT の検証の弱い UI e2e を直す`（`mqtt.spec.ts` の 3 件とコメント）
  11. `test(e2e): 接続済みブローカーの操作の UI e2e を追加する`（優先度 中。`messages.spec.ts`）
  12. `test(e2e): ブローカーの削除・スキーム・ダイアログ・スキャンの UI e2e を追加する`（優先度 中。`profiles.spec.ts`・`mqtt.spec.ts`）
  13. `test(e2e): MQTT の起動時の復元・失敗時の表示・キーボード操作の UI e2e を追加する`（優先度 低）
  14. `test(e2e): e2e ブローカーにクライアントの切断と購読の拒否を追加する`（`tools/e2e-broker` とそのテスト・`fixtures/mqtt-broker.ts`）
  15. `test(e2e): MQTT の接続失敗・切断・リロード・保存ファイルのフルスタック e2e を追加する`
- **完了条件**: すべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task frontend:test:e2e`（UI の振る舞いと偽バックエンドを変えるため）
  - `task go:test:integration`（`GetProfiles`・`GetTargets` の順序が変わるため）
  - `task frontend:test:e2e:fullstack`（フルスタックの spec と `tools/e2e-broker` を変えるため。CI 非対象なのでローカルで実行する）
  - `task go:test:race`（`tools/e2e-broker` の hook は、ブローカーの接続ごとの goroutine と操作用 HTTP から同時に呼ばれるため。`main_test.go` の同時呼び出しのテストをここで確かめる。`GetAll` は今と同じ読み取りロックの中で並べるので、並行処理は変わらない）
