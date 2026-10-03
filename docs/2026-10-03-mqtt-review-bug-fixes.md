# 変更計画書: MQTT のレビュー指摘 3 件の不具合修正

## 概要

`docs/refactor-report-backend-mqtt.md` のレビューで指摘された不具合 3 件を、実際のコードで確認した。3 件とも再現する順序・入力がコード上に存在するので修正する。

| # | 指摘 | 確認結果 | 実害 |
|---|------|----------|------|
| 1 | `Disconnect` と `Shutdown` が重なると、`Shutdown` の後にイベントが出る | 正しい | 小。終了時に `Disconnect` の RPC が実行中のときだけ起きる |
| 2 | 張り直しに失敗した購読が画面に残る | 正しい（一部補足あり）。手動の再接続で購読の RPC が失敗した行も同じく残る | 中。購読中と表示されたままメッセージが届かない |
| 3 | 共有購読（`$share/`・`$queue/`）のメッセージが表示されない | 正しい | 中。共有購読だけの接続では受信が 1 件も表示されない |

指摘 4（イベントを型付きにする案に JSON 化の検証が無い）は、現在のコードの不具合ではなくリファクタリング案への補足なので、この計画には含めない。型付きにする作業を行うときに、`json.Marshal` の結果を検証するテストを足す。

### 確認した内容

**1. `Disconnect` と `Shutdown` の競合**

- `detach`（`internal/application/mqtt/service.go:351-368`）は接続を `s.conns` から外して `stateDisconnecting` にする。`Disconnect` はその後に `opMu` を待ち、`client.Disconnect(1000)` を呼び、`mqtt:disconnected` を発行する（`service.go:333-343`）。発行の前に状態は確認しない。
- この間に `Shutdown` が走ると、接続は `s.conns` に無いので収集（`service.go:752-762`）に入らない。`connWg` は接続 goroutine とスキャンの開始だけを数えるので、`Shutdown` は実行中の `Disconnect` を待たない。
- その結果、`Shutdown` が `true` を返した後に、`client.Disconnect` の実行と `mqtt:disconnected` の発行が起きる。`Shutdown` の doc コメント（「この区間を抜けた後はこの service からイベントは発行されない」「true は全接続を切断し」）と `app.go:235` のコメント（「上限を過ぎてもイベント発行は止まっている」）に反する。
- `TestMQTTService_Stress_ConcurrentOperations` 相当のテスト（`service_test.go:1568-1582`）は、`Disconnect` を含む全 goroutine の完了を待ってからイベント数を数えるので、この順序を検出しない。
- 実害: `client.Disconnect(1000)` は最大 1 秒待つので、タブを閉じた直後（`closeConnection` は `disconnect` の完了を待たない）にアプリを終了すると起きる。終了後の `runtime.EventsEmit` と、切断が終わる前のプロセス終了になる。

**2. 張り直しに失敗した購読**

- `resubscribe`（`service.go:494-505`）は、ブローカーが拒否した購読と、接続が開いたまま失敗した購読を `subs` から外す。イベントは出さない。`mqtt:connected` は張り直しの前に発行される（`service.go:297`）。
- フロントの `mqtt:connected` の処理（`frontend/src/application/mqtt/connections.ts:308-314`）は `connected` を変えるだけ。`getConnections` を呼ぶのは起動時の `restore` だけ（`connections.ts:381`）。
- したがって、外された購読の行は画面に残り、メッセージは届かない。`handleReconnect` が確立前に送った購読（`connections.ts:558-572`）が確立時に拒否された場合も同じ。
- `handleReconnect` の購読は、新しい接続の確立が先に終わっていると張り直しを通らない。`Subscribe` は client を直接呼び（`service.go:419`）、拒否は RPC のエラーで返る。`handleReconnect` の `catch`（`connections.ts:566-571`）は通知するだけなので、前の接続から引き継いだ行が残る。バックエンドの `subs` には入っていないので、この経路ではイベントを足しても行は外れない。
- `addSubscription`（`frontend/src/application/mqtt/subscriptions.ts:45-64`）は、`Subscribe` の RPC の完了を待ってから行を足す。確立前の `Subscribe` は登録だけして返り、その直後に `onConnected` の張り直しが走る。RPC の応答とイベントは別の goroutine から送られるので、届く順序は決まっていない。張り直しの失敗を知らせるイベントが応答より先に届くと、外す行がまだ無く、応答の後に足された行が残る。
- 補足: レビューは「同じトピックを追加し直すこともできない」としているが、行を削除してから追加し直せば購読できる（`removeSubscription` → `addSubscription`）。重複判定（`subscriptions.ts:38`）で弾かれるのは、行を残したまま追加した場合だけ。ただし、ユーザーには購読が外れたことが分からない。

**3. 共有購読**

- バックエンドは `$share/<group>/` と `$queue/` を外して照合する（`internal/infrastructure/mqtt/paho_client.go:74-81`）ので、`$share/group/sensors/#` を購読すると `sensors/temp` が `mqtt:message` で届く。
- フロントの `makeSubscription`（`frontend/src/application/mqtt/subscription.ts:6-15`）は接頭辞を付けたまま `patternParts` を作り、`flushMessages`（`connections.ts:249-255`）は `s.topic === data.topic` か `patternParts` の一致で表示を決める。`["$share","group","sensors","#"]` は `sensors/temp` に一致しないので、メッセージは捨てられる。
- ワイルドカードの無い共有購読（`$share/group/a/b`）は `patternParts` が無く、`s.topic === data.topic` も成り立たないので、同じく表示されない。
- `ValidateTopicFilter`（`internal/domain/mqtt/topic.go:33-47`）は `$share/` を拒否しないので、共有購読は入力できる。

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `internal/application/mqtt/service.go` | Application | 変更 | (1) 実行中の `Disconnect` を `Shutdown` が待ち、イベントを止める。(2) 張り直しで購読を外したときにイベントを発行する |
| `internal/application/mqtt/service_test.go` | Application | 変更 | (1)(2) のテストを足す |
| `internal/domain/events.go` | Domain | 変更 | `EventMQTTSubscriptionDropped = "mqtt:subscription-dropped"` を足す |
| `internal/domain/mqtt/types.go` | Domain | 変更 | イベントのペイロード `SubscriptionDropped` を足す |
| `tools/gen-events/main.go` | ツール | 変更 | `{"mqttSubscriptionDropped", domain.EventMQTTSubscriptionDropped}` を足す |
| `frontend/src/shared/wails-events.ts` | Shared | 再生成 | `task go:generate:events` で生成する（手で編集しない） |
| `frontend/src/domain/mqtt/topic.ts` | Domain | 変更 | 共有購読の接頭辞を外す `stripSharedPrefix` を足す |
| `frontend/src/domain/mqtt/topic.test.ts` | Domain | 変更 | `stripSharedPrefix` のテストを足す |
| `frontend/src/application/mqtt/subscription.ts` | Application | 変更 | 共有購読では、接頭辞を外したフィルターから `patternParts` を作る |
| `frontend/src/application/mqtt/subscriptions.ts` | Application | 変更 | `mqtt:subscription-dropped` を受けて購読の行を外し、通知する。`addSubscription` の RPC の実行中にイベントが届いた購読は、行を足さない |
| `frontend/src/application/mqtt/connections.ts` | Application | 変更 | `handleReconnect` の購読の RPC が失敗したら、`getConnections` でバックエンドの購読を確かめ、無い購読の行を外す |
| `frontend/src/presentation/providers/mqtt-provider.tsx` | Presentation | 変更 | `createSubscriptionsState` に `onMqttEvent` を渡す |
| `frontend/src/application/mqtt/*.test.ts` | Application | 変更 | 上の 3 つのテストを足す（`subscriptions.test.ts`・`connections.test.ts` に足す。`makeSubscription` のテストファイルが無ければ `subscription.test.ts` を作る） |
| `frontend/src/infrastructure/mqtt/events.test.ts` | Infrastructure | 確認 | イベント名の一覧を持つテストなので、新しいイベントで落ちないかを確かめる |
| `frontend/e2e/ui/`（MQTT の spec） | e2e | 変更 | 共有購読の表示と、購読が外れたときの表示のテストを足す |

`frontend/e2e/fake-backend/` は変えない。バインド API の振る舞いは変わらず、新しいイベントは既存の `mqtt:message` と同じくテストが emit して模す。

## 実装方針

### 1. `Disconnect` と `Shutdown` の競合（backend application）

`Shutdown` の契約（区間を抜けた後はイベントを出さない、`true` は全接続を切断済み）を変えずに、実行中の `Disconnect` をその契約に含める。

- `MQTTService` に `closing map[*connection]struct{}`（`mu` で保護）を足す。「`detach` 済みで `Disconnect` が終わっていない接続」を表す。
- `detach` は、`s.conns` から外すのと同じ `mu` の区間で `closing` に足し、`s.connWg.Add(1)` する。`Connect`・`reserveScan` と同じく、`mu` の保持中に計上するので、`Shutdown` の `Wait` の後に計上する窓はできない（`closed` の後は `s.conns` が空なので `detach` は `NotFoundError` を返す）。
- `Disconnect` は、切断とイベント発行を終えて `opMu`・`stateMu` を放してから、`mu` を取って `closing` から外し、`connWg.Done()` する。ロック順序 `mu → opMu → stateMu` を守るため、`opMu`・`stateMu` を持ったまま `mu` を取らない（今の `defer` 2 つを内側の関数へ移す）。
- `Disconnect` のイベント発行は、`stateMu` の保持中に `conn.state == stateClosed` なら行わない（`Shutdown` が先に閉じた）。そうでなければ今と同じく `stateClosed` にして発行する。
- `Shutdown` は、`s.conns` を処理するのと同じ `mu` の区間で、`closing` の各接続を `stateMu.Lock` して `stateClosed` にする。この `Lock` は実行中の発行の完了を待つので、区間を抜けた後は `Disconnect` もイベントを出さない。`client.Disconnect` は `Disconnect` 側が行うので、`Shutdown` は `connWg.Wait()` でその完了を待つだけにする。
- `connWg`・`closing`・`Shutdown` の doc コメントを、実行中の `Disconnect` を含む内容に直す。

採らなかった案: `detach` で `s.conns` から外すのをやめ、`Disconnect` の最後に外す案。`closing` は要らなくなるが、`StopTopicScan` が切断中の接続に `NotFoundError` を返さなくなり、`Shutdown` の収集で「切断中の接続」と「接続失敗で閉じた接続」を区別する必要が出るので、変更が広がる。

### 2. 張り直しに失敗した購読の通知（backend domain / application、frontend application）

- `internal/domain/events.go` に `EventMQTTSubscriptionDropped` を足す。ペイロードは `internal/domain/mqtt/types.go` に `ScannedTopic` と同じ扱いの型で足す。

  ```go
  // SubscriptionDropped は mqtt:subscription-dropped のペイロード。
  type SubscriptionDropped struct {
      ConnectionID string `json:"connectionId"`
      Topic        string `json:"topic"`
      Error        string `json:"error"`
  }
  ```

- `resubscribe` は、購読を外す `stateMu.Lock` の区間（`service.go:495-497`）で、接続が `terminal()` でなければ `deleteSub` に続けてイベントを発行する。「状態を見てからイベントを出す」を 1 区間に収める既存の規約どおりで、`client.Unsubscribe` より前に発行する（`stateMu` の保持中に client 操作をしない）。
- フロントは `subscriptions.ts` の `createSubscriptionsState` に `mqtt:subscription-dropped` のリスナーを足す（引数に `onEvent: MqttEventListener` を足し、`mqtt-provider.tsx` が `onMqttEvent` を渡す）。`updateConnection` で `topic` が一致する購読の行を外し、`notifier.error` でトピックと理由を出す（`key` は接続 ID とトピック）。イベントハンドラからの書き込みなので、「presentation の effect から application の状態を書き戻さない」には当たらない。`onCleanup` で解除する。リスナーを `connections.ts` ではなく `subscriptions.ts` に置くのは、次の項目の「RPC の実行中の購読」を同じファイルで持つため。
- イベントが RPC の応答より先に届く場合に備える。`addSubscription` は、`api.subscribe` を呼ぶ前に接続 ID とトピックの組を「実行中」として登録し、RPC が終わったら外す。リスナーは、届いたイベントの組が実行中なら「外された」と印を付ける。`addSubscription` は RPC が成功しても、印が付いていれば行を足さない（通知はリスナーが出している）。実行中でない組のイベントは印を残さない（残すと、後で同じトピックを追加したときに行が足されなくなる）。
- `handleReconnect` の購読の RPC が失敗した場合は、イベントが出ない（張り直しを通らない）ので、フロントで行を外す。`catch` で今と同じ通知を出した後に `api.getConnections()` を呼び、結果で分ける。
  - 新しい接続があり、その購読に `sub.topic` が無い: その行（`sub.id`）を外し、残りの購読の送信を続ける。
  - 新しい接続が無い（接続に失敗して閉じた）: 行は残し、残りの購読は送らずにループを抜ける。行を残すのは、次の Reconnect で購読を引き継ぐため。
  - `getConnections` が失敗した: 行は残し、ログに記録する。
  
  エラーの文言では分けない（Go 側にもフロント側にも、文言で比較している箇所は無い）。
- 依存方向: domain に型と定数を足し、application が発行する。adapters・`app.go`・infrastructure は変えない。フロントは既存の `MqttEventListener` と `MqttConnectionApi.getConnections` を使うので、新しいポートは要らない。注入は `mqtt-provider.tsx` で `onMqttEvent` を `createSubscriptionsState` にも渡すだけ。`MqttEventName` は再生成で広がる。

採らなかった案: フロントが `mqtt:connected` のたびに `getConnections` で購読を取り直す案。`mqtt:connected` は張り直しの前に出るので、取り直した時点では失敗が反映されていない。

### 3. 共有購読の照合（frontend domain / application）

- `frontend/src/domain/mqtt/topic.ts` に `stripSharedPrefix(filter: string): string` を足す。バックエンドの `filterMatches` と同じ規則にする。
  - `$share/<group>/<filter>`（`/` で 3 つ以上に分かれるとき）は `<filter>` を返す。`$share/group` のように 3 つに分かれないものはそのまま返す。
  - `$queue/<filter>` は `<filter>` を返す。
  - それ以外はそのまま返す。
- `makeSubscription` は、接頭辞を外したフィルターを `matchFilter` とし、`matchFilter` にワイルドカードがあるか、`matchFilter !== topic`（共有購読）のときに `patternParts = compilePattern(matchFilter)` を入れる。`topic`（表示と RPC に使う値）は変えない。`flushMessages` は変えずに済む（ワイルドカードの無い共有購読も `patternParts` の完全一致で表示される）。
- `Subscription` 型（`frontend/src/domain/mqtt/types.ts`）は変えない。

## 永続化への影響

なし。購読は保存していない。プリセットなど localStorage のキーと値も変えない。

## コード生成

- [ ] `task go:generate:events`（`events.go` と `tools/gen-events/main.go` に `mqtt:subscription-dropped` を足した後）
- [ ] `task wails:generate`（`SubscriptionDropped` はイベント専用の型でバインド対象ではないので、差分は出ない見込み。実行して `git diff --exit-code frontend/wailsjs` で確かめる）

## テスト方針

**Go ユニット**（`internal/application/mqtt/service_test.go`）

- `Disconnect` が `client.Disconnect` の中で止まっている間に `Shutdown` を呼ぶ。`Disconnect` を進めるまで `Shutdown` が返らないこと、返った後に `mqtt:disconnected` が出ないこと、`Disconnect` が `nil` を返すことを確かめる。
- 同じ状況で `Disconnect` が進まない場合、`Shutdown` が `timeout` で `false` を返し、その後に `Disconnect` を進めてもイベントが出ないことを確かめる。
- `Shutdown` と重ならない `Disconnect` は、今までどおり `mqtt:disconnected` を 1 回出すこと（既存の `TestMQTTService_Disconnect_Success` が守る）。
- `TestMQTTService_Reconnect_ResubscribeFailures` に、外した購読ごとに `mqtt:subscription-dropped`（`connectionId`・`topic`・`error`）が 1 回出ること、接続が切れて残した購読では出ないことを足す。
- 張り直しの途中で `Disconnect` された接続では `mqtt:subscription-dropped` が出ないこと。
- `SubscriptionDropped` を `json.Marshal` した結果のキーが `connectionId`・`topic`・`error` であること（`internal/domain/mqtt/` のテスト）。

**Go 統合**: 足さない。張り直しの拒否は `tools/e2e-broker` のブローカーでは起こせず、`Shutdown` との競合はユニットテストでクライアントを止めて再現するほうが確実。

**フロント ユニット**

- `topic.test.ts`: `stripSharedPrefix` が `$share/g/a/#` → `a/#`、`$queue/a/b` → `a/b`、`$share/g` → そのまま、`a/b` → そのまま、を返すこと。
- `makeSubscription`: `$share/g/sensors/#` の `patternParts` が `["sensors","#"]`、`$share/g/a/b` の `patternParts` が `["a","b"]`、`a/b` は `patternParts` 無し、`topic` はどれも入力のまま、であること。
- `subscriptions`: `mqtt:subscription-dropped` で該当する行だけが外れ、通知が 1 回出ること。存在しない接続・トピックでは何も起きないこと。
- `subscriptions`: `addSubscription` の RPC を止めたまま同じ接続・トピックの `mqtt:subscription-dropped` を流し、その後に RPC を成功させても行が足されないこと。その後にもう一度同じトピックを追加すると行が足されること（印が残っていない）。
- `connections`: `handleReconnect` で 2 つ目の購読の RPC だけを失敗させ、`getConnections` が新しい接続（その購読なし）を返すと、その行だけが外れ、残りの購読は送られて行が残ること。
- `connections`: `handleReconnect` で購読の RPC が失敗し、`getConnections` に新しい接続が無いと、行はすべて残り、残りの購読は送られないこと。`getConnections` が失敗した場合も行が残ること。
- `connections`: 共有購読の接続に `mqtt:message` を流すと表示されること。

**UI e2e**（`frontend/e2e/ui`）

- `$share/group/sensors/#` を購読し、テストから `sensors/temp` の `mqtt:message` を emit して、メッセージが表示されること。
- 購読の行がある状態で `mqtt:subscription-dropped` を emit して、行が消えて通知が出ること。

**フルスタック e2e**: 足さない。

## 副作用・注意事項

- `Shutdown` と重なった `Disconnect` は `mqtt:disconnected` を出さなくなる。終了処理中なので、受け取る画面は無い。`Disconnect` の戻り値は変わらない。
- `Shutdown` は実行中の `Disconnect` を待つので、その分だけ返るのが遅くなる（上限は今と同じ `mqttShutdownTimeout` の 3 秒）。
- 張り直しで購読が外れると、画面から行が消えてエラー通知が出るようになる。今は行が残って何も出ない。
- `mqtt:subscription-dropped` は、確立前に受け付けた購読（`handleReconnect` が送ったものを含む）が確立時に拒否された場合にも出る。`handleReconnect` の行は前の接続から引き継いでいるので、イベントで外れる。`addSubscription` の行は RPC の応答の後に足すので、イベントが先に届いた場合は行を足さない（実装方針 2）。
- `handleReconnect` で購読の RPC が失敗すると、新しい接続が生きていればその行が消える。今は行が残る。新しい接続が閉じていた場合は、残りの購読を送らなくなるので、「Failed to re-subscribe」の通知は購読の数だけではなく 1 回になる。
- `handleReconnect` の購読の失敗時に `getConnections` の RPC が 1 回増える。成功時には呼ばない。
- 共有購読のメッセージが表示されるようになる。muted の判定は今と同じく購読の行ごとに行う。
- フロントの照合規則はバックエンドの `filterMatches` と同じにする。片方を変えるときはもう片方も直す必要があるので、`stripSharedPrefix` のコメントに `paho_client.go` の `filterMatches` と対応していることを書く。
- 並行処理に触れるので、`task go:test:race` を必ず実行する。

## Git運用

- **ブランチ名**: `fix/mqtt-review-bugs`
- **コミット分割方針**: 不具合ごとに分ける。
  1. `fix(mqtt): Shutdown が実行中の Disconnect を待ち、終了後にイベントを出さないようにする`（`service.go`・`service_test.go`）
  2. `fix(mqtt): 張り直しで外れた購読を mqtt:subscription-dropped で通知する`（domain・application・`tools/gen-events`・生成物・Go のテスト）
  3. `fix(frontend): 張り直しで外れた購読の行を外して通知する`（`subscriptions.ts`・`mqtt-provider.tsx` とテスト）
  4. `fix(frontend): 再接続で購読に失敗した行を外す`（`connections.ts` とテスト）
  5. `fix(frontend): 共有購読のメッセージを表示する`（`topic.ts`・`subscription.ts` とテスト）
  6. `test(e2e): 共有購読の表示と購読が外れたときの表示を検証する`
- **完了条件**: 次をすべて通してから main へマージする。
  - `task format` → `task lint` → `task test`
  - `task go:test:race`（cgo/gcc が無くて実行できなければ、その旨を報告する）
  - `task go:test:integration`（`internal/integration/mqtt_test.go` が切断と終了処理を検証しているため）
  - `task frontend:test:e2e`
  - `git diff --exit-code frontend/wailsjs`（`task wails:generate` の後）
