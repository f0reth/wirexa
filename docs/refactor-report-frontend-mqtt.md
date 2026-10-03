# リファクタリング調査レポート（対象: frontend mqtt）

調査範囲は `frontend/src/` の `{domain,application,infrastructure}/mqtt/`、`presentation/components/mqtt/`、`presentation/providers/mqtt-provider.tsx`、`presentation/components/sidebar/{broker-tree,profile-list}.tsx`、`infrastructure/storage/local-storage.ts` の mqtt 部分（テストを除いて約 3,500 行）。パスは `frontend/src/` からの相対で書く。

検証コマンドは全候補で `task format` → `task lint` → `task test` を前提とし、各候補には追加で必要なものだけを書く。

## 改善候補一覧

| 優先度 | ファイル | 内容 |
|--------|----------|------|
| 高 | `infrastructure/mqtt/client.ts` | 送信方向のフィールド列挙と受信方向の `as` キャストを、udp と同じ `createFrom`・スプレッドの変換に揃える |
| 中 | `application/mqtt/connections.ts`・`subscriptions.ts`・`infrastructure/mqtt/events.ts` | イベントのペイロード型を 1 か所にまとめ、8 か所の `as` キャストを無くす |
| 中 | `application/mqtt/connections.ts` | 654 行のファイルを、状態の生成・受信バッファ・接続操作に分ける。あわせてファイル内の重複 4 種をまとめる |
| 中 | `presentation/components/mqtt/publish-tab.tsx`・`presentation/providers/mqtt-provider.tsx` | publish の送信可否の判定と RPC 呼び出しを application 層へ移す |
| 中 | `presentation/components/mqtt/panels/subscriptions-panel.tsx`・`publish-tab.tsx`・`sidebar/broker-tree.tsx`・`application/mqtt/connections.ts` | 接続中かどうかの判定を domain の `isConnected` に揃える |
| 中 | `presentation/components/mqtt/broker-manager.tsx`・`broker-settings-dialog.tsx`・`application/mqtt/broker-url.ts` | スキームの一覧と既定の URL を 1 か所で定義する |
| 低 | `domain/mqtt/types.ts` ほか | QoS の型 `0 \| 1 \| 2` を名前付きの型にし、`as` キャスト 3 か所を型ガードに替える |
| 低 | `application/mqtt/connections.ts` | `MqttConnectionApi` から使っていない `unsubscribe` を外す |
| 低 | `application/mqtt/presets.ts`・`connections.ts`・`infrastructure/mqtt/client.ts` ほか | 参照されていない export・再 export・引数を整理する |
| 低 | `application/mqtt/presets.ts`・`profiles.ts` | 「更新して保存」の繰り返しをヘルパーにまとめる |
| 低 | `presentation/providers/mqtt-provider.tsx` | 4 つの hook の重複と、手書きのコンテキスト型を整理する |
| 低 | `presentation/components/sidebar/broker-tree.tsx` | 薄いラッパー関数と `isActive` を整理する |

## 改善候補の詳細

### 生成型との変換を udp と同じ書き方に揃える
- **該当箇所**: `infrastructure/mqtt/client.ts:18-28`（`connect`）、`:75-85`（`saveProfile`）、`:30-73`（`as` キャスト）
- **現状**:
  - `saveProfile` は `BrokerProfile` の 7 フィールドを列挙して渡している。Go 側の `BrokerProfile` に項目を足しても、ここを直さない限り送信時に黙って落ちる。udp（`infrastructure/udp/client.ts:21-36`）は `createFrom` とスプレッドで素通ししているので、同じ事故が起きない。
  - `connect` も 7 フィールドを列挙している。こちらは `id` → `profileId` の付け替えがあるので変換自体は必要。
  - `Disconnect(...) as Promise<void>` など 6 か所の `as Promise<void>` と `as Promise<string>` は、生成された `MQTTHandler.d.ts` が既にその型を返すので不要。
  - `getConnections`・`getProfiles` は生成クラスを `as` でフロントの型へキャストしている。`saveProfile`・`deleteProfile` はキャストも無く、書き方が 3 通りある。
- **改善案**:
  - `saveProfile` は `SaveProfile(mqttdomain.BrokerProfile.createFrom(profile))` にし、戻り値は `fromWailsProfile`（`{ ...p }`）を通す。
  - `connect` は `mqttdomain.ConnectionConfig.createFrom({ ...profile, profileId: profile.id })` にする。`createFrom` は自分のフィールドだけを拾うので、余分な `id` は渡らない。
  - `getProfiles` は `result.map(fromWailsProfile)`、`getConnections` は `fromWailsConnectionStatus`（`{ ...s, subscriptions: s.subscriptions.map((x) => ({ ...x })) }`）を通す。
  - 不要な `as Promise<void>`・`as Promise<string>` を消す。
  - ファイル先頭の `export type { ConnectionStatus }`（`:16`）は誰も import していないので消す（fake-backend は `domain/mqtt/types` から import している）。
- **期待効果**: Go 側の項目追加で送信時に値が落ちる事故の防止、プロトコル間の書き方の統一
- **挙動を守っているテスト**: `infrastructure/mqtt/client.test.ts`（`passes the profile fields to the backend`・`passes all profile fields to the backend`・`returns the saved profile from the backend`）。引数は `toHaveBeenCalledWith` でオブジェクトの中身を比べているので、クラスのインスタンスに変わっても通る見込み（実行して確かめる）。e2e は `e2e/ui/mqtt/mqtt.spec.ts`・`profiles.spec.ts`
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し（生成物・fake-backend は変えない）

### イベントのペイロード型を 1 か所にまとめる
- **該当箇所**: `application/mqtt/connections.ts:53-56`（`MqttEventListener`）、`:67-79`（`ScannedTopic`・`RawMessage`）、`:282`・`:290`・`:297-300`・`:309`・`:317`・`:327-330`・`:343-346`（キャスト）、`application/mqtt/subscriptions.ts:17-21`・`:54`、`infrastructure/mqtt/events.ts:7-12`
- **現状**: イベントのハンドラは `(data: unknown) => void` で、受け取る側が 8 か所で `as` キャストしている。`{ connectionId: string; error: string }` は 3 か所、`{ connectionId: string }` は 2 か所にインラインで書かれている。ペイロードの型は application の 2 ファイルに散らばっていて、Go 側（`internal/domain/mqtt/types.go`）との対応が追いにくい。
- **改善案**: `domain/mqtt/types.ts` にペイロードの型（`MqttRawMessage`・`ScannedTopic`・`SubscriptionDropped`・`ConnectionEvent`・`ConnectionErrorEvent`）と、イベント名からペイロード型への対応表 `MqttEventPayloads` を置く。`MqttEventListener` を `<E extends MqttEventName>(event: E, handler: (data: MqttEventPayloads[E]) => void) => () => void` にする。`unknown` からのキャストは `infrastructure/mqtt/events.ts` の 1 か所だけになる。検証（型ガード）を足すかどうかは別の判断で、足すと不正なペイロードを捨てる挙動が加わるため、この候補には含めない。
- **期待効果**: 型安全性の向上、重複するインライン型の削減、Go の型との対応の明確化
- **挙動を守っているテスト**: `application/mqtt/connections.test.ts`・`subscriptions.test.ts`（どちらも自前の `onEvent` と `emit` を持つので、ヘルパーの型を合わせる必要がある）、`infrastructure/mqtt/events.test.ts`、`e2e/ui/mqtt/messages.spec.ts`
- **検証コマンド**: 追加で `task frontend:tsc`
- **付随作業**: `connections.ts:51` と `events.ts:4` の `MqttEventName` の再 export を使っているテスト 3 本の import 元を `shared/wails-events` に直すかどうかを決める。fake-backend への影響は無し

### `connections.ts` を責務で分け、ファイル内の重複をまとめる
- **該当箇所**: `application/mqtt/connections.ts`（654 行）
- **現状**: 1 つの `createConnectionsState` に、状態の生成、受信バッファ、イベントの購読、復元、接続操作が入っている。加えて次の重複がある。
  - `makeOfflineState`（`:86-100`）と `makeOnlineState`（`:115-133`）が、ランタイム状態の 7 項目の初期値を 2 回書いている。
  - `flushTopics`（`:192-201`）と `flushMessages`（`:230-239`）が、接続 ID ごとにまとめる同じループを書いている。
  - `createOfflineConnection`（`:460-467`）と `handleConnect`（`:481-488`）が、「同じプロファイルのタブを消して新しいタブを入れる」同じ `produce` を書いている。
  - 「オンラインのときだけ `connected: false`（と `isScanning: false`）にする」更新が 4 か所にある（`:318-321`・`:333-336`・`:349-352`・`:516-519`）。
  - 受信メッセージと購読の照合（`:249-255`）が、購読の作り方を知っている `subscription.ts` ではなくここにある。
- **改善案**: 公開シンボルと挙動は変えずに、同じ `application/mqtt/` の中で分ける。
  - `connection-state.ts`: `ConnectionRuntimeState`・`ConnectionStateExt`・`offlineId`・`makeOfflineState`・`makeOnlineState`・`synthesizeProfile`。初期値は `emptyRuntimeState()` にまとめる。型は `connections.ts` から再 export して、既存の import を変えない。
  - `message-buffer.ts`: `messageBuffer`・`topicBuffer`・`scheduleFlush`・`flushTopics`・`flushMessages` を `createEventBuffer(updateConnection, maxMessages, maxTopics)` として切り出し、`pushMessage`・`pushTopic` を返す。まとめるループは `groupBy` のローカル関数にする。
  - `subscription.ts`: `subscriptionMatches(sub, topic, topicParts)` を足し、`flushMessages` から呼ぶ。
  - `connections.ts` に残る接続操作では、`replaceProfileTab(profileId, entry)` と `markOffline(connId, { stopScan })` をローカル関数にする。
- **期待効果**: 可読性の向上、重複の削減、受信バッファを単体でテストできるようになる
- **挙動を守っているテスト**: `application/mqtt/connections.test.ts`（1,310 行。復元・接続・再接続・バッファの上限・イベントの解除を検証している）、`e2e/ui/mqtt/messages.spec.ts`（`message cap` の 2 本・`muted subscription hides its messages`・`shared subscription shows its messages`）、`e2e/ui/mqtt/profiles.spec.ts`
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し

### publish の判定と送信を application 層へ移す
- **該当箇所**: `presentation/components/mqtt/publish-tab.tsx:174-180`、`presentation/providers/mqtt-provider.tsx:168-179`
- **現状**: 「トピックが空なら送らない」「retain 付きの空ペイロードは retained メッセージの削除なので許可する」という業務上の判定がコンポーネントにあり、RPC の呼び出しと失敗時の通知は Provider にある。publish の RPC は application のポートを通らず、Provider が `mqttClient.publish` を直接呼んでいる。udp は `application/udp/send.ts` の `createUdpSendState` が `UdpSendApi` を受けて同じ役割を持っている。この判定にはユニットテストが無い。
- **改善案**: `application/mqtt/publish.ts` に `MqttPublishApi`（`publish` だけ）と `createPublishState(api, activeConnectionId, draft, notifier)` を置き、`canPublish(draft)`（純粋関数）と `publishDraft()` を返す。Provider は注入だけを行い、`PublishForm` は `publishDraft()` を呼ぶ。コンテキストの `publish(topic, payload, qos, retain)` は `PublishForm` からしか呼ばれていないので、置き換えてよい。
- **期待効果**: presentation へのロジック漏れの解消、udp との構造の統一、判定をユニットテストできるようになる
- **挙動を守っているテスト**: `e2e/ui/mqtt/messages.spec.ts`（`publish sends topic, QoS, retain and payload`）、`e2e/ui/mqtt/publish.spec.ts`（`publish button is disabled while offline`）。空トピック・空ペイロードの判定を検証するテストは無し（先に `canPublish` のユニットテストを追加）
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し（バインド API の呼び方は変わらない）

### 接続中かどうかの判定を `isConnected` に揃える
- **該当箇所**: `presentation/components/mqtt/panels/subscriptions-panel.tsx:28-31`、`presentation/components/mqtt/publish-tab.tsx:169-172`、`presentation/components/sidebar/broker-tree.tsx:41-44`、`application/mqtt/connections.ts:449`
- **現状**: `domain/mqtt/types.ts:77` に `isConnected(conn)` があるのに、使っているのは `broker-manager.tsx` だけ。ほかの 4 か所は `conn?.type === "online" && conn.connected` を書き直している。パネルの 2 か所は同じ名前のローカル関数 `isConnected` を定義していて、domain の関数と紛らわしい。
- **改善案**: 4 か所を domain の `isConnected` に替える。未選択（`null`）を扱うパネル側は `const conn = activeConnection(); return conn !== null && isConnected(conn);` とする。
- **期待効果**: 重複の削減、判定の基準を 1 か所にする
- **挙動を守っているテスト**: `e2e/ui/mqtt/mqtt.spec.ts`（`subscribe button is disabled when broker is not connected`）、`e2e/ui/mqtt/publish.spec.ts`（`publish button is disabled while offline`）、`application/mqtt/connections.test.ts`（`updateConnectionBroker`）
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し

### スキームの一覧と既定の URL を 1 か所で定義する
- **該当箇所**: `application/mqtt/broker-url.ts:1-7`・`:18-19`、`presentation/components/mqtt/broker-manager.tsx:105-109`、`presentation/components/mqtt/broker-settings-dialog.tsx:29-31`・`:56-70`・`:121-125`、`application/mqtt/profiles.ts:12`
- **現状**:
  - スキームの一覧（mqtt・mqtts・tcp・ws・wss）が、`DEFAULT_PORT_MAP` のキー、`parseBrokerUrl` の正規表現、2 つのコンポーネントの `<option>` の計 4 か所にある。スキームを足すときに 1 か所でも漏れると、選べるのに読み戻せない値ができる。
  - 既定の URL `mqtt://localhost:1883` が、`createEmptyProfile`、ダイアログの初期値、`parseBrokerUrl` の失敗時の戻り値の 3 か所にある。
  - ダイアログの `handleSave` と `handleSaveAndConnect` が、同じプロファイルの組み立てを 2 回書いている。
- **改善案**:
  - `broker-url.ts` に `BROKER_SCHEMES`（`as const` の配列）を export し、`DEFAULT_PORT_MAP` の型と正規表現をそこから作る。2 つのコンポーネントは `<For each={BROKER_SCHEMES}>` で `<option>` を描く。
  - `DEFAULT_BROKER_URL` を `broker-url.ts` に置き、`createEmptyProfile` と `parseBrokerUrl` の失敗時の戻り値で使う。ダイアログは `props.profile ?? createEmptyProfile()` を 1 回作り、その `broker` を `parseBrokerUrl` に渡す。
  - ダイアログに `buildProfile()` を作り、2 つのハンドラから呼ぶ。
- **期待効果**: 同期漏れの防止、重複の削減
- **挙動を守っているテスト**: `application/mqtt/broker-url.test.ts`、`application/mqtt/profiles.test.ts`（`createEmptyProfile`）、`e2e/ui/mqtt/mqtt.spec.ts`（`new broker dialog rejects a port or host that cannot be read back`）、`e2e/ui/mqtt/profiles.spec.ts`（`edit broker loads the saved values and updates the profile`・`editing the connection bar after switching saves only the selected broker`）
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し（`broker.module.css` のクラスはそのまま使う）

### QoS を名前付きの型にする
- **該当箇所**: `domain/mqtt/types.ts:27`・`:44`・`:54`、`application/mqtt/presets.ts:13`、`application/mqtt/subscription.ts:19`、`application/mqtt/connections.ts:262`、`presentation/components/mqtt/publish-tab.tsx:197`、`presentation/components/mqtt/qos-select.tsx:11-13`・`:19`、`infrastructure/storage/local-storage.ts:58`
- **現状**: `0 | 1 | 2` を 4 か所に直接書き、`number` から `as 0 | 1 | 2` へのキャストが 3 か所ある。`local-storage.ts:58` は同じ条件を手で書いている。`QosSelect` は `number` で受け渡すので、呼び出し側がキャストしている。
- **改善案**: `domain/mqtt/types.ts` に `type Qos = 0 | 1 | 2` と `isQos(v: unknown): v is Qos` を置き、型の記述と `local-storage.ts` の検査で使う。`QosSelect` の `value`・`onChange` を `Qos` にし、`parseInt` の結果を `isQos` で確かめてから渡す。`newQos` の signal と `addSubscription`・`makeSubscription` の引数も `Qos` にする。RPC とイベントから来る `number`（`connections.ts:262`・`:411`）は、今と同じ結果になるようキャストを残すか、`isQos` で確かめるかを決める（確かめて捨てると挙動が変わるので、この候補ではキャストを残す）。
- **期待効果**: 型安全性の向上、重複の削減
- **挙動を守っているテスト**: `application/mqtt/subscription.test.ts`、`application/mqtt/presets.test.ts`、`infrastructure/storage/local-storage.test.ts`、`e2e/ui/mqtt/mqtt.spec.ts`（`can select QoS level 0, 1, and 2`）
- **検証コマンド**: 追加で `task frontend:tsc`・`task frontend:test:e2e`
- **付随作業**: 無し

### `MqttConnectionApi` から使っていない `unsubscribe` を外す
- **該当箇所**: `application/mqtt/connections.ts:62`
- **現状**: `createConnectionsState` は `api.connect`・`disconnect`・`subscribe`・`stopTopicScan`・`getConnections` しか呼ばない。`unsubscribe` はポートにあるだけで使われていない（購読の解除は `subscriptions.ts` の `SubscriptionApi` が受け持つ）。
- **改善案**: `MqttConnectionApi` から `unsubscribe` を消す。`mqttClient` は構造的に満たすので Provider は変わらない。
- **期待効果**: ポートの肥大化の解消
- **挙動を守っているテスト**: `application/mqtt/connections.test.ts`（`:46` のフェイク API から `unsubscribe` を消す）
- **検証コマンド**: 追加で `task frontend:tsc`
- **付随作業**: 無し

### 参照されていない export・再 export・引数を整理する
- **該当箇所と現状**:
  - `application/mqtt/presets.ts:57-67`・`mqtt-provider.tsx:97` の `savePreset`: 呼び出すのは `presets.test.ts:147` だけで、コンポーネントからは使われていない。
  - `application/mqtt/presets.ts:89` の `addPreset(name?)`: 引数を渡すのは `presets.test.ts:173` だけ。
  - `presentation/components/mqtt/publish-tab.tsx:27`・`:227-232`: `PresetsPanel` は自分で `useMqttPublish()` を呼んでいるのに、`addPreset` だけを親から props で受けている。
  - `mqtt-provider.tsx:81` の `messages`: コンテキストの型にあるが、コンポーネントは `visibleMessages` しか使わない（`createMessagesState` の戻り値としては `messages.test.ts` が使う）。
  - `application/mqtt/connections.ts:27` の `export type { ConnectionPersistence }`、`application/mqtt/presets.ts:7` の `export type { PresetStorage }`: 誰も import していない（テストも `domain/mqtt/ports` から import している）。
  - `application/mqtt/connections.ts:47-48` の `OfflineStateExt`・`OnlineStateExt`: ファイル内でしか使われていない。
  - `application/mqtt/profile-validation.ts:4` の `ProfileDraftInput`: ファイル内でしか使われていない。
  - `application/mqtt/connections.ts:503` の `handleDisconnect(connectionId?)`: コンポーネントは必ず ID を渡す（`broker-manager.tsx:150`）。省略するのは `connections.test.ts:763`・`:776` だけ。
- **改善案**: 再 export 2 つと不要な `export` 3 つを消す。`PresetsPanel` の props を消して `useMqttPublish()` から `addPreset` を取る。`savePreset`・`addPreset` の引数・`handleDisconnect` の省略可能な引数・コンテキストの `messages` は、消すならテストも合わせて直す（残すなら、使う予定を書いたコメントを付ける）。
- **期待効果**: 公開面の縮小、可読性の向上
- **挙動を守っているテスト**: `application/mqtt/presets.test.ts`、`application/mqtt/connections.test.ts`、`e2e/ui/mqtt/publish.spec.ts`（`presets can be added, selected, renamed and deleted`）
- **検証コマンド**: 追加で `task frontend:tsc`・`task frontend:test:e2e`
- **付随作業**: 無し

### 「更新して保存」の繰り返しをまとめる
- **該当箇所**: `application/mqtt/presets.ts:60-64`・`:73-77`・`:81-85`・`:99-103`・`:110-115`、`application/mqtt/profiles.ts:43-51`・`:57-61`・`:65-70`
- **現状**: `setPresets((prev) => { const next = ...; storage.save(next); return next; })` が 5 回、`profiles.ts` の同じ形が 3 回ある。setter の updater の中で保存という副作用を起こしている。`savePreset` と `addPreset` は末尾への追加と選択を別々に書いている。`PublishDraft`（`:10-15`）は `PublishPreset` から `id`・`name` を除いたものと同じ形で、`loadDraftFromPreset`（`:31-36`）と `emptyDraft`・`addPreset` の初期値がフィールドを列挙している。
- **改善案**: `presets.ts` に `commit(update: (prev) => PublishPreset[] | null)`（次の値を計算し、signal に入れてから保存する）を作り、5 か所から呼ぶ。`profiles.ts` も同じ形の `commit` で順序を保存する。`PublishDraft` は `Omit<PublishPreset, "id" | "name">` として定義し、型の同期漏れを防ぐ。
- **期待効果**: 重複の削減、保存の呼び出しを 1 か所にする
- **挙動を守っているテスト**: `application/mqtt/presets.test.ts`、`application/mqtt/profiles.test.ts`（`saves the order when profiles are reordered or deleted`・`does not save when the indices are out of range`）、`e2e/ui/mqtt/publish.spec.ts`（`publish presets survive a reload`）、`e2e/ui/mqtt/mqtt.spec.ts`（`broker order`）
- **検証コマンド**: 追加無し
- **付随作業**: 無し

### Provider の hook とコンテキスト型を整理する
- **該当箇所**: `presentation/providers/mqtt-provider.tsx:62-115`・`:212-236`
- **現状**: 4 つの hook が、同じ `useContext` と例外の送出を書いている。`SubscribeContextValue`・`MessagesContextValue`・`PublishContextValue` は、`createSubscriptionsState`・`createMessagesState`・`createPresetsState` の戻り値をそのままスプレッドしたものなのに、型を手で書き写している（application 側に項目を足しても、ここを直さないとコンポーネントから見えない）。
- **改善案**: `useMqttContext(hookName)` を作って 4 つの hook から呼ぶ。3 つの型は `ReturnType<typeof createXxxState>` から作る（`PublishContextValue` は publish の分を足す）。`ConnectionContextValue` は `ConnectionStateExt` を `ConnectionState` に狭めて公開する意図があるので、手書きのままにする。
- **期待効果**: 重複の削減、型の同期漏れの防止
- **挙動を守っているテスト**: `e2e/ui/mqtt/*.spec.ts` の全体（Provider のユニットテストは無いが、型の変更なので `tsc` が検出する）
- **検証コマンド**: 追加で `task frontend:tsc`・`task frontend:test:e2e`
- **付随作業**: 無し。udp の Provider（`udp-provider.tsx`）も型を手で書いているので、揃えるなら udp 側は別の調査で扱う

### `broker-tree.tsx` のラッパー関数を整理する
- **該当箇所**: `presentation/components/sidebar/broker-tree.tsx:46-50`・`:87-92`
- **現状**: `getConnectionForProfile` と `getConnectionIdForProfile` は `connectionByProfileId().get(...)` を包むだけで、後者は 1 か所からしか呼ばれない。`isActive` は `activeConnectionId()` から `connections` を引き直しているが、コンテキストには同じ結果を返す `activeConnection()` がある。
- **改善案**: `getConnectionIdForProfile` を消して `getConnectionForProfile(id)?.connectionId` にする。`isActive` は `activeConnection()?.profileId === profileId` にする。
- **期待効果**: 可読性の向上
- **挙動を守っているテスト**: `e2e/ui/mqtt/mqtt.spec.ts`（`can delete a broker profile`）、`e2e/ui/mqtt/profiles.spec.ts`（`multiple brokers`）、`e2e/ui/common/keyboard-accessibility.spec.ts`（`Enter on a focused broker selects it`）
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し（`profile-list.tsx` は変えないので udp への影響は無い）

## 対応不要と判断した箇所（理由つき）

- **`domain/mqtt/types.ts` と Go の `internal/domain/mqtt/types.go` の型の重複**: 意図的な重複。Go の型が配線型の正で、フロントは `Subscription`・`ConnectionState` などの UI 向けの型を足す層。
- **`domain/mqtt/topic.ts` の `stripSharedPrefix` とバックエンドの `filterMatches`**: 両側で同じ規則を持つ必要があり、コメント（`topic.ts:7-8`）で対応付けている。削除や統合はできない。
- **`application/mqtt/profile-validation.ts` のポート・ホストの検証**: 送信前の先回り検証。`parseBrokerUrl` で読み戻せる形に限るというフロント固有の理由もコメントに書かれている。
- **`application/mqtt/connections.ts:9` の `application/logger` の `Logger`**: application に置かれたポートとして扱う現行の設計。
- **`application/mqtt/` が signal・store で状態を持つこと**: 現行の設計。
- **`application/mqtt/profiles.ts` と `application/udp/targets.ts` の共通化**: 並び順の読み込みは既に `applyOrder`・`moveItem` を共有している。残りは挙動が違う（mqtt は signal を手元で更新し、削除の失敗を呼び出し側に伝える。udp は store を RPC で読み直し、削除の失敗を通知だけで終える）ので、まとめると挙動が変わる。
- **`sidebar/profile-list.tsx`**: mqtt と udp で既に共通化されている。相違点は props で受けていて、追加でまとめる箇所は無い。
- **`broker-settings-dialog.tsx` と udp の `TargetDialog`（`target-tree.tsx`）の共通化**: 入力項目・検証・ボタンの構成が違い、`profile-list.tsx:38-40` のコメントも編集ダイアログは共通化しないと決めている。
- **`broker-manager.tsx:35-47` の effect**: 書き換えるのはコンポーネント内の入力欄の signal だけで、application の状態は書き戻していない。規約の範囲内で、理由もコメントに書かれている。
- **`panels/messages-panel.tsx:47-53` の effect**: スクロールの追従だけを行う。選択の追従は `application/mqtt/messages.ts` の 1 か所にある。
- **`application/mqtt/subscriptions.ts` の `setIsScanning`（`:173-231`）**: 長いが、開始と停止の順序を守るための処理で、分けると順序の前提が読み取りにくくなる。`subscriptions.test.ts` が順序を細かく検証している。
- **`application/mqtt/connections.ts:331`・`:347` の `console.error`**: 注入された `logger` を使わず、コンソールに出している。`logger.error` に替えるとログの出力先（バックエンドのログファイル）が変わるので、挙動変更にあたる。替えるかどうかは別に決める。
- **`infrastructure/storage/local-storage.ts` の mqtt 部分**: `ConnectionPersistence`（`loadLastProfileId` など）だけメソッド名がほかのポート（`load`・`save`）と違うが、実害が無く、揃える効果が小さい。`StoredPreset`（`:49`）は `retain` 導入前の保存値を読むためのもので必要。
- **`presentation/components/mqtt/utils.ts` の `getTopicColor`**: 表示用の色の計算で、presentation に置くのが適切。
- **`application/mqtt/connections.ts` の `switchConnection`**: `setActiveConnectionId` を包むだけだが、setter をそのまま公開しないための入口として残す。
