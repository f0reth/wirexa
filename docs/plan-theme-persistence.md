# 計画: テーマ設定の永続化

## 概要

再起動後もライト/ダークモードの選択が維持されるよう、テーマ設定を `localStorage` に永続化する。

### 現状の問題

`App.tsx` のテーマ状態はメモリ上の SolidJS シグナル（初期値 `"light"` 固定）にのみ保持されているため、アプリを再起動するたびにライトモードにリセットされる。

```typescript
// App.tsx (現状)
const [theme, setTheme] = createSignal<"light" | "dark">("light"); // 常に "light" から開始
```

---

## 方針

フロントエンドの `localStorage` を使用して永続化する。Go バックエンドへの変更は不要。

### なぜ localStorage か

| 観点 | localStorage | Go バックエンド |
|------|-------------|----------------|
| テーマは UI 固有の設定 | 適切 | 過剰 |
| 既存 `local-storage.ts` が利用可能 | 変更不要 | 新規実装必要 |
| バックエンド通信コスト | なし | あり |

---

## アーキテクチャ上の位置づけ

```
Domain      → 変更なし（テーマは UI 関心事のため domain に持ち込まない）
Application → 新規追加: frontend/src/application/ui/theme.ts
Infrastructure → 変更なし（既存の local-storage.ts をそのまま使用）
Presentation → 変更: frontend/src/App.tsx（application 層を使用）
```

既存の MQTT プロファイル永続化（`mqtt-provider.tsx` で `loadFromStorage` / `saveToStorage` を使用）と同じパターンを踏襲する。

---

## 変更ファイル一覧

| ファイル | 種別 | 内容 |
|---------|------|------|
| `frontend/src/application/ui/theme.ts` | **新規作成** | テーマ状態管理ファクトリ |
| `frontend/src/App.tsx` | **変更** | インライン状態管理を `createThemeStore()` に置き換え |

---

## 実装詳細

### 1. `frontend/src/application/ui/theme.ts`（新規作成）

```typescript
import { createEffect, createSignal } from "solid-js";
import {
  loadFromStorage,
  saveToStorage,
} from "../../infrastructure/storage/local-storage";

const THEME_KEY = "app:theme";
type Theme = "light" | "dark";

export function createThemeStore() {
  const initial = loadFromStorage<Theme>(THEME_KEY, "light");
  const [theme, setTheme] = createSignal<Theme>(initial);

  createEffect(() => {
    const current = theme();
    saveToStorage(THEME_KEY, current);
    if (current === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  });

  return {
    theme,
    toggleTheme: () => setTheme((t) => (t === "light" ? "dark" : "light")),
  };
}
```

**ポイント:**
- `loadFromStorage<Theme>(THEME_KEY, "light")` でアプリ起動時に保存済みテーマを読み込む
- `createEffect` がテーマ変更を検知し、localStorage 保存と DOM クラス操作を同期する
- ストレージキーは既存の `"mqtt:lastActiveProfileId"` のネーミング規則に合わせ `"app:theme"` とする

---

### 2. `frontend/src/App.tsx`（変更）

**変更前（該当部分）:**
```typescript
import { createEffect, createSignal } from "solid-js";

// ...

const [theme, setTheme] = createSignal<"light" | "dark">("light");

createEffect(() => {
  if (theme() === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
});

// JSX 内
onThemeToggle={() => setTheme((t) => (t === "light" ? "dark" : "light"))}
```

**変更後（該当部分）:**
```typescript
import { createSignal } from "solid-js"; // createEffect は不要になる
import { createThemeStore } from "./application/ui/theme";

// ...

const { theme, toggleTheme } = createThemeStore();

// JSX 内
onThemeToggle={toggleTheme}
```

**変更点まとめ:**
- `createEffect` のインポートを削除（不要になる）
- テーマ用 `createSignal` とその `createEffect` を削除
- `createThemeStore()` を呼び出して `theme` と `toggleTheme` を取得
- `onThemeToggle` にインライン関数ではなく `toggleTheme` を渡す

---

## 実装手順

1. `frontend/src/application/ui/` ディレクトリを作成する
2. `frontend/src/application/ui/theme.ts` を作成する
3. `frontend/src/App.tsx` を上記の通り変更する
4. `task fmt` を実行してエラーがないことを確認する

---

## 動作確認手順

1. アプリを起動し、ダークモードに切り替える
2. アプリを再起動する → **ダークモードで起動することを確認**
3. ライトモードに切り替える
4. アプリを再起動する → **ライトモードで起動することを確認**
5. 初回起動（localStorage クリア後）でライトモードがデフォルトになることを確認

---

## 影響範囲

- Go バックエンド: **変更なし**
- Wails バインディング: **変更なし**
- CSS / スタイル: **変更なし**
- 他のコンポーネント: **変更なし**（`ProtocolSwitcher` は `theme()` と `onThemeToggle` を同じシグネチャで受け取る）
