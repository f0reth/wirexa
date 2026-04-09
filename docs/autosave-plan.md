# 修正計画書: dirty 比較削除 + 自動保存への移行

作成日: 2026-04-09  
関連調査: [freeze-investigation.md](freeze-investigation.md)

---

## 目的

- `dirty` 機能（変更検知・保存状態アイコン）を完全に削除する
- 代わりに、フィールドが変更されるたびにバックエンドへ自動保存する
- フリーズの根本原因（`JSON.stringify` による大容量データ比較）を除去する

---

## 変更方針

### 削除するもの

| 対象 | 内容 |
|---|---|
| `dirty` メモ | `JSON.stringify` × 2 によるフリーズの直接原因 |
| `savedSnapshot` シグナル | dirty 比較用のスナップショット。不要になる |
| `SavedSnapshot` 型 | 同上 |
| `dirty` の UI 表示 | サイドバーのツリーアイテム横の `●` アイコン |
| `dirtyRequestId` prop | `collection-tree` → `collection-node` → `tree-item-node` の prop チェーン |
| Ctrl+S の dirty ガード | `dirty()` が true のときのみ保存する条件を削除（または Ctrl+S 自体を削除） |

### 追加するもの

| 対象 | 内容 |
|---|---|
| 自動保存 Effect | `http-provider.tsx` に `createEffect` を追加し、リクエスト状態の変化を検知して自動保存 |
| デバウンス | 高頻度な変更（body テキスト入力など）での過剰な保存を防ぐため 500ms デバウンスを挟む |

---

## 自動保存の仕様

- `activeRequestId` が `null`（= 未保存の新規リクエスト）の場合は保存しない
- 保存対象: method / url / headers / params / body / auth / settings すべて
- デバウンス: 500ms（最後の変更から 500ms 後に保存）
- 保存失敗時: `console.error` のみ（UI への通知は行わない）

```
変更発生 → 500ms デバウンス → saveCurrentRequest() → バックエンドへ永続化
```

---

## 変更ファイル一覧

### 1. `frontend/src/presentation/providers/http-provider.tsx`

**削除:**
- `SavedSnapshot` 型定義
- `savedSnapshot` シグナルと `setSavedSnapshot` の呼び出し箇所（`loadRequest`, `saveCurrentRequest` 内）
- `dirty` メモ (`createMemo`)
- `RequestContextValue` インターフェースから `dirty: Accessor<boolean>`

**追加:**
- デバウンス付き自動保存 Effect (`createEffect` + `setTimeout`)
  - `activeRequestId()` が null なら何もしない
  - 全フィールドのシグナルを購読し変更を検知
  - `saveCurrentRequest()` をデバウンス呼び出し

**変更前 (抜粋):**
```tsx
const dirty = createMemo(() => {
  const snap = savedSnapshot();
  if (snap === null || requestState.activeRequestId() === null) return false;
  return (
    ...
    JSON.stringify(requestState.body()) !== JSON.stringify(snap.body) ||
    ...
  );
});
```

**変更後 (抜粋):**
```tsx
// 自動保存: 変更から 500ms 後にバックエンドへ保存
createEffect(() => {
  // シグナルを購読して変化を追跡
  requestState.method();
  requestState.url();
  requestState.headers();
  requestState.params();
  requestState.body();
  requestState.auth();
  requestState.settings();

  const id = requestState.activeRequestId();
  if (!id) return;

  const timer = setTimeout(() => {
    requestState.saveCurrentRequest().catch((err) => {
      console.error("Auto-save failed:", err);
    });
  }, 500);

  onCleanup(() => clearTimeout(timer));
});
```

---

### 2. `frontend/src/presentation/components/http/index.tsx`

**削除:**
- `dirty` のインポートと使用
- `dirty()` による Ctrl+S のガード条件

**変更前:**
```tsx
const { dirty, saveCurrentRequest } = useHttpRequest();
// ...
if ((e.ctrlKey || e.metaKey) && e.key === "s" && dirty()) {
  saveCurrentRequest();
}
```

**変更後:**
- `dirty` / `saveCurrentRequest` のインポートごと削除（Ctrl+S 自体不要になる）

---

### 3. `frontend/src/presentation/components/sidebar/collection-tree.tsx`

**削除:**
- `dirtyRequestId` prop への `requestCtx.dirty()` の渡し

**変更前:**
```tsx
dirtyRequestId={
  requestCtx.dirty() ? requestCtx.activeRequestId() : null
}
```

**変更後:**
- `dirtyRequestId` prop の渡し自体を削除

---

### 4. `frontend/src/presentation/components/sidebar/collection-node.tsx`

**削除:**
- `dirtyRequestId: string | null` prop 定義
- `TreeItemNode` への `dirtyRequestId` 渡し

---

### 5. `frontend/src/presentation/components/sidebar/tree-item-node.tsx`

**削除:**
- `dirtyRequestId: string | null` prop 定義
- `isDirty()` 関数
- `<Show when={isDirty()}>` ブロック（`<Circle>` アイコン含む）
- 再帰的な子 `TreeItemNode` への `dirtyRequestId` 渡し
- `Circle` の import（他で使っていなければ）

---

### 6. `frontend/src/presentation/components/sidebar/sidebar.module.css`

**削除:**
- `.dirtyDot` スタイル定義

---

## 変更しないもの

| ファイル | 理由 |
|---|---|
| `frontend/src/application/http/request.ts` | `saveCurrentRequest()` の実装はそのまま使う |
| `frontend/src/infrastructure/http/client.ts` | IPC 呼び出しはそのまま |
| バックエンド全般 | 変更不要 |

---

## 想定される副作用

| 項目 | 影響 |
|---|---|
| 新規リクエスト（未保存）への変更 | `activeRequestId` が null のため自動保存されない。既存動作と同じ |
| リクエスト切り替え時 | 切り替え前の変更は 500ms 以内に保存される。ただし即時切り替えでは保存されない可能性あり |
| Ctrl+S | 削除。自動保存で代替 |
| dirty アイコン | 削除。UI 上の変更状態表示はなくなる |

---

## コミット分割方針

1. `feat: remove dirty tracking — delete SavedSnapshot, dirty memo, dirtyRequestId prop chain`
2. `feat: add auto-save on request state change with 500ms debounce`
