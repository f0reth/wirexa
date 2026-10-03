# リファクタリング調査レポート（対象: backend mqtt）

調査範囲: `internal/domain/mqtt/`・`internal/application/mqtt/`・`internal/infrastructure/mqtt/`・`internal/adapters/mqtt_handler.go`（テストを除いて約 1,560 行）。
比較と影響範囲の確認のために、範囲外の `internal/application/store/cached_store.go`・`internal/infrastructure/json_store.go`・`internal/domain/*.go`・udp の対応ファイル・`app.go`・`internal/integration/mqtt_test.go`・`frontend/wailsjs/go/models.ts`・`frontend/src/domain/mqtt/topic.ts` を読んだ。

不具合や規約違反は見つからなかった。候補はどれも可読性と保守性の改善で、優先度「高」は無い。

## 改善候補一覧

| 優先度 | ファイル | 内容 |
|--------|----------|------|
| 中 | `internal/application/mqtt/service.go` | 800 行のファイルを、接続・購読・スキャンの責務ごとに同じパッケージ内で分割する |
| 中 | `internal/application/mqtt/service.go` | ライフサイクルイベントのペイロードが `map[string]any` で、`mqtt:message`・`mqtt:scan-topic` だけが型付き。型付きの struct に揃える |
| 低 | `internal/domain/mqtt/topic.go`・`internal/application/mqtt/service.go` | `ValidateTopicName`・`ValidateTopicFilter` の `field` 引数が常に `"topic"`。QoS の検証と `"broker URL"` のフィールド名も重複している |
| 低 | `internal/domain/mqtt/broker_port.go` ほか | `BrokerClient.Unsubscribe(topics ...string)` は常に 1 トピックで呼ばれる |
| 低 | `internal/application/mqtt/service.go` | 接続の検索と `NotFoundError` の生成が 7 か所に散っている |
| 低 | `internal/application/mqtt/service.go` | 切断の待機時間 `1000` が名前無しで 2 か所にある（スキャン側は `scanQuiesce`） |
| 低 | `internal/application/mqtt/service.go` | `reserveScan` が戻り値 4 つで、`ctx == nil` を「既存のスキャンに合流」の合図に使っている |
| 低 | `internal/domain/mqtt/topic.go`・`broker_port.go`・`port.go` | センチネルエラーと検証関数の置き場所がファイル名と合っていない。パッケージコメントも重複している |
| 低 | `internal/infrastructure/mqtt/profile_repository.go` | udp の `TargetRepository` と、DTO 変換以外が同じ形 |

## 改善候補の詳細

### service.go を責務ごとのファイルに分割する

- **該当箇所**: `internal/application/mqtt/service.go:1-800`
- **現状**: 1 ファイルに 3 つの責務が入っている。接続のライフサイクル（`Connect`・`runConnect`・`onConnected`・`onConnectionLost`・`Disconnect`・`detach`・`withConn`・`GetConnections`・`Shutdown`）、購読（`setSub`・`deleteSub`・`orderedSubs`・`updatePendingSubs`・`Publish`・`Subscribe`・`messageHandler`・`resubscribe`・`Unsubscribe`）、Broker Topics のスキャン（`topicScan`・`detachScan`・`disconnectScan`・`StartTopicScan`・`reserveScan`・`StopTopicScan`・`scanHandler`・`onScanConnectionLost`）。`connection` のメソッドも `updatePendingSubs` だけが 433 行目に離れている。
- **改善案**: 同じ `mqttapp` パッケージのまま、関数を移すだけで 3 ファイルにする。シグネチャ・ロック順序・コメントは変えない。
  - `service.go`: 定数・エラー・`connState`・`connection`・`MQTTService`・接続のライフサイクル・`GetConnections`・`Shutdown`
  - `subscription.go`: `connection` の購読メソッド 4 つと `Publish`・`Subscribe`・`Unsubscribe`・`messageHandler`・`resubscribe`
  - `topic_scan.go`: スキャン用の定数 3 つ・`errScanStopped`・`topicScan` と、スキャンの関数すべて
- **期待効果**: 可読性向上。ロック順序の規約（`mu → opMu → stateMu`）を確認するときに、読む範囲が責務ごとに区切られる。
- **挙動を守っているテスト**: `internal/application/mqtt/service_test.go`（全体）、`internal/integration/mqtt_test.go`
- **検証コマンド**: `task format` → `task lint` → `task test` → `task go:test:race` → `task go:test:integration`
- **付随作業**: 無し。`service_test.go`（約 2,770 行）を同じ区切りで分けるかは任意。

### ライフサイクルイベントのペイロードを型付きにする

- **該当箇所**: `internal/application/mqtt/service.go:260-263`・`297-299`・`317-320`・`341-343`・`684-687`、キーの定数は `service.go:20`
- **現状**: `mqtt:connected`・`mqtt:disconnected`・`mqtt:connection-failed`・`mqtt:connection-lost`・`mqtt:scan-stopped` は `map[string]any{keyConnectionID: ..., "error": ...}` を発行する。`mqtt:message`（`domain.MQTTMessage`）と `mqtt:scan-topic`（`domain.ScannedTopic`）は型付き。キーは `connectionId` が定数、`"error"` がリテラルで、配線形式がコードの 5 か所に分かれている。udp はイベントをすべて型で発行している（`listener_service.go:143`）。
- **改善案**: `internal/domain/mqtt/types.go` に、`ScannedTopic` と同じ扱いのイベント用の型を 2 つ足す。JSON は今と同じにする。
  - `ConnectionEvent{ConnectionID string `json:"connectionId"`}`: `mqtt:connected`・`mqtt:disconnected`
  - `ConnectionErrorEvent{ConnectionID string `json:"connectionId"`; Error string `json:"error"`}`: `mqtt:connection-failed`・`mqtt:connection-lost`・`mqtt:scan-stopped`
  - `omitempty` は付けない（1 つの型にまとめて `omitempty` にすると、エラー文言が空のときにキーが消えて配線形式が変わる）。`keyConnectionID` は不要になる。
- **期待効果**: 配線形式の定義が 1 か所になる。Go 側のフィールド名の打ち間違いをコンパイル時に検出できる。json タグ（配線上のキー名）の誤りはコンパイルでは検出できないので、付随作業のテストで守る。
- **挙動を守っているテスト**: `service_test.go` の `TestMQTTService_ConnectionLost_EmitsEventWithError`・`TestMQTTService_Connect_FailureEventCarriesError`・`TestMQTTService_TopicScan_ConnectionLost_StopsScan`、`internal/integration/mqtt_test.go` の `TestMQTT_LifecycleEvents`・`TestMQTT_TopicScan_ConnectionLost`
- **検証コマンド**: `task format` → `task lint` → `task test` → `task go:test:integration`
- **付随作業**:
  - テストが `map[string]any` へ型アサーションしているので書き換える（`service_test.go:1010`・`1619`・`1651`・`2710`、`internal/integration/mqtt_test.go:100`・`186`）。
  - JSON 化した結果を検証するテストを `internal/domain/mqtt/` に足す。既存のテストのエミッター（`service_test.go:33` の `mockEmitter`、`internal/integration/mqtt_test.go:174` の `mqttMockEmitter`）は Go の値をそのまま記録し、JSON 化を通さない。型アサーションを struct に書き換えるだけでは、json タグの誤記やキーの欠落を検出できない。検証する内容は次のとおり。
    - `ConnectionEvent` を `json.Marshal` した結果が `{"connectionId":"..."}` で、ほかのキーが無い。
    - `ConnectionErrorEvent` を `json.Marshal` した結果のキーが `connectionId` と `error` で、`Error` が空文字でも `error` キーが残る（`"error":""`）。
  - `task wails:generate` は差分が出ない見込み。イベント専用の `MQTTMessage`・`ScannedTopic` は現在の `frontend/wailsjs/go/models.ts` に生成されていない。念のため実行して `git diff --exit-code frontend/wailsjs` で確かめる。
  - `frontend/e2e/fake-backend/` は自前でペイロードを組み立てるので、追従は要らない。

### 検証関数の `field` 引数と、QoS・ブローカー URL の検証の重複

- **該当箇所**: `internal/domain/mqtt/topic.go:20`・`33`・`50`・`79`、`internal/application/mqtt/service.go:21`・`193`・`393-398`・`409-414`・`512`
- **現状**:
  - `ValidateTopicName(field, topic)` と `ValidateTopicFilter(field, filter)` の `field` は、本体の 3 か所すべてが `fieldTopic`（`"topic"`）を渡す。テストも `"topic"` だけ。同じファイルの `ValidateBrokerScheme` はフィールド名 `"broker URL"` を内部に持っていて、書き方が揃っていない。
  - `qos > 2` の検証と `ValidationError{Field: "qos", Message: "must be 0, 1, or 2"}` が `Publish` と `Subscribe` に重複している。
  - `"broker URL"` が `service.go:193`（必須の検証）と `topic.go:79`（スキームの検証）の 2 か所にある。
- **改善案**: エラー文言は変えない。
  - `field` 引数を消し、`topic.go` 内の定数 `"topic"` を使う。`service.go` の `fieldTopic` は削除する。
  - `domain` に `ValidateQoS(qos byte) error` を足して 2 か所から呼ぶ。
  - ブローカー URL の必須の検証を `domain` へ移し（例: `ValidateBroker(broker string, useTLS bool)` が必須とスキームの両方を検証する）、フィールド名を 1 か所にする。
- **期待効果**: 重複削減。検証と文言が `domain` に集まる。
- **挙動を守っているテスト**: `internal/domain/mqtt/topic_test.go`、`service_test.go` の `TestMQTTService_Connect_EmptyBroker`・`TestMQTTService_Connect_UseTLS_RejectsUnsupportedScheme`・`TestMQTTService_InvalidTopics_RejectedBeforeClient`・`TestMQTTService_Publish_InvalidQoS`・`TestMQTTService_Subscribe_InvalidQoS_ReturnsValidationError`、integration の `TestMQTT_Publish_InvalidInput`・`TestMQTT_Subscribe_InvalidInput`・`TestMQTT_InvalidWildcardTopics`
- **検証コマンド**: `task format` → `task lint` → `task test` → `task go:test:integration`
- **付随作業**: `topic_test.go` の呼び出しから `"topic"` 引数を外す。`ValidateQoS` のテストを足す。

### `BrokerClient.Unsubscribe` の可変長引数

- **該当箇所**: `internal/domain/mqtt/broker_port.go:42`、`internal/infrastructure/mqtt/paho_client.go:110`・`291-305`、呼び出し元は `internal/application/mqtt/service.go:502`・`519`
- **現状**: ポートは `Unsubscribe(topics ...string)` だが、本体の呼び出しは 2 か所とも 1 トピック。対になる `Subscribe` は 1 トピックずつ受ける。複数トピックを渡した場合の部分失敗の扱いは、契約のコメントにも書かれていない。
- **改善案**: `Unsubscribe(topic string) error` にする。`pahoClient.removeRoutes` も 1 フィルターを受ける形にできる（`slices.Contains` が不要になる）。
- **期待効果**: ポートが実際の使い方と一致する。使われていない複数指定の経路が無くなる。
- **挙動を守っているテスト**: `paho_client_test.go` の `TestPahoClient_Unsubscribe_StopsDeliveryWhenNoFilterMatches`・`TestPahoClient_Unsubscribe_AckTimeout_RemovesRoute`、`service_test.go` の `TestMQTTService_Unsubscribe_*`・`TestMQTTService_Reconnect_ResubscribeFailures`
- **検証コマンド**: `task format` → `task lint` → `task test`
- **付随作業**: `service_test.go` の `mockBrokerClient.Unsubscribe` と `unsubscribeFn`（`56`・`87`・`340`・`590`・`877`・`901`・`1810`・`1893` 行目）を新しいシグネチャに直す。出力ポートなので RPC には影響しない。

### 接続の検索と `NotFoundError` の生成の重複

- **該当箇所**: `internal/application/mqtt/service.go:356`・`362`・`377`・`386`・`610`・`615`・`634`
- **現状**: `&cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}` が 7 か所にある。`mu.RLock` で map を引いて無ければ `NotFoundError` を返す処理も、`withConn`（373-378 行目）と `StopTopicScan`（630-635 行目）で同じ。
- **改善案**: `connNotFound(id string) error` と、`lookup(id string) (*connection, error)`（`RLock` で引くだけ）を足す。`detach` と `reserveScan` は `mu` の保持中に続きの処理をするので、`lookup` は使わず `connNotFound` だけを使う。
- **期待効果**: 重複削減。
- **挙動を守っているテスト**: `service_test.go` の `TestMQTTService_Disconnect_NotFound`・`TestMQTTService_Publish_ConnectionNotFound`・`TestMQTTService_Subscribe_ConnectionNotFound`・`TestMQTTService_Unsubscribe_ConnectionNotFound`・`TestMQTTService_TopicScan_UnknownConnection`・`TestMQTTService_Disconnect_RejectsQueuedOperation`、integration の `TestMQTT_Disconnect_NotFound`・`TestMQTT_Disconnect_Twice`・`TestMQTT_OperationsAfterDisconnectAndShutdown`
- **検証コマンド**: `task format` → `task lint` → `task test` → `task go:test:race`
- **付随作業**: 無し

### 切断の待機時間のマジックナンバー

- **該当箇所**: `internal/application/mqtt/service.go:335`・`776`（`Disconnect(1000)`）、比較対象は `service.go:30` の `scanQuiesce`
- **現状**: 通常の接続の切断は `1000` をリテラルで 2 か所に書き、スキャン用の接続は定数 `scanQuiesce = 250` を使う。`252`・`591`・`689` 行目の `Disconnect(0)` は「待たずに捨てる」意味。
- **改善案**: `disconnectQuiesce = 1000`（ms）を定数にして 2 か所で使う。`Disconnect(0)` はそのままにする。
- **期待効果**: 可読性向上。`Disconnect` と `Shutdown` の待機時間が揃っていることがコードから分かる。
- **挙動を守っているテスト**: `service_test.go` の `TestMQTTService_Disconnect_Success`・`TestMQTTService_Shutdown_DisconnectsAll`
- **検証コマンド**: `task format` → `task lint` → `task test`
- **付随作業**: 無し

### `reserveScan` の戻り値

- **該当箇所**: `internal/application/mqtt/service.go:540-548`・`602-625`
- **現状**: `reserveScan` は `(*connection, *topicScan, context.Context, error)` を返し、呼び出し元は `ctx == nil` で「既にスキャンがあるので、その結果を待つ」と判定する。`context.Context` の nil を合図に使っていて、型からは読み取れない。
- **改善案**: 予約の結果を小さな struct（例: `scanReservation{conn, scan, ctx, joined bool}`）にまとめるか、戻り値に `joined bool` を足して `ctx` の nil 判定をやめる。ロックの区間と `connWg.Add(1)` の位置は変えない。
- **期待効果**: 可読性向上。
- **挙動を守っているテスト**: `service_test.go` の `TestMQTTService_TopicScan_StartTwiceStopAndRestart`・`TestMQTTService_TopicScan_SecondStartJoinsTheFirst`・`TestMQTTService_StartTopicScan_AfterShutdown_Rejected`・`TestMQTTService_TopicScan_StoppedWhileStarting`
- **検証コマンド**: `task format` → `task lint` → `task test` → `task go:test:race`
- **付随作業**: 無し。「service.go を責務ごとのファイルに分割する」と一緒に行うと差分が読みやすい。

### domain のセンチネルエラーと検証関数の置き場所

- **該当箇所**: `internal/domain/mqtt/topic.go:13-14`・`63-80`、`internal/domain/mqtt/broker_port.go:1`・`9-11`、`internal/domain/mqtt/port.go:1`、`internal/application/mqtt/profile_service.go:1`・`service.go:1`
- **現状**:
  - `BrokerClient` の契約が返すセンチネルエラーのうち、`ErrAckTimeout` は `broker_port.go`、`ErrSubscriptionRejected` は `topic.go` にある。
  - ブローカー URL のスキームを検証する `ValidateBrokerScheme` が `topic.go` にある（テストも `topic_test.go`）。
  - パッケージコメントが `port.go` と `broker_port.go` に同じ文で重複している。`mqttapp` も `service.go` と `profile_service.go` に別々の文がある。godoc は両方を連結して表示する。
- **改善案**: `ErrSubscriptionRejected` を `broker_port.go` の `ErrAckTimeout` の隣へ移す。`ValidateBrokerScheme` を `broker.go`（テストは `broker_test.go`）へ移す。パッケージコメントは各パッケージ 1 ファイルだけに残す。シンボル名とパッケージは変えない。
- **期待効果**: 可読性向上。
- **挙動を守っているテスト**: `internal/domain/mqtt/topic_test.go` の `TestValidateBrokerScheme`、`service_test.go` の `TestMQTTService_Reconnect_ResubscribeFailures`
- **検証コマンド**: `task format` → `task lint` → `task test`
- **付随作業**: 無し。「検証関数の `field` 引数」の候補でブローカー URL の検証をまとめる場合は、同じ作業で行う。

### `ProfileRepository` と udp の `TargetRepository` の重複

- **該当箇所**: `internal/infrastructure/mqtt/profile_repository.go:15-54`、比較対象は `internal/infrastructure/udp/target_repository.go:15-54`
- **現状**: コンストラクタ（`NewJSONStore` → `logger != nil` なら `SetLogger`）、`Load`（stored を domain へ変換するループ）、`Save`、`Delete` が、型名以外は同じ。違いは stored DTO と変換関数だけ。`logger != nil` の分岐は、`JSONStore.SetLogger` が nil を「記録しない」として受け付ける（`json_store.go:20-24`）ので無くても同じ結果になる。
- **改善案**: 共通部分（`internal/infrastructure/`）に、変換関数を受け取る汎用のラッパー（例: `ConvertingStore[D, S any]`。`toStored func(*D) S` と `toDomain func(*S) D` を渡す）を置き、`ProfileRepository` と `TargetRepository` をそれで実装する。stored DTO・json タグ・変換関数は各リポジトリに残し、保存形式は変えない。
- **期待効果**: 重複削減（1 リポジトリあたり約 25 行）。削減量は小さく、共通部分と udp にも手が入るので、`backend` または `backend common` の調査で http の `CollectionRepository` も含めて判断するのがよい。mqtt 単独で先に行う必要は無い。
- **挙動を守っているテスト**: `internal/infrastructure/mqtt/profile_repository_test.go`（`RoundTripKeepsEveryField`・`GoldenFormat`・`StoredDTOHasNoDomainTypes`・`QuarantinesCorruptFile`・`SaveRejectsUnsafeID`）、`internal/infrastructure/udp/target_repository_test.go`、integration の `TestMQTT_ProfilePersistenceRoundTrip`・`TestMQTT_CorruptProfileAmongValidOnes`
- **検証コマンド**: `task format` → `task lint` → `task test` → `task go:test:integration`
- **付随作業**: 無し。`testutil.AssertNoTypesFrom` の検査対象（stored DTO）は変わらない。

## 対応不要と判断した箇所（理由つき）

| 箇所 | 理由 |
|---|---|
| `storedBrokerProfile` と `domain.BrokerProfile` が同じフィールド（`profile_repository.go:59-91`） | 意図的な重複。保存形式を RPC の json タグから独立させるため。統合・埋め込みはしない |
| `MQTTHandler` の全メソッドがサービスへの素通し（`mqtt_handler.go:42-96`） | adapters は RPC の面と入力ポートの定義を受け持つ層で、`app.go` の 2 段階の配線に必要。DTO を足す提案は規約違反 |
| `MQTTConnectionUseCase` が 8 メソッド（`mqtt_handler.go:9-18`） | バインド対象のメソッドと 1 対 1 で、使っていないメソッドは無い。分割しても使う側は `MQTTHandler` だけ |
| `ProfileService` と udp の `TargetService` が `CachedStore` の薄いラッパーで同じ形（`profile_service.go`） | 共通化は既に `store.CachedStore` で済んでいる。残りは型の指定と入力ポートのメソッド名だけ |
| `ProfileService.SaveProfile` に入力検証が無い（udp の `SaveTarget` は検証する） | 検証を足すと、今は保存できるプロファイルが拒否されるようになる（挙動変更） |
| `domain.ProfileRepository` が `store.Repository[T]` と同じ形（`port.go:5-9`） | 出力ポートは domain に置く規約。`store.Repository` は application 側の構造的な制約で、domain からは import できない |
| `ProfileRepository.Save` がポインタ、`Load` が値のスライス | `store.Repository[T]` の形に合わせている。3 プロトコルで共通 |
| `MQTTClientConfig.ConnectTimeout`・`TokenTimeout` を本体が設定していない（`app.go:140` は空の値） | テスト用の差し込み口。`paho_client_test.go` と `internal/integration/mqtt_test.go` が短いタイムアウトを渡している |
| `pahoClient` の `errors.New("not connected")`・`errors.New("connection timed out")` がセンチネルでない（`paho_client.go:221`・`247`） | 文字列で比較している箇所は Go 側にもフロント側にも無い。application は `errors.Is`（`ErrAckTimeout`・`ErrSubscriptionRejected`）と `IsConnected()` で判定している |
| `Publish` が `token.Error()` を wrap せずに返す（`paho_client.go:254`） | 呼び出し元の `MQTTService.Publish` が `failed to publish: %w` で wrap している |
| `errShuttingDown` が `application/http/request_service.go:20` にもある | 非公開のセンチネルで、パッケージごとに `errors.Is` の対象が分かれている。共有するには `internal/domain` に公開シンボルを足すことになり、範囲外（共通部分）の変更になる |
| ログの `"source", "mqtt"` が 14 か所で繰り返される | http・udp と同じ書き方。まとめるには `cmn.Logger` に `With` 相当を足す必要があり、共通部分の変更になる |
| `filterMatches`（`paho_client.go:74-94`）とフロントの `topicMatches`（`frontend/src/domain/mqtt/topic.ts`） | 目的が違う。バックエンドは重なる購読への配送を 1 回にするための振り分け、フロントは受信メッセージを購読の表示へ割り当てるための照合。RPC では共有できない。フロント側は `$share/`・`$queue/` の接頭辞を外さないが、範囲外なので候補にはしない |
| `filterMatches` が infrastructure にある | 呼び出し元は `pahoClient.dispatch` だけで、paho のルーターの代わりをする実装の詳細 |
| `applyTLSScheme`（`paho_client.go:139`）と `ValidateBrokerScheme`（`topic.go:67`）がスキームの一覧を別々に持つ | 役割が違う（検証と変換）。対応付けはコメントに書かれ、`TestApplyTLSScheme` と `TestValidateBrokerScheme` がそれぞれ守っている |
| `Connect(config domain.ConnectionConfig)` が値渡し、`SaveProfile` も値渡し | RPC の境界。ポインタと値の変更は禁止事項 |
| `connection`・`topicScan` のロック構成（`mu → opMu → stateMu`、`pendingMu`） | コメントで順序と理由が決められ、`service_test.go` の並行テストが守っている。作り直しは大規模リライトにあたる |
| `SubscriptionInfo`・`ConnectionStatus` などのドメイン型とフロントの `domain/mqtt/types.ts` | 意図的な重複（Go の型が配線型の正） |
