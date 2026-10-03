# 変更計画書: MQTT の起動時のプロファイル読み込みの失敗を通知する

## 概要

`mqtt-provider.tsx` の `onMount` は `await loadProfiles()` の失敗を受け止めていない。Go の `GetProfiles` はエラーを返さないが、RPC の Promise 自体は失敗し得る（バインディングが無い、Wails のランタイムの不調など）。失敗すると次のようになる。

- 未捕捉の rejection になり、画面には何も出ない。サイドバーは「No brokers yet」を表示し、ブローカーが 1 件も無いように見える。
- 同じ `onMount` の後続（`connState.restore()` とプリセットの初期作成）が実行されない。

この計画では、失敗を通知し、RPC に依存しない後続（プリセットの初期作成）を実行する。udp の同じ問題は別の計画で扱う（`docs/2026-10-03-udp-target-load-failure.md`）。http は対応済み（`http-provider.tsx` の `onMount`）で、この計画も同じ書き方に揃える。

対象は、フロントエンドの `loadProfiles()` が投げる例外だけ。Go 側のファイル読み込みの失敗では、このトーストは出ない。`GetProfiles` は起動時に読み込んだキャッシュを返すだけで（`ProfileService.GetProfiles`）、ファイルは `NewProfileService` が読む。破損したファイルや読めないファイルは `JSONStore.Load` がスキップしてログに残し、ディレクトリを読めなければ起動に失敗してダイアログを出して終了する（CLAUDE.md の「設定データの分類と復旧方針」のとおり）。

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `frontend/src/presentation/providers/mqtt-provider.tsx` | Presentation | 変更 | `onMount` で `loadProfiles()` の失敗を通知し、失敗時は `restore()` を呼ばず、プリセットの初期作成は続ける |
| `frontend/e2e/fake-backend/types.ts` | e2e | 変更 | seed に `getProfilesError` を追加する |
| `frontend/e2e/fake-backend/install.ts` | e2e | 変更 | `GetProfiles` が上の seed で失敗する |
| `frontend/e2e/ui/mqtt/profiles.spec.ts` | e2e | 変更 | 読み込みの失敗のテストを追加する |

バックエンドと application 層は変更しない。

## 実装方針

### 通知する場所

合成ルートの `onMount` で受け止める。`createProfilesState` の `loadProfiles` は今のまま例外を伝える（`profiles.test.ts` の「propagates a load failure and keeps the list empty」はそのまま通る）。「読み込めなかった」と「0 件だった」の区別を application 層に残すため、`loadProfiles` の中では握りつぶさない。

```ts
onMount(async () => {
  // プロファイルをロードしてからバックエンドの実接続状態を復元する。
  // 読み込めなかったときは復元しない（プロファイルが空のまま restore() すると、
  // 最後に選んだプロファイルが接続中でない限り、その保存値を消してしまう）。
  const loaded = await notifyOnError(
    notify,
    "Failed to load brokers",
    loadProfiles,
  ).then(
    () => true,
    () => false,
  );
  if (loaded) await connState.restore();
  if (presetState.presets().length === 0) {
    presetState.addPreset();
  }
});
```

`notifyOnError` は `application/ui/guard` から import する。`mqtt-provider.tsx` はこのモジュールをまだ import していないので、import を 1 行足す（`http-provider.tsx` が同じものを使っている）。`notify` は既に `createConnectionsState` に渡しているものと同じ。依存方向は presentation → application のまま。

### 失敗時に `restore()` を呼ばない理由

`restore()`（`connections.ts:179`）は、プロファイルが空のまま実行すると保存値を壊すことがある。

- `persistence.loadLastProfileId()` の ID は `profiles()` に無い。アクティブな接続が決まるのは、その ID のプロファイルがバックエンドで接続中の場合だけ（`GetConnections` の結果に同じ `profileId` があれば、`synthesizeProfile` で作ったプロファイルのタブがアクティブになる）。
- 接続中でなければアクティブな接続が決まらない。`GetConnections` も失敗した場合も同じ（`restore()` は失敗をログに出して、接続 0 件として続ける）。
- そのあと `setPersistActive(true)` で永続化の effect が動き、アクティブな接続が無いので `removeLastProfileId()` を呼ぶ。`mqtt:lastActiveProfileId` が消える。

呼ばなければ `persistActive` は false のままで、保存値は残る。次回の起動で読み込めれば、最後に選んだブローカーが復元される。

`restore()` を呼ばないと、バックエンドに残っている接続はタブとして復元されない。`GetProfiles` と `GetConnections` は別のサービスの呼び出しなので（`MQTTHandler` の `profileSvc` と `svc`）、片方だけ失敗することはあり得て、その場合は復元できたはずの接続が画面に出ない。それでも呼ばないのは、保存値を守るほうを優先するため。今の挙動（`loadProfiles()` の失敗で `restore()` が実行されない）からも変わらない。

### プリセットの初期作成は続ける

`presetState` は localStorage だけで完結し、RPC の成否と関係が無い。今は `loadProfiles()` の失敗で巻き添えになっているので、失敗時も実行する。

### やらないこと

- 自動の再試行。Wails の RPC は同一プロセス内の呼び出しなので、待てば直る失敗は少ないと見ている（実測はしていない）。http の対応でも再試行はしていない。
- `logger` への出力。`runGuarded`・`notifyOnError` を使うほかの箇所と同じく、通知だけにする。

### 範囲に含めないもの

- 通知が消えたあとも「No brokers yet」と表示されること。一覧の空状態と読み込み失敗を表示で区別する変更は、3 プロトコル共通の UI の話なので別に扱う。
- 読み込みに失敗したセッションでブローカーを新規作成すると、`saveProfile` が `mqtt:profileOrder` をその 1 件だけで上書きすること（今も起きる。次回の起動で、ほかのブローカーは保存順ではなくバックエンドの返す順に並ぶ）。

## 永続化への影響

なし。localStorage のキーと値の形は変えない。読み込み失敗時に `mqtt:lastActiveProfileId` を消さないことは、今の挙動と同じ。

## コード生成

不要。

## テスト方針

- **Go ユニット / Go 統合**: 変更なし。
- **フロント ユニット**: 追加なし。変更は Provider の `onMount` だけで、application 層の振る舞いは変わらない。`loadProfiles` が失敗を伝えることは既存のテストが検証している。
- **UI e2e**（`e2e/ui/mqtt/profiles.spec.ts`、seed は `{ mqttProfiles: [ALPHA], getProfilesError: "rpc down" }`）:
  - fixture は `goto` を済ませてからテストに入るので、`page.on("pageerror", ...)` を登録してから `page.reload()` し、起動時の失敗をもう一度起こして検証する。fixture が `localStorage` を消すのは初回の読み込みだけなので、reload の前に入れた値は残る。
  - reload の前に `localStorage` を次のようにしておく。
    - `mqtt:lastActiveProfileId` に `JSON.stringify(ALPHA.id)` を入れる（`saveToStorage` は JSON で保存する）。
    - `mqtt:presets` を消す。最初の `goto` でも読み込みに失敗して初期プリセットが作られているので、消さないと reload のあとの 1 件が「保存済みを読んだだけ」なのか「この起動で作った」のか区別できない。
  - `Failed to load brokers` のトーストが `rpc down` を含んで出る。
  - 「No brokers yet」が表示され、未捕捉の例外が出ていない。
  - `fake.calls("GetConnections")` が 0（`restore()` を呼んでいない。呼び出し回数はページの読み込みごとに 0 から数え直す）。
  - `mqtt:lastActiveProfileId` に reload の前と同じ値が残っている。
  - `mqtt:presets` に初期プリセットが 1 件できるまで `expect.poll` で待つ（`onMount` の続きで作るので、トーストの表示より後になることがある）。
  - fake-backend の `GetProfiles` は、seed が未設定なら今と同じ動きをする。
- **フルスタック e2e**: 追加なし。

## 副作用・注意事項

- **起動時にトーストが出るようになる**: 読み込みに失敗したときだけ。Provider は 3 つとも起動時にマウントされるので、RPC 全体が失敗する状況では、対応済みの http と udp の計画の分を合わせて最大 3 つ並ぶ（通知の上限は 5）。
- **失敗時もプリセットの初期作成が走る**: 今は実行されない。
- **プロファイルの保存・削除の失敗の通知との関係**: そちらは main に入っている（`createProfilesState` に渡す `notify`、seed の `saveProfileError`・`deleteProfileError`、`profiles.spec.ts` の失敗のテスト）。この計画はその上に足す。seed の `getProfilesError` は同じ並びに追加し、`GetProfiles` の失敗は `SaveProfile`・`DeleteProfile` と同じ書き方にする。
- **udp の計画との関係**: `fake-backend/types.ts` と `fake-backend/install.ts` を udp の計画も触るが、追加する seed と対象のバインディングが別。

## Git運用

- **ブランチ名**: `fix/mqtt-profile-load-failure`
- **コミット分割方針**:
  1. `docs: MQTT のプロファイル読み込み失敗の修正計画を追加する`
  2. `fix(frontend): MQTT のプロファイル読み込みの失敗を通知する`（`mqtt-provider.tsx`）
  3. `test(e2e): MQTT のプロファイル読み込みの失敗を検証する`（fake-backend の seed と spec）
- **完了条件**: 次がすべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task frontend:test:e2e`（UI の振る舞いと fake-backend を変えるため）
  - `task go:test:integration`・`task go:test:race` は対象外（バックエンドを変えない）
