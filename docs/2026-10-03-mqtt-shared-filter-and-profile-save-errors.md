# 変更計画書: MQTT の共有購読のトピックフィルターと、プロファイル保存・削除の失敗の通知

## 概要

`docs/refactor-report-frontend-mqtt.md` の末尾に書いた 2 点が実際に起きることを確認した。どちらもフロントエンドだけの不具合で、この計画で直す。

### 確認結果 1: 共有購読をトピックフィルターに選ぶと一覧が空になる（再現した）

`frontend/src/application/mqtt/messages.ts` の `collectFilterTopics` と `filterMessagesByTopic` を、共有購読を含む入力で実行して確かめた。

| 購読 | 受信トピック | フィルターの選択肢 | その購読をフィルターに選んだ結果 |
| --- | --- | --- | --- |
| `$share/g/sensors/#` | `sensors/temp` | `$share/g/sensors/#` だけ（`sensors/temp` が並ばない） | 0 件 |
| `$share/g/a` | `a` | `$share/g/a` | 0 件 |
| `$queue/q/+` | `q/1` | `$queue/q/+` だけ | 0 件 |

原因は、受信したメッセージのトピックに接頭辞（`$share/<group>/`・`$queue/`）が付かないのに、フィルターが接頭辞の付いたままの文字列で照合していること。一覧への表示（`connections.ts` の `flushMessages`）は `makeSubscription` が `stripSharedPrefix` を通した `patternParts` で照合するので正しく、フィルターだけが漏れている。

### 確認結果 2: プロファイルの保存・削除の失敗が通知されない（コード上で確認した）

`createProfilesState` の `saveProfile`・`deleteProfile` は失敗をそのまま投げ、呼び出し側のどこも捕まえていない。`unhandledrejection` のハンドラや ErrorBoundary も無いので、失敗はコンソールに出るだけで、画面には何も出ない。呼び出しは 4 か所ある。

| 箇所 | 失敗したときに今起きること |
| --- | --- |
| `sidebar/broker-tree.tsx:63-67` `handleProfileSave` | 通知が出ない。ダイアログは開いたまま（結果としては望ましい） |
| `sidebar/broker-tree.tsx:69-79` `handleProfileSaveAndConnect` | 同上。接続も始まらない |
| `sidebar/broker-tree.tsx:81-85` `handleProfileDelete` | 先にタブを閉じ（オンラインなら切断し）、そのあと削除する。削除に失敗すると、通知が出ないまま切断だけが済み、ブローカーは一覧に残る |
| `application/mqtt/connections.ts:455` `updateConnectionBroker` | 接続バーの入力のたびに保存する。失敗しても通知が出ず、タブの URL だけが変わって保存されていない状態になる |

失敗は実際に起こり得る。Go の `ProfileService.SaveProfile` は未知の非空 ID に `NotFoundError` を返す。プロファイルを削除したあとに残った接続を復元すると `synthesizeProfile` が元の ID でプロファイルを合成するので、そのタブの接続バーを編集すると必ず失敗する。ディスクへの書き込みの失敗も同じ経路を通る。

udp は `createTargetsState` が `notifyOnError`・`runGuarded` で通知し、`TargetTree` が失敗時にダイアログを閉じない。mqtt をこれに揃える。

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `frontend/src/domain/mqtt/topic.ts` | Domain | 変更 | `hasWildcard` を追加する（`messages.ts` から移す） |
| `frontend/src/application/mqtt/messages.ts` | Application | 変更 | `collectFilterTopics`・`filterMessagesByTopic` が、購読の文字列に限って `stripSharedPrefix` を通したフィルターで照合する。`filterMessagesByTopic` は購読の一覧を引数に受け取る |
| `frontend/src/application/mqtt/profiles.ts` | Application | 変更 | `createProfilesState` が `Notifier` を受け取り、`saveProfile`・`deleteProfile` の失敗を通知してから例外を伝える |
| `frontend/src/application/mqtt/connections.ts` | Application | 変更 | `updateConnectionBroker` が `saveProfile` の失敗を受け止める（通知は `saveProfile` が出す） |
| `frontend/src/presentation/providers/mqtt-provider.tsx` | Presentation | 変更 | `createProfilesState` に `notify` を注入する |
| `frontend/src/presentation/components/sidebar/broker-tree.tsx` | Presentation | 変更 | 保存の失敗でダイアログを開いたままにし、削除は成功してからタブを閉じる |
| `frontend/src/domain/mqtt/topic.test.ts` | Domain | 変更 | `hasWildcard` のテストを追加する |
| `frontend/src/application/mqtt/messages.test.ts` | Application | 変更 | 共有購読のフィルターのテストを追加する |
| `frontend/src/application/mqtt/profiles.test.ts` | Application | 変更 | 通知のテストを追加し、既存の呼び出しに notifier を渡す |
| `frontend/src/application/mqtt/connections.test.ts` | Application | 変更 | `updateConnectionBroker` の保存失敗のテストを追加する |
| `frontend/e2e/fake-backend/types.ts` | e2e | 変更 | seed に `saveProfileError`・`deleteProfileError` を追加する |
| `frontend/e2e/fake-backend/install.ts` | e2e | 変更 | `SaveProfile`・`DeleteProfile` が上の seed で失敗する |
| `frontend/e2e/ui/mqtt/messages.spec.ts` | e2e | 変更 | 共有購読をフィルターに選ぶテストを追加する |
| `frontend/e2e/ui/mqtt/profiles.spec.ts` | e2e | 変更 | 保存・削除の失敗のテストを追加する |
| `docs/refactor-report-frontend-mqtt.md` | docs | 変更 | 直した 2 点の記述（「対応不要」の `broker-tree.tsx` の項と末尾の節）を消す。`hasWildcard` の候補も、この計画で済むので消す |

バックエンドは変更しない。

## 実装方針

### 1. 共有購読のトピックフィルター

フィルターの値は今と同じく購読のトピック文字列（`$share/g/sensors/#`）のままにし、照合のときだけ接頭辞を外す。選択肢の表示が購読の一覧と一致し、signal に入る値も変わらない。

- `domain/mqtt/topic.ts` に `hasWildcard(topic)` を置く。`stripSharedPrefix` と同じファイルに並べ、`messages.ts` はここから import する（`subscription.ts` のインラインの式は、この計画では触らない）。
- `collectFilterTopics`: 購読ごとに `const match = stripSharedPrefix(sub.topic)` を求め、`hasWildcard(match)` のときに `topicMatches(match, msg.topic)` で一致した実トピックを足す。ワイルドカードの無い共有購読（`$share/g/a`）は、購読そのものだけを選択肢にする（実トピック `a` を足すと、同じ結果になる選択肢が 2 つ並ぶ）。
- `filterMessagesByTopic(messages, filter, subscriptions)`: 選択肢には購読の文字列と実トピックが同じ文字列として並ぶので、接頭辞を外すのは `filter` が購読の文字列（`subscriptions` のどれかの `topic` と一致する）のときだけにする。実トピックにまで `stripSharedPrefix` を通すと、`$share/g/$queue/q/+` の購読で受信した実トピック `$queue/q/1` を選んだときに `q/1` で照合してしまい、0 件になる。
  - 購読でない（実トピック）: 今と同じく `m.topic === filter` で絞る。
  - 購読: `const match = stripSharedPrefix(filter)` を求め、`hasWildcard(match)` なら `topicMatches(match, m.topic)`、そうでなければ `m.topic === match` で絞る。同じ文字列が実トピックとしても受信されている場合（選択肢は `Set` なので 1 つにまとまる）に備えて、`m.topic === filter` のメッセージも残す。
  - 空のフィルターで元の配列をそのまま返す挙動は変えない。
  - `createMessagesState` の `visibleMessages` は `activeConnection()?.subscriptions ?? []` を渡す。
- 選択肢から消えたフィルターを解除する effect と、選択を追従させる effect は変えない。書き込む effect は増えない。

依存方向は application → domain のままで、新しい import は `domain/mqtt/topic` だけ。

### 2. プロファイルの保存・削除の失敗の通知

- **`application/mqtt/profiles.ts`**: `createProfilesState(api, notifier, orderStorage)` にする（引数の順は udp の `createTargetsState` に合わせる）。
  - `saveProfile`: 失敗したら `notifier.error("Failed to save broker", errorMessage(err), { key: "mqtt:save-profile:" + profile.id })` を出して、例外をそのまま投げ直す。`key` を付けるのは、接続バーの入力のたびに保存が走るため。同じプロファイルの失敗が続いても通知は 1 つにとどまる（`notifyOnError` は `key` を渡せないので、ここでは直接書く）。
  - `deleteProfile`: `notifyOnError(notifier, "Failed to delete broker", ...)` を使い、失敗を呼び出し側に伝える。udp の `deleteTarget` は `runGuarded` で握りつぶすが、mqtt は成功したときだけタブを閉じる必要があるので、成否を伝える。
  - 失敗時に一覧と並び順を変えない挙動は今のまま。
- **`application/mqtt/connections.ts`**: `updateConnectionBroker` の `saveProfile(updatedProfile)` に `.catch(() => {})` を付け、「通知は注入された `saveProfile` が出す」とコメントに書く。失敗してもタブの URL は入力した値のままにする（入力欄を巻き戻すと打てなくなる）。保存されていないことは通知で伝わる。
- **`presentation/providers/mqtt-provider.tsx`**: 合成ルートとして `createProfilesState(mqttClient, notify, createProfileOrderStorage())` に変える。`notify` は既に `createConnectionsState` に渡しているものと同じ。
- **`presentation/components/sidebar/broker-tree.tsx`**（すべてユーザー操作のイベントハンドラで、effect からの書き戻しは無い）:
  - `handleProfileSave`・`handleProfileSaveAndConnect`: `const saved = await saveProfile(profile).catch(() => null); if (!saved) return;` とし、失敗時はダイアログを開いたままにする（`TargetTree.handleSave` と同じ書き方）。
  - `handleProfileDelete`: `async` にし、`await deleteProfile(id)` が成功してから `closeConnection` を呼ぶ。失敗時はタブも接続もそのまま残す。`deleteProfile` は通知のあと例外を投げ直し、`ProfileList` は `onDelete` の戻り値（Promise）を受け止めないので、`handleProfileDelete` の中で失敗を捕まえて終える（`try { await deleteProfile(id); } catch { return; }`。通知は `deleteProfile` が出す）。捕まえないと未捕捉の rejection になる。`profile-list.tsx` と udp 側は変えない。

`presentation/components/` から `infrastructure/` への import は増えない。application の `profiles.ts` が新しく import するのは `domain/ui/ports`（`Notifier`）、`shared/error`、`application/ui/guard` で、どれも既存の依存方向の範囲内。

### 範囲に含めないもの

- `mqtt-provider.tsx` の `onMount` で呼ぶ `loadProfiles()` の失敗。Go の `GetProfiles` はエラーを返さないが、RPC の Promise が失敗することはあり得て、そのときは後続の `connState.restore()` も実行されない。RPC 自体の失敗への対応は `docs/2026-10-03-mqtt-profile-load-failure.md` で扱う。
- 削除済みプロファイルの接続を復元したタブで、接続バーを編集できること自体。この計画では失敗が通知されるようになるだけで、編集の可否は変えない。

## 永続化への影響

なし。localStorage のキー（`mqtt:profileOrder` など）と値の形は変えない。バックエンドの保存形式にも触れない。

## コード生成

不要（バインド対象の Go の型・メソッドとイベント定数は変えない）。

## テスト方針

- **Go ユニット / Go 統合**: 変更なし。
- **フロント ユニット**:
  - `domain/mqtt/topic.test.ts`: `hasWildcard` が `+`・`#` を含むトピックで true、含まないトピックで false を返す。
  - `application/mqtt/messages.test.ts`:
    - `collectFilterTopics` が、`$share/g/sensors/#` と `$queue/q/+` の購読に対して、接頭辞の無い実トピックを選択肢に足す。
    - `collectFilterTopics` が、ワイルドカードの無い `$share/g/a` に対しては購読そのものだけを返す。
    - `filterMessagesByTopic` が、上の 3 種類のフィルターで一致するメッセージだけを返す。
    - `filterMessagesByTopic` が、購読でない実トピックには接頭辞を外さずに完全一致で絞る（`$share/g/$queue/q/+` の購読で受信した `$queue/q/1` を選ぶと、そのメッセージが残る）。
    - 共有購読でないフィルターの結果が変わらない（既存のテストに購読の引数を足す）。
  - `application/mqtt/profiles.test.ts`:
    - 保存の失敗で `Failed to save broker` を `key` 付きで通知し、例外を伝え、一覧と並び順を変えない。
    - 削除の失敗で `Failed to delete broker` を通知し、例外を伝え、プロファイルを残す。
    - 成功時は通知しない。既存の `withState` に notifier を足す。
  - `application/mqtt/connections.test.ts`: `updateConnectionBroker` で `saveProfile` が失敗しても、未捕捉の例外にならず、タブの URL は入力した値になる。
- **UI e2e**（`task frontend:test:e2e`）:
  - `e2e/ui/mqtt/messages.spec.ts`: `$share/group/sensors/#` を購読してメッセージを 2 トピック受信し、フィルターにその購読を選ぶと受信したメッセージが残る。実トピックを選ぶとそのトピックだけに絞られる。
  - `e2e/ui/mqtt/profiles.spec.ts`:
    - `seed.saveProfileError` で、Save を押すと `Failed to save broker` のトーストが出て、ダイアログが開いたまま、一覧にブローカーが増えない。
    - 同じ seed で、接続バーのホストを書き換えるとトーストが 1 つだけ出る。
    - `seed.deleteProfileError` と `mqttConnect: "ok"` で、接続中のブローカーを削除すると `Failed to delete broker` のトーストが出て、ブローカーが一覧に残り、接続が切れていない（`fake.snapshot().mqttConnections` が残る）。未捕捉の例外が出ていないことも確かめる（fixture は `pageerror` を検査しないので、`messages.spec.ts` と同じく `page.on("pageerror", ...)` で集める）。
  - fake-backend の `SaveProfile`・`DeleteProfile` は、seed が未設定なら今と同じ動きをする。
- **フルスタック e2e**: 追加なし。

## 副作用・注意事項

- **フィルターの選択肢が増える**: ワイルドカード付きの共有購読では、今まで並ばなかった実トピックが選択肢に並ぶ。
- **削除の順序が変わる**: 今は「タブを閉じる → 削除」、変更後は「削除 → 成功したらタブを閉じる」。成功時の結果は同じだが、削除の RPC が返るまでタブが残る。Go の `DeleteProfile` は接続を切らない（接続を切るのは `closeConnection` の `Disconnect`）ので、順序を入れ替えてもバックエンドの状態は変わらない。
- **`createProfilesState` と `filterMessagesByTopic` の引数が変わる**: 呼び出し元は、前者が `mqtt-provider.tsx` とテスト、後者が `messages.ts` の `createMessagesState` とテストだけ。
- **保存の失敗時の通知は同じプロファイルにつき 1 つ**: 通知が消えるまで、同じプロファイルの次の失敗は新しいトーストにならない（`key` による抑制。接続バーの連続入力を想定した動き）。
- **`hasWildcard` の移動**: `messages.ts` のローカル関数が無くなり、domain から import する。export が 1 つ増える。
- リファクタリング調査レポートの候補（`connections.ts` の分割など）は、この計画では実施しない。`docs/refactor-report-frontend-mqtt.md` は未コミットなので、このブランチの最初のコミットで追加する。

## Git運用

- **ブランチ名**: `fix/mqtt-shared-filter-and-profile-errors`
- **コミット分割方針**:
  1. `docs: フロントエンド MQTT のリファクタリング調査レポートと修正計画を追加する`
  2. `fix(frontend): 共有購読をトピックフィルターで絞り込めるようにする`（`topic.ts`・`messages.ts` とそのユニットテスト）
  3. `test(e2e): 共有購読のトピックフィルターを検証する`
  4. `fix(frontend): ブローカーの保存と削除の失敗を通知する`（`profiles.ts`・`connections.ts`・`mqtt-provider.tsx`・`broker-tree.tsx` とそのユニットテスト）
  5. `test(e2e): ブローカーの保存と削除の失敗を検証する`（fake-backend の seed と spec）
  6. `docs: 修正した 2 点をリファクタリング調査レポートから消す`
- **完了条件**: 次がすべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task frontend:test:e2e`（UI の振る舞いと fake-backend を変えるため）
  - `task go:test:integration`・`task go:test:race` は対象外（バックエンドを変えない）
