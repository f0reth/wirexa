# 変更計画書: UDP の起動時のターゲット読み込みの失敗を通知する

## 概要

`udp-provider.tsx` の `onMount` は `void targetsState.refreshTargets()` で一覧を読み込み、失敗を受け止めていない。Go の `GetTargets` はエラーを返さないが、RPC の Promise 自体は失敗し得る（バインディングが無い、Wails のランタイムの不調など）。失敗すると未捕捉の rejection になり、画面には何も出ない。サイドバーは「No targets yet」を表示し、ターゲットが 1 件も無いように見える。

この計画では、失敗を通知する。http・mqtt の同じ問題は修正済みで（http: `48d369c`・`e5f471f`、mqtt: `b61462a`・`c95d55e`）、この計画はそれらと同じ形にそろえる。

## 変更対象ファイル

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `frontend/src/presentation/providers/udp-provider.tsx` | Presentation | 変更 | `onMount` で `refreshTargets()` の失敗を通知する |
| `frontend/e2e/fake-backend/types.ts` | e2e | 変更 | seed に `getTargetsError` を追加する |
| `frontend/e2e/fake-backend/install.ts` | e2e | 変更 | `GetTargets` が上の seed で失敗する |
| `frontend/e2e/ui/udp/targets.spec.ts` | e2e | 変更 | 読み込みの失敗のテストを追加する |

バックエンドと application 層は変更しない。

## 実装方針

### 通知する場所

合成ルートの `onMount` で受け止める。

```ts
// 起動時のシーケンスはここに集約する（TargetTree を表示しなくても一覧を読み込む）。
onMount(() => {
  void runGuarded(notify, "Failed to load targets", targetsState.refreshTargets);
});
```

`runGuarded` は `application/ui/guard` から import する（`udp-provider.tsx` には新しい import になる）。`notify` は既に `createTargetsState` に渡しているものと同じ。依存方向は presentation → application のままで、新しい層間依存は増えない。

http・mqtt は読み込みのあとに復元の処理（`restoreActiveRequest`、`connState.restore()`）があるので、`notifyOnError` で成否を受けて分岐している。udp は後続の処理が無いので、分岐せず `runGuarded` で足りる。

### `refreshTargets` の中で通知しない理由

`createTargetsState` の `refreshTargets` は今のまま例外を伝える。

- `saveTarget`・`deleteTarget` は、`notifyOnError`・`runGuarded` の中で `refreshTargets` を呼ぶ。`refreshTargets` 自身が通知すると、保存後の再読み込みの失敗で通知が 2 つ出る。
- `targets.test.ts` の「propagates getTargets failures」が、ガードしないことを検証している。

### 保存データを壊さないこと

読み込みに失敗しても `udp:targetOrder` は書き換わらない。並び順を保存するのは `reorderTargets` だけで、一覧が空のあいだは並べ替えが起きない。失敗したあとにターゲットを保存すると `saveTarget` が `refreshTargets` を呼び直すので、それが成功すれば、保存済みの並び順に含まれるターゲットはその順で戻る。並び順に含まれないターゲットは末尾に並び、その中での順序は `GetTargets` の返す順による（Go の `CachedStore.GetAll` は map を走査するので、一定しない）。これは今の動きと同じで、この計画では変えない。

### やらないこと

- 自動の再試行。http・mqtt の修正でも入れておらず、それにそろえる。再試行で直る失敗がどれだけあるかは確かめていない。
- `logger` への出力。`runGuarded` を使うほかの箇所と同じく、通知だけにする。

### 範囲に含めないもの

- 通知が消えたあとも「No targets yet」と表示されること。一覧の空状態と読み込み失敗を表示で区別する変更は、3 プロトコル共通の UI の話なので別に扱う。

## 永続化への影響

なし。localStorage のキーと値の形は変えない。

## コード生成

不要。

## テスト方針

- **Go ユニット / Go 統合**: 変更なし。
- **フロント ユニット**: 追加なし。変更は Provider の `onMount` だけで、application 層の振る舞いは変わらない。`refreshTargets` が失敗を伝えることは既存のテストが検証している。
- **UI e2e**（`e2e/ui/udp/targets.spec.ts`、seed は `{ udpTargets: [...], getTargetsError: "rpc down" }`）:
  - fixture は `goto` を済ませてからテストに入るので、localStorage の `udp:targetOrder` に空でない並び順（seed のターゲットの ID）を書き、`page.on("pageerror", ...)` を登録してから `page.reload()` し、起動時の失敗をもう一度起こして検証する。
  - `Failed to load targets` のトーストが 1 件だけ、`rpc down` を含んで出る（`UdpProvider` は起動時にマウントされるので、UDP の画面に切り替える前に出る）。
  - UDP に切り替えると「No targets yet」が表示され、未捕捉の例外が出ていない。
  - `udp:targetOrder` の値が reload の前と同じで、`fake.snapshot()` の `udpTargets` に seed のターゲットが残っている。
  - fake-backend の `GetTargets` は、seed が未設定なら今と同じ動きをする。
- **フルスタック e2e**: 追加なし。

## 副作用・注意事項

- **起動時にトーストが出るようになる**: 読み込みに失敗したときだけ。Provider は 3 つとも起動時にマウントされるので、RPC 全体が失敗する状況では http の `Failed to load collections`、mqtt の `Failed to load brokers`、udp の既存の `Failed to restore listeners`（`createUdpReceiveState` が生成時に `GetListeners` を呼ぶ）と合わせて最大 4 つ並ぶ（通知の上限は 5）。
- **http・mqtt の修正との関係**: どちらも main に入っている。`fake-backend/types.ts` と `fake-backend/install.ts` には、それらが足した seed（`getSidebarLayoutError`、`getProfilesError`）の並びに `getTargetsError` を足す。

## Git運用

- **ブランチ名**: `fix/udp-target-load-failure`
- **コミット分割方針**:
  1. `docs: UDP のターゲット読み込み失敗の修正計画を追加する`
  2. `fix(frontend): UDP のターゲット読み込みの失敗を通知する`（`udp-provider.tsx`）
  3. `test(e2e): UDP のターゲット読み込みの失敗を検証する`（fake-backend の seed と spec）
- **完了条件**: 次がすべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task frontend:test:e2e`（UI の振る舞いと fake-backend を変えるため）
  - `task go:test:integration`・`task go:test:race` は対象外（バックエンドを変えない）
