---
name: refactor-assistant
description: コードを安全にリファクタリングし、可読性・保守性・パフォーマンスを改善
disable-model-invocation: true
---

このプロジェクト（Wails + Go + SolidJS）を対象に、既存機能を壊さずにリファクタリング可能な箇所を調査してください。

## 実行手順

**推測・記憶・想像による判断は禁止。** 実際のコードを開いて確認してから判断してください。

1. 以下の調査対象を実際に読む
2. 調査観点に基づいてリファクタリング候補を洗い出す
3. `docs/refactor-report.md` に結果を出力する

## 調査対象

**バックエンド（`internal/`）**

| 層 | 調査パス |
|---|---|
| Domain | `internal/domain/http/`・`internal/domain/mqtt/`・`internal/domain/udp/` |
| Application | `internal/application/http/`・`internal/application/mqtt/`・`internal/application/udp/` |
| Infrastructure | `internal/infrastructure/http/`・`internal/infrastructure/mqtt/`・`internal/infrastructure/udp/` |
| Adapters | `internal/adapters/http_handler.go`・`mqtt_handler.go`・`udp_handler.go`・`openapi_handler.go`・`log_handler.go`・`http_dto.go`・`mqtt_dto.go`・`udp_dto.go` |

**フロントエンド（`frontend/src/`）**

| 層 | 調査パス |
|---|---|
| Domain | `frontend/src/domain/` (http/・mqtt/・udp/・openapi/・ui/) |
| Application | `frontend/src/application/` (http/・mqtt/・udp/・openapi/・shared/) |
| Infrastructure | `frontend/src/infrastructure/` (http/・mqtt/・udp/・openapi/・storage/・logger/) |
| Presentation | `frontend/src/presentation/` |
| Shared | `frontend/src/shared/`・`frontend/src/lib/`・`frontend/src/config/` |

## 調査観点

**Go バックエンド**

- 重複コードの共通化（特に http/mqtt/udp 3モジュール間の類似処理）
- エラーハンドリングの不統一（`fmt.Errorf` とカスタムエラー型の混在など）
- インターフェース・ポートの不必要な肥大化
- 不要なポインタ渡し・値渡しの混在

**TypeScript フロントエンド**

- `application/` 層での状態管理の重複（http/mqtt/udp 間の類似パターン）
- `presentation/` コンポーネントへのロジック漏れ
- 型定義の重複（バックエンドと同じ型をフロント側で再定義していないか）
- Solid.js のリアクティビティを正しく活用していない箇所

**共通**

- 同じロジックがバックエンド・フロントエンド両方に実装されている箇所
- 不使用コード（dead code）

## 禁止事項

- 大規模リライト
- フレームワーク変更
- 挙動変更
- クリーンアーキテクチャの破壊

  依存方向（厳守）: `adapters → application → domain ← infrastructure`
  Domain は他の層に依存してはならない。

## 出力形式

`docs/refactor-report.md` に以下の構成でMarkdownを出力してください。

```
# リファクタリング調査レポート

## 改善候補一覧

| 優先度 | ファイル | 内容 |
|--------|----------|------|
| 高     | ...      | ...  |

## 改善候補の詳細

### [タイトル]
- **該当箇所**: `ファイルパス:行番号`
- **現状**: 何が問題か
- **改善案**: どう直すか
- **期待効果**: 可読性向上 / 重複削減 / パフォーマンス改善 など

## 対応不要と判断した箇所（理由つき）
```
