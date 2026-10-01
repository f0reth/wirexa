# バグ調査レポート（対象: backend mqtt）

生成日: 2026-10-02

調査範囲: `internal/domain/mqtt/`・`internal/application/mqtt/`・`internal/infrastructure/mqtt/`・`internal/adapters/mqtt_handler.go`。
paho の挙動は `github.com/eclipse/paho.mqtt.golang@v1.5.1` のソースを読んで確かめた（実機での再現はしていない）。

## サマリー

| 深刻度 | 件数 |
|--------|------|
| 高     | 0    |
| 中     | 1    |
| 低     | 5    |
| 要確認 | 2    |

**最優先で対処すべき問題**: 無し（高深刻度は無い。中は #1 の「接続が開いていない間の Publish の結果が実態と食い違う」）

## 問題一覧

| # | 深刻度 | 観点 | ファイル | 概要 |
|---|--------|------|----------|------|
| 1 | 中     | G6・G2 | internal/infrastructure/mqtt/paho_client.go:231 | 接続が開いていない間の Publish が、QoS 0 では送らずに成功を返し、QoS 1/2 では timeout エラーを返した後で再接続時に送信される |
| 2 | 低     | G6   | internal/infrastructure/mqtt/paho_client.go:135 | `UseTLS=true` でもスキーム無し・大文字を含むスキーム（`TCP://` など）の Broker は平文で接続する |
| 3 | 低     | G6・C2 | internal/application/mqtt/service.go:649 | `GetConnections` が返す接続と購読の並び順が呼び出しのたびに変わる（画面に出るのは購読一覧の並び） |
| 4 | 低     | G6   | internal/application/mqtt/service.go:445 | 確立前に受け付けた購読の初回購読が拒否以外で失敗すると、購読中と表示されたままメッセージが届かない |
| 5 | 低     | C2   | frontend/e2e/fake-backend/install.ts:662 | fake backend の `DeleteProfile` と `Connect` の失敗の出方が Go と違う |
| 6 | 低     | G6   | internal/application/mqtt/service.go:379 | 確立済みの接続での Subscribe / Unsubscribe が timeout すると、ブローカー側の成否が分からないまま片方の結果に決めてしまう |

## 問題の詳細

### #1 [深刻度: 中] 接続が開いていない間の Publish の結果が実態と食い違う（G6・G2）

- **該当箇所**: `internal/infrastructure/mqtt/paho_client.go:231`（呼び出し元は `internal/application/mqtt/service.go:360`）
- **コード抜粋**:
  ```go
  func (p *pahoClient) Publish(topic string, qos byte, retained bool, payload string) error {
  	token := p.client.Publish(topic, qos, retained, payload)
  	if !token.WaitTimeout(p.tokenTimeout) {
  		return errors.New("publish timed out")
  	}
  	return token.Error()
  }
  ```
- **問題の説明**: `MQTTService.withConn`（`service.go:332`）は終端状態かどうかしか見ず、接続が切れて paho が自動再接続している間も `stateConnected` のまま Publish を client に渡す。paho v1.5.1 の `client.Publish`（`client.go:772`）は自動再接続中を「接続中」と扱うので、次のようになる。
  - **QoS 0**: `client.go:779` の `reconnecting && qos == 0` で、送信も保存もせずに token を完了させる。`Publish` RPC は成功を返すが、メッセージは捨てられている。
  - **QoS 1/2**: `client.go:809` でストアに保存するだけで送らない（`client.go:813`）。token は完了しないので `tokenTimeout`（30 秒）待って `publish timed out` を返す。その間 `opMu` を持ち続けるため、同じ接続の `Disconnect`・`Subscribe`・`onConnected`（`service.go:248`）が待たされる。保存されたメッセージは再接続時の `resume`（`client.go:346`）で送信されるので、ユーザーにはエラーと表示されたメッセージが後から届く。再送していれば重複する。
  - 同じ「timeout を返したのに後で届く」は、接続が開いていても PUBACK が 30 秒以内に返らない場合（回線が無応答になり、keepalive が切断を検知する前）にも起きる。
- **到達経路**: フロントの Publish ボタン → `MQTTHandler.Publish`（`mqtt_handler.go:53`）→ `MQTTService.Publish`（`service.go:352`）→ `withConn` → `pahoClient.Publish`。
- **再現条件**:
  - QoS 1/2: ブローカーとの回線が無応答になった直後（画面はまだ Connected）に QoS 1 で Publish する。30 秒後にエラーになり、回線が戻ると相手にメッセージが届く。
  - QoS 0: フロントは `mqtt:connection-lost` を受けると Publish ボタンを無効にする（`publish-tab.tsx:217`）ので、paho が再接続状態に入ってからフロントがイベントを処理するまでの短い間に Publish した場合だけ起きる。
- **修正案**: `pahoClient.Publish` の先頭で接続が開いていることを確かめ、開いていなければ送らずにエラーを返す。これで再接続中の「黙って捨てる」「保存して後で送る」の両方が無くなる。
  ```go
  func (p *pahoClient) Publish(topic string, qos byte, retained bool, payload string) error {
  	// 自動再接続中の paho は QoS 0 を送らずに成功させ、QoS 1/2 を保存して再接続後に送る。
  	// 呼び出し元に返した結果と食い違うので、接続が開いていなければ送らない。
  	if !p.client.IsConnectionOpen() {
  		return errors.New("not connected")
  	}
  	...
  }
  ```
  接続が開いている間の PUBACK 待ち timeout は paho のストアから取り消せないので、エラーメッセージを「送達を確認できなかった（再接続後に届く可能性がある）」という内容に変えるに留める。
  この確認は確認と送信の間に切断された場合までは防げない（確認の直後に paho が再接続状態に入ると、従来どおり QoS 0 は捨てられ、QoS 1/2 は保存される）。起きる窓を「切断を検知してから再接続するまで」から「確認から `client.Publish` までの一瞬」に狭める修正である。
- **修正の影響範囲**: `pahoClient.Publish` だけ。`BrokerClient.Publish` のシグネチャは変わらない。保存形式・復旧方針・RPC のシグネチャ・イベント名の変更は無い。`domain/mqtt/broker_port.go:23` の `Publish` のコメントに「接続が開いていなければエラーを返す」を足す。
- **挙動を守っている・再現するテスト**: 無し（先に再現テストを追加）。`internal/infrastructure/mqtt/paho_client_test.go` の `newFakeBroker`（`TestPahoClient_ConnectionLost_InvokesCallback` が使っている、再接続を受け付けないブローカー）で接続を切り、再接続中の `Publish(qos 0)` がエラーを返すことを確かめるテストを足す。
- **検証コマンド**: `task format` → `task lint` → `task test`。加えて `task go:test:race`（gcc の無い環境ではローカルで実行できないため CI の結果で確認する）と `task go:test:integration`。
- **付随作業**: 無し（バインド対象の型・メソッドは変わらない。fake backend の `Publish` は接続の有無しか見ていないので追従不要）。

### #2 [深刻度: 低] `UseTLS=true` でもスキームによっては平文で接続する（G6）

- **該当箇所**: `internal/infrastructure/mqtt/paho_client.go:135`（使う側は `paho_client.go:163`）
- **コード抜粋**:
  ```go
  func applyTLSScheme(broker string) string {
  	switch {
  	case strings.HasPrefix(broker, "tcp://"):
  		return "ssl://" + broker[len("tcp://"):]
  	case strings.HasPrefix(broker, "mqtt://"):
  		return "mqtts://" + broker[len("mqtt://"):]
  	case strings.HasPrefix(broker, "ws://"):
  		return "wss://" + broker[len("ws://"):]
  	}
  	return broker
  }
  ```
- **問題の説明**: スキーム無し（`broker:1883`）や大文字を含むスキーム（`TCP://`・`Mqtt://`・`WS://`）はそのまま返る。どちらも `UseTLS=true` なのに TLS を使わずに接続し、ユーザー名とパスワードを平文で送る。エラーにもならない。
  - スキーム無し: paho の `AddBroker`（`options.go:174`）が `tcp://` を補う。
  - 大文字を含むスキーム: `AddBroker` が呼ぶ `url.Parse` はスキームを小文字にする（`TCP://h:1883` の `Scheme` は `tcp`。`go run` で確かめた）ので、`openConnection`（`netconn.go:41`）の `case "mqtt", "tcp"` / `case "ws"` に入って平文で繋がる。
  - 上記以外の未知のスキーム（`http://` など）は `openConnection` が `unknown protocol` を返すので、平文では繋がらず `mqtt:connection-failed` になる。こちらは問題ではない。
  
  `TestApplyTLSScheme`（`paho_client_test.go:285`）は `broker:1883` がそのまま返ることを期待値にしている。
- **到達経路**: `MQTTHandler.Connect` → `MQTTService.Connect`（`service.go:157` は空文字しか弾かない）→ `clientFactory`。ただし現状のフロントは `composeBrokerUrl`（`broker-url.ts:27`）で必ず `mqtt|mqtts|tcp|ws|wss` のスキームを付けるので、UI からは到達しない。
- **再現条件**: スキーム無し、または大文字を含むスキームの Broker と `UseTLS=true` で `Connect` を呼ぶ（現状は RPC を直接呼ぶ場合だけ）。
- **修正案**: `applyTLSScheme` のスキームの照合を大文字小文字を区別しないようにする。`UseTLS=true` で TLS のスキームにできなかった場合は、スキーム無しなら `ssl://` を補い、それ以外の未知のスキームは接続前に拒否する。拒否は `MQTTService.Connect` の検証（`ValidationError`）に置くと RPC のエラーとして返せる。
- **修正の影響範囲**: `applyTLSScheme` と `TestApplyTLSScheme` の期待値。`Connect` で拒否する場合は、フロントが通す入力（5 つのスキーム）を拒否しないことを確かめる。保存形式・RPC のシグネチャの変更は無い。
- **挙動を守っている・再現するテスト**: `TestApplyTLSScheme`（現状の挙動を固定している）・`TestNewPahoClientFactory_UseTLS`。
- **検証コマンド**: `task format` → `task lint` → `task test`
- **付随作業**: `Connect` に検証を足すなら fake backend の `Connect`（`install.ts:675`）にも同じ検証を足す。

### #3 [深刻度: 低] `GetConnections` の並び順が不定（G6・C2）

- **該当箇所**: `internal/application/mqtt/service.go:649`・`internal/application/mqtt/service.go:665`
- **コード抜粋**:
  ```go
  for id, conn := range s.conns {
  	ids = append(ids, id)
  	conns = append(conns, conn)
  }
  ...
  for topic, qos := range conn.subs {
  	subs = append(subs, domain.SubscriptionInfo{Topic: topic, QoS: qos})
  }
  ```
- **問題の説明**: どちらも map の反復なので、接続の順序と購読の順序が呼び出しのたびに変わる。フロントは復元時に返った順のまま接続と購読を作る（`connections.ts:404`・`connections.ts:410`）。
  - **購読の順序**: Subscriptions パネルは `subscriptions()` をそのまま並べる（`subscriptions-panel.tsx:72`）ので、リロードのたびに購読一覧の並びが入れ替わり得る。画面に出る影響はこれが主。
  - **接続の順序**: 接続を一覧として並べる画面は無い。サイドバーは `GetConnections` ではなく `profiles()` の順で描く（`broker-tree.tsx:100`）ので、並びは変わらない。影響するのは、アクティブなタブを閉じたときに次にアクティブにする接続を `Object.keys(connections)[0]` で選ぶ箇所（`connections.ts:597`）だけで、接続が 2 本以上あるとリロードの前後で選ばれる接続が変わり得る。
  
  fake backend（`install.ts:666`）は挿入順で返すため、e2e では起きない。
- **到達経路**: フロントの状態復元 → `MQTTHandler.GetConnections`（`mqtt_handler.go:78`）→ `MQTTService.GetConnections`。
- **再現条件**: 購読を 2 つ以上持った状態で WebView をリロードする（購読一覧の並び）。接続の順序は、接続を 2 本以上持った状態でリロードし、アクティブなタブを閉じたときにだけ表に出る。
- **修正案**: 購読はトピック名で並べる（購読した順を保ちたいなら `subs` に順序を持たせる）。接続は `connection` に作成順（連番か作成時刻）を持たせてその順に並べる（画面への影響が小さいので優先度は購読より低い）。
- **修正の影響範囲**: `GetConnections` の戻り値の順序だけ。ドメイン型・RPC のシグネチャは変わらない。
- **挙動を守っている・再現するテスト**: 無し（先に再現テストを追加）。`service_test.go` の `TestMQTTService_GetConnections_TracksProfileIDAndSubscriptions` の近くに、複数の接続・購読の順序を確かめるテストを足す。
- **検証コマンド**: `task format` → `task lint` → `task test`
- **付随作業**: 無し（fake backend は既に挿入順）。

### #4 [深刻度: 低] 確立前に受け付けた購読が、拒否以外の失敗で「表示だけ購読中」になる（G6）

- **該当箇所**: `internal/application/mqtt/service.go:445`（関連: `internal/infrastructure/mqtt/paho_client.go:246`）
- **コード抜粋**:
  ```go
  err := conn.client.Subscribe(topic, qos, s.messageHandler(connID, conn))
  if err == nil {
  	continue
  }
  s.logger.Error("MQTT resubscribe failed", ...)
  // ブローカーが拒否した購読は表示から外す。それ以外 (再び切断した等) は次の再接続で張り直す。
  if errors.Is(err, domain.ErrSubscriptionRejected) {
  ```
- **問題の説明**: 接続の確立前に受け付けた購読（`service.go:376`）は、`onConnected` の `resubscribe` で初めて購読する。このとき `pahoClient.Subscribe` は新規のフィルターとして扱うので、失敗すると振り分け先を外す（`paho_client.go:246` の `!replaced`）。一方 `resubscribe` は拒否以外の失敗では `conn.subs` を残す。接続が切れずに失敗した場合（SUBACK が 30 秒以内に返らない `subscribe timed out`）は次の再接続が来ないので、`GetConnections` は購読中と返し続けるのにメッセージは届かない。SUBACK が遅れて届いてブローカー側で購読が成立しても、振り分け先が無いので `dispatch`（`paho_client.go:119`）が捨てる。呼び出し元の `Subscribe` RPC は既に成功を返しているので、ユーザーにはログ以外で伝わらない。
- **到達経路**: 確立前の `MQTTHandler.Subscribe` → `updatePendingSubs` → 確立時の `onConnected`（`service.go:247`）→ `resubscribe`（`service.go:431`）。
- **再現条件**: 接続の確立前に Subscribe し、確立直後の SUBSCRIBE に対する SUBACK が 30 秒以内に返らず、かつ接続は切れない。発生条件は狭い。
- **修正案**: `resubscribe` で、拒否以外の失敗のうち接続が開いたままのもの（`conn.client.IsConnected()` が true）は、拒否と同じく `conn.subs` から外して `Unsubscribe` で片付ける。フロントに知らせるならイベントの追加が要る。
  timeout ではブローカーが購読を受理したかどうかが分からない。`Unsubscribe` まで送れば、受理されていた場合もブローカー側の購読が消えて「購読していない」に揃う。その `Unsubscribe` も失敗した場合はブローカー側に購読が残り得るが、振り分け先が無いので届いたメッセージは捨てられ、表示とは食い違わない。購読を残したい場合は、外す代わりに同じ接続のまま数回再試行する案もある（再試行中は `opMu` を持ち続けるので、要確認 1 と同じ待ち時間の問題を広げる）。
- **修正の影響範囲**: `resubscribe` だけ。イベントを足す場合は `internal/domain/events.go` と `tools/gen-events/main.go` の変更になる（イベント名の追加）。
- **挙動を守っている・再現するテスト**: `TestMQTTService_Reconnect_ResubscribeFailures` の `transient failure`（`service_test.go:1614`）が「拒否以外では購読を残す」を固定している。確立前の購読で同じ失敗を起こすテストは無いので、`newPendingConn` を使って足す。
- **検証コマンド**: `task format` → `task lint` → `task test`。加えて `task go:test:race`（gcc の無い環境では CI の結果で確認する）。
- **付随作業**: イベントを足す場合は `task go:generate:events` と fake backend の追従。足さなければ無し。

### #5 [深刻度: 低] fake backend の `DeleteProfile` と `Connect` の失敗の出方が Go と違う（C2）

- **該当箇所**: `frontend/e2e/fake-backend/install.ts:662`・`frontend/e2e/fake-backend/install.ts:682`
- **コード抜粋**:
  ```ts
  DeleteProfile: mutates("DeleteProfile", (id: string) => {
    db.mqttProfiles = db.mqttProfiles.filter((p) => p.id !== id);
  }),
  ...
  if (seed.mqttConnect !== "ok") throw new Error("connection refused");
  ```
- **問題の説明**:
  - Go の `DeleteProfile` は未知の ID に `NotFoundError` を返す（`cached_store.go:95`）が、fake は何もせず成功する。
  - Go の `Connect` はブローカーに繋がらなくても接続 ID を返し、失敗は後から `mqtt:connection-failed` で知らせる（`service.go:220`）。fake の既定は RPC 自体を `connection refused` で失敗させるので、e2e は「ID を受け取った後にイベントで失敗する」経路を通らない。
- **到達経路**: UI e2e（`e2e/ui`）の MQTT のテスト。
- **再現条件**: 既定の seed で Connect する e2e、存在しないプロファイルを削除する e2e。
- **修正案**: `DeleteProfile` は未知の ID で `profile not found: <id>` を投げる。`Connect` の失敗は、接続 ID を返してから `mqtt:connection-failed` を emit する seed を足す。Go は失敗時に接続を map から外す（`service.go:227`）ので、fake も emit と同時に `db.mqttConnections` からその接続を消す（消さないと、失敗した接続が `GetConnections` に残り、その ID への `Disconnect` が成功してしまう）。
- **修正の影響範囲**: fake backend と、既定の失敗に依存している e2e の期待値。製品コードへの影響は無い。
- **挙動を守っている・再現するテスト**: 無し
- **検証コマンド**: `task format` → `task lint` → `task test` → `task frontend:test:e2e`
- **付随作業**: 無し

### #6 [深刻度: 低] 確立済みの接続での Subscribe / Unsubscribe の timeout を、片方の結果に決めてしまう（G6）

- **該当箇所**: `internal/application/mqtt/service.go:379`・`internal/application/mqtt/service.go:473`（関連: `internal/infrastructure/mqtt/paho_client.go:246`・`paho_client.go:275`）
- **コード抜粋**:
  ```go
  if err := conn.client.Subscribe(topic, qos, s.messageHandler(connectionID, conn)); err != nil {
  	return fmt.Errorf("failed to subscribe: %w", err)
  }
  ...
  if err := conn.client.Unsubscribe(topic); err != nil {
  	return fmt.Errorf("failed to unsubscribe: %w", err)
  }
  ```
- **問題の説明**: SUBACK / UNSUBACK が `tokenTimeout`（30 秒）以内に返らないとき、ブローカーが要求を処理したかどうかは分からない。現状はどちらも「失敗した」に決めている。#4 は確立前に受け付けた購読の話で、こちらは確立済みの接続での通常の呼び出し。
  - **Subscribe**: `conn.subs` に足さず、新規のフィルターなら振り分け先も外す（`paho_client.go:246`）。ブローカーが受理していた場合、ブローカーはメッセージを送り続けるが `dispatch` が捨てる。表示（購読していない）と受信（届かない）は一致するので、害は不要な通信だけ。同じトピックをもう一度 Subscribe すれば成立する。
  - **Unsubscribe**: `conn.subs` も振り分け先も残す（`paho_client.go:275` で戻るので `removeRoutes` に届かない）。ブローカーが解除していた場合、購読中と表示されたままメッセージが届かない。もう一度 Unsubscribe すれば表示から消える。
- **到達経路**: `MQTTHandler.Subscribe` / `Unsubscribe` → `MQTTService.Subscribe` / `Unsubscribe` → `withConn` → `pahoClient.Subscribe` / `Unsubscribe`。
- **再現条件**: 確立済みの接続で Subscribe / Unsubscribe し、応答が 30 秒以内に返らず、かつブローカーは要求を処理している。発生条件は狭い。
- **修正案**: Unsubscribe の timeout は「解除した」に倒す（`conn.subs` と振り分け先を外す）。ブローカー側に購読が残っていても、届いたメッセージは捨てられるので表示と食い違わない。Subscribe の timeout は現状のままでよい（表示と受信が一致している）。エラーメッセージは #1 と同じく「結果を確認できなかった」という内容にする。
- **修正の影響範囲**: `pahoClient.Unsubscribe` か `MQTTService.Unsubscribe` のどちらか。timeout を他のエラーと区別するには、`ErrSubscriptionRejected` と同じように domain のエラー値を足す。RPC のシグネチャ・保存形式・イベント名の変更は無い。
- **挙動を守っている・再現するテスト**: 無し（先に再現テストを追加）。
- **検証コマンド**: `task format` → `task lint` → `task test`
- **付随作業**: 無し（fake backend にブローカーの応答待ちは無い）。

## 要確認（到達経路を確かめきれなかったもの）

| # | 観点 | ファイル:行 | 懸念 | 確かめられなかった点 |
|---|------|-------------|------|----------------------|
| 1 | G3 | internal/application/mqtt/service.go:248 | `onConnected` は状態遷移と `mqtt:connected` の発行より先に `opMu` を取る。Subscribe / Unsubscribe の SUBACK 待ち中に接続が切れると、paho は `ResumeSubs=true` のため token を完了させず（`client.go:576`）、再接続時の `resume` は別の token で送り直す（`client.go:1072`）。元の呼び出しは `tokenTimeout`（30 秒）まで `opMu` を持ち続けるので、再接続しても最大 30 秒 `mqtt:connected` が出ず、`Disconnect` も待たされる | paho のソースを読んだだけで、切断のタイミングを合わせた再現はしていない。元の token が本当に完了しないかを `newFakeBroker` を使ったテストで確かめる必要がある |
| 2 | G3 | internal/application/mqtt/service.go:190 | `Connect` は接続 goroutine を起動してから接続 ID を返す。フロントは `await api.connect()` の後で接続を登録する（`connections.ts:479`〜`486`）ので、`mqtt:connected` / `mqtt:connection-failed` が RPC の応答より先に WebView に届くと、`updateConnection` は未登録の接続への更新として捨てる（`connections.ts:168`）。`mqtt:connected` を取りこぼすと、バックエンドは接続済みなのに画面は未接続のままになる | Wails が RPC の応答とイベントを WebView に渡す順序を確かめていない。ローカルのブローカーのように確立がすぐ終わる相手で実際にこの順序になるかは、fullstack e2e か実機で確かめる必要がある |

## lint の指摘

無し（`task lint`: `go vet`・golangci-lint・golangci-lint（integration タグ）・Biome・`tsc -b` のいずれも指摘 0 件）。

## 対応不要と判断した箇所（理由つき）

| ファイル:行 | 観点 | 判断根拠 |
|-------------|------|----------|
| internal/adapters/mqtt_handler.go:43 | G1 | ハンドラが `svc` の nil を確かめないのは二段階初期化の設計（前提となる設計）。`initialize` が失敗したら `runtime.Quit` する |
| internal/application/mqtt/service.go:190 | G3 | 接続の登録と `connWg` への計上を `mu` の同じ区間で行い、`Shutdown` は `closed` を立ててから `Wait` するので、`Wait` の後に計上する経路は無い。`reserveScan`（`service.go:572`）も `mu.RLock` の保持中に `closed` を見てから `Add` する |
| internal/application/mqtt/service.go:311 | G3 | ロック順序は `mu` → `opMu` → `stateMu` で、`detach`・`Shutdown` は `opMu` を飛ばして `mu` → `stateMu` を取るだけ。逆順で取る箇所は無い |
| internal/application/mqtt/service.go:503 | G3 | `scan.client` はロック無しで代入するが、他の goroutine が読むのは `started` が true のとき（`detachScan`・`onScanConnectionLost`）だけで、`started` は代入より後に `stateMu` の下で立つ |
| internal/application/mqtt/service.go:529 | G3 | `scan.pending` の読み出しとクリアは `stateMu.Lock` の下、追記は `stateMu.RLock` と `pendingMu` の下なので競合しない |
| internal/application/mqtt/service.go:543 | G3 | `scan.done` を閉じるのはスキャンを予約した 1 つの goroutine だけ（`reserveScan` が `ctx` を返すのは 1 回）。二重 close は無い |
| internal/application/mqtt/service.go:237 | G6 | `runConnect` が `onConnected` より先に遷移した場合の重複購読はコメントに書かれた既知の挙動で、QoS は揃う |
| internal/application/mqtt/service.go:414 | G5 | 非 UTF-8 のペイロードは `EncodeMaybeBase64` で base64 にして渡している。Publish の `payload` は JSON 由来の文字列なので常に妥当な UTF-8 |
| internal/application/mqtt/service.go:356 | G5 | `qos` は `byte` で受けて 2 以下に限っている。範囲外の数値は Wails の引数変換で失敗する |
| internal/application/mqtt/service.go:688 | G7・G10 | `Shutdown` は全接続を終端状態にしてから `stop()` で Connect を打ち切り、上限時間で戻る。上限を過ぎてもイベントは出ない。`app.go:236` は `ready` を確かめてから呼ぶ |
| internal/infrastructure/mqtt/paho_client.go:198 | G7 | 打ち切り後も接続試行の完了まで待つのは `BrokerClient.Connect` の契約。`TestPahoClient_Connect_Cancel_WaitsForAttempt` などが守っている |
| internal/infrastructure/mqtt/paho_client.go:119 | G6 | 重なる購読で最初の 1 つにだけ渡すのは設計（`broker_port.go:27`）。ハンドラは接続ごとに同じ内容なので、どの購読に渡しても結果は同じ。`$` で始まるトピックとワイルドカードの照合を区別していないが、同じ理由で結果に影響しない |
| internal/infrastructure/mqtt/paho_client.go:157 | G6 | ユーザー名が空ならパスワードも設定しないのは MQTT 3.1.1（ユーザー名無しのパスワードは不可）に沿う |
| internal/infrastructure/mqtt/profile_repository.go:59 | G8 | 保存形式は `testdata/profile.golden.json` が基準。stored DTO は domain 型を埋め込まず、全フィールドを往復させている（`TestProfileRepository_RoundTripKeepsEveryField`）。パスワードを平文で保存するのも現行の保存形式どおり |
| internal/infrastructure/mqtt/profile_repository.go:33 | G8 | 破損ファイルの退避とスキップは `JSONStore.Load`（範囲外）が復旧方針の表どおりに行う。`Load` は空でも非 nil のスライスを返す |
| internal/application/mqtt/profile_service.go:35 | G6 | `SaveProfile` は Name・Broker を検証しないが、空の Broker は `Connect` が拒否し、フロントが保存前に検証する（入力検証の重複は前提となる設計） |
| internal/application/mqtt/service.go:655 | G1・C1 | `GetConnections` の戻り値と `Subscriptions` は `make` で作るので、空でも JSON では `[]` になる |
| internal/domain/mqtt/topic.go:33 | G6 | フィルターの検証（`#` は最後の階層全体、`+` は階層全体）は MQTT 3.1.1 4.7.1 どおりで、fake backend（`install.ts:620`）と一致する |

## 調査対象外・未確認

- フロントエンドの観点（F1〜F7）は範囲外のためスキップした。フロントのコードは #1〜#3 の到達経路の確認のためだけに読んだ。
- G4（UDP ソケット・HTTP の一時ファイル）・G9（ファイルアクセス）は MQTT のパスに該当する処理が無いためスキップした。
- `internal/infrastructure/json_store.go`・`internal/application/store/cached_store.go`・`app.go` は範囲外で、到達経路の確認のためだけに読んだ。`CachedStore.GetAll` が map の反復順で返す点は範囲外のため載せていない（フロントは `applyOrder` で並べ直す）。
- paho の挙動（#1・要確認 1）はソースの読解に基づく。実機・テストでの再現はしていない。
- `task go:test:race`・`task go:test:integration` は実行していない（今回はコードを変更していない）。

## 推奨アクション

1. `pahoClient.Publish` に接続が開いているかの確認を足し、再接続中の Publish をエラーにする（#1）。先に `paho_client_test.go` へ再現テストを足す。
2. 要確認 1 を `newFakeBroker` を使ったテストで再現できるか確かめる。再現するなら #1 と合わせて、`opMu` を持ったまま token を待つ時間の扱いを見直す。
3. 要確認 2 を fullstack e2e か実機で確かめる。再現するなら、フロントが接続を登録する前に届いたイベントを取りこぼさない形（接続 ID の登録を先に行う、または `Connect` の応答後に `GetConnections` で状態を取り直す）を検討する。
4. `GetConnections` の購読の並び順を安定させる（#3）。`UseTLS` とスキームの扱い（#2）、確立前の購読の失敗（#4）、fake backend の追従（#5）、Subscribe / Unsubscribe の timeout の扱い（#6）は優先度を下げて対応する。
