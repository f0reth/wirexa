# 変更計画書: 手動で切断・再接続した MQTT 接続で購読が復元されない不具合の修正

## 概要

MQTT タブで購読中に接続バーの **Disconnect** を押し、続けて **Connect** を押すと、Subscriptions パネルには購読が残っているのにメッセージが届かない。サイドバーの「Save & Connect」で既存の接続を張り直した場合も同じことが起き、さらにこの経路では編集したプロファイル（ブローカー URL・認証など）が接続に使われない。

### 原因

1. Connect ボタン（`broker-manager.tsx`）は `handleReconnect`（`frontend/src/application/mqtt/connections.ts:453`）を呼ぶ。`handleReconnect` は `api.connect(profile)` の戻り値で新しい接続 ID を受け取り、**その直後に** 旧タブの購読を `api.subscribe(newConnId, ...)` で張り直す。
2. Go の `MQTTService.Connect`（`internal/application/mqtt/service.go:96`）は接続 ID を先に返し、実際の接続は接続 goroutine（`runConnect`）で非同期に行う。このため `Subscribe` RPC が届く時点では、ほぼ確実に paho クライアントがまだ `connecting` 状態にある。
3. paho v1.5.1 の `client.Subscribe` は `connecting` 中（`ConnectRetry` 無効）に `IsConnected()` が false になり、`ErrNotConnected` を返す。`MQTTService.Subscribe` はこれをエラーとして返し、`conn.subs` に登録しない。
4. フロントエンドは失敗をトーストで通知するだけで、購読の行は旧タブの状態のまま残す。バックエンドの `conn.subs` は空なので、`onConnected` の張り直し（`resubscribe`）の対象にもならない。結果として「購読は表示されているのに受信しない」状態になる。
5. 「Save & Connect」（`broker-tree.tsx:69`）は保存したプロファイルを `handleReconnect` に渡さず接続 ID だけを渡す。`handleReconnect` はタブが持つ保存前の `conn.profile` で接続するため、編集内容が反映されない。

なお、ブローカー側で接続が切れた場合の**自動再接続**では、`onConnected` → `resubscribe` がバックエンドの `conn.subs` から購読を張り直しており（`TestMQTTService_Reconnect_Resubscribes`、`TestMQTT_ConnectionLostAndReconnect`）、この不具合は起きない。今回の対象は、ユーザー操作で切断して新しい接続を作り直す経路である。

### 方針

- 接続の確立前（`stateConnecting`）に届いた `Subscribe` / `Unsubscribe` は、client を呼ばずに `conn.subs` の登録・削除だけを行って成功を返す。実際の購読は、接続確立時の `onConnected` で既存の `resubscribe` が行う。これで `handleReconnect` の「接続直後の再購読」が接続の確立を待たずに正しく効くようになり、RPC の形は変えずに済む。
- `onConnected` は `opMu` を取ってから状態遷移・`subs` の複製・張り直しを行い、`Subscribe` / `Unsubscribe` と直列化する（「競合の整理」参照）。
- 確立前の購読をバックエンドが受理するようになるので、確立待ちの間に UI で外した購読がバックエンドに残らないよう、フロントエンドも直す（`handleReconnect` の送信前の確認と、`removeSubscription` の `Unsubscribe` 送信）。
- 「Save & Connect」は保存したプロファイルで再接続する。

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `internal/application/mqtt/service.go` | Application | 変更 | `Subscribe` / `Unsubscribe` を、接続確立前（`stateConnecting`）なら client を呼ばずに `conn.subs` の登録・削除だけで成功させる。`onConnected` が `opMu` を取ってから状態遷移・複製・張り直しを行うようにし、`resubscribe` は `opMu` 保持を前提にする。`onConnected` / `resubscribe` / `connection.subs` / `GetConnections` の doc コメントを、保留中の購読を含む意味に直す |
| `internal/application/mqtt/service_test.go` | Application（テスト） | 変更 | 接続確立前の Subscribe / Unsubscribe、確立時の適用（最新の QoS）、接続失敗・切断時の破棄、張り直し中の Subscribe の待ち合わせのテストを追加 |
| `internal/integration/mqtt_test.go` | 統合テスト | 変更 | 実ブローカーに対し「Connect の直後（確立待ちなし）に Subscribe → 受信できる」「Disconnect → 新しい Connect の直後に Subscribe → 受信できる」を追加 |
| `frontend/src/application/mqtt/connections.ts` | Frontend Application | 変更 | `handleReconnect` に省略可能な `profile` 引数を足し、渡されたらそれで接続してタブのプロファイルも置き換える。再購読のループは、各トピックを送る前に新しいタブにその購読がまだあるかを確かめ、無ければ送らない |
| `frontend/src/application/mqtt/subscriptions.ts` | Frontend Application | 変更 | `removeSubscription` は、オンラインのタブなら `connected` に関わらず `Unsubscribe` を送る。未接続のときの失敗はログだけにしてトーストは出さない |
| `frontend/src/presentation/components/sidebar/broker-tree.tsx` | Presentation | 変更 | `handleProfileSaveAndConnect` で、既存の接続があれば保存後のプロファイル（`saved`）を `handleReconnect` に渡す |
| `frontend/src/presentation/providers/mqtt-provider.tsx` | Presentation | 変更（型のみ） | コンテキストの `handleReconnect` の型に `profile?` を足す |
| `frontend/src/application/mqtt/connections.test.ts` | Frontend（テスト） | 変更 | プロファイルを渡した再接続、再購読の途中で外した購読を送らないことのテストを追加 |
| `frontend/src/application/mqtt/subscriptions.test.ts` | Frontend（テスト） | 変更 | 確立待ちのオンラインタブで外した購読に `Unsubscribe` を送ること、未接続時の失敗でトーストを出さないことのテストを追加。オフラインタブでは送らないことは既存どおり |
| `frontend/e2e/integration/mqtt/mqtt.spec.ts` | フルスタック e2e | 変更 | 購読 → Disconnect → Connect → 購読した retained メッセージが一覧に出る、再購読失敗のトーストが出ない、を検証するテストを追加 |
| `frontend/e2e/fake-backend/install.ts` | UI e2e（偽バックエンド） | 変更なし（確認のみ） | 偽の `Subscribe` / `Unsubscribe` は `connected: false` の接続でも既に受理して購読を記録・削除しており、変更後の Go の振る舞いと一致する。必要ならコメントに「接続確立前でも受理する（Go と同じ）」を追記する程度 |

## 実装方針

### `MQTTService.Subscribe` / `Unsubscribe`

`withConn` の中（`opMu` 保持中）で、`stateMu` を取って状態を確認する。

- **`stateConnecting` のとき**: 同じ `stateMu.Lock` の区間で `conn.subs[topic] = qos`（Unsubscribe なら `delete`）だけを行い、`nil` を返す。client は呼ばない。同じトピックを再び Subscribe した場合は QoS を上書きする。
- **それ以外（`stateConnected`）**: 従来どおり client を呼び、成功したら `conn.subs` を更新する。

`stateMu` の保持中に client 操作をしない、というロック規約はそのまま守る（保留の分岐では client を呼ばない。確立済みの分岐は、状態確認の `stateMu` を放してから client を呼ぶ）。

### `onConnected` を `opMu` で直列化する

現状の `onConnected` は「`stateMu.Lock` で `stateConnected` へ遷移して `subs` を複製 → `stateMu` を放す → `resubscribe` が `opMu` を取る」の順に動く。複製から `opMu` の取得までの間に `Subscribe` が割り込めるため、次の問題が起きる。

- 複製に含まれるトピックを別の QoS で `Subscribe` すると、`resubscribe` は存在だけを確認して複製時の古い QoS で購読し直す（`service.go:349`、`358`）。バックエンドの `subs` とブローカー側の QoS が食い違う。
- 保留中の購読を受理するようになると、この割り込みは手動再接続の直後に起こりやすくなる。

そこで `onConnected` を次の順に変える（ロック順序 `opMu → stateMu` は既存の規約どおり）。

1. `opMu.Lock`
2. `stateMu.Lock` → terminal なら何もせず戻る → `stateConnected` へ遷移 → `mqtt:connected` を発行 → `subs` を複製 → `stateMu.Unlock`
3. `opMu` を保持したまま `resubscribe` で複製を購読する（`resubscribe` は自分では `opMu` を取らず、呼び出し元が保持していることを前提にする）
4. `opMu.Unlock`

`Subscribe` / `Unsubscribe` も `opMu` の中で状態を見るので、`onConnected` との順序は次のどちらかに分かれる。

- Subscribe が先: `stateConnecting` を見て `subs` に登録（または上書き・削除）→ `onConnected` はその後に複製するので、最新の QoS で 1 回だけ購読する。
- `onConnected` が先: Subscribe は張り直しが終わるまで `opMu` で待ち、その後 `stateConnected` を見て client を直接呼ぶ。複製には含まれないので重ねて購読しない。

`resubscribe` の「terminal になったら打ち切る」確認は残す（Disconnect の `detach` は `opMu` を取らずに terminal へ遷移させるため）。「Unsubscribe で外された購読は張り直さない」確認は、`opMu` を通して保持するので割り込みが起きなくなるが、害は無いので残してよい。

#### 残る窓: `runConnect` による遷移

paho は OnConnect ハンドラを `go` で起動し（`client.go:635`）、その後に Connect の token を完了させる。このため `runConnect` の `client.Connect` が先に戻り、`runConnect` が `stateConnecting → stateConnected` に遷移させてから `onConnected` が `opMu` を取る、という順序があり得る（`service.go:172` のコメントの逆の順序）。この間に届いた `Subscribe` は client を直接呼び、その後 `onConnected` が同じトピックをもう一度購読する。

- 影響: QoS は最新の値で揃うので食い違いは無い。同じフィルターの再 SUBSCRIBE はブローカー側で置き換えになるが、retained メッセージが再送されるため、UI に同じ retained メッセージが 2 件並ぶことがある。
- 窓の幅: OnConnect の goroutine は token の完了より前に起動されるので、ごく短い。
- 今回は受け入れる。`runConnect` の遷移をやめて `onConnected` だけを遷移の起点にすれば窓は無くなるが、callback を呼ばない `factoryWith` を使う既存のユニットテストの大半が `onConnected` を明示的に起動するよう書き換えになる。必要なら別途対応する。

### 失敗時の扱い

- 接続に失敗した場合（`runConnect` のエラー経路）: 接続ごと破棄されるので保留中の購読も消える。`mqtt:connection-failed` は従来どおり出る。フロントエンドは購読の行を残したままなので、もう一度 Connect を押せば再び張り直される。
- 確立前に Disconnect / Shutdown された場合: `detach` で terminal になった接続は `withConn` が弾き、`onConnected` も terminal を見て何もしない。従来と同じ。
- 確立時にブローカーが購読を拒否した場合（`ErrSubscriptionRejected`）: 既存の `resubscribe` と同じく `conn.subs` から外してエラーログを出す。フロントエンドの行は残る（「副作用・注意事項」参照）。

### フロントエンド

#### `handleReconnect(connectionId, profile?)`

- `profile` が渡されたらそれで接続し、新しいタブの `profile` もそれに置き換える。渡されなければ従来どおりタブの `conn.profile` を使う（Connect ボタンの経路）。
- 再購読のループは旧タブの `conn.subscriptions` を順に送るが、各トピックを送る直前に `connections[newConnId]?.subscriptions` に同じ `id` の購読が残っているかを確かめ、無ければ送らない。`api.subscribe` の `await` の間にユーザーが購読を外した場合に、外した購読をバックエンドへ登録し直さないため。
- タブ自体が閉じられていた（`connections[newConnId]` が無い）場合はループを打ち切る。

#### `removeSubscription`

- オンラインのタブなら、`connected` が false でも `Unsubscribe` を送る。確立待ちの接続では、バックエンドが保留中の購読を `conn.subs` から外す。送らないと、`handleReconnect` が先に送った保留中の購読が確立時に購読されてしまう。
- 未接続のときの失敗はログだけにしてトーストは出さない。未接続で失敗するのは、接続が失敗して既に無い（`NotFoundError`）か、自動再接続中で paho がエラーを返す場合で、どちらも UI の行を外せば足りる。接続済みのときの失敗は従来どおりトーストを出す。
- オフラインのタブでは従来どおり送らない。

#### `broker-tree.tsx`

`handleProfileSaveAndConnect` は、既存の接続があれば `handleReconnect(existingConn.connectionId, saved)` を呼ぶ。

### 依存方向

- バックエンドの変更は application 層の `MQTTService` 内に閉じる。domain の `BrokerClient` / `BrokerClientFactory` の契約、infrastructure の `pahoClient`、adapters の `MQTTHandler`、`app.go` の配線は変えない。
- フロントエンドの変更は application（`connections.ts`、`subscriptions.ts`）と presentation（`broker-tree.tsx`、Provider の型）に閉じる。`presentation → application → domain` の依存と注入（`mqtt-provider.tsx`）の形は変えず、`MqttConnectionApi` / `SubscriptionApi` のポートも変えない。

### 採らなかった案

- **Connect RPC に初期購読を渡す**（`Connect(config, subs)`）: 確実だが RPC の形が変わり、Wails バインディング・`infrastructure/mqtt/client.ts`・偽バックエンドの変更が必要になる。`Subscribe` を確立前に受理すれば同じ効果が得られるので採らない。
- **フロントエンドで `mqtt:connected` を待ってから購読する**: イベントが Connect の戻り値より先に届く可能性、接続失敗時の待ち合わせの打ち切りなどの処理がフロントエンドに増える。バックエンドはもともと購読を `conn.subs` で持ち、再接続時に張り直す責務を負っているので、そちらに寄せる。

## 永続化への影響

なし（購読はメモリ上の `conn.subs` だけで持ち、保存ファイルや localStorage には触れない）。「Save & Connect」の修正も、保存処理（`saveProfile`）自体は変えない。

## コード生成

不要（バインド対象の型・メソッドのシグネチャとイベントは変えない）。

## テスト方針

### Go ユニット（`internal/application/mqtt/service_test.go`）

`callbackRecorder` と、`connectFn` をチャネルでブロックさせるモックを使い、接続を `stateConnecting` に留めて検証する。

- 確立前の `Subscribe` は `nil` を返し、client の `Subscribe` を呼ばない。`GetConnections` の `Subscriptions` には載り、`Connected` は false。
- 接続を確立させて `onConnected` を起動すると、保留中の購読が同じ QoS で 1 回ずつ購読され、登録したハンドラが `mqtt:message` を発行する。
- 確立前に同じトピックを QoS 0 → QoS 1 の順に `Subscribe` すると、`onConnected` は QoS 1 で 1 回だけ購読する。
- 確立前に `Subscribe` → `Unsubscribe` した購読は、`onConnected` で購読されず、client の `Unsubscribe` も呼ばれない。
- 確立前に購読して接続が失敗した場合、client の `Subscribe` は呼ばれず、接続は一覧から消える。
- 確立前に購読して `Disconnect` した場合、その後に `onConnected` が起動しても購読しない。
- 張り直し中の待ち合わせ（決定的に再現する）: モックの `Subscribe` をチャネルでブロックさせて `onConnected` の張り直しを止め、その間に別の goroutine から同じトピックを別の QoS で `Subscribe` する。張り直しを解放するまで後者が client を呼ばないこと、解放後に後者の QoS で 1 回だけ client が呼ばれ、`subs` がその QoS になることを確かめる。
- 既存の `TestMQTTService_Subscribe_ClientError` など、確立済みの接続でのエラー伝搬は変わらないこと（既存テストがそのまま通る）。
- `task go:test:race` で上記と既存の並行テストを通す。

### Go 統合（`internal/integration/mqtt_test.go`）

「`mqtt:connected` が出た」「`GetConnections` に購読が載った」はどちらも SUBACK の完了を意味しない（`mqtt:connected` は張り直しより先に出る）。受信の検証は、`TestMQTT_ConnectionLostAndReconnect` と同じくブローカー側の購読数（`Info.Subscriptions`）が増えるのを待ってから publish するか、retained メッセージを使って行う。

- `TestMQTT_SubscribeBeforeConnected`: `Connect` の直後に `waitConnected` を待たずに `Subscribe` し、ブローカー側の購読成立を待ってから publish したメッセージが届く。
- `TestMQTT_ReconnectAfterDisconnect_Resubscribes`: 接続・購読 → `Disconnect` → 新しい `Connect` の直後に同じトピックを `Subscribe` → ブローカー側の購読成立を待って外部クライアントから publish したメッセージが新しい接続 ID で届く（フロントエンドの `handleReconnect` と同じ呼び出し順）。

### フロント ユニット

- `connections.test.ts`
  - 既存の「moves state to the new connection id and re-subscribes」はそのまま通る。
  - `handleReconnect(id, profile)` は渡したプロファイルで `api.connect` を呼び、新しいタブの `profile` もそれになる。
  - 1 件目の `api.subscribe` を保留した状態で 2 件目の購読をタブから外すと、2 件目の `api.subscribe` は呼ばれない。
- `subscriptions.test.ts`
  - `connected: false` のオンラインタブで購読を外すと `api.unsubscribe` が呼ばれ、行が消える。
  - そのとき `api.unsubscribe` が失敗してもトーストは出ず、行は消える。
  - 接続済みのタブでの失敗は従来どおりトーストを出す。オフラインタブでは `api.unsubscribe` を呼ばない。

### UI e2e

変更なし（偽バックエンドは既に確立前の Subscribe / Unsubscribe を受理している）。既存のスペックが通ることだけ確認する。

### フルスタック e2e（`frontend/e2e/integration/mqtt/mqtt.spec.ts`）

- 「reconnecting after Disconnect keeps receiving subscribed topics」: テスト固有のトピックを購読 → Disconnect → その間に `publishFromBroker(topic, <固有の値>, { retain: true })` → Connect → 一覧にその retained メッセージが出る。retained メッセージは新しい接続で購読が成立したときにしか届かないので、購読の成立を待つ処理が要らず、タイミングで揺れない。`Failed to re-subscribe` のアラートが出ないこと、`mqttConnections(page)` の購読が 1 件残っていることも確かめる。`finally` で空ペイロードの retain を publish して後始末する（既存のスキャンのテストと同じ）。修正前はこのテストが落ちることを確認してから修正する。

## 副作用・注意事項

- **`Subscribe` / `Unsubscribe` RPC の意味が変わる**: 接続確立前でもエラーにならず成功を返す（実際の購読は確立時）。
- **確立時の購読拒否は画面に伝わらない**: 確立前に受理した購読をブローカーが拒否した場合、バックエンドはログを出して `conn.subs` から外すが、フロントエンドにはトーストが出ず、行も残る（自動再接続の張り直しと同じ扱い）。表示との食い違いはリロード時の `GetConnections` で解消される。**後続課題**: 拒否を通知するイベント（例: `mqtt:subscription-rejected`）を追加し、フロントエンドで行を外してトーストを出す。
- **確立前の購読は `GetConnections` に載る**: 確立待ちの間に WebView をリロードすると、保留中の購読も購読済みとして復元される（確立すれば実際に購読されるので、表示と実態は一致する）。
- **retained メッセージの重複**: 「残る窓: `runConnect` による遷移」のとおり、ごく短い窓で購読が 2 回送られ、retained メッセージが 2 件並ぶことがある。
- **スキャン（`#` の購読）**も確立前に受理されるようになる。現状の UI では確立前に Scan を押せないので実害はない。
- **範囲外の既存の挙動**:
  - 接続が切れて自動再接続中（`stateConnected` のまま paho が `reconnecting`）の `Subscribe` / `Unsubscribe` は、従来どおり paho がエラーを返す。自動再接続中に UI で外した購読は、`removeSubscription` の失敗がログだけになり、バックエンドの `conn.subs` に残って再接続時に張り直される（受信は UI の購読一覧で絞り込まれるので表示はされない）。
  - `mqtt:connected` が `api.connect` の戻り値より先にフロントエンドへ届くと、新しいタブがまだ無いので `connected: true` が反映されず、タブが未接続表示のまま残る可能性がある（未検証の仮説）。`handleConnect` にも同じ構造があり、今回の変更で新たに起きるものではない。フルスタック e2e がこれで揺れるようなら別途調査する（例: タブ登録後に `getConnections` で状態を照合する）。

## Git運用

- **ブランチ名**: `fix/mqtt-resubscribe-on-manual-reconnect`
- **コミット分割方針**:
  1. `test(e2e): 手動で再接続した MQTT 接続が購読を受信し続けることを検証する`（フルスタック e2e。修正前に落ちることを確認）
  2. `fix(mqtt): 接続確立前の購読を保留し、確立時に購読する`（`service.go` と Go ユニットテスト。`onConnected` の `opMu` による直列化を含む）
  3. `fix(mqtt): 再接続待ちの間に外した購読をバックエンドに残さない`（`connections.ts` の再購読ループ、`subscriptions.ts`、フロントのユニットテスト）
  4. `fix(mqtt): Save & Connect で保存したプロファイルを使って再接続する`（`connections.ts` の `profile` 引数、`broker-tree.tsx`、Provider の型、フロントのユニットテスト）
  5. `test(integration): 接続確立前と再接続直後の購読を検証する`（`internal/integration/mqtt_test.go`）
- **完了条件**: 次をすべて通してから main へマージする
  - 常に: `task format` → `task lint` → `task test`
  - `task frontend:tsc`（`handleReconnect` のシグネチャを変えるため）
  - `task go:test:integration`（`internal/integration/` にテストを追加するため）
  - `task go:test:race`（接続 goroutine・`onConnected` と `Subscribe` の並行処理に触れるため。cgo/gcc が無く実行できなければその旨を報告する）
  - `task frontend:test:e2e`（UI e2e。フロントの挙動を変えるため）
  - 任意（CI 非対象）: `task frontend:test:e2e:fullstack`（追加したフルスタック e2e で実際の不具合の解消を確認する。今回は実施を推奨）
