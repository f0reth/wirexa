# OpenAPI エディター機能 実装計画書

## 概要

MQTT / HTTP / UDP と同じサイドバーに、OpenAPI ファイルの編集・確認機能を追加する。
プレビューは VS Code 拡張機能「OpenAPI (Swagger) Editor」(42Crunch) と同様の表示を目標とする。

---

## 要件整理

| 項目 | 内容 |
|------|------|
| サイドバー追加 | protocol-switcher に「OpenAPI」を追加 |
| ファイル履歴サイドバー | HTTP の CollectionTree と同じ幅・スタイルで、最近開いたファイル名を一覧表示 |
| ドラッグ移動 | ファイル履歴の並び替えを HTTP と同じドラッグ実装で対応 |
| エディター | 左半分：YAML/JSON コードエディター |
| プレビュー | 右半分：VS Code OpenAPI Editor スタイルの表示（エンドポイント一覧・展開・パラメーター） |
| トグルボタン | 右半分のプレビューパネルを開閉するボタン |
| ファイル保存 | `Ctrl+S` でアクティブファイルに上書き保存 |

---

## ライブラリ選定（調査結果）

### コードエディター：CodeMirror 6 ✅（当初案を維持）

| 項目 | 内容 |
|------|------|
| バンドルサイズ | ~88–100KB (gzip) |
| Monaco Editor との比較 | Monaco は ~2.4MB (gzip) で起動コスト大、不採用 |
| SolidJS 統合 | vanilla JS API で DOM ref に直接 mount 可能 |
| 対応言語 | `@codemirror/lang-yaml` + `@codemirror/lang-json` |
| エラー表示 | `@codemirror/lint` 拡張でエディター内インライン表示可能 |

**Wails デスクトップアプリにおいてバンドルサイズが起動時間に直結するため、Monaco は採用しない。**

---

### YAML/JSON パーサー：`yaml`（eemeli/yaml）に変更

当初案の `js-yaml` から変更する。

| ライブラリ | バンドルサイズ | 読み込み速度 | エラー位置情報 | 推奨用途 |
|------------|--------------|------------|--------------|---------|
| `js-yaml` | ~30–40KB | 高速 (504ms/14000行) | あり（簡易） | シンプルなパース |
| `yaml` (eemeli) | ~27KB (全機能) | 中速 (2108ms/14000行) | **詳細** (line/col/range) | **エディター連携向き** ← 採用 |

**採用理由：**
- `yaml` はエラー発生箇所の `line` / `column` / `range` を詳細に返す
- CodeMirror の `@codemirror/lint` とマッピングしやすく、エディター内でエラー位置をハイライト表示できる
- OpenAPI 3.x のパース精度が高い
- バンドルサイズは `js-yaml` より小さい

---

### OpenAPI プレビューレンダラー：RapiDoc（Web Component）

| ライブラリ | gzip | 統合方法 | VS Code 風表示 | 動的更新 |
|------------|------|---------|--------------|---------|
| **RapiDoc** | **<125KB** | Web Component（直接 mount） | ◎ | ◎ |
| Swagger UI | ~1MB | iframe 推奨 | ◎ | △（再初期化が必要） |
| ReDoc | ~350KB | iframe 推奨 | ○ | △ |
| Scalar | ~477KB | iframe/React | ◎ | ○ |
| Stoplight Elements | ~2.81MB | React | ◎ | ○（重すぎ、不採用） |

**RapiDoc を採用する理由：**
- VS Code OpenAPI Editor（42Crunch）と同様のエンドポイント一覧・展開・パラメーター表示が可能
- Web Component として SolidJS に直接組み込み可能（iframe 不要）
- `spec` attribute を更新するだけでリアルタイム再描画される
- 最軽量クラス（<125KB）で起動コストが低い
- ダークモード対応（既存テーマシステムと連携可能）
- `allow-try` を `false` にすれば Try it out は無効化可能

**RapiDoc の表示例（VS Code OpenAPI Editor と同等）：**
```
GET    /users         Get all users
POST   /users         Create a user
GET    /users/{id}    Get a user by ID
  └─ Parameters: id (path, required)
  └─ Responses: 200 OK, 404 Not Found
```

---

### 採用ライブラリ まとめ

```
# 追加パッケージ
codemirror                    # エディターコア (~80KB gzip)
@codemirror/lang-yaml         # YAML シンタックスハイライト (~5KB)
@codemirror/lang-json         # JSON シンタックスハイライト (~3KB)
@codemirror/lint              # インラインエラー表示 (含むcore)
yaml                          # YAML/JSON パーサー (~27KB)
rapidoc                       # OpenAPI プレビュー Web Component (<125KB)

合計追加バンドル: ~240KB (gzip)
```

---

## アーキテクチャ設計

### レイヤー構成

```
domain/openapi/
  types.ts                    # OpenAPI ファイル履歴のドメイン型

application/openapi/
  files.ts                    # ファイル履歴の状態管理（シグナル + localStorage）
  editor.ts                   # エディター内容・パース結果・UI 状態の管理

infrastructure/openapi/
  file-io.ts                  # Wails の OpenFilePicker / ファイル読み書き
  parser.ts                   # yaml (eemeli) でパース + エラー位置情報の変換

presentation/components/
  openapi/
    index.tsx                 # OpenAPI メインレイアウト（左右 Resizable 分割）
    editor-panel.tsx          # 左半分：CodeMirror エディター
    preview-panel.tsx         # 右半分：RapiDoc プレビュー
    preview-toggle.tsx        # プレビュー開閉トグルボタン
  sidebar/
    openapi-file-tree.tsx     # ファイル履歴ツリー（collection-tree.tsx に準拠）
    openapi-file-node.tsx     # ファイルノード（tree-item-node.tsx に準拠）

presentation/providers/
  openapi-provider.tsx        # OpenAPI Context Provider
```

---

## 型定義

```typescript
// domain/openapi/types.ts

export type OpenApiFile = {
  id: string;           // uuid
  name: string;         // ファイル名（表示用）
  path: string;         // 絶対パス
  order: number;        // 表示順（ドラッグ並び替え用）
  lastOpenedAt: string; // ISO8601
};
```

---

## 状態管理

### `application/openapi/files.ts`

- `openApiFiles`: `OpenApiFile[]` — 最近開いたファイル一覧（localStorage 永続化）
- `activeFileId`: `string | null` — 現在選択中のファイル
- `addFile(path, name)` — 履歴に追加（同 path は更新）
- `removeFile(id)` — 履歴から削除
- `moveFile(id, newOrder)` — ドラッグ並び替え
- 最大保持件数：50件（超えた場合は `lastOpenedAt` が古いものを削除）

### `application/openapi/editor.ts`

- `editorContent`: `string` — エディター内のテキスト
- `parsedSpec`: `object | null` — パース済み OpenAPI オブジェクト（RapiDoc に渡す）
- `parseErrors`: `LintDiagnostic[]` — CodeMirror lint 形式のエラー一覧
- `isPreviewing`: `boolean` — プレビューパネルの表示状態
- `setEditorContent(text)` — テキスト更新 → 500ms デバウンスでパース → RapiDoc 更新
- `togglePreview()` — プレビューの開閉

---

## UI 設計

### レイアウト全体

```
┌──────────────────────────────────────────────────────────────────────┐
│ Activity Bar │ File History │         Main Content Area              │
│  (48px 固定) │  Sidebar     │                                        │
│              │  (HTTP と    │  ┌──────────────┬──────────────────┐   │
│  [MQTT]      │  同じ幅・    │  │              │                  │   │
│  [HTTP]      │  スタイル)   │  │  CodeMirror  │   RapiDoc        │   │
│  [UDP]       │              │  │  Editor      │   Preview        │   │
│  [OpenAPI]   │  ファイル名  │  │  (左 50%)    │   (右 50%)       │   │
│              │  一覧        │  │              │                  │   │
│              │  (ドラッグ   │  │  YAML/JSON   │  GET  /users     │   │
│              │   並び替え)  │  │  シンタックス  │  POST /users     │   │
│              │              │  │  ハイライト   │  GET  /users/{id}│   │
│              │  [+ 開く]    │  │              │                  │   │
│              │              │  └──────────────┴──────────────────┘   │
│              │              │                ↑ [◀ トグル]            │
└──────────────────────────────────────────────────────────────────────┘
```

### メインエリアの分割

- `@corvu/resizable` で左右分割（既存 HTTP レイアウトと同パターン）
- 初期比率：50% / 50%
- プレビュー閉時：エディターが幅 100% に展開
- トグルボタン：パネル境界付近に配置（`ChevronLeft` / `ChevronRight` アイコン）

### RapiDoc プレビューの設定

```html
<rapi-doc
  spec="{{ JSON.stringify(parsedSpec) }}"
  render-style="read"
  theme="dark"
  allow-try="false"
  show-header="false"
  primary-color="#6366f1"
/>
```

- `render-style="read"` — VS Code OpenAPI Editor スタイルの読み取り専用表示
- `allow-try="false"` — Try it out 無効（エディターツールなので不要）
- `show-header="false"` — RapiDoc のヘッダー非表示

---

## ファイル操作フロー

### ファイルを開く

```
[+ 開く] ボタン
  → Wails OpenFilePicker（.yaml / .yml / .json フィルター）
  → Go ReadFile(path) → ファイル内容を返す
  → エディターに内容をセット
  → yaml.parse() でパース → RapiDoc に渡す
  → ファイル履歴に追加（localStorage 保存）
```

### ファイルを保存（Ctrl+S）

```
Ctrl+S キー
  → activeFileId から path を取得
  → Go WriteFile(path, editorContent)
```

### 履歴からファイルを開く

```
ファイル名をクリック
  → Go ReadFile(path) → エディターに内容をセット
  → lastOpenedAt を更新
```

---

## Go バックエンド追加 API

```go
// internal/application/openapi_service.go

type OpenApiService interface {
    ReadFile(path string) (string, error)
    WriteFile(path string, content string) error
}
```

Wails バインディング経由でフロントエンドから呼び出す。  
※ OpenAPI のパースはフロントエンド（yaml ライブラリ）で行うため、バックエンドはファイル I/O のみ。

---

## 実装ステップ

### Step 1: バックエンド（Go）

1. `internal/application/openapi_service.go` 作成（ReadFile / WriteFile）
2. `app.go` に Wails バインディング追加

### Step 2: ドメイン・インフラ層（フロントエンド）

3. `domain/openapi/types.ts` 作成（型定義）
4. `application/openapi/files.ts` 作成（ファイル履歴状態管理）
5. `application/openapi/editor.ts` 作成（エディター・パース状態管理）
6. `infrastructure/openapi/file-io.ts` 作成（Wails 連携）
7. `infrastructure/openapi/parser.ts` 作成（yaml パース・エラー変換）

### Step 3: サイドバー

8. `openapi-file-node.tsx` 作成（tree-item-node.tsx に準拠）
9. `openapi-file-tree.tsx` 作成（collection-tree.tsx に準拠、ドラッグ実装含む）
10. `sidebar/index.tsx` に OpenAPI の `<Show>` ブロック追加
11. `protocol-switcher.tsx` に OpenAPI ボタン追加（`FileCode` アイコン）

### Step 4: メインコンテンツエリア

12. `openapi/editor-panel.tsx` 作成（CodeMirror mount・lint 連携）
13. `openapi/preview-panel.tsx` 作成（RapiDoc Web Component）
14. `openapi/preview-toggle.tsx` 作成（トグルボタン）
15. `openapi/index.tsx` 作成（Resizable 左右分割レイアウト）

### Step 5: 統合

16. `presentation/providers/openapi-provider.tsx` 作成
17. `App.tsx` に OpenAPI プロバイダーと表示切り替えを追加

### Step 6: 確認・整形

18. `task fmt` を実行してエラーがないことを確認
19. 動作確認（ファイル開く・編集・プレビュー・ドラッグ並び替え・Ctrl+S 保存）

---

## Git ブランチ・コミット計画

```
ブランチ: feat/openapi-editor

コミット:
  feat: add Go ReadFile/WriteFile API for OpenAPI files
  feat: add OpenAPI domain types and state management
  feat: add OpenAPI file history sidebar with drag reorder
  feat: add CodeMirror editor panel with YAML/JSON lint
  feat: add RapiDoc preview panel with toggle
  feat: integrate OpenAPI into main layout and protocol switcher
```

---

## パフォーマンス懸念点と対策

| 懸念事項 | 対策 |
|---------|------|
| yaml パースが UI をブロックする | 500ms デバウンス + 将来的に WebWorker 化 |
| RapiDoc の初回描画が遅い | 動的 import（OpenAPI 選択時のみ読み込み） |
| 大きな spec ファイルでのエディター動作 | CodeMirror 6 は仮想 DOM 不使用で大ファイルに強い |
| RapiDoc への spec 再渡しのオーバーヘッド | `parsedSpec` が変化した場合のみ attribute 更新 |
