# HTTPクライアント: 大容量JSON貼り付け時フリーズ調査

調査日: 2026-04-09

## 現象

HTTP クライアントの Body タブで JSON を選択し、約 50MB のデータを貼り付けると UI がフリーズする。

---

## 根本原因

**フロントエンドの `dirty` メモが、キー入力のたびに 50MB × 2 回の `JSON.stringify` を実行する。**  
これが UI メインスレッドをブロックし、アプリ全体がフリーズする。

---

## 問題箇所の詳細

### 1. `dirty` メモ — 致命的 🔴

**ファイル:** `frontend/src/presentation/providers/http-provider.tsx:134-146`

```tsx
const dirty = createMemo(() => {
  const snap = savedSnapshot();
  if (snap === null || requestState.activeRequestId() === null) return false;
  return (
    requestState.method() !== snap.method ||
    requestState.url() !== snap.url ||
    JSON.stringify(requestState.headers()) !== JSON.stringify(snap.headers) ||
    JSON.stringify(requestState.params()) !== JSON.stringify(snap.params) ||
    JSON.stringify(requestState.body()) !== JSON.stringify(snap.body) ||   // ← ここが致命的
    JSON.stringify(requestState.auth()) !== JSON.stringify(snap.auth) ||
    JSON.stringify(requestState.settings()) !== JSON.stringify(snap.settings)
  );
});
```

**問題:**

- `requestState.body()` のシグナルが変化するたびに `createMemo` が再実行される
- `JSON.stringify(body())` が 50MB の文字列を 2 回（現在値 + スナップショット）シリアライズ
- 1 回の貼り付けで DOM の `onInput` が数百〜数千回発火し、その都度 **100MB 分の文字列操作 + GC 圧力** が発生
- JavaScript はシングルスレッドのため、UI の再描画・イベント処理がすべてブロックされる

---

### 2. textarea の `onInput` — 高 🔴

**ファイル:** `frontend/src/presentation/components/http/request-editor.tsx:166-173`

```tsx
<Textarea
  value={body().content}
  onInput={(e) =>
    setBody({
      ...body(),
      content: e.currentTarget.value,   // ← 毎文字入力で 50MB をコピーしシグナル更新
    })
  }
  onBlur={(e) => {
    if (body().type === "json") {
      setBody({
        ...body(),
        content: formatJsonBody(e.currentTarget.value),  // ← JSON.parse + stringify
      });
    }
  }}
/>
```

**問題:**

- `onInput` は入力のたびに `setBody` を呼び、Solid のシグナルを更新する
- シグナル更新 → `dirty` メモ再計算のチェーンが確定する
- 50MB の貼り付け時は `paste` イベント後に一括更新されるが、その 1 回でも `dirty` メモが実行されれば十分致命的

---

## フリーズの発生フロー

```
1. ユーザーが 50MB JSON を貼り付け
2. textarea の onInput イベント発火
3. setBody({ content: "<50MB 文字列>" }) でシグナル更新
4. Solid の反応システムが dirty メモを再計算
5. JSON.stringify(body())       → 50MB 文字列化
   JSON.stringify(snap.body)    → 50MB 文字列化
   計 100MB のメモリ割り当て + 文字列比較
6. JS メインスレッドが CPU/メモリ圧力でブロック
7. UI 再描画・イベント処理が停止 → フリーズ
```

---

## バックエンドは問題なし ✅

**ファイル:** `internal/infrastructure/http/net_client.go:53-78`

```go
var bodyReader io.Reader
switch req.Body.Type {
case "json":
    bodyReader = strings.NewReader(req.Body.Content)
    contentType = "application/json"
}
httpReq, err := http.NewRequestWithContext(ctx, req.Method, parsedURL.String(), bodyReader)
```

- Go 側は `io.Reader` としてストリーミング処理するため大容量ボディでも問題ない
- Wails IPC 経由の 50MB 転送自体は許容範囲（フリーズより後の話）

---

## 問題箇所まとめ

| ファイル | 行 | 問題 | 深刻度 |
|---|---|---|---|
| `frontend/src/presentation/providers/http-provider.tsx` | 142 | `JSON.stringify(body())` × 2 を毎変更実行 | 🔴 致命的 |
| `frontend/src/presentation/components/http/request-editor.tsx` | 168-173 | `onInput` でシグナル更新（dirty 再計算のトリガー） | 🔴 高 |

---

## 修正方針（参考）

### 1. `dirty` 比較でボディの内容を直接比較しない

50MB の body は `JSON.stringify` せず、**参照比較または長さ比較などで早期リターン**する。  
あるいは、body の変更を検知する専用のシグナル（フラグベース）に切り替える。

### 2. `onInput` をデバウンスする

`onInput` で毎回シグナルを更新するのではなく、**デバウンス（例: 300ms）** を挟んで更新頻度を下げる。  
または、制御なし入力（uncontrolled）に切り替えて `onBlur` 時のみシグナル更新する。

### 3. body だけ dirty 判定から外す

body は実際のリクエスト送信まで比較する必要がないため、  
`dirty` の判定から body を除外し、別の軽量なフラグで管理する方法も有効。
