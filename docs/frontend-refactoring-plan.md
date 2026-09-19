# フロントエンド段階的リファクタリング計画

作成日: 2026-09-19

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

- 各ステップは独立した PR にし、`bun run tsc` / `bun run test` / `bun run ci` /
  `bun run test:e2e` を通してからマージする。
- UI e2e（`e2e/ui`、フェイクバックエンド）が安全網となる。挙動を変えないリファクタリング
  なので、e2e の期待値は変わらないはず。変わった場合は退行を疑う。
- Context と Provider の構造、`use-tree-drag-drop.ts` の document リスナー方式は現状維持。
- コメント・コミットメッセージは日本語。

---

## Step 1: コンポーネント内の純粋な導出ロジックを application 層へ移す

**目的**: presentation 配下の単体テストが 1 ファイル（`use-long-press-drag.test.ts`）しかなく、
純粋関数がコンポーネント内に埋まっているため未テストになっている。切り出して単体テストを付ける。

**対象**

| 現在の場所 | 内容 | 移動先（案） |
|---|---|---|
| `presentation/components/udp/send-form.tsx` 16-31 行 | `isValidAscii` / `isValidHex` / `hexByteCount` | `application/udp/field-validation.ts` |
| `presentation/components/udp/send-form.tsx` 129-183 行 | フィールドごとの `isValueValid` / `byteCount` / `byteCountLabel` / `valueLabel` / `valuePlaceholder` / `valueInputType` | 同上（フィールドを受け取る純粋関数として） |
| `presentation/components/udp/send-form.tsx` 53-58 行 | `totalBytes` の集計 | 同上 |
| `presentation/components/mqtt/broker-settings-dialog.tsx` 11-38 行 | `defaultPort` / `parseBrokerUrl` / `composeBrokerUrl` | `application/mqtt/broker-url.ts`（既存ファイルに統合できるか確認） |
| `presentation/components/mqtt/broker-settings-dialog.tsx` 83-92 行 | プロファイルのバリデーション | `application/mqtt/profiles.ts` または新規 `profile-validation.ts` |
| `presentation/components/http/request-editor.tsx` 52-84 行 | `bodyContent` / `setBodyContent` / `formPairs` / `setFormPairs` の導出 | `application/http/request.ts` の `createRequestState` に派生アクセサとして追加 |
| `presentation/components/mqtt/panels/messages-panel.tsx` 29-63 行 | `uniqueTopics` / `filteredMessages` | `application/mqtt/messages.ts` |

**手順**

1. 対象関数を application 層に移し、コンポーネントからは import して使う。
2. 移した関数ごとに `*.test.ts` を追加する（境界値: 空文字、非 ASCII、奇数長 hex、
   ポート 0 / 65536、ワイルドカード `#` `+` を含むトピック等）。
3. `bun run lint` と `bun run tsc` と `bun run test` を通す。

**完了条件**: 対象コンポーネントが「state を読んで描画し、イベントを state の関数に渡す」
だけになっている。

---

## Step 2: コンポーネントからの infrastructure 直接 import を排除する

**目的**: リポジトリの層規約（presentation → application → domain）に違反している箇所を直す。
infrastructure を直接触るコンポーネントは、フェイクや差し替えができず、テストしにくい。

**対象**

| ファイル | 違反している import |
|---|---|
| `presentation/components/http/request-editor.tsx` 14 行 | `openFilePicker`（`infrastructure/http/client`） |
| `presentation/components/http/form-row-editor.tsx` 19 行 | `openFilePicker`（同上） |
| `presentation/components/mqtt/broker-settings-dialog.tsx` 8 行 | `generateId`（`infrastructure/id/generator`） |

**手順**

1. `createRequestState` の依存（`RequestApi`）に `openFilePicker` を追加し、
   `HttpProvider` で注入する。コンポーネントは `useHttpRequest()` 経由で呼ぶ。
2. `broker-settings-dialog.tsx` は空プロファイルの生成を `application/mqtt/profiles.ts` に
   移し、`generateId` はそちらで使う。ダイアログは `createEmptyProfile` を props か
   Context から受け取る。
3. 再発防止として、biome の `noRestrictedImports` で
   `presentation/**` から `infrastructure/**` への import を禁止する設定を検討する。

**完了条件**: `presentation/` 配下に `infrastructure/` への import が存在しない
（`grep -r "infrastructure/" frontend/src/presentation` が空）。

---

## Step 3: エラー通知の方式を 1 つに統一する

**目的**: 現在、同じ関心事に対して 3 つの方式が混在している。

| 方式 | 例 |
|---|---|
| コンポーネント内で `runGuarded(label, fn)` | `collection-tree.tsx`（10 箇所）、`publish-tab.tsx` 188 行、`openapi/index.tsx` 54 行 |
| コンポーネント内で `.catch((err) => notify.error(...))` | `send-form.tsx` 314 行、`listen-form.tsx` 89 行、`target-tree.tsx` 146 行、`openapi-file-tree.tsx` 51 行 |
| application 層内で `notify.error(...)` を直接呼ぶ | `application/mqtt/connections.ts`、`application/mqtt/subscriptions.ts`、`application/http/request.ts` |

application 層がシングルトン `notify` に直接依存しているため、application の単体テストが
UI 通知ストアに暗黙に結合している。

**方針**: 通知を「application 層の出力」として扱い、コンポーネントは通知の存在を知らない
形にする。

**手順**

1. `application/logger.ts` の `Logger` と同様に、`Notifier` 型（`domain/ui/ports.ts` に定義）
   を `createXxxState` の引数として注入する。`notificationStore.notify` は Provider で渡す。
2. application 層の各操作（`addSubscription`、`handleConnect`、`sendRequest` 等）は
   内部で try/catch し、失敗時に注入された `notifier.error(label, message)` を呼んで
   正常終了する。呼び出し側に例外を投げない。
3. コンポーネント側の `runGuarded` と `.catch(notify.error)` を削除する。
   `application/ui/guard.ts` は不要になれば削除する。
4. `connections.test.ts` 等で、モック `Notifier` を渡して「失敗時に通知される」ことを
   検証するテストを追加する。

**完了条件**: `presentation/` 配下に `notify` と `runGuarded` の参照がない。
`application/` 配下に `notifications.ts` からの import がない（Provider のみが import する）。

---

## Step 4: publish-tab の双方向 effect 同期を一方向にする

**目的**: `presentation/components/mqtt/publish-tab.tsx` 242-289 行では、フォーム用の
ローカル signal（topic / payload / qos / retain）とプリセット state を 2 本の
`createEffect` で相互に同期している。この構造は過去に 2 件の fix
（PointerEvent がプリセット名として渡る、インライン編集でのフォーカス喪失）を生んでいる。

**方針**: 選択中プリセットを唯一の真実にし、フォームはそれを描画するだけにする。

**手順**

1. `application/mqtt/presets.ts` の `createPresetsState` に、選択中プリセットを返す
   `selectedPreset` アクセサと、フォーム変更を受ける `updateSelectedPreset(patch)` を追加する。
2. `PublishForm` は `selectedPreset()` を直接描画し、入力イベントは
   `updateSelectedPreset({ topic })` のように 1 方向で流す。
   ローカル signal 4 本と `createEffect(on(...))` 2 本を削除する。
3. プリセット未選択時の挙動（現在は `addPreset()` で 1 件作られる）を明示する。
   未選択なら入力を disabled にするか、暗黙に新規作成するかを決めて実装する。
4. `PresetsPanel` 内の `editingName` ローカル signal は「編集中の一時値」なので残してよいが、
   commit 時に `updatePreset` へ流す 1 方向であることを確認する。

**完了条件**: `publish-tab.tsx` に `createEffect` が存在しない。
e2e の MQTT publish 系テストがすべて通る。

---

## Step 5: 起動時の復元順序を 1 箇所に集める

**目的**: 起動時の非同期シーケンスが複数の場所に分散している。

| 場所 | 内容 |
|---|---|
| `presentation/components/sidebar/collection-tree.tsx` 58-61 行 | `onMount` で `refreshCollections()` の後に `restoreActiveRequest()` を呼ぶ |
| `presentation/providers/http-provider.tsx` 140-175 行 | `restoreActiveRequest` の実体（ツリー探索を含むロジック） |
| `presentation/providers/mqtt-provider.tsx` 140-147 行 | `onMount` で `loadProfiles()` → `restore()` → 既定プリセット作成 |

末端コンポーネント（CollectionTree）が「コレクション読込後に復元する」という
アプリ全体の順序を握っており、サイドバーを HTTP 以外にして起動すると復元が走らない。

**手順**

1. `restoreActiveRequest` のロジック（ツリー探索）を `application/http/collections.ts`
   または `request.ts` に移し、単体テストを付ける。
2. `HttpProvider` の `onMount` で `refreshCollections()` → `restoreActiveRequest()` を
   実行し、`CollectionTree` の `onMount` を削除する。
3. `MqttProvider` も同じ形（Provider の `onMount` が起動シーケンスを持つ）で揃える。
   将来的に Provider 横断の順序が必要になった場合のみ、`App.tsx` に集約する。
4. 起動シーケンスをテストする際は、application 層の関数として
   `initialize(api, storage)` のように切り出せば vitest で検証できる。

**完了条件**: `presentation/components/` 配下に `onMount` で RPC を呼ぶ箇所がない。
起動時の復元が、最初に表示するプロトコルに依存しない。

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

## 推奨順序と依存関係

```
Step 1 (導出ロジック移動)
  └─ Step 2 (infra import 排除)   ※ Step 1 で触るファイルと重なるため続けて行う
Step 3 (通知の統一)                 ※ 独立。Step 1-2 と並行可
  └─ Step 4 (publish-tab 一方向化)  ※ Step 3 で presets.ts の形が決まってから
Step 5 (起動順序の集約)             ※ 独立。いつでも可
```
