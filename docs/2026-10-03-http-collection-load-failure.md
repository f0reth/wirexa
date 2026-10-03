# 変更計画書: HTTP の起動時のコレクション読み込みの失敗を通知する

## 概要

`http-provider.tsx` の `onMount` は `await collectionsState.refreshCollections()` の失敗を受け止めていない。`refreshCollections` は `GetCollections`・`GetRootItems`・`GetSidebarLayout` を順に呼ぶ。Go 側はどれも実際にはエラーを返さない（`GetSidebarLayout` は戻り値に `error` を持つが、読み込みに失敗しても空のレイアウトで突合して `nil` を返す）。しかし RPC の Promise 自体は失敗し得る（バインディングが無い、Wails のランタイムの不調など）。失敗すると次のようになる。

- 未捕捉の rejection になり、画面には何も出ない。サイドバーは「No collections yet」を表示し、コレクションが 1 件も無いように見える。
- 後続の `restoreActiveRequest()` が実行されない。

この計画では、失敗を通知する。失敗時に `restoreActiveRequest()` を実行しない挙動は変えない。mqtt・udp の同じ問題は別の計画で扱う（`docs/2026-10-03-mqtt-profile-load-failure.md`、`docs/2026-10-03-udp-target-load-failure.md`）。

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `frontend/src/presentation/providers/http-provider.tsx` | Presentation | 変更 | `onMount` で `refreshCollections()` の失敗を通知し、失敗時は `restoreActiveRequest()` を呼ばない |
| `frontend/e2e/fake-backend/types.ts` | e2e | 変更 | seed に `getSidebarLayoutError` を追加する |
| `frontend/e2e/fake-backend/install.ts` | e2e | 変更 | `GetSidebarLayout` が上の seed で失敗する |
| `frontend/e2e/ui/http/tree.spec.ts` | e2e | 変更 | 読み込みの失敗のテストを追加する |

バックエンドと application 層は変更しない。

## 実装方針

### 通知する場所

合成ルートの `onMount` で受け止める。

```ts
// 起動時のシーケンスはここに集約する（表示中のサイドバーに依存させない）。
onMount(async () => {
  // 読み込めなかったときは、途中まで読めた一覧からアクティブリクエストを復元しない。
  const loaded = await notifyOnError(
    notify,
    "Failed to load collections",
    collectionsState.refreshCollections,
  ).then(
    () => true,
    () => false,
  );
  if (loaded) restoreActiveRequest();
});
```

`notifyOnError` は `application/ui/guard` から import する。`notify` は既に `createCollectionsState` に渡しているものと同じ。`http-provider.tsx` が `application/ui/guard` を import するのは初めてだが、同じ `application/ui` の `notifications` は既に import している。依存方向は presentation → application のままで、新しい層への依存は増えない。

### `refreshCollections` の中で通知しない理由

`createCollectionsState` の `refreshCollections` は今のまま例外を伝える。

- `createCollection`・`deleteCollection` などは、`notifyOnError`・`runGuarded` の中で `refreshCollections` を呼ぶ。`refreshCollections` 自身が通知すると、操作後の再読み込みの失敗で通知が 2 つ出る。
- `collections.test.ts` の「propagates a getCollections rejection from refreshCollections」が、ガードしないことを検証している。

### 失敗時に `restoreActiveRequest()` を呼ばない理由

`refreshCollections` は 3 つの RPC を順に呼ぶので、途中で失敗すると一部だけ読めた状態になる。例えば `GetCollections` が成功して `GetSidebarLayout` が失敗すると、`collections` は入っているのに、サイドバー（`sidebarLayout` から描画する）は空になる。この状態で復元すると、サイドバーに無いリクエストがエディターに開く。

復元しなくても保存値は残る。`wirexa:http:activeRequest` を消すのは、一度リクエストを選んでから外したときだけ（`activeRequestWasSet`）。次回の起動で読み込めれば、選択中のリクエストが復元される。

### 保存データを壊さないこと

- `wirexa:http:activeRequest`: 上のとおり残る。
- `wirexa:http:expandedFolders`: 存在しない ID を消す `pruneExpandedIds` は、`GetCollections` が成功した直後に動く（`GetRootItems`・`GetSidebarLayout` より前）。
  - `GetCollections` が失敗したときは動かず、書き換わらない。
  - `GetCollections` が成功して後続の RPC が失敗したときは動く。消えるのは返ってきた一覧に無い ID だけで、読み込みが最後まで成功したときと同じ結果になる。失敗したせいで余分に消えることはない。

失敗したあとに次の操作をすると、その操作が `refreshCollections` を呼び直す。それが成功すれば一覧は戻る（アクティブリクエストの復元は起動時の 1 回だけで、ここでは行わない）。

- コレクションの作成・削除・名前変更
- フォルダ・リクエストの追加
- アイテムの名前変更・削除・移動（`moveItem`、`moveItemToSidebar`）

次の操作は `refreshCollections` を呼ばないので、一覧は戻らない。

- リクエストの内容（URL・ボディなど）の保存。`saveCurrentRequest` は `afterSave` 経由で `patchRequest` を呼び、手元の一覧を書き換えるだけ。
- サイドバーの並べ替え（`moveSidebarEntry`）。`GetSidebarLayout` だけを呼び直す。

### やらないこと

- 自動の再試行。Wails の RPC は同一プロセス内の呼び出しで、ネットワーク越しの呼び出しのような一過性の失敗は想定しにくい（失敗の頻度や原因を計測した結果ではない）。
- `logger` への出力。`runGuarded`・`notifyOnError` を使うほかの箇所と同じく、通知だけにする。

### 範囲に含めないもの

- 通知が消えたあとも「No collections yet」と表示されること。`collectionsLoaded` は今どの画面も参照していない。これを使って一覧の空状態と読み込み失敗を表示で区別する変更は、3 プロトコル共通の UI の話なので別に扱う。
- `refreshCollections` が途中で失敗したときに、読めた分を巻き戻すこと。
- バックエンドが起動時に読めなかったコレクションのファイルをスキップしたこと（`JSONStore.Load`）。この場合も `GetCollections` は成功し、スキップされたコレクションを除いた一覧を返すので、この計画の通知では分からない。そのコレクションの ID は `pruneExpandedIds` が `wirexa:http:expandedFolders` から消す（今も起きる）。

## 永続化への影響

なし。localStorage のキーと値の形は変えない。読み込み失敗時に `wirexa:http:activeRequest` を消さないことは、今の挙動と同じ。

## コード生成

不要。

## テスト方針

- **Go ユニット / Go 統合**: 変更なし。
- **フロント ユニット**: 追加なし。変更は Provider の `onMount` だけで、application 層の振る舞いは変わらない。`refreshCollections` が失敗を伝えることは既存のテストが検証している。
- **UI e2e**（`e2e/ui/http/tree.spec.ts`、seed は `{ collections: [{ id, name, requests: [{ id, name, url }] }], getSidebarLayoutError: "rpc down" }`）:
  - 失敗させるのは最後の `GetSidebarLayout` にする。最初の `GetCollections` を失敗させると `collections` が空になり、誤って `restoreActiveRequest()` を呼んでもリクエストが見つからず何も起きないので、復元していないことを検証できない。`GetSidebarLayout` の失敗なら `collections` は入っているので、復元してしまえば URL の入力欄に値が入る。
  - seed のリクエストには空でない `url` を付ける（省くと空文字になり、復元されても入力欄は空のままで区別できない）。
  - fixture は `goto` を済ませてからテストに入るので、`page.on("pageerror", ...)` を登録してから `page.reload()` し、起動時の失敗をもう一度起こして検証する。
  - `Failed to load collections` のトーストが `rpc down` を含んで出る（`HttpProvider` は起動時にマウントされるので、HTTP の画面に切り替える前に出る）。
  - HTTP に切り替えると「No collections yet」が表示され、未捕捉の例外が出ていない。
  - reload の前に `localStorage` の `wirexa:http:activeRequest` に seed のリクエストを指す値（`createActiveRequestStorage` の保存形式）を入れておき、reload のあとも同じ値が残っている。URL の入力欄は空のまま（復元していない）。
  - fake-backend の `GetSidebarLayout` は、seed が未設定なら今と同じ動きをする。
- **フルスタック e2e**: 追加なし。

## 副作用・注意事項

- **起動時にトーストが出るようになる**: 読み込みに失敗したときだけ。Provider は 3 つとも起動時にマウントされるので、RPC 全体が失敗する状況では最大 4 つ並ぶ（通知の上限は 5）。内訳は、この計画と mqtt・udp の計画の 3 つに、UDP のリスナー復元が今も出している `Failed to restore listeners` の 1 つ。
- **失敗時の後続の挙動は変わらない**: 今も失敗時は `restoreActiveRequest()` が実行されない。変わるのは、未捕捉の rejection が通知になることだけ。
- **ほかの計画との関係**: `fake-backend/types.ts` と `fake-backend/install.ts` は mqtt・udp の計画も触るが、追加する seed と対象のバインディングは別になる。`install.ts` は別々のハンドラを触るので重ならない見込み。`types.ts` は `FakeSeed` に 1 行ずつ足すので、同じ位置（末尾など）に足すと競合する。各プロトコルの既存の seed の隣に足す。マージの順序に依存はない。

## Git運用

- **ブランチ名**: `fix/http-collection-load-failure`
- **コミット分割方針**:
  1. `docs: HTTP のコレクション読み込み失敗の修正計画を追加する`
  2. `fix(frontend): HTTP のコレクション読み込みの失敗を通知する`（`http-provider.tsx`）
  3. `test(e2e): HTTP のコレクション読み込みの失敗を検証する`（fake-backend の seed と spec）
- **完了条件**: 次がすべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task frontend:test:e2e`（UI の振る舞いと fake-backend を変えるため）
  - `task go:test:integration`・`task go:test:race` はローカルでは実行しない（バックエンドを変えない）。CI は変更範囲によらず両方を実行するので、CI が通ることは必要
