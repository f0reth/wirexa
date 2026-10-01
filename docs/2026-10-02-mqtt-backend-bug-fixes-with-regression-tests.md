# 変更計画書: backend MQTT の調査で見つかった不具合の修正と再発防止テストの追加

## 概要

`docs/bug-investigation-backend-mqtt.md` の #1〜#6 は現行コードで未修正である（`paho_client.go`・`service.go`・`install.ts` を読んで確かめた）。正しい挙動を期待するテストは修正なしでは通らないので、問題ごとに「再現テストを先に書く → 修正して通す」の順で進める。要確認 1 は `newFakeBroker` で再現するかをテストで確かめ、再現した場合だけ修正する。要確認 2（フロントのイベント取りこぼし）は backend のテストでは確かめられないので対象外とする。

修正後の挙動は次のとおり。

| # | 修正後の挙動 |
| --- | --- |
| 1 | 接続が開いていない間の `Publish` は送らずにエラーを返す。PUBACK 待ちの timeout は「送達を確認できなかった」と分かるエラーにする |
| 2 | `UseTLS=true` では、スキームの大文字小文字を区別せずに TLS のスキームへ変換し、スキーム無しは `ssl://` を補う。TLS にできないスキームは `Connect` が `ValidationError` で拒否する |
| 3 | `GetConnections` は接続を作成順、購読を購読した順で返す |
| 4 | 張り直しが拒否以外で失敗し、かつ接続が開いたままなら、拒否と同じく購読を外して `Unsubscribe` で片付ける（イベントは足さない） |
| 5 | fake backend の `DeleteProfile` は未知の ID でエラーにする。`Connect` の既定の失敗は、接続 ID を返してから `mqtt:connection-failed` を出す |
| 6 | `Unsubscribe` の timeout は「解除した」に倒し（`conn.subs` と振り分け先を外す）、エラーは返す。`Subscribe` の timeout は現状のまま、エラーの内容だけ変える |

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `internal/domain/mqtt/broker_port.go` | Domain | 変更 | `ErrAckTimeout`（ブローカーの応答を時間内に確認できなかった）を追加。`Publish` に「接続が開いていなければエラーを返す」、`Publish` / `Subscribe` / `Unsubscribe` に「応答を確認できなければ `ErrAckTimeout`」、`Unsubscribe` に「`ErrAckTimeout` のときも振り分け先は外す」をコメントで追記 |
| `internal/domain/mqtt/topic.go` | Domain | 変更 | `ValidateBrokerScheme(broker string, useTLS bool) error` を追加（#2。`UseTLS=true` で TLS にできないスキームを `ValidationError` にする） |
| `internal/domain/mqtt/topic_test.go` | Domain | 変更 | `ValidateBrokerScheme` の表テスト |
| `internal/infrastructure/mqtt/paho_client.go` | Infrastructure | 変更 | `Publish` の先頭に `IsConnectionOpen` の確認（#1）。3 つの timeout を `ErrAckTimeout` を包んだエラーにする（#1・#6）。`Unsubscribe` は timeout でも `removeRoutes` する（#6）。`applyTLSScheme` を大文字小文字を区別しない照合にし、スキーム無しに `ssl://` を補う（#2） |
| `internal/infrastructure/mqtt/paho_client_test.go` | Infrastructure | 変更 | #1・#2・#6・要確認 1 のテストを追加。`TestApplyTLSScheme` の期待値を変更 |
| `internal/application/mqtt/service.go` | Application | 変更 | `Connect` で `ValidateBrokerScheme` を呼ぶ（#2）。`connection` に作成順の連番、購読に順序を持たせて `GetConnections` と `resubscribe` をその順にする（#3）。`resubscribe` の失敗の扱い（#4）。`Unsubscribe` が `ErrAckTimeout` でも `conn.subs` から外す（#6） |
| `internal/application/mqtt/service_test.go` | Application | 変更 | #2・#3・#4・#6 のテストを追加。`TestMQTTService_Reconnect_ResubscribeFailures` の `transient failure` を「接続が切れている場合」に直す |
| `internal/integration/mqtt_test.go` | Integration | 変更 | 再接続中の Publish（#1）、`GetConnections` の並び順（#3）、`UseTLS` とスキームの拒否（#2）を実ブローカーで確かめるテストを追加 |
| `frontend/e2e/fake-backend/types.ts` | e2e | 変更 | `FakeSeed.mqttConnect` を `"ok" \| "reject"` にし、未設定の説明を「ID を返してから `mqtt:connection-failed`」に直す |
| `frontend/e2e/fake-backend/install.ts` | e2e | 変更 | `DeleteProfile` が未知の ID で `profile not found: <id>` を投げる。`Connect` の既定を「ID を返す → 次のタスクで接続を消して `mqtt:connection-failed` を emit」にする。`"reject"` は従来どおり RPC を失敗させる。#2 の検証を足す（#5・#2） |
| `frontend/e2e/ui/mqtt/profiles.spec.ts` | e2e | 変更 | 接続失敗の 3 テストを新しい既定に合わせる。RPC 自体が失敗する経路（`"reject"`）のテストを 1 件残す |

製品コードのフロントエンド（`frontend/src/`）は変更しない。

## 実装方針

依存方向は変えない。エラー値と検証関数は domain に置き、infrastructure（`pahoClient`）と application（`MQTTService`）がそれぞれ domain だけを import する。adapters・`app.go`・RPC のシグネチャ・イベント名は変わらない。

### #1 Publish（infrastructure）

- `pahoClient.Publish` の先頭で `p.client.IsConnectionOpen()` を確かめ、false なら `errors.New("not connected")` を返す。確認と送信の間に切断された場合までは防げない（レポートに書いたとおり、窓を狭める修正）ので、その旨をコメントに書く。
- `token.WaitTimeout` が false のときは `fmt.Errorf("publish was not acknowledged in time (it may still be delivered after reconnecting): %w", domain.ErrAckTimeout)` を返す。
- `MQTTService.Publish` は変えない（`failed to publish: %w` で包むので `errors.Is` が通る）。

### #2 UseTLS とスキーム（domain・application・infrastructure）

- `ValidateBrokerScheme(broker, useTLS)`: `useTLS` が false なら何もしない。true なら、`://` を含まない（スキーム無し。paho の `AddBroker` と同じ判定）か、小文字にしたスキームが `tcp` / `mqtt` / `ws` / `ssl` / `tls` / `mqtts` / `wss` のどれかなら通す。それ以外は `&cmn.ValidationError{Field: "broker URL", ...}`。フロントの `composeBrokerUrl` が付ける 5 つのスキームはすべて通る。
- `MQTTService.Connect` は空文字の検証の直後にこれを呼ぶ。
- `applyTLSScheme` は `://` の前を小文字にして照合し、`tcp`→`ssl`、`mqtt`→`mqtts`、`ws`→`wss` に変える。スキーム無しは `ssl://` を補う。空文字はそのまま返す（`Connect` が先に拒否する）。
- fake backend の `Connect` にも同じ検証を足す（`config.useTLS` を読む）。

### #3 並び順（application）

- `MQTTService` に `nextSeq uint64`（`mu` で保護）を持たせ、`Connect` が接続を登録する区間で `connection.seq` に入れる。`GetConnections` は `seq` の昇順に並べる。
- 購読は「購読した順」にする。リロード前の画面と fake backend（挿入順）の両方に一致する。`connection.subs` は `map[string]byte` のまま、順序用に `subOrder []string` を足し、変更を `setSub(topic, qos)` / `deleteSub(topic)`（`stateMu` 保持中に呼ぶ）の 2 つにまとめる。既存トピックの QoS 変更では順序を変えない。変更箇所は `Subscribe`（確立前・確立後）・`Unsubscribe`（同）・`resubscribe` の削除の 5 か所。
- `onConnected` は `maps.Clone` の代わりに順序つきのスナップショットを取り、`resubscribe` も購読した順に張り直す（テストが順序に依存しなくなる）。

### #4 確立前に受け付けた購読の失敗（application）

- `resubscribe` で `Subscribe` が失敗したとき、`ErrSubscriptionRejected` であるか、**`conn.client.IsConnected()` が true**（接続が開いたまま失敗した）なら、`conn.subs` から外して `Unsubscribe` で片付ける。接続が切れている失敗は従来どおり残して次の再接続に任せる。
- イベントは足さない。拒否された購読も現状はログだけで、フロントに知らせていない。同じ扱いに揃える（知らせる仕組みを足すなら拒否の場合も含めて別計画にする）。
- 確立済みの接続の張り直し（再接続時）にも同じ条件が掛かる。張り直しでは `pahoClient` が振り分け先を残すが、`Unsubscribe` で片付けるので表示と受信は一致する。

### #6 Subscribe / Unsubscribe の timeout（infrastructure・application）

- `pahoClient.Unsubscribe`: timeout のとき `removeRoutes(topics...)` してから `ErrAckTimeout` を包んだエラーを返す。
- `MQTTService.Unsubscribe`: `errors.Is(err, domain.ErrAckTimeout)` なら `conn.subs` から外してからエラーを返す。フロントの `removeSubscription` は RPC が失敗しても行を外すので、表示・`GetConnections`・受信がすべて「購読していない」に揃う。
- `pahoClient.subscribe`: timeout のエラーを `ErrAckTimeout` を包んだものにするだけ（登録を外す挙動は現状のまま）。
- #4 の判定は `IsConnected()` で行うので、`ErrAckTimeout` には依存しない。

### 要確認 1（infrastructure のテストで再現を確かめる）

`newFakeBroker` で、SUBSCRIBE を読んだら SUBACK を返さずに接続を閉じ、再接続は受け付けるブローカーを作る。`TokenTimeout` を 2 秒程度にして `pahoClient.Subscribe` を呼び、再接続が確立した時点で `Subscribe` がまだ戻っていないか（`TokenTimeout` まで待つか）を測る。

- **再現しない**（切断で token が完了して早く戻る）: テストをその挙動を守るものとして残し、レポートの要確認 1 を「問題なし」として結果を報告する。
- **再現する**: `pahoClient` に切断を知らせるチャネルを持たせ（`SetConnectionLostHandler` で閉じ、`OnConnect` で作り直す）、`subscribe` / `Unsubscribe` の待ちを `token.Done()`・切断・timeout の `select` にして、切断時はすぐ `connection lost` のエラーを返す。`opMu` を持つ時間が切断までに縮み、再接続後の `onConnected` がすぐ走る。テストは「切断後すぐ戻る」に書き換える。この修正の前に結果を報告する。

### #5 fake backend（e2e）

- `DeleteProfile`: ID が無ければ `profile not found: <id>` を投げる（`SaveProfile` の未知 ID と同じ文言）。
- `Connect`: 既定では接続を `db.mqttConnections` に足して ID を返し、`setTimeout` で接続を消してから `mqtt:connection-failed`（`{ connectionId, error: "connection refused" }`）を emit する。確立より先に `Disconnect` されていたら emit しない（Go の `runConnect` と同じ）。`"reject"` は RPC を `connection refused` で失敗させる（Go では終了処理中と検証エラーに当たる経路で、フロントの `Failed to connect` / `Failed to reconnect` の通知を守るために残す）。

## 永続化への影響

なし。stored DTO・golden ファイル・localStorage のキーと値・復旧方針の分類は変わらない。

## コード生成

不要。バインド対象の構造体・メソッドとイベント名は変えない。`ErrAckTimeout` と `ValidateBrokerScheme` は RPC に出ない。

## テスト方針

テストはどれも修正の前に書き、修正前に失敗することを確かめてから修正する（#5 を除く）。

### Go ユニット

`internal/infrastructure/mqtt/paho_client_test.go`

- `TestPahoClient_Publish_WhileReconnecting_ReturnsError`（#1）: `TestPahoClient_ConnectionLost_InvokesCallback` と同じ「1 本目だけ受けて切り、再接続は受けない」ブローカーで `onConnectionLost` を待ち、`Publish` を QoS 0 と QoS 1 で呼ぶ。どちらもエラーで、`TokenTimeout` より十分早く戻ること（QoS 1 が保存されて timeout まで待たないこと）を確かめる。
- `TestPahoClient_Publish_AckTimeout`（#1）: `newSubscribingBroker`（PUBLISH に応答しない）へ QoS 1 で送り、`errors.Is(err, domain.ErrAckTimeout)` を確かめる。
- `TestPahoClient_Publish_Connected`（#1 の修正で正常系を壊さないこと）: 同じブローカーへ QoS 0 で送って成功し、ブローカー側で PUBLISH を読めること。`subscribingBroker` に受信した PUBLISH を記録する口を足す。
- `TestPahoClient_Unsubscribe_AckTimeout_RemovesRoute`（#6）: UNSUBSCRIBE に応答しないブローカーで `Unsubscribe` が `ErrAckTimeout` を返し、その後に届いた PUBLISH で handler が呼ばれないこと。`subscribingBroker` に「応答しないパケット種別」の切り替えを足す。
- `TestPahoClient_Subscribe_AckTimeout_IsNotRouted`（#6）: SUBSCRIBE に応答しないブローカーで `Subscribe` が `ErrAckTimeout` を返し、振り分け先に残らないこと。
- `TestPahoClient_Subscribe_ConnectionLostWhileWaiting`（要確認 1）: 上の「要確認 1」のとおり。
- `TestApplyTLSScheme`（#2）: `TCP://`・`Mqtt://`・`WS://` が TLS のスキームになること、`broker:1883` が `ssl://broker:1883` になることを足す（`broker:1883` の期待値は変更）。
- `TestNewPahoClientFactory_UseTLS`（#2）: スキーム無しと大文字のスキームで `Servers()[0].Scheme` が `ssl` になる場合を表に足す。

`internal/domain/mqtt/topic_test.go`

- `TestValidateBrokerScheme`（#2）: `useTLS=false` は何でも通す。`useTLS=true` は 7 つのスキーム・その大文字・スキーム無しを通し、`http://`・`unix://` を `ValidationError` にする。

`internal/application/mqtt/service_test.go`

- `TestMQTTService_Connect_UseTLS_RejectsUnsupportedScheme`（#2）: `ValidationError` を返し、factory が呼ばれず、`GetConnections` が空のまま。
- `TestMQTTService_GetConnections_StableOrder`（#3）: 接続を 5 本作り、うち 1 本に購読を 8 個（名前順と異なる順で）足す。`GetConnections` を 50 回呼び、毎回「接続は作成順、購読は購読した順」であること。途中の購読を 1 つ解除して別の購読を足しても順序が保たれること、既存トピックの QoS 変更で位置が変わらないことも確かめる。
- `TestMQTTService_Reconnect_ResubscribesInOrder`（#3）: 再接続時の `Subscribe` の呼び出し順が購読した順であること。`TestMQTTService_Reconnect_Resubscribes` の `slices.SortFunc` は外す。
- `TestMQTTService_Reconnect_ResubscribeFailures`（#4）: `transient failure` を `isConnectedFn: false` の「接続が切れた失敗」に直し（購読を残す）、`fails while connected`（`isConnectedFn: true` で拒否以外のエラー → 購読を外して `Unsubscribe` を呼ぶ）と、その `Unsubscribe` も失敗する場合を足す。
- `TestMQTTService_Subscribe_BeforeConnected_FailsOnConnected`（#4）: `newPendingConn` で確立前に 2 つ購読し、確立時の `Subscribe` を片方だけ（接続は開いたまま）失敗させる。失敗した方が `GetConnections` から消えて `Unsubscribe` が呼ばれ、もう片方は残ること。
- `TestMQTTService_Unsubscribe_AckTimeout_RemovesSubscription`（#6）: `unsubscribeFn` が `ErrAckTimeout` を包んで返すと、`Unsubscribe` はエラーを返すが `GetConnections` から購読が消えること。他のエラーでは残ること（`TestMQTTService_Unsubscribe_ClientError` に購読が残る確認を足す）。
- `TestMQTTService_Subscribe_AckTimeout_NotTracked`（#6）: `Subscribe` が `ErrAckTimeout` でも `GetConnections` に載らないこと（現状の挙動を固定）。

### Go 統合（`internal/integration/mqtt_test.go`）

- `TestMQTT_Publish_WhileReconnecting`（#1）: `TestMQTT_ConnectionLostAndReconnect` と同じ手順でブローカーを止め、`mqtt:connection-lost` の後の `Publish`（QoS 0・1）がすぐエラーを返すこと。ブローカーを戻して別のクライアントで購読し、止まっていた間のメッセージが届かないこと（QoS 1 が保存されていないこと）を `noMessage` で確かめる。
- `TestMQTT_GetConnections_Order`（#3）: 実ブローカーに 3 本つなぎ、購読を複数足して、ハンドラ経由の `GetConnections` の順序を確かめる。
- `TestMQTT_Connect_UseTLS_UnsupportedScheme`（#2）: ハンドラの `Connect` が `ValidationError` を返し、イベントが出ないこと（`assertSilent`）。

### フロント ユニット

変更なし（`frontend/src/` を変えない）。

### UI e2e（`frontend/e2e/ui/mqtt/profiles.spec.ts`）

- 既定の失敗（ID を受け取った後に `mqtt:connection-failed`）で、エラーの通知が出て、`fake.snapshot().mqttConnections` が空で、もう一度 Connect を押せること。通知の文言と接続バーの表示は、実装時に実際の画面で確かめて期待値にする。
- `Save & Connect` で接続が失敗してもプロファイルが残ること（既定の失敗に合わせて通知の期待値を直す）。
- `mqttConnect: "reject"` で、従来の `Failed to reconnect` の通知と Dismiss のテストを残す。

### フルスタック e2e

変更なし。

## 副作用・注意事項

- **Publish**: 自動再接続中の QoS 1/2 の Publish は、これまで「timeout のエラーの後で再接続時に届く」だったが、すぐエラーを返して届かなくなる。
- **UseTLS**: スキーム無しの Broker は、これまで平文の `tcp://` で繋がっていたが `ssl://` になる。保存済みプロファイルにスキーム無しで `UseTLS=true` のものがあれば、接続先のポートが TLS を話さない場合に接続が失敗するようになる（UI からは作れない）。TLS にできないスキームは `mqtt:connection-failed` ではなく `Connect` の RPC のエラーになる。
- **購読の張り直し**: 接続が開いたまま拒否以外で失敗した購読は、次の再接続を待たずに外れる。フロントの一覧には残る（拒否の場合と同じで、リロードすると消える）。
- **Unsubscribe の timeout**: エラーは返すが購読は外れる。ブローカー側に購読が残っても、届いたメッセージは捨てる。
- **エラーメッセージ**: `publish timed out` / `subscribe timed out` / `unsubscribe timed out` の文言が変わる。この文言に依存しているテストとフロントのコードは無い（検索で確かめた）。
- **`GetConnections` の順序**: 接続は作成順、購読は購読した順になる。フルスタック e2e（`e2e/integration/mqtt/mqtt.spec.ts`）に順序を前提にした比較があれば、安定する方向の変更なので期待値はそのままで通る見込みだが、実装時に確かめる。
- **fake backend**: `mqttConnect` 未設定の Connect が RPC では成功するようになる。`profiles.spec.ts` 以外に既定の失敗に依存する spec が無いことは検索で確かめた。
- **時間のかかるテスト**: timeout を待つテストは `TokenTimeout` を 1〜2 秒にして、テスト全体の所要時間を数秒の増加に収める。
- 要確認 1 が再現した場合の修正は、結果を報告してから行う。

## Git運用

- **ブランチ名**: `fix/mqtt-backend-bug-investigation`
- **コミット分割方針**: 問題ごとに 1 コミットにし、それぞれテストと修正を一緒に入れる（どのコミットでもテストが通る状態を保つ）。
  1. `fix(mqtt): 接続が開いていない間の Publish をエラーにする`（#1。`ErrAckTimeout` の追加を含む）
  2. `fix(mqtt): UseTLS でスキームによって平文で接続しないようにする`（#2。fake backend の検証を含む）
  3. `fix(mqtt): GetConnections が接続と購読を決まった順で返すようにする`（#3）
  4. `fix(mqtt): 接続が開いたまま張り直しに失敗した購読を外す`（#4）
  5. `fix(mqtt): Unsubscribe の応答を確認できなかったときに購読を外す`（#6）
  6. `test(mqtt): SUBACK 待ち中に接続が切れたときの Subscribe の待ち時間を確かめる`（要確認 1。再現して修正する場合は `fix(mqtt): ...` にする）
  7. `test(integration): 再接続中の Publish と GetConnections の並び順を検証する`
  8. `fix(e2e): fake backend の DeleteProfile と Connect の失敗を Go に合わせる`（#5）
  9. `docs: backend MQTT の不具合修正と再発防止テストの変更計画書を追加する`
- **完了条件**: すべて通ってから main へマージする。
  - `task format` → `task lint` → `task test`
  - `task go:test:integration`（統合テストを足し、`internal/integration/` が検証する振る舞いを変えるため）
  - `task frontend:test:e2e`（fake backend と spec を変えるため）
  - `task go:test:race`（`opMu`・`stateMu` の下の処理に触れるため。gcc が無くて実行できなければその旨を報告し、CI の結果で確認する）
  - 任意: `task frontend:test:e2e:fullstack`（`GetConnections` の順序の変更が実バックエンドとの結合に影響しないことを確かめる）
