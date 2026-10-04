# 変更計画書: MQTT の接続中ブローカーの編集と接続バーの入力検証

## 概要

MQTT の E2E レビューで見つかった本体の問題 2 件を直す。どちらも一時的な UI e2e（偽バックエンド）を実行して再現を確かめた（確認に使った spec は削除済み）。

**問題 1: 接続中のブローカーを編集ダイアログで Save すると、画面だけが未接続になる**

接続して `sensors/#` を購読した状態で、編集ダイアログから名前を変えて Save した結果:

| 観測 | 値 |
| --- | --- |
| `Disconnect` の呼び出し | 0 回 |
| バックエンドの接続 | 残っている（`connected: true`、購読 `sensors/#` つき） |
| 画面の表示 | Disconnected。購読の行は消える |
| リロード後 | Connected に戻り、購読の行も戻る |

原因は `broker-tree.tsx:62-67` の `handleProfileSave` が `createOfflineConnection` を呼ぶこと。これは同じプロファイルのタブをすべて消して空のオフラインのタブに置き換える（`connections.ts:90-99・265-269`）。接続中でなくても、切断後に残していた購読の行と受信済みメッセージが消える。

**問題 2: 接続バーの入力が検証なしで保存される**

| 操作 | 保存された `broker` | その後 |
| --- | --- | --- |
| ホストを空にする（元は `tcp://alpha.local:1883`） | `tcp://:1883` | リロード後、接続バーは `mqtt` / `localhost` / `1883` を表示する（保存値と食い違う） |
| 上の状態からホストを入れ直し、ポートを `70000` にする | `mqtt://alpha.local:70000` | スキームが `tcp` から `mqtt` に変わっている |

原因は `broker-manager.tsx:50-72` が入力のたびに `updateConnectionBroker` を呼び、`connections.ts:252-263` がそのまま `saveProfile` へ渡すこと。読み戻せない URL は `parseBrokerUrl`（`broker-url.ts:45-53`）が既定値として読むので、次の入力で既定値を元にした URL が上書き保存される。ダイアログ側は `isValidProfileDraft` で防いでいる。

**修正後の挙動**

この文書では、バックエンドに接続が残っている状態（Connected・確立待ち・自動再接続中）をまとめて「接続が生きている」と書く。

1. 接続が生きているブローカーの編集ダイアログでは Save を押せない。押せるのは Save & Connect だけで、保存した設定で張り直す（今の動きのまま）。ダイアログには、Save & Connect で反映する旨を表示する。
2. 接続が生きていないブローカーの Save は、タブを作り直さずにプロファイルだけを差し替える。購読の行と受信済みメッセージは残る。
3. 接続バーは、ホストとポートが読み戻せる値のときだけ保存する。読み戻せない間は保存せず、入力欄にエラー表示を出し、Connect を無効にする。
4. 接続バーの入力欄は、接続が生きている間は編集できない（今は Connected の間だけ）。確立待ち・自動再接続中に宛先を書き換えると、確立したときに繋がっていない宛先を Connected と表示するため。
5. 確立待ち・自動再接続中は、接続バーのボタンが `Connecting…` になり、押すと接続を中止する（今は Connect ボタンで、押すと張り直す。接続を止めるだけの操作は無い）。中止したあとは Save も接続バーの編集もできる。

## 変更対象ファイル

バックエンドとバインド API は変えない。

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| frontend/src/domain/mqtt/types.ts | Frontend domain | 変更 | `OnlineConnectionState` に `closed: boolean` を足す。`hasLiveConnection(conn)` を export する |
| frontend/src/application/mqtt/connection-state.ts | Frontend application | 変更 | `makeOnlineState` が `closed: false` を入れる |
| frontend/src/application/mqtt/broker-url.ts | Frontend application | 変更 | `tryParseBrokerUrl(url)` を追加する（読めない URL は `null`）。`parseBrokerUrl` はこれを呼び、`null` のときに既定値を返す（挙動は今と同じ） |
| frontend/src/application/mqtt/profile-validation.ts | Frontend application | 変更 | `isValidBrokerAddress(host, port)` を export する（今は非公開の `isValidBrokerHost` と `isValidBrokerPort` を合わせたもの）。`isValidProfileDraft` はこれを使う |
| frontend/src/application/mqtt/connections.ts | Frontend application | 変更 | `closed` を切断・接続失敗で立てる。`applySavedProfile(profile)` を追加する。`updateConnectionBroker` は接続が生きているタブと読み戻せない URL を無視する |
| frontend/src/application/mqtt/broker-url.test.ts | テスト | 変更 | `tryParseBrokerUrl` のテスト |
| frontend/src/presentation/providers/mqtt-provider.tsx | Frontend presentation（合成ルート） | 変更 | `ConnectionContextValue` に `applySavedProfile` を足して公開する |
| frontend/src/presentation/components/sidebar/broker-tree.tsx | Frontend presentation | 変更 | `handleProfileSave` が `createOfflineConnection` の代わりに `applySavedProfile` を呼ぶ。編集中のプロファイルの接続が生きているかをダイアログへ渡す |
| frontend/src/presentation/components/mqtt/broker-settings-dialog.tsx | Frontend presentation | 変更 | `connectionLive` の props を足す。真の間は Save を無効にし、Save & Connect で反映する旨の文言を出す |
| frontend/src/presentation/components/mqtt/broker-manager.tsx | Frontend presentation | 変更 | 接続が生きている間は入力欄を `disabled` にする。確立待ち・自動再接続中はボタンを `Connecting…`（押すと中止）にする。入力が有効なときだけ `updateConnectionBroker` を呼ぶ。無効な間は入力欄に `aria-invalid` を付け、Connect を無効にする |
| frontend/src/presentation/components/mqtt/broker.module.css | Frontend presentation | 変更 | ダイアログの文言と、接続バーの入力欄のエラー表示（`aria-invalid` のときの枠線）のスタイル |
| frontend/src/application/mqtt/profile-validation.test.ts | テスト | 変更 | `isValidBrokerAddress` のテスト |
| frontend/src/application/mqtt/connections.test.ts | テスト | 変更 | `closed`・`applySavedProfile`・`updateConnectionBroker` のテスト |
| frontend/src/infrastructure/mqtt/client.test.ts、frontend/src/application/mqtt/publish.test.ts、frontend/src/application/mqtt/subscriptions.test.ts | テスト | 変更 | オンラインのタブをリテラルで組み立てている箇所に `closed` を足す（型エラーになる箇所だけ） |
| frontend/e2e/fake-backend/types.ts、frontend/e2e/fake-backend/install.ts | テスト | 変更 | `seed.mqttConnect` に `"pending"` を足す（接続 ID を返したあと、確立も失敗もしない） |
| frontend/e2e/ui/mqtt/profiles.spec.ts | テスト | 変更 | 問題 1・2 の UI e2e |
| frontend/e2e/fixtures/app.ts | テスト | 変更 | 接続バーのホスト・ポート欄と、ブローカーの編集ダイアログを開く操作を `App` に足す（追加するテストで使う分だけ） |

## 実装方針

### 問題 1: 接続が生きているかを状態に持つ

今の状態では、確立待ち・自動再接続中（バックエンドに接続がある）と、手動切断・接続失敗のあと（無い）を区別できない。どちらも `type: "online"` で `connected: false` になる（`connections.ts:102-109`）。Save を押せなくする範囲を決めるために、この区別を状態に足す。

- `domain/mqtt/types.ts`: `OnlineConnectionState` に `closed: boolean` を足す。バックエンドの接続が終わった（手動切断・接続失敗）ら真。`hasLiveConnection(conn)` は `conn.type === "online" && !conn.closed` を返す。`isConnected` は変えない。
- `connection-state.ts`: `makeOnlineState` は `closed: false`。`restore` が復元するのは `GetConnections` が返した接続だけなので（`connections.ts:208-220`）、復元したタブも `closed: false` でよい（Go は失敗した接続を一覧から外す。`service.go:179-183`）。
- `connections.ts` の `markOffline`: 引数を `{ stopScan }` から `{ closed }` に変える。今の `stopScan: true` の呼び出し（`mqtt:disconnected`・`mqtt:connection-failed`・`handleDisconnect`）は接続が終わる場合と一致するので `closed: true` にし、`connected: false`・`isScanning: false`・`closed: true` を書く。`mqtt:connection-lost` は `closed: false` で、`connected: false` だけを書く（paho が自動再接続する。`service.go:219`）。
- `mqtt:connected`: `closed` のタブでは何もしない。切断の直前に出た確立イベントが遅れて届いても、Connected に戻さない。
- `handleReconnect`: 新しいタブは元のタブをスプレッドして作る（`connections.ts:330-338`）ので、`closed: false` を明示する。

### 問題 1: 接続が生きている間は Save を無効にする

- `broker-tree.tsx` は、編集中のプロファイルのタブに `hasLiveConnection` を当てた結果を `BrokerSettingsDialog` の `connectionLive` に渡す。新規作成（`editingProfile() === "new"`）とタブが無いプロファイルでは偽。サイドバーの状態表示（`isProfileConnected`、`:45-48`）は変えない。
- `broker-settings-dialog.tsx` は `connectionLive` が真の間、Save を `disabled` にし、ボタンの上に `This broker has an active connection. Use Save & Connect to apply changes.` を出す。Save & Connect と Cancel は変えない。props はアクセサ経由で読むので、ダイアログを開いている間に接続が失敗・切断したら Save を押せるようになる。

### 問題 1: `applySavedProfile`

`createConnectionsState`（`connections.ts`）に `applySavedProfile(profile)` を足す。

- そのプロファイルのタブが無い: `createOfflineConnection(profile)` と同じ（オフラインのタブを作って選ぶ）。新規作成はこの経路。
- タブがあり、接続が生きていない（オフライン、または `closed`）: タブの `profile` だけを差し替えて、そのタブを選ぶ。`connectionId`・購読・メッセージは触らない。RPC は呼ばない。
- タブがあり、接続が生きている: 画面からは Save を押せないので通らないが、通った場合に繋がっていない宛先を Connected と表示しないよう、接続を止めてから差し替える。`api.disconnect` を呼び（完了は待たない。失敗は通知する）、`markOffline(connectionId, { closed: true })` のあとで `profile` を差し替えて選ぶ。

`createOfflineConnection` は変えない。`broker-tree.tsx:56` のタブが無いプロファイルをクリックしたときの経路で使い続ける。

タブの `profile` を差し替えるだけで後続の経路は揃う。

- 接続バーの入力欄は `broker-manager.tsx:36-48` の effect が読み直す。
- 次の Connect は `handleReconnect` が `conn.profile` で接続する（`connections.ts:317`）。
- Save & Connect（`broker-tree.tsx:69-80`）は変えない。既存のタブがあれば `handleReconnect(connectionId, saved)` で切断してから張り直し、購読を引き継ぐ。

### 問題 1: 接続バーも接続が生きている間は編集させない

接続バーは Connected でない間、入力欄を出して入力のたびに保存する（`broker-manager.tsx:95-126`）。`updateConnectionBroker` が弾くのは Connected のタブだけ（`connections.ts:254`）なので、確立待ち・自動再接続中に宛先を書き換えられ、ダイアログの Save と同じずれが起きる。

- application（`updateConnectionBroker`）: 弾く条件を `isConnected(conn)` から `hasLiveConnection(conn)` に変える。
- presentation（`broker-manager.tsx`）: `hasLiveConnection(conn())` が真で Connected でない間、スキームの選択とホスト・ポートの入力欄を `disabled` にする。状態の文言（`Disconnected`）は変えない。
- presentation（`broker-manager.tsx`）のボタン: 今は Connected なら Disconnect、それ以外は Connect の 2 通り（`:134-153`）。これを 3 通りにする。
  - Connected: Disconnect（今と同じ）。
  - 接続が生きていて Connected でない（確立待ち・自動再接続中）: `Connecting…`。押すと `handleDisconnect(conn().connectionId)` を呼ぶ。文言だけでは押せることが分からないので、`title="Cancel connection"` を付ける。アクセシブルネームは文言の `Connecting…` のままにする（`aria-label` は付けない）。
  - 接続が生きていない: Connect（今と同じ）。
- 中止に新しい処理は要らない。`handleDisconnect` は `api.disconnect` のあと `markOffline(…, { closed: true })` を呼ぶので、タブは `closed` になり、ボタンは Connect に戻る。Go の `Disconnect` は進行中の Connect を打ち切り（`service.go:239-240`）、打ち切った接続は `mqtt:connected` も `mqtt:connection-failed` も出さない（`service.go:157-170・203-206`）。
- 確立待ち・自動再接続中に接続バーから張り直す操作（今の Connect）は無くなる。張り直したいときは、中止してから Connect を押す。

### 問題 2: 接続バーの入力検証

検証は application と presentation の両方に置く。

- application（`broker-url.ts`）: `parseBrokerUrl` は読めない URL を既定値（`mqtt` / `localhost` / `1883`）として返す（`broker-url.ts:45-47`）ので、戻り値を検証しても `tcp://:1883`・`not-a-url`・`tcp://host:1e3` は既定値として検証を通ってしまう。読めたかどうかを返す `tryParseBrokerUrl(url): BrokerUrlParts | null` を足し、`parseBrokerUrl` は `tryParseBrokerUrl(url) ?? { ...DEFAULT_BROKER }` にする。表示用（既定値で埋める）は `parseBrokerUrl`、保存前の検証は `tryParseBrokerUrl` と使い分ける。
- application（`updateConnectionBroker`）: `tryParseBrokerUrl` が `null` でなく、ホストとポートが `isValidBrokerAddress` を通る URL だけを受け付ける。それ以外は何もしない（タブの `profile` も保存値も変えない）。画面を経由しない呼び出しでも、読み戻せない URL を保存しない。引数は今の `(connectionId, broker)` のままにする。
- presentation（`broker-manager.tsx`）: 入力欄の値はこれまでどおりローカルの signal に持つ（打っている途中の値を巻き戻さない）。`isValidBrokerAddress(host(), port())` が偽の間は `updateConnectionBroker` を呼ばず、ホストとポートの入力欄に `aria-invalid` を付け、Connect を `disabled` にする。Connect は `conn.profile`（最後に有効だった URL）で接続するので、入力欄と違う宛先へ繋がないために無効にする。

無効な入力のまま別のブローカーへ切り替えると、`broker-manager.tsx:36-48` の effect が切り替え先の URL で入力欄を読み直す。戻ってきたときは最後に保存した有効な URL が表示される。

### 依存方向

- 追加する関数は `domain/mqtt/`（`hasLiveConnection`）と `application/mqtt/` の中にあり、`infrastructure/` も `wailsjs/` も import しない。`closed` はフロントエンドだけが持つ画面の状態で、Go の `ConnectionStatus` には無い（バインドは変わらない）。
- `broker-manager.tsx` は `application/mqtt/profile-validation` を import する（presentation → application。`broker-settings-dialog.tsx:9` と同じ）。
- `applySavedProfile` と `updateConnectionBroker` はイベントハンドラから呼ぶ。presentation の effect から application の状態を書き戻す箇所は増えない。
- 新しいポートは要らない。`mqtt-provider.tsx` は `connState.applySavedProfile` を context に足すだけ。

## 永続化への影響

なし。保存形式（`mqtt-profiles/*.json`、localStorage のキー）は変えない。

既に保存されている読み戻せない broker URL は変換しない（「副作用・注意事項」）。

## コード生成

不要（バインド対象の Go 構造体・メソッド、イベントとも変更なし）。

## テスト方針

**Go ユニット / Go 統合**: なし。

**フロント ユニット**

- `profile-validation.test.ts`: `isValidBrokerAddress` が空のホスト、空白・コロンを含むホスト、ポート `0`・`65536`・空・`1e3` を拒否し、`65535` を通す。
- `broker-url.test.ts`: `tryParseBrokerUrl` が `tcp://:1883`・`not-a-url`・`tcp://host:1e3`・未知のスキームで `null` を返し、読める URL では `parseBrokerUrl` と同じ値を返す。`parseBrokerUrl` の既存のテストはそのまま通る。
- `connections.test.ts`:
  - `closed`: 確立待ち・`mqtt:connected` のあと・`mqtt:connection-lost` のあとは `hasLiveConnection` が真、`mqtt:disconnected`・`mqtt:connection-failed`・`handleDisconnect` のあとは偽。`handleReconnect` で張り直したタブは真に戻る。
  - `closed` のタブに `mqtt:connected` が届いても Connected にならない。
  - `applySavedProfile`: 手動切断したオンラインのタブは `connectionId`・購読・メッセージを保ったまま `profile` だけが変わり、`disconnect` を呼ばない。
  - `applySavedProfile`: オフラインのタブは購読の行を保つ。
  - `applySavedProfile`: タブが無ければオフラインのタブを作って選ぶ。
  - `applySavedProfile`: 確立待ちのタブでは `disconnect` を 1 回呼んで `profile` を差し替え、そのあと元の接続 ID の `mqtt:connected` が届いても Connected にならない（編集後の宛先を Connected と表示しない）。
  - `updateConnectionBroker`: 確立待ち・自動再接続中のタブでは `profile` を変えず、`saveProfile` を呼ばない。手動切断したタブでは保存する。
  - `updateConnectionBroker`: `tcp://:1883`・`tcp://host:70000`・`tcp://host:1e3`・`not-a-url` ではタブの `profile` を変えず、`saveProfile` を呼ばない。
  - 既存の `updateConnectionBroker` のテスト（`connections.test.ts:1097-1170`）は有効な URL を使っている。確立待ちのオンラインのタブを書き換えているものがあれば、手動切断したタブかオフラインのタブに直す。

**UI e2e**（`ui/mqtt/profiles.spec.ts`）

確立待ちは今の偽バックエンドでは作れない（`Connect` の次のタスクで必ず `mqtt:connected` か `mqtt:connection-failed` を出す。`install.ts:734-752`）。`seed.mqttConnect: "pending"` を足し、接続を一覧に残したままイベントを出さないようにする（Go で確立待ちが続いている状態と同じ。`GetConnections` は `connected: false` で返す）。

- `Save and the connection bar are locked while a connection is pending`: `mqttConnect: "pending"` で Connect → 編集ダイアログの Save が無効で文言が出ている。Cancel で閉じると、接続バーのホスト・ポート欄が `disabled`。`fake.calls("SaveProfile")` は増えない。接続バーのボタンは `Connecting…`。リロード後も同じ（復元したタブも接続が生きている）。
- `clicking Connecting… cancels a pending connection`: `mqttConnect: "pending"` で購読を足して Connect → `Connecting…` を押す。`Disconnect` が 1 回、`fake.snapshot().mqttConnections` は 0 件、ボタンは Connect に戻り、購読の行は残る。接続バーの入力欄を編集でき、編集ダイアログの Save を押せる。
- `Save becomes available after a failed connection`: 既定の seed（`mqtt:connection-failed`）で Connect → 編集ダイアログの Save を押せ、接続バーの入力欄を編集できる。
- `Save is disabled for a connected broker and Save & Connect reconnects with the edit`: 接続して購読 → 編集ダイアログを開く。Save が無効で、文言が出ている。ホストを変えて Save & Connect を押すと、`Disconnect` が 1 回、`fake.args("Connect")` の最後が編集後の URL、新しい接続 ID で `Subscribe` が送られ、`fake.snapshot().mqttConnections` は 1 件、Connected と購読の行が残る。
- `saving an edit of a disconnected broker keeps its subscription rows`: 接続 → 購読 → Disconnect → 編集して Save（押せる）。エラーの通知は出ず、購読の行が残り、Connect で編集後の URL が `fake.args("Connect")` に渡る。
- `the connection bar does not save a host or port that cannot be read back`: ホストを空にする、ポートを `0`・`65536`・空にする。`SaveProfile` の呼び出しが増えず、保存値が変わらない。入力欄が `aria-invalid`、Connect が無効。有効な値に戻すと保存され、Connect が有効に戻る。リロード後は保存した有効な URL を表示する。
- 既存のテストはそのまま通る見込み: 未接続での編集（`:72・116・136`）は Save を押せる。接続バーのテスト（`:182・435`）は有効な値を入力している。

**フルスタック e2e**: なし（バックエンドの振る舞いは変えない）。

## 副作用・注意事項

- **接続が生きているブローカーは、名前だけの変更でも張り直しになる。** Connected・確立待ち・自動再接続中に設定を変える手段は Save & Connect だけになる。張り直しの間は受信が途切れる（購読は引き継ぐ）。
- **接続が生きていないタブを編集しても購読の行とメッセージが残るようになる。** 今は Save で空のタブに置き換わる。
- **確立待ち・自動再接続中は、接続バーの宛先も編集できなくなる。** 今は編集できる。繋がらない宛先を直したいときは、`Connecting…` を押して中止してから編集するか、編集ダイアログの Save & Connect を使う。
- **確立待ち・自動再接続中の状態の文言は `Disconnected` のまま。** ボタンの `Connecting…` で状態が分かるので、文言の変更はこの変更に含めない。
- **`mqtt:connection-lost` のあと、ボタンは Connect ではなく `Connecting…` になる。** バックエンドが自動再接続を続けている間は、押すと再接続を止める。
- **`handleDisconnect` は `Disconnect` の RPC が失敗しても `closed` にする**（今も失敗時に未接続の表示にしている。`connections.ts:295-307`）。RPC が失敗して接続が残った場合は、Save を押せて、編集前の設定の接続がバックエンドに残る。`mqtt:connected` は `closed` のタブで無視するので、Connected とは表示しない。
- **接続バーで無効な値を打っている間は保存されない。** 無効なまま別のブローカーへ切り替えるか再起動すると、入力は捨てられて最後に保存した URL に戻る。
- **既に保存されている読み戻せない URL は直らない。** 接続バーは既定値（`mqtt://localhost:1883`）を表示し、有効な値を入力した時点で上書き保存される（この動きは今と同じ）。保存値の自動修復はこの変更に含めない。
- **Go の `SaveProfile` は broker URL を検証しない**（`internal/application/mqtt/profile_service.go:35-37`）。画面を経由しない RPC では読み戻せない URL を保存できる。Go 側の検証の追加はこの変更に含めない。

## Git運用

- **ブランチ名**: `fix/mqtt-connected-edit-and-connection-bar-validation`
- **コミット分割方針**:
  1. `fix(mqtt): 接続が生きているブローカーは Save & Connect でだけ編集でき、保存でタブを作り直さない`（`types.ts`・`connection-state.ts`・`connections.ts`・`mqtt-provider.tsx`・`broker-tree.tsx`・`broker-settings-dialog.tsx`・`broker-manager.tsx`・`broker.module.css` とユニットテスト）
  2. `fix(mqtt): 接続バーで読み戻せないホストとポートを保存しない`（`broker-url.ts`・`profile-validation.ts`・`connections.ts`・`broker-manager.tsx`・`broker.module.css` とユニットテスト）
  3. `test(e2e): 接続中ブローカーの編集と接続バーの入力検証の UI e2e を追加する`（`profiles.spec.ts`・`fixtures/app.ts`・`fake-backend/types.ts`・`fake-backend/install.ts`）
- **完了条件**: すべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task frontend:test:e2e`（UI の振る舞いを変えるため）
  - `task go:test:integration`・`task go:test:race`・`task frontend:test:e2e:fullstack` は不要（Go と実バックエンドとの結合に触れない）
