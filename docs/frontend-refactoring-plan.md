# フロントエンド段階的リファクタリング計画

作成日: 2026-09-19

最終更新: 2026-09-19 — Step 1 の残課題と Step 2〜5 の実施結果を反映（全ステップ完了）。
それ以前の更新は [`frontend-refactoring-plan-review.md`](./frontend-refactoring-plan-review.md)
のレビュー指摘と、その後の追加調査（対象漏れ・行番号のずれ）の反映。

## 進捗

| ステップ | 状態 | コミット |
|---|---|---|
| Step 1: 導出ロジックの移動 | 完了 | `d7d67a9` |
| Step 1 残課題: UI 文言を presentation へ戻す | 完了 | `0311327` |
| Step 2: infrastructure 直接 import の排除 | 完了 | `dc975e5` |
| Step 3: 通知の統一（契約 C） | 完了 | `5644d90` |
| Step 4: publish-tab の一方向化（方針 A） | 完了 | `3b0618d` |
| Step 5: 起動時の復元順序の集約 | 完了 | `86639bf` |

## 背景

「単一 Root ツリー / 受動的な描画コンポーネント / 親へのイベント伝搬 / 中央司令塔」への
アーキテクチャ変更が有効かを調査した結果、以下の結論に至った。

- **単一 Root ツリー**: すでに成立している（`App.tsx` が唯一の根）。変更不要。
- **受動的な描画コンポーネント**: 方向性は正しく途中まで進んでいる。残りを仕上げる価値がある。
- **親へのイベント順次伝搬（Context 廃止）**: 推奨しない。SolidJS では Context の読み取りに
  再描画コストがなく、再帰ツリー（HTTP サイドバー）と document レベルのドラッグ&ドロップが
  伝搬モデルに乗らない。
- **単一の中央司令塔**: 4 プロトコルは独立しているため、グローバルな司令塔は god object になる。
  現在の `application/` の `createXxxState` 群がプロトコル単位の司令塔として機能しており、
  この粒度を維持する。

フロントエンドの fix コミット約 38 件を分類すると、状態同期・ライフサイクル起因が約 13 件、
DOM・CSS・フォーカス・ドラッグ判定起因が約 16 件、バックエンド仕様起因が約 7 件だった。
状態管理の設計変更で防げるのは最初の群だけであり、全面書き換えはコストに見合わない。

したがって、既存の層構造（domain → application → presentation、infrastructure は
ports 実装）を維持したまま、以下の 5 ステップを順に適用する。

## 進め方の原則

- 各ステップは独立した PR にし、`task frontend:tsc` / `task frontend:test` /
  `task frontend:ci` / `task frontend:test:e2e` を通してからマージする。
  Taskfile がカバーしていない絞り込み実行（例: `bunx vitest run src/shared/array.test.ts`）は
  `frontend/` 内で直接実行してよい。
- UI e2e（`e2e/ui`、フェイクバックエンド）が安全網となる。挙動を変えないリファクタリング
  なので、e2e の期待値は変わらないはず。変わった場合は退行を疑う。
- Context と Provider の構造、`use-tree-drag-drop.ts` の document リスナー方式は現状維持。
- Provider は依存を組み立てる composition root として扱う。infrastructure の import や
  通知ストアの参照が Provider に残ることは層規約違反としない（Step 2・Step 3 の完了条件も
  この前提で `presentation/components/` に範囲を限定している）。
- コメント・コミットメッセージは日本語。

---

## Step 1: コンポーネント内の純粋な導出ロジックを application 層へ移す（完了）

**状態**: コミット `d7d67a9`（2026-09-19）で対象表の 7 行すべてと単体テストの追加を実施済み。
本計画書自体も同じコミットで追加されている。以下の対象表は実施結果として残す。
未着手なのは Step 2 以降。

**目的**: presentation 配下の単体テストが 1 ファイル（`use-long-press-drag.test.ts`）しかなく、
純粋関数がコンポーネント内に埋まっているため未テストになっている。切り出して単体テストを付ける。

**対象（実施済み）**

| 移動前の場所 | 内容 | 移動先 |
|---|---|---|
| `presentation/components/udp/send-form.tsx` | `isValidAscii` / `isValidHex` / `hexByteCount` | `application/udp/field-validation.ts` |
| `presentation/components/udp/send-form.tsx` | フィールドごとの `isValueValid` / `byteCount` / `byteCountLabel` / `valueLabel` / `valuePlaceholder` / `valueInputType` | 同上 |
| `presentation/components/udp/send-form.tsx` | `totalBytes` の集計 | 同上（`totalFieldBytes`） |
| `presentation/components/mqtt/broker-settings-dialog.tsx` | `defaultPort` / `parseBrokerUrl` / `composeBrokerUrl` | `application/mqtt/broker-url.ts` |
| `presentation/components/mqtt/broker-settings-dialog.tsx` | プロファイルのバリデーション | `application/mqtt/profile-validation.ts` |
| `presentation/components/http/request-editor.tsx` | `bodyContent` / `setBodyContent` / `formPairs` / `setFormPairs` の導出 | `application/http/request.ts` の `createRequestState` |
| `presentation/components/mqtt/panels/messages-panel.tsx` | `uniqueTopics` / `filteredMessages` | `application/mqtt/messages.ts` |

**手順（実施済み）**

1. 対象関数を application 層に移し、コンポーネントからは import して使う。
2. 移した関数ごとに `*.test.ts` を追加する（境界値: 空文字、非 ASCII、奇数長 hex、
   ポート 0 / 65536、ワイルドカード `#` `+` を含むトピック等）。
3. `task frontend:lint` / `task frontend:tsc` / `task frontend:test` を通す。

**完了条件**: 対象コンポーネントが「state を読んで描画し、イベントを state の関数に渡す」
だけになっている。

**残課題: UI 表示の都合が application に入り込んでいる（完了 — `0311327`）**

実施内容: `fieldByteCountLabel` は `ByteCountInfo`（`"non-ascii" | "invalid-hex" |
"out-of-range" | "var-length" | "fixed-size"` の判別共用体）を返す `fieldByteCountInfo` に、
`fieldValueLabel` / `fieldValuePlaceholder` / `fieldValueInputType` は `FieldValueKind`
（`"ascii" | "hex" | "integer" | "wide-integer" | "float"`）を返す `fieldValueKind` に置き換えた。
文言と HTML input type の対応表は `presentation/components/udp/field-display.ts` に置き、
同ファイルの単体テストで検証している。以下は当時の課題記述。

`application/udp/field-validation.ts` に描画ポリシーが移ってしまっている。

- `fieldValueLabel` / `fieldValuePlaceholder` / `fieldValueInputType`（70-92 行）は
  英語ラベル・placeholder・HTML input type そのもので、application が presentation の都合を
  知る構造になっている。
- `fieldByteCountLabel`（58-69 行）も `"non-ASCII"` / `"invalid hex"` / `"3 bytes"` という
  表示文字列を返しており、性質は同じ。

同ファイルの `ByteCountStatus`（`"ok" | "warn" | "error"` を返し、CSS クラスへの対応付けは
presentation が行う）は正しい形になっているので、これに揃える。

- application: 検証、バイト数計算、フィールド種別の判定を、列挙型・数値・真偽値で返す。
- presentation: ラベル、placeholder、HTML input type、CSS クラスへの対応表を持つ。

受動的な描画コンポーネントであっても、静的な表示文言や HTML 属性の対応表まで application に
移す必要はない。この是正は Step 2 と同じファイル群には触らないため、独立した PR にしてよい。

---

## Step 2: コンポーネントからの infrastructure 直接 import を排除する（完了 — `dc975e5`）

**実施結果**: `RequestApi` に `openFilePicker` / `guessFormPartContentType` /
`saveResponseBody` / `saveResponseBinary` を追加し `HttpProvider` で注入。コンポーネントは
`useHttpRequest()` の `pickFilePath` / `guessFormPartContentType` / `saveResponseToFile` を使う。
レスポンス保存の分岐（切り詰め時は temp ファイル、それ以外は base64）は `createRequestState`
に移して単体テストを付けた。失敗時の扱いは当時未決だったため現状維持とし、Step 3 でも
この操作は通知対象に含めていない（従来どおり例外は握らない）。
`createEmptyProfile` は手順 3 の props/Context 注入ではなく
`application/mqtt/profiles.ts` からの直接 import にした（`isValidProfileDraft` と同じ形で、
presentation → application は層規約上問題ないため）。手順 4 の biome 設定も実施済み。

**目的**: リポジトリの層規約（presentation → application → domain）に違反している箇所を直す。
infrastructure を直接触るコンポーネントは、フェイクや差し替えができず、テストしにくい。

**対象**（行番号は `d7d67a9` 時点）

| ファイル | 違反している import |
|---|---|
| `presentation/components/http/request-editor.tsx` 13 行 | `openFilePicker`（`infrastructure/http/client`） |
| `presentation/components/http/form-row-editor.tsx` 17-20 行 | `openFilePicker` と `guessFormPartContentType`（同上） |
| `presentation/components/http/response-viewer.tsx` 7-10 行 | `saveResponseBody` と `saveResponseBinary`（同上） |
| `presentation/components/mqtt/broker-settings-dialog.tsx` 14 行 | `generateId`（`infrastructure/id/generator`） |

`guessFormPartContentType` は Go の判定を呼ぶ RPC ラッパー（`infrastructure/http/client.ts`
173 行）なので、`openFilePicker` と同様に注入が必要。拡張子判定を TS 側へ複製してはいけない。

Provider（`http-provider.tsx` 28-30 行、`mqtt-provider.tsx` 25-31 行、
`openapi-provider.tsx` 23-30 行、`udp-provider.tsx` 15-16 行）も infrastructure を import
しているが、これは composition root としての依存組み立てであり違反として扱わない。

**手順**

1. `createRequestState` の依存（`RequestApi`）に `openFilePicker` と
   `guessFormPartContentType` を追加し、`HttpProvider` で注入する。
   コンポーネントは `useHttpRequest()` 経由で呼ぶ。
2. レスポンス保存（`saveResponseBody` / `saveResponseBinary`）も同じ依存に追加し、
   `response-viewer.tsx` は `useHttpRequest()` 経由で呼ぶ。
   保存失敗時の扱いは Step 3 で決める契約に従う。
3. `broker-settings-dialog.tsx` は空プロファイルの生成を `application/mqtt/profiles.ts` に
   移し、`generateId` はそちらで使う。ダイアログは `createEmptyProfile` を props か
   Context から受け取る。
4. 再発防止として、biome の `noRestrictedImports` で
   `presentation/components/**` から `infrastructure/**` への import を禁止する設定を検討する
   （Provider は対象から除外する）。

**完了条件**: `presentation/components/` 配下に `infrastructure/` への直接 import が存在しない
（`rg -n "infrastructure/" frontend/src/presentation/components` が空）。

`presentation/` 全体から排除したい場合は、Provider の依存組み立てを `composition/` など
presentation 外の層へ移す必要があり、それは本ステップの範囲外の別作業とする。

---

## Step 3: エラー通知の方式を 1 つに統一する（完了 — `5644d90`）

**実施結果**: 未決事項 1 は **C（対象を限定）** を採用。`domain/ui/ports.ts` に `Notifier`
（`Record<NotificationLevel, NotifyFn>`）を定義し、`createXxxState` の引数として Provider が
`notificationStore.notify` を注入する。`application/ui/guard.ts` は notifier を受け取る
純粋なヘルパーとして残し、再 throw する `notifyOnError` を追加した。
`openapi` のドロップ処理は読み出し失敗も通知対象に含めるため
`openDropped(content, name)` を `openDroppedFile(file)` に変更して Provider 側に集約した。
MQTT の `publish` は Provider（composition root）で `runGuarded` する。
`target-tree` は保存失敗時にダイアログを閉じないよう修正した（計画書が指摘していた退行）。

**目的**: 現在、同じ関心事に対して複数の方式が混在している。

| 方式 | 例 |
|---|---|
| コンポーネント内で `runGuarded(label, fn)` | `collection-tree.tsx`（10 箇所）、`publish-tab.tsx` 188 行、`openapi/index.tsx` 54 行 |
| コンポーネント内で `.catch((err) => notify.error(...))` | `send-form.tsx` 262 行、`listen-form.tsx` 89 行、`target-tree.tsx` 146 行、`openapi-file-tree.tsx` 51 行 |
| Provider 内で `runGuarded` / `notify` | `openapi-provider.tsx` 118、133、155 行 |
| application 層内で `notify.error(...)` を直接呼ぶ | `application/mqtt/connections.ts`、`application/mqtt/subscriptions.ts`、`application/http/request.ts`、`application/udp/receive.ts`、`application/ui/guard.ts` |

application 層がシングルトン `notify` に直接依存しているため、application の単体テストが
UI 通知ストアに暗黙に結合している。`application/ui/guard.ts` はその依存を内側に抱える本体
なので、移行対象に含める。

**方針**: 通知を「application 層の出力」として扱い、コンポーネントは通知の存在を知らない
形にする。ただし「Notifier を注入するか」と「例外を握るか」は別の設計判断として扱う。

**手順**

1. `application/logger.ts` の `Logger` と同様に、`Notifier` 型（`domain/ui/ports.ts` に新規定義。
   現状このファイルには `Notification` 型はあるが `Notifier` はない）を `createXxxState` の
   引数として注入する。`notificationStore.notify` は Provider で渡す。
2. **【要決定】失敗時の契約を決める。** 「例外を投げず正常終了する」を全操作に適用すると、
   現在の型契約が壊れる。

   - `createCollection` は `Promise<Collection>`、`addFolder` / `addRequest` は `TreeItem` を
     返し（`application/http/collections.ts` 28、31、36 行）、
     `collection-tree.tsx` 76、86、111、127 行が戻り値の `id` でリネーム編集を開始している。
   - `target-tree.tsx` 129-132 行のように `await` の完了だけを成功判定にしている箇所は、
     保存に失敗しても画面を閉じてしまう。

   次のいずれかを選ぶ。

   - **A: `Result<T, E>` を返す** — 呼び出し側が成功と失敗を明示的に分岐する。型で漏れを
     防げるが、変更範囲が最も広い。
   - **B: 通知してから再 throw する** — 通知の注入だけを目的にする。呼び出し側の
     `runGuarded` は残るが、既存挙動を一切変えない。
   - **C: 例外を握る対象を限定する（推奨）** — 戻り値も後続処理も持たない `Promise<void>` の
     操作だけ application 層で握って正常終了し、戻り値を持つ操作は B の扱いにする。
     影響範囲を抑えつつ、通知の重複と握りつぶしの両方を避けられる。

   いずれの場合も「失敗を成功として扱う」変更（戻り値の捏造、`await` 完了＝成功の維持）は
   採らない。
3. コンポーネント側の `runGuarded` と `.catch(notify.error)` のうち、手順 2 で application 層が
   責任を持つことになったものを削除する。`application/ui/guard.ts` は不要になれば削除する。
4. `connections.test.ts` 等で、モック `Notifier` を渡して「失敗時に通知される」ことを
   検証するテストを追加する。C を選んだ場合は「戻り値を持つ操作では例外が伝播する」ことも
   テストする。

**完了条件**: `presentation/components/` 配下に `notify` と `runGuarded` の参照がない。
`application/` 配下に `notifications.ts` からの直接 import がない（Provider が注入する）。
Step 2 と同様に Provider は対象外とする（現状 `openapi-provider.tsx` が両方を使用している）。

---

## Step 4: publish-tab の双方向 effect 同期を一方向にする（完了 — `3b0618d`）

**実施結果**: 未決事項 2 は **A（挙動維持・`publishDraft`）** を採用。`createPresetsState` に
`draft` / `updateDraft(patch)` / `selectPreset(id)` を追加した。プリセット選択で draft へ読み込み、
フォーム入力は draft を更新したうえで選択中プリセットへ書き戻す（保存タイミングは入力のたび。
retain トグルが即座に一覧の Retained バッジへ反映される既存挙動を保つため）。
`publish-tab.tsx` のローカル signal 4 本と `createEffect` 2 本は削除。e2e の期待値は変更なし。

**目的**: `presentation/components/mqtt/publish-tab.tsx` 242-289 行では、フォーム用の
ローカル signal（topic / payload / qos / retain）とプリセット state を 2 本の
`createEffect` で相互に同期している。この構造は過去に 2 件の fix
（PointerEvent がプリセット名として渡る、インライン編集でのフォーカス喪失）を生んでいる。

**現状の確認**

- `application/mqtt/presets.ts` 11-13 行の `selectedPresetId` は初期値が `null`。
- `mqtt-provider.tsx` 143-145 行の `addPreset()` は**プリセットが 0 件のときだけ**呼ばれる。
  したがって保存済みプリセットがある状態で起動すると、どのプリセットも選択されていない。
- その未選択状態でもローカル signal（`publish-tab.tsx` 242-245 行）を編集して、
  一時的なメッセージを publish できる。

**【要決定】方針**: 上記の現状があるため、「選択中プリセットを唯一の真実にする」をそのまま
適用すると既存挙動が変わる（未選択時にフォームを disabled にする／暗黙に新規作成する、
のどちらを選んでも変わる）。次のどちらかを選ぶ。

- **A: 挙動を維持する（推奨）** — application 層にプリセットとは独立した `publishDraft` を置く。
  フォームは draft だけを描画・更新し、プリセット選択時に draft へ値をロードする。
  未選択でも publish できる現在の挙動が保たれ、「挙動を変えないリファクタリング」の前提に収まる。
- **B: 選択中プリセットを唯一の状態にする** — 仕様変更として扱う。初期選択（起動時に先頭を
  選ぶか）、削除後の選択、保存タイミングを決めたうえで、e2e の期待値も更新対象にする。

**手順**

1. A の場合: `application/mqtt/` に `publishDraft`（topic / payload / qos / retain）と
   `loadDraftFromPreset(id)` / `updateDraft(patch)` を追加する。
   B の場合: `createPresetsState` に選択中プリセットを返す `selectedPreset` アクセサと、
   フォーム変更を受ける `updateSelectedPreset(patch)` を追加する。
2. `PublishForm` は draft（A）または `selectedPreset()`（B）を直接描画し、入力イベントは
   `updateDraft({ topic })` のように 1 方向で流す。
   ローカル signal 4 本と `createEffect(on(...))` 2 本を削除する。
3. A では draft からプリセットへ保存する操作（`savePreset` / `updatePreset`）のタイミングを
   決める。B ではプリセット未選択時に入力を disabled にするか暗黙に新規作成するかを決め、
   仕様変更として PR に明記する。
4. `PresetsPanel` 内の `editingName` ローカル signal は「編集中の一時値」なので残してよいが、
   commit 時に `updatePreset` へ流す 1 方向であることを確認する。

**完了条件**: `publish-tab.tsx` に `createEffect` が存在しない。
e2e の MQTT publish 系テストがすべて通る。A を選んだ場合は e2e の期待値を変更せずに通ること、
B を選んだ場合は変更した期待値と変更理由を PR に明記すること。

---

## Step 5: 起動時の復元順序を 1 箇所に集める（完了 — `86639bf`）

**実施結果**: ツリー探索を `application/http/collections.ts` の `findRequestById` に切り出して
単体テストを付け、`HttpProvider` の `onMount` が `refreshCollections()` →
`restoreActiveRequest()` を実行する。`restoreActiveRequest` は Context から除去した
（呼び出し元は Provider だけになったため）。`UdpProvider` の `onMount` で `refreshTargets()`
を実行し、`createTargetsState` 内にあった暗黙の初回読み込みも削除して起動シーケンスを
Provider に一本化した。`MqttProvider` は元から同じ形。

**目的**: 起動時の非同期シーケンスが複数の場所に分散している。

| 場所 | 内容 |
|---|---|
| `presentation/components/sidebar/collection-tree.tsx` 58-61 行 | `onMount` で `refreshCollections()` の後に `restoreActiveRequest()` を呼ぶ |
| `presentation/components/sidebar/target-tree.tsx` 121-123 行 | `onMount` で `refreshTargets()` を呼ぶ |
| `presentation/providers/http-provider.tsx` 140-175 行 | `restoreActiveRequest` の実体（ツリー探索を含むロジック） |
| `presentation/providers/mqtt-provider.tsx` 140-147 行 | `onMount` で `loadProfiles()` → `restore()` → 既定プリセット作成 |

末端コンポーネント（CollectionTree）が「コレクション読込後に復元する」という
アプリ全体の順序を握っており、サイドバーを HTTP 以外にして起動すると復元が走らない。
UDP も同様に、TargetTree を表示するまでターゲット一覧が読み込まれない。

**手順**

1. `restoreActiveRequest` のロジック（ツリー探索）を `application/http/collections.ts`
   または `request.ts` に移し、単体テストを付ける。
2. `HttpProvider` の `onMount` で `refreshCollections()` → `restoreActiveRequest()` を
   実行し、`CollectionTree` の `onMount` を削除する。
3. `UdpProvider` の `onMount` で `refreshTargets()` を実行し、`TargetTree` の `onMount` を
   削除する。
4. `MqttProvider` も同じ形（Provider の `onMount` が起動シーケンスを持つ）で揃える。
   将来的に Provider 横断の順序が必要になった場合のみ、`App.tsx` に集約する。
5. 起動シーケンスをテストする際は、application 層の関数として
   `initialize(api, storage)` のように切り出せば vitest で検証できる。

**完了条件**: `presentation/components/` 配下に `onMount` で RPC を呼ぶ箇所がない
（対象は `collection-tree.tsx` と `target-tree.tsx` の 2 箇所）。
起動時の復元が、最初に表示するプロトコルに依存しない。

`openapi/index.tsx` 23 行、`openapi-file-tree.tsx` 26 行、`preview-panel.tsx` 11 行、
`use-tree-drag-drop.ts` 38 行の `onMount` は DOM リスナー登録であり RPC を呼ばないため、
本ステップの対象外とする。

---

## 対象外（現状維持とするもの）

- **Context / Provider 構造**: `tree-ui-context.tsx` は再帰ツリーへの prop drilling を
  解消するために導入されたもので、廃止すると 8 種類のイベントを全段で中継する必要が生じる。
- **ドラッグ&ドロップ**: `use-tree-drag-drop.ts` の document リスナーと
  `elementFromPoint` によるドロップ先判定は、コンポーネントツリーを遡るイベント伝搬では
  表現できない。`drag-state.ts` のモジュールレベル signal も、この方式の前提として残す。
- **MqttContext の統合**: 4 つの `useMqttXxx` フックが同一オブジェクトを返す構造は
  型レベルの分離にすぎないが、実害は出ていないため後回しでよい。
  ただし `...subsState, ...msgState, ...presetState` のスプレッドで名前衝突が
  静かに上書きされる点は認識しておく。

## 未決事項（決定済み）

| # | 対象 | 決めたこと |
|---|---|---|
| 1 | Step 3 手順 2 | **C: 例外を握る対象を限定する**（`Promise<void>` の操作は通知して正常終了、戻り値を持つ操作は通知して再 throw） |
| 2 | Step 4 方針 | **A: 挙動維持（`publishDraft` を追加）** |

## 残っている関連課題

- レスポンス保存（`saveResponseToFile`）は失敗時に通知しない（Step 2 実施時の現状維持）。
  Step 3 の契約に合わせるなら `Promise<void>` の操作として `runGuarded` の対象にできる。
- `application/openapi/files.ts` の `removeFile`、`createCollectionsState` の
  `refreshCollections` など、元から通知していなかった失敗は今回も通知対象に含めていない。
- 「対象外（現状維持とするもの）」の 3 項目（Context/Provider 構造、ドラッグ&ドロップ、
  MqttContext の統合）は方針どおり未着手。
