# 変更計画書: Broker Topics のスキャンを専用の接続に分ける

## 概要

MQTT で 1 件だけ publish したメッセージが複数件表示される不具合の、残りの原因を直す。

- これまでの修正（未コミット、`pahoClient.dispatch`）で、受信した PUBLISH 1 件につき `mqtt:message` を 1 回だけ発行するようにした。
- しかし利用中のブローカーは、重なる購読ごとに PUBLISH を 1 件ずつ送る（MQTT 3.1.1 3.3.5 が認める動作）。Broker Topics のスキャンは同じ接続で `#` を購読するので、スキャン中は全ての購読と重なり、修正後も 2 件届く。
- スキャンの `#` を専用の接続（別のクライアント ID）で購読し、ユーザーの購読と同じセッションで重ならないようにする。

あわせて次の 2 点を直す。

- 「重なる購読ごとに PUBLISH を送るブローカーでも、スキャン中に 1 件だけ届く」ことを確認するテストを足す。`subscribingBroker` の実態と合わないコメントも直す。
- 再接続時の張り直しをブローカーが拒否したとき、`pahoClient.routes` に残るルートを片付ける。

ユーザーが自分で張った重なる購読（例: `a/#` と `a/b`）で 2 件届くのは、ブローカーの実際の動作なのでそのまま表示する（重複排除はしない）。

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `internal/domain/events.go` | Domain | 変更 | `EventMQTTScanTopic`（`mqtt:scan-topic`）と `EventMQTTScanStopped`（`mqtt:scan-stopped`）を追加 |
| `tools/gen-events/main.go` | ツール | 変更 | 上記 2 イベントを一覧に追加 |
| `internal/domain/mqtt/types.go` | Domain | 変更 | `ConnectionStatus.Scanning bool`（`json:"scanning"`）を追加。`mqtt:scan-topic` のペイロード `ScannedTopic{ConnectionID, Topic}` を追加 |
| `internal/domain/mqtt/broker_port.go` | Domain | 変更 | `BrokerClientFactory` のコメントに「`onConnectionLost` から `Disconnect` を呼んでよく、呼ぶと自動再接続は止まる」を追記 |
| `internal/application/mqtt/service.go` | Application | 変更 | `StartTopicScan` / `StopTopicScan` を追加。`connection` にスキャンの状態（開始中 / 稼働中）とスキャン用クライアントを持たせ、終端状態への遷移で必ず止める。`GetConnections` が `Scanning` を返す。`resubscribe` で拒否された購読を `Unsubscribe` する |
| `internal/application/mqtt/service_test.go` | Application | 変更 | スキャンの開始・停止・失敗・切断時の後始末、開始中の Stop / 二重 Start / Disconnect、拒否時の `Unsubscribe` のテスト |
| `internal/adapters/mqtt_handler.go` | Adapters | 変更 | `MQTTConnectionUseCase` と `MQTTHandler` に `StartTopicScan(connectionID)` / `StopTopicScan(connectionID)` を追加 |
| `internal/infrastructure/mqtt/paho_client.go` | Infrastructure | 変更 | `Subscribe` のコメントを直す（張り直しの失敗ではルートを残し、後始末は呼び出し側が `Unsubscribe` で行う） |
| `internal/infrastructure/mqtt/paho_client_test.go` | Infrastructure | 変更 | `subscribingBroker` のコメントを実態に合わせる |
| `internal/testutil/mqtt_broker.go` | テスト補助 | 新規 | 重なる購読ごとに PUBLISH を 1 件ずつ送る偽ブローカー `DuplicatingBroker`（複数接続、QoS 0 のみ） |
| `internal/integration/mqtt_test.go` | 統合テスト | 変更 | スキャンの統合テスト（mochi）と、`DuplicatingBroker` で「スキャン中も 1 件だけ届く」テスト |
| `frontend/wailsjs/` | 生成物 | 再生成 | `task wails:generate` |
| `frontend/src/shared/wails-events.ts` | 生成物 | 再生成 | `task go:generate:events` |
| `frontend/src/domain/mqtt/types.ts` | Domain | 変更 | `ConnectionStatus.scanning: boolean` を追加 |
| `frontend/src/infrastructure/mqtt/client.ts` | Infrastructure | 変更 | `startTopicScan` / `stopTopicScan` を追加 |
| `frontend/src/application/mqtt/subscriptions.ts` | Application | 変更 | `SubscriptionApi` に `startTopicScan` / `stopTopicScan` を追加し、`setIsScanning` が `subscribe("#")` の代わりに呼ぶ。接続ごとの連番で、古くなった Start の結果を反映しない |
| `frontend/src/application/mqtt/connections.ts` | Application | 変更 | `brokerTopics` を `mqtt:scan-topic` から作る（`mqtt:message` からは作らない）。`mqtt:scan-stopped` で `isScanning` を戻して通知する。`MqttConnectionApi` に `stopTopicScan` を追加し、`restore` が `scanning` の接続のスキャンを止める。`handleReconnect` が `isScanning` を false にする。`closeConnection` が未接続のオンラインタブでも `disconnect` を呼ぶ |
| `frontend/src/application/mqtt/subscriptions.test.ts`、`connections.test.ts` | Application | 変更 | 上記に合わせる |
| `frontend/e2e/fake-backend/install.ts` | e2e | 変更 | `StartTopicScan` / `StopTopicScan` を追加（`scanning` を切り替える）。`Connect` が `scanning: false` を入れる |
| `frontend/e2e/ui/mqtt/mqtt.spec.ts` | UI e2e | 変更 | スキャンの UI テストを追加 |
| `frontend/e2e/integration/mqtt/mqtt.spec.ts` | フルスタック e2e | 変更 | スキャンのコメントを直し、スキャン中は `subscriptions` に `#` が無いことを確認する |

`app.go` は変更しない（`MQTTService` が新しい入力ポートを満たすことは既存のコンパイル時検査が確かめる）。`presentation/` も変更しない（`setIsScanning` の形は変わらない）。

## 実装方針

### バックエンド

- **新しい RPC を足す**: `StartTopicScan(connectionID)` と `StopTopicScan(connectionID)`。フロントエンドが `Subscribe(connId, "#", 0)` をスキャンに流用するのをやめる。ユーザーが `#` を通常の購読として張ることは引き続きできる。
- **スキャン用クライアント**は既存の `BrokerClientFactory` で作る。接続設定は元の接続と同じで、クライアント ID だけ `wirexa-scan-<8 桁>`（20 文字）にする。同じクライアント ID では元の接続が切られるため。
- **`StartTopicScan` は同期**で、スキャン用クライアントの接続と `#`（QoS 0）の購読が済んでから返る。失敗したらエラーを返し、スキャン用クライアントは残さない。元の接続が確立前でも受け付ける（スキャン用の接続は独立している）。
- **スキャンの状態**: `connection.scan *topicScan`（`stateMu` で保護）で「なし（nil）/ 開始中 / 稼働中」を表す。`topicScan` は、スキャン用クライアント、接続を打ち切る `cancel`、開始の完了を知らせる `done` とその結果 `err`、稼働中かを表す `started` を持つ。
  - **Start**: `conn.scan` が nil なら、I/O の前に `topicScan` を作って `conn.scan` に入れる（予約）。I/O の後に `stateMu` を取り、`conn.scan` がまだ自分の `topicScan` で、接続も購読も成功していれば `started` を true にして成功を返す。そうでなければ、スキャン用クライアントを切断してエラーを返す（`conn.scan` が自分のものなら nil に戻す）。どちらの場合も `err` を入れて `done` を閉じる。
  - **二重の Start**: `conn.scan` が nil でなければクライアントを作らず、`done` を待って同じ結果を返す（稼働中なら `done` は閉じているので、すぐ成功を返す）。
  - **Stop**: `stateMu` の保持中に `conn.scan` を nil にしてから `cancel` を呼ぶ。稼働中だったら Stop がクライアントを切断する。開始中だったら切断は Start 側に任せる（接続試行の途中のクライアントを別の goroutine から切断しない）。Start は `conn.scan` が自分のものでないことを見てエラーを返す。スキャンが無ければ何もしない。
  - **元の接続が終端状態になる全ての経路**（`Disconnect`、`Shutdown`、接続失敗）は、終端状態への遷移と同じ `stateMu` の区間で Stop と同じ手順を踏む。
- **受信したメッセージ**は `mqtt:scan-topic`（`{connectionId, topic}`）として発行する。ペイロードは運ばない。トピックの重複排除と上限（`MQTT_MAX_TOPICS`）は今までどおりフロントエンドが行う。
- **スキャン用の接続が切れたら**スキャンを終える。既存のファクトリは常に `SetAutoReconnect(true)` で、接続設定（`ConnectionConfig`）は RPC の型なので再接続の有無は載せない。代わりに、スキャン用クライアントの `onConnectionLost` で次のようにして自動再接続を止める。
  - 稼働中なら、`stateMu` の保持中に `conn.scan` を nil にして `mqtt:scan-stopped`（`{connectionId, error}`）を発行し、ロックを放してからクライアントを `Disconnect` する。paho（v1.5.1）の `Disconnect` は再接続の試行が終わるのを待ってから切断するので、再接続は続かない。
  - 開始中なら、イベントは発行せず `topicScan` に「切れた」ことだけを記録する。Start は I/O の後にそれを見て失敗として扱う（エラーを返すので、通知は Start の失敗の 1 回だけになる）。
  - `conn.scan` が自分のものでなければ（Stop 済み）何もしない。
  - スキャン用クライアントの `onConnected` は何もしない。`Disconnect` と行き違いで再接続が成立しても、直後の `Disconnect` が閉じる。張り直しの処理は持たない。
- **イベントの発行条件**: `mqtt:scan-topic` は `stateMu`（RLock）の保持中に「`conn.scan` が自分の `topicScan` で、稼働中で、元の接続が終端状態でない」ことを確かめてから発行する。`mqtt:scan-stopped` は上記のとおり稼働中のスキャンの接続が切れたときだけ発行する（Stop や Disconnect では発行しない）。これで「Stop の後と Shutdown の後はイベントを出さない」が保たれる。
- **`GetConnections`** は、稼働中のときだけ `Scanning: true` を返す（開始中は false）。
- **ロック**: ネットワーク I/O（接続、購読、切断）の間は `stateMu` を持たない。`opMu` は取らないので、スキャンの接続待ちが Publish / Subscribe を止めない。
- **`Shutdown` の保証を保つ**: スキャン用クライアントの接続は `s.root` の子 context で打ち切れるようにし、Start の I/O を `connWg` に計上する。予約と `connWg` への計上は、`Connect` と同じく `s.mu` の保持中に `closed` を確かめてから行う（分けると、`Shutdown` が `Wait` した後に計上する窓ができる）。
- **張り直しの拒否**: `resubscribe` で `ErrSubscriptionRejected` になった購読は、`subs` から消すときに `conn.client.Unsubscribe(topic)` も呼ぶ。失敗はログに出すだけにする。`pahoClient.Subscribe` 側で常にルートを消す案は採らない（接続中の QoS 変更が拒否された場合、サービスは元の購読を残すので、ルートだけ消えると届いたメッセージを捨てる）。
  - `pahoClient.Unsubscribe` は成功したときだけルートを消すので、解除に失敗する（直後にまた切断した、など）とルートは残る。これは許容する。ブローカーには購読が無いので、残ったルートに一致するメッセージが届くのは、重なる別の購読があるときだけで、同じ接続のルートは全て同じハンドラ（`messageHandler`）なので、どのルートが選ばれても発行するイベントは変わらない。同じフィルターを次に購読したときは `setRoute` が置き換える。

依存方向は変わらない。入力ポートは adapters が定義し、application は domain のポートだけを使う。

### フロントエンド

- RPC は `SubscriptionApi`（`application/mqtt/subscriptions.ts`）に足し、`infrastructure/mqtt/client.ts` が構造的に満たす。注入は既存の `mqtt-provider.tsx` の `mqttClient` のままで、Provider の変更は不要。
- イベントは既存の `onEvent`（`MqttEventListener`）で受ける。`MqttEventName` は生成物なので再生成で 2 イベントが加わる。
- `mqtt:scan-topic` は `mqtt:message` と同じく 1 フレームにまとめて反映する。`isScanning` が false の間に届いたものは捨てる（Stop と行き違いで届いたもの）。
- **Start と Stop の行き違い**: `setIsScanning` は RPC の完了前に `isScanning` を切り替えるので、Start の完了前に Stop を押せる。次の 2 つで扱う。
  - 接続ごとに連番を持ち、`setIsScanning` を呼ぶたびに進める。Start の結果（失敗時に `isScanning` を false に戻して通知する）は、連番が呼び出し時のままのときだけ反映する。Stop で打ち切られた Start のエラーは通知しない。
  - Start は、同じ接続で先に送った Start / Stop の RPC が終わってから送る。Stop は待たずに送る（バックエンドが開始中のスキャンを打ち切る）。Start → Stop → Start と続けて押しても、2 回目の Start が打ち切られる前の開始に合流しない。
- `restore` は、`status.scanning` が true の接続に `stopTopicScan` を呼び、`isScanning` は false のままにする（失敗はログに出すだけ）。Broker Topics の一覧はフロントエンドだけが持つのでリロードで消え、スキャン用の接続が続いていても retained メッセージは再送されない。スキャン中として復元すると、retained のトピックが欠けた一覧を「スキャン中」と表示してしまう。
- `closeConnection` は、オンラインタブなら `connected` に関係なく `disconnect` を呼ぶ。今は `connected` が true のときしか呼ばないので、確立待ちや自動再接続中のタブを閉じるとバックエンドに接続が残る。スキャンは確立前でも始められるので、スキャン用の接続も一緒に残ってしまう。失敗を通知するのは `connected` が true だったときだけにする（接続が失敗して既に無い場合は `NotFound` が返るため）。

### RPC 型の確認

- `map[string][]Struct` は使わない（`bool` と `string` のフィールドだけ）。
- `ConnectionStatus` はフロントエンドでキャストして素通ししているので、`types.ts` にフィールドを足せば変換関数の変更は不要。
- RPC 引数は接続 ID だけで、ローカルのファイルやパスには触れない。

## 永続化への影響

なし。

## コード生成

- [ ] `task wails:generate`（`MQTTHandler` のメソッド追加と `ConnectionStatus.Scanning`。`models.ts` の `ConnectionStatus` を目視）
- [ ] `task go:generate:events`（`tools/gen-events/main.go` に 2 イベントを追記してから）

## テスト方針

### Go ユニット（`internal/application/mqtt/service_test.go`）

- `StartTopicScan` が別のクライアント ID でクライアントを作り、`#` を QoS 0 で購読する。元の接続のクライアントは `#` を購読しない。
- スキャン用クライアントが受信すると `mqtt:scan-topic` を発行し、`mqtt:message` は発行しない。
- 接続または購読に失敗したらエラーを返し、クライアントを切断し、`Scanning` は false。
- 二重の `StartTopicScan` はクライアントを増やさない。`StopTopicScan` は切断し、スキャンしていなければ何もしない。
- `Disconnect` / `Shutdown` / 元の接続の失敗でスキャン用クライアントを切断する。開始の途中で `Disconnect` されたら接続を捨てる。
- 開始の途中で `StopTopicScan` を呼ぶと、`StartTopicScan` はエラーを返し、スキャン用クライアントは切断され、`Scanning` は false で、以後 `mqtt:scan-topic` も `mqtt:scan-stopped` も発行しない。
- 開始の途中の 2 回目の `StartTopicScan` はクライアントを増やさず、1 回目と同じ結果（成功 / 失敗）を返す。
- 止めた後にもう一度 `StartTopicScan` すると、新しいクライアントで始まる。
- スキャン用の接続が切れたら `mqtt:scan-stopped` を 1 回発行して切断する。開始の途中で切れた場合は発行せず、`StartTopicScan` がエラーを返す。
- `StopTopicScan` の後にスキャン用クライアントの `onConnectionLost` やメッセージが届いても、イベントを発行しない。
- 張り直しが拒否された購読だけ `Unsubscribe` を呼ぶ（既存の `TestMQTTService_Reconnect_ResubscribeFailures` を拡張）。`Unsubscribe` が失敗しても `subs` からは消える。

### Go 統合（`internal/integration/mqtt_test.go`）

- mochi: スキャンを始めると publish したトピックが `mqtt:scan-topic` で届き、`GetConnections` は `Scanning: true` で `Subscriptions` に `#` を含まない。止めると届かなくなる。
- **`DuplicatingBroker`（1 件のみ届くことの確認）**: 購読 1 つ + スキャン中に 1 件 publish すると、`mqtt:message` は 1 回だけ。
- 同じブローカーで、同じ接続に `#` を通常の購読として足すと 2 回届く。偽ブローカーが実際に重複して送ることの確認と、重複排除をしないことの固定を兼ねる。

### フロント ユニット

- `subscriptions.test.ts`: `setIsScanning(true/false)` が `startTopicScan` / `stopTopicScan` を呼び、`subscribe` / `unsubscribe` は呼ばない。開始の失敗で `isScanning` を戻して通知する。Start の完了前に Stop すると `stopTopicScan` をすぐ呼び、その後の Start の失敗は通知しない。Start → Stop → Start では、2 回目の `startTopicScan` を 1 回目の完了後に呼び、1 回目の失敗で `isScanning` を戻さない。
- `connections.test.ts`: `mqtt:scan-topic` で `brokerTopics` が増える（重複なし、上限あり）。`isScanning` が false なら増えない。`mqtt:message` では増えない。`mqtt:scan-stopped` で `isScanning` が false になり通知する。`restore` が `scanning` の接続に `stopTopicScan` を呼び、`isScanning` は false。`closeConnection` が未接続のオンラインタブでも `disconnect` を呼び、その失敗は通知しない（既存の「disconnects only a connected online tab when closing it」を書き換える）。オフラインタブでは呼ばない。

### UI e2e（`frontend/e2e/ui/mqtt/mqtt.spec.ts`）

- Scan を押すと `StartTopicScan` が呼ばれ、`mqtt:scan-topic` のトピックが一覧に出る。`Subscribe` は呼ばれない。Stop で `StopTopicScan` が呼ばれる。`mqtt:scan-stopped` でボタンが Scan に戻る。

### フルスタック e2e（`frontend/e2e/integration/mqtt/mqtt.spec.ts`）

- 既存のスキャンのテストを新しい動作に合わせる（期待値は変わらず、メッセージは 1 件）。

## 副作用・注意事項

- **スキャン中はブローカーへの接続が 1 本増える。** 接続数の上限、クライアント ID で絞る ACL、同時ログインを 1 つに制限する認証では、スキャンの開始が失敗することがある。失敗は「Failed to start topic scan」で通知する。
- **スキャンを始めたときに、購読済みトピックの retained メッセージが Messages に並ばなくなる。** 今までは `#` の購読で再送されたものが表示されていた。
- **スキャン用の接続が切れるとスキャンは止まる**（自動では再開しない）。元の接続が切れて自動再接続している間は、スキャンは続く。
- **webview をリロードしたあと**、スキャン中だった接続のスキャンは止まる（一覧は消えるので、必要ならもう一度 Scan を押す）。今までは `#` が通常の購読として一覧に出ていた。
- **確立待ち・自動再接続中のタブを閉じると、バックエンドの接続も切断するようになる。** 今までは接続が残り、確立後も画面から見えないまま続いていた。
- スキャン中のメッセージはペイロードをフロントエンドへ送らなくなるので、イベントの負荷は下がる。
- 偽バックエンドに 2 つの RPC を足す。足さないと UI e2e で Scan が失敗する。

## Git運用

- **ブランチ名**: `fix/mqtt-scan-separate-connection`
- **コミット分割方針**:
  1. `fix(mqtt): 重なる購読があっても受信 1 件につき 1 回だけ発行する`（作業ツリーにある未コミットの修正）
  2. `fix(mqtt): 張り直しを拒否された購読を解除する`（`resubscribe`、`paho_client.go` のコメント）
  3. `feat(mqtt): Broker Topics のスキャンを専用の接続で行う`（domain / application / adapters と生成物）
  4. `feat(frontend): スキャンを StartTopicScan / StopTopicScan に切り替える`（フロントエンドと fake-backend、UI e2e）
  5. `test(integration): 重なる購読ごとに配信するブローカーでも 1 件だけ届くことを検証する`（`DuplicatingBroker`、統合テスト、`subscribingBroker` のコメント、フルスタック e2e）
- **完了条件**: すべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task go:test:integration`
  - `task go:test:race`（並行処理に触れるため）
  - `task frontend:test:e2e`
  - `task frontend:test:e2e:fullstack`（任意。実施できなければその旨を報告する）
  - 実機確認: 発生時のブローカーで「スキャン中 + 購読 1 つ」に 1 件 publish し、1 件だけ表示されること
