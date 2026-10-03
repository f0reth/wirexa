# リファクタリング調査レポート（対象: frontend mqtt）

調査範囲は `frontend/src/` の `{domain,application,infrastructure}/mqtt/`、`presentation/components/mqtt/`、`presentation/providers/mqtt-provider.tsx`、`presentation/components/sidebar/{broker-tree,profile-list}.tsx`、`infrastructure/storage/local-storage.ts` の mqtt 部分（テストを除いて約 3,500 行）。パスは `frontend/src/` からの相対で書く。

## 改善候補一覧

挙げた 12 件はすべて対応した。残っている候補は無い。

## 対応不要と判断した箇所（理由つき）

- **`domain/mqtt/types.ts` と Go の `internal/domain/mqtt/types.go` の型の重複**: 意図的な重複。Go の型が配線型の正で、フロントは `Subscription`・`ConnectionState` などの UI 向けの型を足す層。
- **`domain/mqtt/topic.ts` の `stripSharedPrefix` とバックエンドの `filterMatches`**: 両側で同じ規則を持つ必要があり、コメント（`topic.ts:7-8`）で対応付けている。削除や統合はできない。
- **`application/mqtt/profile-validation.ts` のポート・ホストの検証**: 送信前の先回り検証。`parseBrokerUrl` で読み戻せる形に限るというフロント固有の理由もコメントに書かれている。
- **`application/mqtt/connections.ts:9` の `application/logger` の `Logger`**: application に置かれたポートとして扱う現行の設計。
- **`application/mqtt/` が signal・store で状態を持つこと**: 現行の設計。
- **`application/mqtt/profiles.ts` と `application/udp/targets.ts` の共通化**: 並び順の読み込みは既に `applyOrder`・`moveItem` を共有している。残りは挙動が違う（mqtt は signal を手元で更新し、削除の失敗を呼び出し側に伝える。udp は store を RPC で読み直し、削除の失敗を通知だけで終える）。まとめるには差を引数で切り替える抽象化が要り、共通になる部分が小さいので効果が見合わない。
- **`sidebar/profile-list.tsx`**: mqtt と udp で既に共通化されている。相違点は props で受けていて、追加でまとめる箇所は無い。
- **`broker-settings-dialog.tsx` と udp の `TargetDialog`（`target-tree.tsx`）の共通化**: 入力項目・検証・ボタンの構成が違い、`profile-list.tsx:38-40` のコメントも編集ダイアログは共通化しないと決めている。
- **`broker-manager.tsx:36-48` の effect**: 書き換えるのはコンポーネント内の入力欄の signal だけで、application の状態は書き戻していない。規約の範囲内で、理由もコメントに書かれている。
- **`panels/messages-panel.tsx:47-53` の effect**: スクロールの追従だけを行う。選択の追従は `application/mqtt/messages.ts` の 1 か所にある。
- **`application/mqtt/subscriptions.ts` の `setIsScanning`（`:167-225`）**: 長いが、開始と停止の順序を守るための処理で、分けると順序の前提が読み取りにくくなる。`subscriptions.test.ts` が順序を細かく検証している。
- **`application/mqtt/connections.ts:143`・`:154` の `console.error`**: 注入された `logger` を使わず、コンソールに出している。`logger.error` に替えるとログの出力先（バックエンドのログファイル）が変わるので、挙動変更にあたる。替えるかどうかは別に決める。
- **`infrastructure/storage/local-storage.ts` の mqtt 部分**: `ConnectionPersistence`（`loadLastProfileId` など）だけメソッド名がほかのポート（`load`・`save`）と違うが、実害が無く、揃える効果が小さい。`StoredPreset`（`:49`）は `retain` 導入前の保存値を読むためのもので必要。
- **`presentation/components/mqtt/utils.ts` の `getTopicColor`**: 表示用の色の計算で、presentation に置くのが適切。
- **`application/mqtt/connections.ts` の `switchConnection`**: `setActiveConnectionId` を包むだけだが、setter をそのまま公開しないための入口として残す。

## 調査中に見つけた不具合の候補（この調査の範囲外）

リファクタリングでは挙動を変えないので、ここでは直さない。直すなら候補ごとに変更計画を分ける。いずれもコードを読んで確かめたもので、画面での再現はしていない。

- **接続中のブローカーを編集して「Save」で保存すると、バックエンドの接続が画面から見えなくなる。** `sidebar/broker-tree.tsx:62-67` の `handleProfileSave` は、保存に成功すると必ず `createOfflineConnection(saved)` を呼ぶ。`createOfflineConnection`（`application/mqtt/connections.ts:265-269`）は同じプロファイルのタブを消してオフラインのタブに置き換えるが、`api.disconnect` は呼ばない。編集は接続中でも開ける（`sidebar/profile-list.tsx` の `onEdit` に条件は無い）。画面はオフラインになり、接続・購読・スキャンはバックエンドに残る。そのまま接続し直すと、同じプロファイルの接続が 2 つになる。「Save & Connect」は `handleReconnect` を通るので起きない。`e2e/ui/mqtt/profiles.spec.ts` の編集のテストは、接続中の「Save」を検証していない。
- **接続バーの URL を続けて編集すると、古い保存の応答が新しい値を上書きし得る。** `updateConnectionBroker`（`application/mqtt/connections.ts:252-263`）は入力のたびに `saveProfile` を呼び、前の保存を待たない。`saveProfile`（`application/mqtt/profiles.ts:58-76`）は応答を無条件に一覧へ入れる。応答が送った順に返らないと、一覧（サイドバーの URL）が古い値に戻る。タブの URL は入力した値のままなので、2 つが食い違う。直すなら、保存を直列にするか、最後に送った保存の応答だけを反映する。
- **Subscribe ボタンが無効でも、トピック入力欄の Enter で購読を送れる。** `panels/subscriptions-panel.tsx:45` の `onKeyDown` は接続を確かめずに `addSubscription()` を呼ぶ。`addSubscription`（`application/mqtt/subscriptions.ts:77-122`）も接続の種類を見ないので、オフラインのタブでは `offline-<profileId>` を接続 ID にして RPC を送る。確立待ちのオンラインのタブへの購読は意図して許している（バックエンドが確立時に購読する）ので、止めるのはオフラインのタブだけにする。
- **起動時にプロファイルの読み込みが失敗すると、後続の初期化も止まる。** `providers/mqtt-provider.tsx:116-123`。`docs/2026-10-03-mqtt-profile-load-failure.md` で扱う。
