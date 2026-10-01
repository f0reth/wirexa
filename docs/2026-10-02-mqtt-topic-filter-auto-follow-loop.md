# 変更計画書: MQTT Subscribe タブの Messages 表示の不具合修正（フィルター中の Auto 追従ループ・行の高さ・横はみ出し）

## 概要

MQTT の Subscribe タブで、トピックフィルターを使うと画面が壊れるという報告を調べ、UI e2e の偽バックエンドで次の 3 件を再現した。1 が報告の本体で、2・3 は調査中に見つけた別件（フィルターの有無に関係なく起きる）。3 件ともこの計画書で直す。

### 1. フィルター中の Auto 追従が無限ループする

再現手順:

1. `sensors/#` を購読し、`sensors/temp` / `sensors/hum` などを受信する
2. Auto を ON にする
3. フィルターで `sensors/temp` を選ぶ
4. `sensors/hum`（フィルター外）を 1 件受信する

→ `pageerror: Maximum call stack size exceeded`（スタックは solid-js の `runUpdates` ↔ `completeUpdates` の繰り返し）。以後は詳細ペインにフィルター外のメッセージが表示される。

Auto が OFF、Auto ON でも受信が無い、フィルター無し、のいずれかでは起きない（既存の e2e が通っている理由）。

**原因**: 「Auto のとき選択メッセージを末尾に追従させる」effect が 2 つあり、追従先が食い違う。

| 場所 | 追従先 |
| --- | --- |
| `application/mqtt/messages.ts` の `createMessagesState` | **全メッセージ**の末尾 |
| `presentation/components/mqtt/panels/messages-panel.tsx` | **フィルター後**の末尾 |

どちらの effect も追跡スコープの中で `updateConnection(id, (state) => ({ ...state, selectedMessage }))` を呼ぶ。`state` は solid の store プロキシで、スプレッド（`ownKeys`）は接続オブジェクト全体（`$SELF`）への依存を effect に登録するので、どのプロパティが変わっても両方が再実行される。フィルター外のメッセージが届くと互いに選択を書き換え続け、effect の世代ごとに `runUpdates` が 1 段深くなってスタックがあふれる。（依存の付き方は solid-js の store 実装の読解による。ループそのものは再現で確認済み。）

### 2. 仮想リストの行の高さ計測が効かず、行間が空く・重なる

全行が見積もりの 80px 間隔で並ぶ。ペイロードが 1 行の行（実高 65px）は行間が 15px 空き、2 行の行（81px）は 1px 重なる。

**原因**は 2 つ重なっている。

- 行の `ref` で `virtualizer.measureElement(el)` を呼ぶ時点では `data-index` 属性がまだ付いておらず、計測が捨てられる（コンソールに `Missing attribute name 'data-index={index}' on measured element.`）。
- `ref` を遅延させて計測を通しても直らない（試して確認済み）。`@tanstack/solid-virtual` は `count` が変わるたびに `virtualizer.measure()` を呼んで計測キャッシュを全消去するが、表示中の行は再計測されない。初回は正しく並ぶものの、1 件受信した時点で 80px 見積もりに戻る。

### 3. 長い 1 行ペイロードを選択すると画面が横にはみ出す

改行の無い長いペイロード（400 文字など）のメッセージを選択すると、レイアウト全体が右に 48px 伸び、Messages ヘッダーの Clear ボタンなどがウィンドウの外に出る。

**原因**: 詳細ペインの `<pre>` の最小コンテンツ幅が、`overflow: hidden` のパネルを通り抜けて祖先の `.panelGroup`（`components/ui/resizable.module.css`）の固有幅に効く。アプリ最上位の `.panelGroup` は flex アイテムで `min-width: auto` のため、幅 100%（1280px）より縮めなくなり、左のアイコンバー（48px）の分だけはみ出す。`.panelGroup` に `min-width: 0` を当てると収まることを確認済み。

## 変更対象ファイル

対象プロトコルは `mqtt`（3 の CSS は共通 UI 部品）。フロントエンドのみ。バックエンド・バインド API・fake-backend の振る舞いは変えない。

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `frontend/src/application/mqtt/messages.ts` | Frontend / application | 変更 | 【1】`createMessagesState` に `topicFilter` / `setTopicFilter` / `filterTopics` / `visibleMessages` を追加。追従 effect の追従先を `visibleMessages()` の末尾に変更。選択肢から消えたフィルターを解除する effect を panel から移す |
| `frontend/src/application/mqtt/connections.ts` | Frontend / application | 変更 | 【1】`updateConnection` の本体（接続の読み取りと updater の呼び出し）を `untrack` で包む。理由をコメントに書く |
| `frontend/src/presentation/providers/mqtt-provider.tsx` | Frontend / presentation (providers) | 変更 | 【1】`MessagesContextValue` に上記 4 つを追加（値は `...msgState` でそのまま流れる） |
| `frontend/src/presentation/components/mqtt/panels/messages-panel.tsx` | Frontend / presentation (components) | 変更 | 【1】ローカルのフィルター signal・memo・フィルター解除 effect・選択追従 effect を削除し、context の値を使う。【2】行を固定高にし、`measureElement` オプションと行の `ref` を削除、`estimateSize` を行高の定数にする |
| `frontend/src/presentation/components/mqtt/messages.module.css` | Frontend / presentation (components) | 変更 | 【2】行のボタンをラッパーの高さいっぱいにし（`height: 100%; box-sizing: border-box; overflow: hidden;`）、1 行ペイロードの行も 2 行分の高さに揃える |
| `frontend/src/components/ui/resizable.module.css` | Frontend / 共通 UI 部品 | 変更 | 【3】`.panelGroup` に `min-width: 0; min-height: 0;` を追加 |
| `frontend/src/application/mqtt/messages.test.ts` | Frontend / application（テスト） | 変更 | 【1】フィルターと追従の再発防止テスト |
| `frontend/src/application/mqtt/connections.test.ts` | Frontend / application（テスト） | 変更 | 【1】`updateConnection` が呼び出し元の effect に依存を足さないことのテスト |
| `frontend/e2e/ui/mqtt/messages.spec.ts` | e2e (UI) | 変更 | 【1】【2】【3】の再発防止テストを追加 |

## 実装方針

### 1-a. フィルター状態と追従を application 層に集約する（`messages.ts`）

`createMessagesState(activeConnection, updateConnection)` の引数は変えない（購読は `activeConnection()?.subscriptions` から読める）。

```ts
const [topicFilter, setTopicFilter] = createSignal("");
const filterTopics = createMemo(() =>
  collectFilterTopics(activeConnection()?.subscriptions ?? [], messages()),
);
const visibleMessages = createMemo(() =>
  filterMessagesByTopic(messages(), topicFilter()),
);

// 選択肢から消えたフィルターは解除する（panel から移動）
createEffect(() => {
  const filter = topicFilter();
  if (filter && !filterTopics().includes(filter)) setTopicFilter("");
});

// autoFollow が true のとき selectedMessage を「表示中の一覧」の末尾に追従させる
createEffect(() => {
  const visible = visibleMessages();
  if (!autoFollow() || visible.length === 0) return;
  const last = visible[visible.length - 1];
  const connId = activeConnection()?.connectionId;
  if (!connId) return;
  if (untrack(selectedMessage) !== last) {
    updateConnection(connId, (state) => ({ ...state, selectedMessage: last }));
  }
});
```

- 追従の書き手が 1 つになるので、追従先の食い違いが構造的に起きなくなる。
- フィルター中に一致するメッセージが 1 件も無いときは選択を変えない（現状の panel 側 effect と同じ）。
- フィルター無しのときは `visibleMessages() === messages()`（`filterMessagesByTopic` が同じ参照を返す）ので、既存の振る舞いと既存テストはそのまま。
- フィルターは現状どおり「接続ごと」ではなくパネルで 1 つの値とする（`ConnectionStateExt` は変えない）。

### 1-b. `updateConnection` の読み取りを `untrack` する（`connections.ts`）

```ts
// updateConnection は書き込みなので、呼び出し元の effect に依存を足さない。
// updater は store プロキシをスプレッドすることが多く、追跡したままだと接続全体への
// 依存が付いて、無関係なプロパティの変更でも effect が再実行されてしまう。
untrack(() => {
  const existing = connections[connId];
  if (!existing) return;
  setConnections(connId, updater(existing));
});
```

updater の呼び出しだけでなく `connections[connId]` の読み取り（接続の追加・削除への依存が付く）も `untrack` の中に入れ、関数全体で依存を足さないようにする。`updateConnection` は書き込みであり、呼び出し元の effect に依存を足す理由が無い。1-a だけでもループは止まる見込みだが（追従 effect が 1 回余分に再実行されて収束する）、1-b で他の呼び出し元も同じ事故から守る。

### 1-c. panel は表示だけにする（`messages-panel.tsx`）

`useMqttMessages()` から `visibleMessages` / `topicFilter` / `setTopicFilter` / `filterTopics` を受け取り、`<select>`・仮想リスト・スクロール追従（`virtualizer.scrollToIndex`、表示の責務なので残す）に使う。`useMqttSubscribe()` と `collectFilterTopics` / `filterMessagesByTopic` の import は不要になる。

### 2. 行を固定高にして計測をやめる（`messages-panel.tsx` / `messages.module.css`）

動的計測は solid アダプタが受信のたびにキャッシュを消すため、最大 5000 件が流れ続けるこの一覧では安定させにくい（消されるたびに表示中の行を測り直す回避策は、画面外の行が見積もりに戻ってスクロール位置が揺れる）。行の高さの差はペイロードのプレビューが 1 行か 2 行かだけなので、**全行を 2 行分の固定高に揃え、計測そのものをやめる**。

- panel に行高の定数（例: `const MESSAGE_ROW_HEIGHT = 81;`、今の 2 行ペイロードの行と同じ高さ）を置き、`estimateSize: () => MESSAGE_ROW_HEIGHT` にする。`measureElement` オプションと行の `ref`（`virtualizer.measureElement`）は削除する。
- 行のラッパーに `height: ${virtualItem.size}px` を style で与え、`.messageItem` は `height: 100%; box-sizing: border-box; overflow: hidden;` にする。行の箱の高さが常に仮想リストの計算と一致するので、行間は空かない。CSS 側の寸法（すべて `rem`）と定数（px）がずれて中身が箱より高くなった場合は、`overflow: hidden` で切り落として次の行に重ならないようにする。
- `.messagePayload` は 2 行クランプのまま。ボタンが高さいっぱいに伸びるので、1 行ペイロードの行は下に 1 行分の余白ができる。

これで `Missing attribute name 'data-index'` の警告も出なくなる（`data-index` は e2e の位置検査に使うので残す）。

### 3. `.panelGroup` を縮められるようにする（`resizable.module.css`）

```css
.panelGroup {
  display: flex;
  height: 100%;
  width: 100%;
  /* 中身の最小コンテンツ幅（長い 1 行の <pre> など）で flex の親からはみ出さないようにする */
  min-width: 0;
  min-height: 0;
}
```

個別の `<pre>` ではなく共通部品で直すのは、同じ構造（リサイズパネルの中の `<pre>`）が HTTP のレスポンス表示などにもあり、原因が `.panelGroup` 側にあるため。`<pre>` 自体は `overflow: auto` なので、収まった幅の中で横スクロールできる。

### 依存方向

- application（`messages.ts` / `connections.ts`）は `solid-js` と `domain/mqtt/topic` だけに依存したまま。`infrastructure/` も `wailsjs/` も import しない。
- 新しい外部作用のポートは無い。注入箇所（`mqtt-provider.tsx`）は `MessagesContextValue` の型を広げるだけ。
- `presentation/components/` は provider 経由で application の値を使うだけで、`infrastructure/` を import しない。

## 永続化への影響

なし（フィルターはメモリ上の signal のまま。localStorage にも保存しない）。

## コード生成

不要（Go の構造体・ハンドラ・イベントは変更しない）。

## テスト方針

### フロント ユニット

`application/mqtt/messages.test.ts`（既存の `setupMessages` ハーネスに `subscriptions` を渡して使う）

- **【1】再発防止の本体**: Auto ON・フィルター `sensors/temp` の状態で `sensors/hum` を push しても例外にならず、`selectedMessage` が `sensors/temp` の末尾のままで、`updateConnection` の呼び出し回数が増え続けないこと。ハーネスの `updateConnection` は `untrack` していない素の実装なので、1-b に頼らず 1-a だけで収束することを確かめられる。application 側の追従先が全メッセージの末尾に戻ると、このテストが落ちる。ただしこのハーネスは `createMessagesState` しか作らないので、panel 側に追従 effect が復活する回帰（書き手が 2 つに戻る）は検出できない。そちらは UI e2e の【1】で検出する。
- フィルター中に一致するメッセージを push すると、それが選択されること。
- フィルターを解除すると全体の末尾に追従し直すこと。
- フィルターに一致するメッセージが 0 件なら選択を変えないこと。
- `filterTopics` が購読トピックとワイルドカード一致の実トピックを返すこと、`visibleMessages` がフィルター無しで `messages()` と同じ参照を返すこと。
- 購読が外れて選択肢から消えたフィルターは `""` に戻ること。

`application/mqtt/connections.test.ts`

- `createEffect` の中で `updateConnection(id, (s) => ({ ...s, selectedMessage: … }))` を呼んだあと、別のプロパティ（例: `isScanning`）を更新しても、その effect が再実行されないこと（`untrack` を外すと落ちる）。
- 同じ effect が、別の接続を追加しても再実行されないこと（`connections[connId]` の読み取りを `untrack` の外に出すと落ちる）。

### UI e2e（`frontend/e2e/ui/mqtt/messages.spec.ts`）

今の fixture は pageerror を検査していないので、追加する各テストの中で `page.on("pageerror")` を集め、最後に 0 件であることを確かめる。

- **【1】Auto + フィルター + フィルター外の受信**: `sensors/#` を購読 → `sensors/temp` と `sensors/humidity` を受信 → Auto ON → `sensors/temp` でフィルター → `sensors/humidity` を 1 件 emit → 一覧に出ないこと・詳細ペインが `sensors/temp` のままなこと → `sensors/temp` を 1 件 emit → 一覧と詳細の両方に出ること（追従が生きている）。
- **【2】行が隙間なく並ぶ**: 1 行ペイロードと長いペイロードを混ぜて受信 → `[data-index]` の行を index 順に並べ、隣り合う行の「上端 − 前の行の下端」がすべて 0（±1px）であること。さらに 1 件受信した後、フィルターを切り替えた後にも同じ検査をする（初回だけ正しく、受信後に崩れるのが今の壊れ方なので、受信後の検査が要点）。
- **【3】長い 1 行ペイロードで横にはみ出さない**: 改行の無い 400 文字のペイロードを受信して選択 → `document.documentElement.scrollWidth` が `window.innerWidth` 以下で、Clear ボタンがビューポート内にあること。

3 件とも、修正前のコードで落ちることを確認してから修正を入れる。

### Go ユニット / Go 統合 / フルスタック e2e

変更なし。

## 副作用・注意事項

- **フィルターの寿命は実質変わらない。** 今は `MessagesPanel` のローカル signal だが、Subscribe ↔ Publish のタブ切り替え（`mqtt/index.tsx`）もプロトコルの切り替え（`App.tsx`）も `display` を切り替えるだけで panel を破棄しないので、今でもフィルターは残る。panel が破棄されるのはアクティブな接続が無くなったとき（`mqtt/index.tsx` の `<Show when={activeConnectionId()}>`）だけで、provider に移した後もそのときは `filterTopics()` が空になり、フィルター解除の effect が `""` に戻す。接続を切り替えたときにフィルターが引き継がれ、切り替え先の選択肢に無ければ解除されるのも今と同じ。
- **一覧の見た目が変わる。** ペイロードが 1 行のメッセージの行が 65px → 81px になる（今は計測が効かず 80px 間隔で並んでいるので、行の位置はほぼ変わらず、15px の隙間が行の中の余白に変わる）。1 画面に並ぶ件数は今と同じ。プレビューを 1 行に減らして全行を 65px に詰める案もあるが、表示する情報が減るので採らない。
- 行高の定数（81px）はルートのフォントサイズ 16px を前提にした値（行の中の寸法はすべて `rem`）。アプリはルートのフォントサイズを変えていない（`index.css` に指定が無く、変更する設定も無い）ので、通常はずれない。WebView 側の既定フォントサイズが大きい環境では中身が 81px を超えるが、`.messageItem` の `overflow: hidden` でペイロードのプレビューの下側が切れるだけで、次の行には重ならない。フォントサイズへの追随（定数を `rem` から計算する）は、必要になった時点で対応する。
- `updateConnection` の `untrack` は全呼び出し元に効く。updater の中の読み取りに依存して再実行されることを期待している effect は見当たらない。
- **`.panelGroup` の変更は全画面に効く**（HTTP / UDP / OpenAPI / MQTT Publish とアプリ最上位の分割）。縮められるようになるだけなので、今はみ出していない画面の見た目は変わらない想定。既存の UI e2e 全体（`task frontend:test:e2e`）で確かめる。

## Git運用

- **ブランチ名**: `fix/mqtt-messages-panel-display`
- **コミット分割方針**（不具合ごとに、テストと修正を同じコミットに入れる）:
  1. `fix(mqtt): トピックフィルターと Auto 追従を application 層の 1 つの effect にまとめる`（`messages.ts`、`mqtt-provider.tsx`、`messages-panel.tsx`、`messages.test.ts`、e2e の【1】）
  2. `fix(mqtt): updateConnection が呼び出し元の effect に依存を足さないようにする`（`connections.ts`、`connections.test.ts`）
  3. `fix(mqtt): メッセージ一覧の行を固定高にして行間のずれを無くす`（`messages-panel.tsx`、`messages.module.css`、e2e の【2】）
  4. `fix(frontend): リサイズパネルが長い 1 行の内容で親からはみ出さないようにする`（`resizable.module.css`、e2e の【3】）
- **完了条件**: すべて通ってから main へマージする
  - 常に: `task format` → `task lint` → `task test`
  - 変更内容に応じて: `task frontend:test:e2e`（UI の振る舞いと共通 UI 部品を変えるため必須）
  - 対象外: `task go:test:integration` / `task go:test:race`（Go は変更しない）
  - 任意: `task frontend:test:e2e:fullstack`
