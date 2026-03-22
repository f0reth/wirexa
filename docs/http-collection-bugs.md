# HTTP コレクションツリー バグ調査レポート

## 概要

HTTPクライアントのコレクションツリーで報告されている以下の不具合について、コードを調査した。

- 2個目以降のトップレベルフォルダが表示されない
- フォルダ内にサブフォルダ・ファイル（リクエスト）を作成できない

---

## Bug 1: リクエストアイテムの `children` が `null` → TypeError クラッシュ（最重大）

### 場所

- `internal/application/http/collection_service.go:143-176`（`AddRequest`）
- `frontend/src/infrastructure/http/client.ts:66-74`（`fromWailsTreeItem`）

### 原因

`AddRequest` でリクエスト `TreeItem` を生成する際に `Children` フィールドを初期化していない。

```go
// collection_service.go:147-153
item := domain.TreeItem{
    Type:    domain.ItemTypeRequest,
    ID:      req.ID,
    Name:    req.Name,
    Request: &req,
    // Children が未初期化 → Go の nil スライス
}
```

Go の nil スライスは JSON で `null` にシリアライズされる（`json:"children"` タグに `omitempty` がない）。
Wails がこれを JavaScript に渡すと `item.children === null` になる。

フロントエンドの `fromWailsTreeItem` はこれを保護せず直接 `.map()` を呼ぶ:

```ts
// client.ts:70
children: item.children.map(fromWailsTreeItem),  // null.map() → TypeError!
```

### 影響

1. `AddRequest` の Wails RPC が返す `TreeItem` を `fromWailsTreeItem` で変換した時点でクラッシュ
2. コレクションにリクエストが1件でも存在すると、以降の `refreshCollections()` が**常に失敗**する
3. クラッシュは `handleAddFolder` / `handleAddRequest` の `try { } catch { // ignore }` で握り潰されるため、ユーザーにはエラーメッセージが一切表示されない
4. 結果として：リクエストが追加されたあとはフォルダ追加も含め**すべての操作でUIが更新されなくなる**

### 修正方針

**Go 側**：`AddRequest` で `Children` を初期化する。

```go
item := domain.TreeItem{
    Type:     domain.ItemTypeRequest,
    ID:       req.ID,
    Name:     req.Name,
    Request:  &req,
    Children: []domain.TreeItem{},  // 追加
}
```

**TypeScript 側**：`fromWailsTreeItem` にフォールバックを追加する。

```ts
children: (item.children ?? []).map(fromWailsTreeItem),
```

---

## Bug 2: フォルダの展開状態が `refreshCollections()` ごとにリセットされる

### 場所

- `frontend/src/presentation/components/sidebar/tree-item-node.tsx:34`
- `frontend/src/application/http/collections.ts:30-33`

### 原因

`TreeItemNode` コンポーネントのローカル展開状態がデフォルト **`false`（折り畳み）**。

```tsx
// tree-item-node.tsx:34
const [expanded, setExpanded] = createSignal(false);
```

すべての操作（フォルダ追加・リネーム・リクエスト追加など）の後に `refreshCollections()` が呼ばれる:

```ts
// collections.ts:30-33
async function refreshCollections(): Promise<void> {
    const cols = await api.getCollections();
    setCollections(cols);   // 毎回新しいオブジェクト配列で上書き
}
```

SolidJS の `<For>` はオブジェクトの参照が変わると**コンポーネントインスタンスを再生成**する。バックエンドから取得するたびに新しいオブジェクト参照が返るため、全 `CollectionNode` と `TreeItemNode` が毎回破棄・再生成される。
その結果、`expanded` シグナルが毎回 `false` にリセットされる。

### 影響

1. サブフォルダを作成するために親フォルダを展開（クリック）する
2. 「Add Folder」ボタンを押す → バックエンドで正常に作成される
3. `refreshCollections()` が呼ばれ、親フォルダの `TreeItemNode` が再生成 → `expanded = false`（折り畳み）
4. 親フォルダが折り畳まれるため、新しいサブフォルダが**見えなくなる**
5. ユーザーには「サブフォルダが作成できない」と見える（実際にはバックエンドに存在している）

これは「フォルダ内にサブフォルダ・ファイルも作成できない」の主要因。

### 修正方針

**案A（シンプル）**：新規作成したアイテムの親フォルダを自動展開する。
`handleAddFolder` / `handleAddRequest` 内で `setExpandedItems` のような外部シグナルにアイテムIDを登録し、`TreeItemNode` がそれを参照する。

**案B（根本的）**：`collections` シグナルを SolidJS の `createStore` + `reconcile` に変更し、変更のあったノードだけを更新する。これにより展開状態を保持したままツリーを更新できる。

```ts
// 例（案B の一部）
import { createStore, reconcile } from "solid-js/store";
const [collections, setCollections] = createStore<Collection[]>([]);

async function refreshCollections() {
    const cols = await api.getCollections();
    setCollections(reconcile(cols, { key: "id", merge: true }));
}
```

---

## Bug 3: `GetCollections()` でコレクション順序が毎回変わる

### 場所

- `internal/application/http/collection_service.go:44-52`

### 原因

Go のマップ（`map[string]*domain.Collection`）のイテレーション順序は**ランダム**。

```go
func (s *CollectionService) GetCollections() []domain.Collection {
    s.mu.RLock()
    defer s.mu.RUnlock()
    result := make([]domain.Collection, 0, len(s.cache))
    for _, c := range s.cache {  // イテレーション順序が不定
        result = append(result, *c)
    }
    return result
}
```

### 影響

- `refreshCollections()` を呼ぶたびにコレクションの表示順が変わる
- SolidJS の `For` はリスト全体を再生成するため、コレクションが視覚的に「ジャンプ」する
- 2個目のコレクションを作成した際に、順序が変わることで「前のコレクションが消えて新しいものだけになった」ように見える場合がある

### 修正方針

キャッシュとは別に、作成順を管理する順序付きスライスを持つ。または返却時に名前や作成時刻でソートする。

```go
// 修正例：名前でソート
sort.Slice(result, func(i, j int) bool {
    return result[i].Name < result[j].Name
})
```

---

## Bug 4: エラーの無言握り潰し

### 場所

- `frontend/src/presentation/components/sidebar/collection-tree.tsx`（全 `handleXxx` 関数）

### 原因

すべてのイベントハンドラーで例外が無言で無視されている。

```tsx
// collection-tree.tsx の各ハンドラー
} catch {
    // ignore
}
```

### 影響

- Bug 1 のように `fromWailsTreeItem` がクラッシュしても、ユーザーには何も通知されない
- デバッグが極めて困難になる
- 操作が「失敗」したのか「成功したが表示されないだけ」なのか区別できない

### 修正方針

最低限 `console.error` でログを残す。理想的にはトースト通知などでユーザーに伝える。

```ts
} catch (err) {
    console.error("Failed to add folder:", err);
    // 必要に応じてトースト通知
}
```

---

## バグ優先度まとめ

| # | バグ | 優先度 | 影響範囲 |
|---|------|--------|----------|
| 1 | リクエストの `children` が `null` → `refreshCollections()` 常時クラッシュ | **高** | リクエスト追加後の全操作 |
| 2 | フォルダ展開状態が refresh でリセット | **高** | サブフォルダ・ファイルの視認 |
| 3 | `GetCollections()` の非決定的順序 | 中 | コレクションの表示順 |
| 4 | エラーの無言握り潰し | 中 | デバッグ・UX |

---

## 推奨修正順序

1. **Bug 1 を修正**（Go の `AddRequest` で `Children` 初期化 + TS の `fromWailsTreeItem` に null ガード）
   → これだけでリクエスト追加後の全操作クラッシュが解消される

2. **Bug 2 を修正**（Store + reconcile 化 or 展開状態の外部管理）
   → サブフォルダ・ファイルが作成後に正しく表示されるようになる

3. **Bug 3 を修正**（`GetCollections()` のソート）
   → コレクション表示が安定する

4. **Bug 4 を修正**（エラーログ追加）
   → 今後の問題発見が容易になる
