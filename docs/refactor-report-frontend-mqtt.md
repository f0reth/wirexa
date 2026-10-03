# リファクタリング調査レポート（対象: frontend mqtt）

調査範囲は `frontend/src/` の `{domain,application,infrastructure}/mqtt/`、`presentation/components/mqtt/`、`presentation/providers/mqtt-provider.tsx`、`presentation/components/sidebar/{broker-tree,profile-list}.tsx`、`infrastructure/storage/local-storage.ts` の mqtt 部分（テストを除いて約 3,500 行）。パスは `frontend/src/` からの相対で書く。

検証コマンドは全候補で `task format` → `task lint` → `task test` を前提とし、各候補には追加で必要なものだけを書く。

## 改善候補一覧

| 優先度 | ファイル | 内容 |
|--------|----------|------|
| 中 | `infrastructure/mqtt/client.ts` | 不要な `as` キャスト 9 か所を無くし、生成型との受け渡しを型検査が効く書き方に揃える |
| 中 | `application/mqtt/connections.ts`・`subscriptions.ts`・`infrastructure/mqtt/events.ts` | イベントのペイロード型を 1 か所にまとめ、8 か所の `as` キャストを無くす |
| 中 | `application/mqtt/connections.ts` | 656 行のファイルを、状態の生成・受信バッファ・接続操作に分ける。あわせてファイル内の重複 4 種をまとめる |
| 中 | `presentation/components/mqtt/publish-tab.tsx`・`presentation/providers/mqtt-provider.tsx` | publish の送信可否の判定と RPC 呼び出しを application 層へ移す |
| 中 | `presentation/components/mqtt/panels/subscriptions-panel.tsx`・`publish-tab.tsx`・`sidebar/broker-tree.tsx`・`application/mqtt/connections.ts` | 接続中かどうかの判定を domain の `isConnected` に揃える |
| 中 | `presentation/components/mqtt/broker-manager.tsx`・`broker-settings-dialog.tsx`・`application/mqtt/broker-url.ts` | スキームの一覧と既定の URL を 1 か所で定義する |
| 低 | `domain/mqtt/types.ts` ほか | QoS の型 `0 \| 1 \| 2` を名前付きの型にし、UI 内部の `as` キャストを型ガードに替える（RPC・イベントの境界のキャストは残す） |
| 低 | `application/mqtt/connections.ts` | `MqttConnectionApi` から使っていない `unsubscribe` を外す |
| 低 | `application/mqtt/presets.ts`・`connections.ts`・`infrastructure/mqtt/client.ts` ほか | 参照されていない export・再 export・引数を整理する |
| 低 | `application/mqtt/presets.ts`・`profiles.ts` | 「更新して保存」の繰り返しをヘルパーにまとめる |
| 低 | `presentation/providers/mqtt-provider.tsx` | 4 つの hook の重複と、手書きのコンテキスト型を整理する |
| 低 | `presentation/components/sidebar/broker-tree.tsx` | 薄いラッパー関数と `isActive` を整理する |

## 改善候補の詳細

### 生成型との受け渡しを、型検査が効く書き方に揃える
- **該当箇所**: `infrastructure/mqtt/client.ts:18-28`（`connect`）、`:75-85`（`saveProfile`）、`:30-73`（`as` キャスト）
- **現状**:
  - `saveProfile` と `connect` は 7 フィールドを列挙して渡している。引数の型は生成された `mqttdomain.BrokerProfile`・`mqttdomain.ConnectionConfig` で、フィールドは必須なので、Go 側に項目を足して `task wails:generate` を実行すれば、列挙に足りない項目は `tsc` がエラーにする（黙って落ちることはない。`omitempty` 付きで省略可能になる項目だけは検出されない）。
  - udp（`infrastructure/udp/client.ts:21-36`）は送信方向に `createFrom` を使っている。`createFrom` は引数を `any` で受ける（`wailsjs/go/models.ts:381`）ので、フロントの型に無い項目は検査されずに `undefined` になり、JSON 化で落ちる。送信漏れの検出という点では、mqtt の今の列挙のほうが強い。
  - `as Promise<void>` 6 か所と `as Promise<string>` 1 か所（計 7 か所）は、生成された `MQTTHandler.d.ts` が既にその型を返すので不要。
  - `getConnections`・`getProfiles` は生成クラスを `as` でフロントの型へキャストしている。キャストがあると、Go 側の型の変更でフロントの型と合わなくなっても `tsc` が検出しにくい。`saveProfile`・`deleteProfile` はキャストも無く、書き方が 3 通りある。
- **改善案**:
  - 送信方向は `createFrom` に替えない。`saveProfile` は `SaveProfile(profile)` とそのまま渡す（フロントの `BrokerProfile` は生成型に構造的に代入でき、生成型に項目が増えれば `tsc` がここで止める）。`connect` は `id` → `profileId` の付け替えがあるので、今の列挙を残す。
  - 受信方向は `as` を消す。`getProfiles` は `result.map(fromWailsProfile)`（`{ ...p }`）、`getConnections` は `fromWailsConnectionStatus`（`{ ...s, subscriptions: s.subscriptions.map((x) => ({ ...x })) }`）を通し、戻り値の型注釈で検査させる。`saveProfile` の戻り値も `fromWailsProfile` を通す。
  - 不要な `as Promise<void>`・`as Promise<string>` を消す。
  - ファイル先頭の `export type { ConnectionStatus }`（`:16`）は誰も import していないので消す（fake-backend は `domain/mqtt/types` から import している）。
- **期待効果**: 受信方向の型のずれを `tsc` で検出できるようになる、不要なキャストの削減、ファイル内の書き方の統一
- **挙動を守っているテスト**: `infrastructure/mqtt/client.test.ts`（`passes the profile fields to the backend`・`passes all profile fields to the backend`・`returns the saved profile from the backend`）。e2e は `e2e/ui/mqtt/mqtt.spec.ts`・`profiles.spec.ts`
- **検証コマンド**: 追加で `task frontend:tsc`・`task frontend:test:e2e`
- **付随作業**: 無し（生成物・fake-backend は変えない）。udp の `createFrom` による送信は上と同じ弱点を持つが、udp 側は別の調査で扱う

### イベントのペイロード型を 1 か所にまとめる
- **該当箇所**: `application/mqtt/connections.ts:53-56`（`MqttEventListener`）、`:67-79`（`ScannedTopic`・`RawMessage`）、`:282`・`:290`・`:297-300`・`:309`・`:317`・`:327-330`・`:343-346`（キャスト）、`application/mqtt/subscriptions.ts:17-21`・`:54`、`infrastructure/mqtt/events.ts:7-12`
- **現状**: イベントのハンドラは `(data: unknown) => void` で、受け取る側が 8 か所で `as` キャストしている。`{ connectionId: string; error: string }` は 3 か所、`{ connectionId: string }` は 2 か所にインラインで書かれている。ペイロードの型は application の 2 ファイルに散らばっていて、Go 側（`internal/domain/mqtt/types.go`）との対応が追いにくい。
- **改善案**: `domain/mqtt/types.ts` にペイロードの型（`MqttRawMessage`・`ScannedTopic`・`SubscriptionDropped`・`ConnectionEvent`・`ConnectionErrorEvent`）と、イベント名からペイロード型への対応表 `MqttEventPayloads` を置く。`MqttEventListener` を `<E extends MqttEventName>(event: E, handler: (data: MqttEventPayloads[E]) => void) => () => void` にする。`unknown` からのキャストは `infrastructure/mqtt/events.ts` の 1 か所だけになる。検証（型ガード）を足すかどうかは別の判断で、足すと不正なペイロードを捨てる挙動が加わるため、この候補には含めない。
- **期待効果**: 型安全性の向上、重複するインライン型の削減、Go の型との対応の明確化
- **挙動を守っているテスト**: `application/mqtt/connections.test.ts`・`subscriptions.test.ts`（どちらも自前の `onEvent` と `emit` を持つので、ヘルパーの型を合わせる必要がある）、`infrastructure/mqtt/events.test.ts`、`e2e/ui/mqtt/messages.spec.ts`
- **検証コマンド**: 追加で `task frontend:tsc`
- **付随作業**: `connections.ts:51` と `events.ts:4` の `MqttEventName` の再 export を使っているテスト 3 本の import 元を `shared/wails-events` に直すかどうかを決める。fake-backend への影響は無し

### `connections.ts` を責務で分け、ファイル内の重複をまとめる
- **該当箇所**: `application/mqtt/connections.ts`（656 行）
- **現状**: 1 つの `createConnectionsState` に、状態の生成、受信バッファ、イベントの購読、復元、接続操作が入っている。加えて次の重複がある。
  - `makeOfflineState`（`:86-100`）と `makeOnlineState`（`:115-133`）が、ランタイム状態の 7 項目の初期値を 2 回書いている。
  - `flushTopics`（`:192-201`）と `flushMessages`（`:230-239`）が、接続 ID ごとにまとめる同じループを書いている。
  - `createOfflineConnection`（`:462-469`）と `handleConnect`（`:483-490`）が、「同じプロファイルのタブを消して新しいタブを入れる」同じ `produce` を書いている。
  - 「オンラインのときだけ `connected: false`（と `isScanning: false`）にする」更新が 4 か所にある（`:318-321`・`:333-336`・`:349-352`・`:518-521`）。このうち `mqtt:connection-lost`（`:333-336`）だけは `isScanning` を変えない。
  - 受信メッセージと購読の照合（`:249-255`）が、購読の作り方を知っている `subscription.ts` ではなくここにある。
- **改善案**: 公開シンボルと挙動は変えずに、同じ `application/mqtt/` の中で分ける。
  - `connection-state.ts`: `ConnectionRuntimeState`・`ConnectionStateExt`・`offlineId`・`makeOfflineState`・`makeOnlineState`・`synthesizeProfile`。初期値は `emptyRuntimeState()` にまとめる。型は `connections.ts` から再 export して、既存の import を変えない。
  - `message-buffer.ts`: `messageBuffer`・`topicBuffer`・`scheduleFlush`・`flushTopics`・`flushMessages` を `createEventBuffer(updateConnection, maxMessages, maxTopics)` として切り出し、`pushMessage`・`pushTopic` を返す。まとめるループは `groupBy` のローカル関数にする。
  - `subscription.ts`: `subscriptionMatches(sub, topic, topicParts)` を足し、`flushMessages` から呼ぶ。
  - `connections.ts` に残る接続操作では、`replaceProfileTab(profileId, entry)` と `markOffline(connId, { stopScan })` をローカル関数にする。
- **分けるときに保つ条件**:
  - 受信バッファは、メッセージ・トピックとも 5,000 件を超えた分を捨てる（`:281`・`:289`）。
  - 1 フレームの中で、トピックを反映してからメッセージを反映する（`:184-185`）。
  - スキャンを止めた後に届いたトピックは捨てる（`:206`）。
  - 購読と mute の照合は、受信した時点ではなく反映する時点の `state.subscriptions` で行う（`:249`）。
  - `updateConnection` の `untrack`（`:166`）を残す（呼び出し元の effect に接続全体への依存を足さない）。
  - `mqtt:connection-lost` ではスキャンの状態を変えない（`markOffline` の `stopScan` を `false` で呼ぶ）。
- **期待効果**: 可読性の向上、重複の削減、受信バッファを単体でテストできるようになる
- **挙動を守っているテスト**: `application/mqtt/connections.test.ts`（1,348 行。復元・接続・再接続・バッファの上限・イベントの解除を検証している）、`e2e/ui/mqtt/messages.spec.ts`（`message cap` の 2 本・`muted subscription hides its messages`・`shared subscription shows its messages`）、`e2e/ui/mqtt/profiles.spec.ts`
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し

### publish の判定と送信を application 層へ移す
- **該当箇所**: `presentation/components/mqtt/publish-tab.tsx:174-180`、`presentation/providers/mqtt-provider.tsx:168-179`
- **現状**: 「トピックが空なら送らない」「retain 付きの空ペイロードは retained メッセージの削除なので許可する」という業務上の判定がコンポーネントにあり、RPC の呼び出しと失敗時の通知は Provider にある。publish の RPC は application のポートを通らず、Provider が `mqttClient.publish` を直接呼んでいる。udp は `application/udp/send.ts` の `createUdpSendState` が `UdpSendApi` を受けて同じ役割を持っている。この判定にはユニットテストが無い。
- **改善案**: `application/mqtt/publish.ts` に `MqttPublishApi`（`publish` だけ）と `createPublishState(api, activeConnectionId, draft, notifier)` を置き、`canPublish(draft)`（純粋関数）と `publishDraft()` を返す。Provider は注入だけを行い、`PublishForm` は `publishDraft()` を呼ぶ。コンテキストの `publish(topic, payload, qos, retain)` は `PublishForm` からしか呼ばれていないので、置き換えてよい。オフラインのタブも `activeConnectionId` を持つ（`offline-<profileId>`）ので、ID があるだけでは送信できるとは限らない。今は Publish ボタンの `disabled` だけが未接続での送信を止めている。`publishDraft()` でも接続を確かめるなら、`activeConnectionId` ではなく `activeConnection` を受けて domain の `isConnected` で判定する（ボタンが無効なので、画面から届く挙動は変わらない）。
- **期待効果**: presentation へのロジック漏れの解消、udp との構造の統一、判定をユニットテストできるようになる
- **挙動を守っているテスト**: `e2e/ui/mqtt/messages.spec.ts`（`publish sends topic, QoS, retain and payload`）、`e2e/ui/mqtt/publish.spec.ts`（`publish button is disabled while offline`）。空トピック・空ペイロードの判定を検証するテストは無し（先に `canPublish` のユニットテストを追加）
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し（バインド API の呼び方は変わらない）

### 接続中かどうかの判定を `isConnected` に揃える
- **該当箇所**: `presentation/components/mqtt/panels/subscriptions-panel.tsx:28-31`、`presentation/components/mqtt/publish-tab.tsx:169-172`、`presentation/components/sidebar/broker-tree.tsx:41-44`、`application/mqtt/connections.ts:449`
- **現状**: `domain/mqtt/types.ts:77` に `isConnected(conn)` があるのに、使っているのは `broker-manager.tsx` だけ。ほかの 4 か所は `conn?.type === "online" && conn.connected` を書き直している。パネルの 2 か所は同じ名前のローカル関数 `isConnected` を定義していて、domain の関数と紛らわしい。
- **改善案**: 4 か所を domain の `isConnected` に替える。未選択（`null`）を扱うパネル側は `const conn = activeConnection(); return conn !== null && isConnected(conn);` とする。パネルの 2 か所は、ローカル関数の名前が import する `isConnected` と衝突するので、ローカル側を `canSend` などに改名する。
- **期待効果**: 重複の削減、判定の基準を 1 か所にする
- **挙動を守っているテスト**: `e2e/ui/mqtt/mqtt.spec.ts`（`subscribe button is disabled when broker is not connected`）、`e2e/ui/mqtt/publish.spec.ts`（`publish button is disabled while offline`）、`application/mqtt/connections.test.ts`（`updateConnectionBroker`）
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し

### スキームの一覧と既定の URL を 1 か所で定義する
- **該当箇所**: `application/mqtt/broker-url.ts:1-7`・`:18-19`、`presentation/components/mqtt/broker-manager.tsx:105-109`、`presentation/components/mqtt/broker-settings-dialog.tsx:29-31`・`:56-70`・`:121-125`、`application/mqtt/profiles.ts:12`
- **現状**:
  - スキームの一覧（mqtt・mqtts・tcp・ws・wss）が、`DEFAULT_PORT_MAP` のキー、`parseBrokerUrl` の正規表現、2 つのコンポーネントの `<option>` の計 4 か所にある。スキームを足すときに 1 か所でも漏れると、選べるのに読み戻せない値ができる。
  - 既定の URL `mqtt://localhost:1883` が、`createEmptyProfile`、ダイアログの初期値、`parseBrokerUrl` の失敗時の戻り値の 3 か所にある（`parseBrokerUrl` は文字列ではなく、分解済みの `{ scheme: "mqtt", host: "localhost", port: "1883" }` を返す）。
  - ダイアログの `handleSave` と `handleSaveAndConnect` が、同じプロファイルの組み立てを 2 回書いている。
- **改善案**:
  - `broker-url.ts` に `BROKER_SCHEMES`（`as const` の配列）を export し、`DEFAULT_PORT_MAP` の型と正規表現をそこから作る。2 つのコンポーネントは `<For each={BROKER_SCHEMES}>` で `<option>` を描く。
  - `broker-url.ts` に分解済みの既定値 `DEFAULT_BROKER`（`{ scheme, host, port }`）を置き、`DEFAULT_BROKER_URL` はそこから `composeBrokerUrl` で作る。`parseBrokerUrl` の失敗時は `DEFAULT_BROKER` のコピーを返し、`createEmptyProfile` は `DEFAULT_BROKER_URL` を使う。ダイアログは `props.profile ?? createEmptyProfile()` を 1 回作り、その `broker` を `parseBrokerUrl` に渡す。
  - ダイアログに `buildProfile()` を作り、2 つのハンドラから呼ぶ。
- **期待効果**: 同期漏れの防止、重複の削減
- **挙動を守っているテスト**: `application/mqtt/broker-url.test.ts`、`application/mqtt/profiles.test.ts`（`createEmptyProfile`）、`e2e/ui/mqtt/mqtt.spec.ts`（`new broker dialog rejects a port or host that cannot be read back`）、`e2e/ui/mqtt/profiles.spec.ts`（`edit broker loads the saved values and updates the profile`・`editing the connection bar after switching saves only the selected broker`）
- **検証コマンド**: 追加で `task frontend:test:e2e`
- **付随作業**: 無し（`broker.module.css` のクラスはそのまま使う）

### QoS を名前付きの型にする
- **該当箇所**: `domain/mqtt/types.ts:27`・`:44`・`:54`、`application/mqtt/presets.ts:13`、`application/mqtt/subscription.ts:19`、`application/mqtt/connections.ts:262`、`presentation/components/mqtt/publish-tab.tsx:197`、`presentation/components/mqtt/qos-select.tsx:11-13`・`:19`、`infrastructure/storage/local-storage.ts:58`
- **現状**: `0 | 1 | 2` を 4 か所に直接書き、`number` から `as 0 | 1 | 2` へのキャストが 3 か所ある（受信メッセージの `connections.ts:262`、`makeSubscription` の `subscription.ts:19`、Publish フォームの `publish-tab.tsx:197`）。`local-storage.ts:58` は同じ条件を手で書いている。`QosSelect` は `number` で受け渡すので、呼び出し側がキャストしている。
- **改善案**: `domain/mqtt/types.ts` に `type Qos = 0 | 1 | 2` と `isQos(v: unknown): v is Qos` を置き、型の記述と `local-storage.ts` の検査で使う。UI の内部は `Qos` に限り、RPC とイベントの境界ではキャストを残す。
  - `QosSelect` の `value`・`onChange` を `Qos` にし、`parseInt` の結果を `isQos` で確かめてから渡す。`publish-tab.tsx:197` のキャストはこれで消える。
  - `newQos` の signal と `addSubscription`・`makeSubscription` の引数を `Qos` にする。`subscription.ts:19` のキャストは消えるが、復元（`connections.ts:411`）は RPC の `SubscriptionInfo.qos: number` を渡しているので、そこに `as Qos` を足す。
  - 受信メッセージ（`connections.ts:262`）のキャストは `as Qos` に書き換えて残す。
  - 境界の 2 か所を `isQos` で確かめて捨てると挙動が変わるので、この候補では行わない。
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
  - `application/mqtt/connections.ts:505` の `handleDisconnect(connectionId?)`: コンポーネントは必ず ID を渡す（`broker-manager.tsx:150`）。省略するのは `connections.test.ts` だけ。
- **改善案**: 再 export 2 つと不要な `export` 3 つを消す。`PresetsPanel` の props を消して `useMqttPublish()` から `addPreset` を取る。`savePreset`・`addPreset` の引数・`handleDisconnect` の省略可能な引数・コンテキストの `messages` は、消すならテストも合わせて直す（残すなら、使う予定を書いたコメントを付ける）。コンテキストから `messages` を消し、あわせて「Provider の hook とコンテキスト型を整理する」も行うなら、`MessagesContextValue` は `Omit<ReturnType<typeof createMessagesState>, "messages">` とし、Provider の値も `messages` を除いて渡す（`createMessagesState` の戻り値には `messages` が残るため、`ReturnType` をそのまま使うと再び公開される）。
- **期待効果**: 公開面の縮小、可読性の向上
- **挙動を守っているテスト**: `application/mqtt/presets.test.ts`、`application/mqtt/connections.test.ts`、`e2e/ui/mqtt/publish.spec.ts`（`presets can be added, selected, renamed and deleted`）
- **検証コマンド**: 追加で `task frontend:tsc`・`task frontend:test:e2e`
- **付随作業**: 無し

### 「更新して保存」の繰り返しをまとめる
- **該当箇所**: `application/mqtt/presets.ts:60-64`・`:73-77`・`:81-85`・`:99-103`・`:110-115`、`application/mqtt/profiles.ts:58-66`・`:75-79`・`:83-88`
- **現状**: `setPresets((prev) => { const next = ...; storage.save(next); return next; })` が 5 回、`profiles.ts` の同じ形が 3 回ある。setter の updater の中で保存という副作用を起こしている。順序は「保存してから signal を更新する」。`savePreset` と `addPreset` は末尾への追加と選択を別々に書いている。`PublishDraft`（`:10-15`）は `PublishPreset` から `id`・`name` を除いたものと同じ形で、`loadDraftFromPreset`（`:31-36`）と `emptyDraft`・`addPreset` の初期値がフィールドを列挙している。
- **改善案**: `presets.ts` に `commit(update: (prev) => PublishPreset[] | null)` を作り、5 か所から呼ぶ。`commit` は今と同じ順序を保つ（`untrack(presets)` から次の値を計算し、`null` なら何もしない。保存してから signal に入れる）。signal を先に更新すると、保存より前に effect が走る順序になり、保存が例外を投げたときの結果も変わる（今の localStorage の実装は `saveToStorage` が例外を握るので実害は出ないが、ポートの実装に頼らない）。`profiles.ts` も同じ形の `commit` で並び順を保存する。`PublishDraft` は `Omit<PublishPreset, "id" | "name">` として定義し、型の同期漏れを防ぐ。
- **期待効果**: 重複の削減、保存の呼び出しを 1 か所にする
- **挙動を守っているテスト**: `application/mqtt/presets.test.ts`、`application/mqtt/profiles.test.ts`（`saves the order when profiles are reordered or deleted`・`does not save when the indices are out of range`）、`e2e/ui/mqtt/publish.spec.ts`（`publish presets survive a reload`）、`e2e/ui/mqtt/mqtt.spec.ts`（`broker order`）
- **検証コマンド**: 追加無し
- **付随作業**: 無し

### Provider の hook とコンテキスト型を整理する
- **該当箇所**: `presentation/providers/mqtt-provider.tsx:62-115`・`:212-236`
- **現状**: 4 つの hook が、同じ `useContext` と例外の送出を書いている。`SubscribeContextValue`・`MessagesContextValue`・`PublishContextValue` は、`createSubscriptionsState`・`createMessagesState`・`createPresetsState` の戻り値をそのままスプレッドしたものなのに、型を手で書き写している（application 側に項目を足しても、ここを直さないとコンポーネントから見えない）。
- **改善案**: `useMqttContext(hookName)` を作って 4 つの hook から呼ぶ。3 つの型は `ReturnType<typeof createXxxState>` から作る（`PublishContextValue` は publish の分を足す）。`ReturnType` は戻り値の全項目を公開するので、「参照されていない export・再 export・引数を整理する」でコンテキストから外す項目（`messages`・`savePreset`）は `Omit` で除く。`ConnectionContextValue` は `ConnectionStateExt` を `ConnectionState` に狭めて公開する意図があるので、手書きのままにする。
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
- **`application/mqtt/profiles.ts` と `application/udp/targets.ts` の共通化**: 並び順の読み込みは既に `applyOrder`・`moveItem` を共有している。残りは挙動が違う（mqtt は signal を手元で更新し、削除の失敗を呼び出し側に伝える。udp は store を RPC で読み直し、削除の失敗を通知だけで終える）。まとめるには差を引数で切り替える抽象化が要り、共通になる部分が小さいので効果が見合わない。
- **`sidebar/profile-list.tsx`**: mqtt と udp で既に共通化されている。相違点は props で受けていて、追加でまとめる箇所は無い。
- **`broker-settings-dialog.tsx` と udp の `TargetDialog`（`target-tree.tsx`）の共通化**: 入力項目・検証・ボタンの構成が違い、`profile-list.tsx:38-40` のコメントも編集ダイアログは共通化しないと決めている。
- **`broker-manager.tsx:35-47` の effect**: 書き換えるのはコンポーネント内の入力欄の signal だけで、application の状態は書き戻していない。規約の範囲内で、理由もコメントに書かれている。
- **`panels/messages-panel.tsx:47-53` の effect**: スクロールの追従だけを行う。選択の追従は `application/mqtt/messages.ts` の 1 か所にある。
- **`application/mqtt/subscriptions.ts` の `setIsScanning`（`:173-231`）**: 長いが、開始と停止の順序を守るための処理で、分けると順序の前提が読み取りにくくなる。`subscriptions.test.ts` が順序を細かく検証している。
- **`application/mqtt/connections.ts:331`・`:347` の `console.error`**: 注入された `logger` を使わず、コンソールに出している。`logger.error` に替えるとログの出力先（バックエンドのログファイル）が変わるので、挙動変更にあたる。替えるかどうかは別に決める。
- **`infrastructure/storage/local-storage.ts` の mqtt 部分**: `ConnectionPersistence`（`loadLastProfileId` など）だけメソッド名がほかのポート（`load`・`save`）と違うが、実害が無く、揃える効果が小さい。`StoredPreset`（`:49`）は `retain` 導入前の保存値を読むためのもので必要。
- **`presentation/components/mqtt/utils.ts` の `getTopicColor`**: 表示用の色の計算で、presentation に置くのが適切。
- **`application/mqtt/connections.ts` の `switchConnection`**: `setActiveConnectionId` を包むだけだが、setter をそのまま公開しないための入口として残す。

## 調査中に見つけた不具合の候補（この調査の範囲外）

リファクタリングでは挙動を変えないので、ここでは直さない。直すなら候補ごとに変更計画を分ける。いずれもコードを読んで確かめたもので、画面での再現はしていない。

- **接続中のブローカーを編集して「Save」で保存すると、バックエンドの接続が画面から見えなくなる。** `sidebar/broker-tree.tsx:64-69` の `handleProfileSave` は、保存に成功すると必ず `createOfflineConnection(saved)` を呼ぶ。`createOfflineConnection`（`application/mqtt/connections.ts:460-471`）は同じプロファイルのタブを消してオフラインのタブに置き換えるが、`api.disconnect` は呼ばない。編集は接続中でも開ける（`sidebar/profile-list.tsx` の `onEdit` に条件は無い）。画面はオフラインになり、接続・購読・スキャンはバックエンドに残る。そのまま接続し直すと、同じプロファイルの接続が 2 つになる。「Save & Connect」は `handleReconnect` を通るので起きない。`e2e/ui/mqtt/profiles.spec.ts` の編集のテストは、接続中の「Save」を検証していない。
- **接続バーの URL を続けて編集すると、古い保存の応答が新しい値を上書きし得る。** `updateConnectionBroker`（`application/mqtt/connections.ts:447-458`）は入力のたびに `saveProfile` を呼び、前の保存を待たない。`saveProfile`（`application/mqtt/profiles.ts:58-66`）は応答を無条件に一覧へ入れる。応答が送った順に返らないと、一覧（サイドバーの URL）が古い値に戻る。タブの URL は入力した値のままなので、2 つが食い違う。直すなら、保存を直列にするか、最後に送った保存の応答だけを反映する。
- **Subscribe ボタンが無効でも、トピック入力欄の Enter で購読を送れる。** `panels/subscriptions-panel.tsx:44` の `onKeyDown` は接続を確かめずに `addSubscription()` を呼ぶ。`addSubscription`（`application/mqtt/subscriptions.ts:83-128`）も接続の種類を見ないので、オフラインのタブでは `offline-<profileId>` を接続 ID にして RPC を送る。確立待ちのオンラインのタブへの購読は意図して許している（バックエンドが確立時に購読する）ので、止めるのはオフラインのタブだけにする。
- **起動時にプロファイルの読み込みが失敗すると、後続の初期化も止まる。** `providers/mqtt-provider.tsx:159-166`。`docs/2026-10-03-mqtt-profile-load-failure.md` で扱う。
