---
name: clean-arch-review
description: クリーンアーキテクチャに違反していないかレビュー
disable-model-invocation: true
---

Wails + Go + SolidJS で構成されたこのプロジェクトのクリーンアーキテクチャ準拠状況をレビューしてください。

## プロジェクト構造

### バックエンド（`internal/`）

| ディレクトリ | 役割 |
|---|---|
| `domain/` | ビジネスロジック・型定義・ポートインターフェース（`http/`・`mqtt/`・`udp/` の3モジュール） |
| `application/` | ユースケースサービス（`http/`・`mqtt/`・`udp/`） |
| `infrastructure/` | 外部実装（Pahoクライアント、net/http、UDPソケット、JSONストレージ） |
| `adapters/` | Wailsバインディング（`*_handler.go`）。ハンドラはドメイン型を直接送受信する薄いパススルー。`*_dto.go` はドメインに対応物の無い**アダプタ固有の入力型のみ**（例: `log_handler.go` の `LogEntry`） |
| `integration/` | 統合テスト（レビュー対象外） |
| `testutil/` | テストヘルパー（レビュー対象外） |

### フロントエンド（`frontend/src/`）

| ディレクトリ | 役割 |
|---|---|
| `domain/` | ビジネスモデル・型定義（`http/`・`mqtt/`・`udp/`・`openapi/`・`ui/`） |
| `application/` | ユースケース・状態管理（`http/`・`mqtt/`・`udp/`・`openapi/`・`shared/`・`ui/`） |
| `infrastructure/` | 外部サービス連携（`http/`・`mqtt/`・`udp/`・`openapi/`・`logger/`・`storage/`） |
| `presentation/` | UIコンポーネント・プロバイダー（`components/`・`providers/`・`constants/`・`utils/`） |
| `components/` | 汎用UIコンポーネント（`presentation/components/` との違いに注意） |
| `config/` | 設定定数 |
| `lib/` | 汎用ユーティリティ |
| `shared/` | 複数レイヤーから使われる共通コード |

### 依存方向ルール（すべて内向き）

```
# バックエンド（コンパイル時の import 方向）
adapters → domain ← application, infrastructure

# フロントエンド
presentation → application → domain ← infrastructure
```

- `domain` は他レイヤーに依存してはいけない
- `infrastructure` は `domain` のポート（インターフェース）を実装する
- `adapters` はビジネスロジックを持たず `application` のユースケースに委譲する。
  委譲先は adapters が自分で定義したインターフェース（ユースケースの入力ポート）で、
  `application` を import しない。`application` のサービスはこれを構造的に満たし、
  その検証は合成ルート `app.go` で行う。`domain` には出力ポートだけを置く。
  **型については重複 DTO 層を撤廃し、ドメイン型を単一の真実源として RPC 境界に直接公開する**
  （ハンドラがドメイン型を引数・戻り値に取る）。adapters が `domain` の型を
  参照するのは正しい依存方向（内向き）であり違反ではない。ドメインと同型の RPC 用 DTO を
  adapters に再定義して手書き変換するのは**禁止**（同期漏れバグの温床）。
- この禁止は adapters の RPC DTO に限る。永続化形式は infrastructure のリポジトリにある
  `storedXxx` DTO が持ち、ドメイン型の json タグ（RPC の配線形式）から独立させる。stored DTO は
  どの階層でも domain 型を埋め込まない。同期漏れは、`testutil.Populate` で全フィールドを埋めた
  往復テストと `testutil.AssertNoTypesFrom` で防ぐ。

### 型契約の単一真実源（重要）

- Go ドメイン型（`internal/domain/<proto>/types.go`、json タグ付き）が配線型の正。
  ハンドラがこれを直接公開し、`wails generate module` で `frontend/wailsjs/go/models.ts`
  （生成物）に反映される。生成物はコミット対象で、CI のバインディング鮮度チェック
  （再生成して差分が出たら落ちる）が再生成忘れを検知する。
- フロント `domain/<proto>/types.ts` はユニオン型・型ガードで UI に意味付けを足す層。
  生成型 → フロントドメイン型の変換は `infrastructure/<proto>/client.ts` でスプレッド素通し＋
  ユニオン項目の検証で行い、プリミティブ項目の追加が黙って落ちないようにする。
- 名前付き文字列型（例 `udpdomain.PayloadEncoding`）を**メソッド引数**に直接使うと Wails が
  models.ts に型を生成せずバインディングが壊れるため、引数は `string` で受けて内部変換する
  （構造体フィールド経由なら `string` に展開され問題ない）。

## レビュー観点

実際のコードを読んでから判断すること（憶測禁止）。

### 必須チェック項目

1. **依存方向の違反** — 上記ルールに反する `import` / 参照をすべて列挙（ファイル名・行番号つき）
2. **レイヤー責務の混在** — 例: `domain` にインフラ依存が入っている、`application` にUI処理が含まれている
3. **アンチパターン検出**:
   - _Fat Application Service_ — ビジネスロジックが `application` に漏れており `domain` が薄い
   - _Anemic Domain_ — `domain` が型定義のみでロジックが皆無
   - _循環依存_ — パッケージ間の循環参照
   - _フロント/バックの責務重複_ — 同じロジックが両側に実装されている
4. **曖昧なディレクトリの扱い** — `shared/`・`lib/`・`config/`・`components/` がレイヤーをまたいだ依存を作っていないか
5. **将来の破綻リスク** — 正誤判定より「機能追加・スケール時に壊れやすい箇所」の指摘を優先

## 出力形式

`docs/clean-arch-review.md` に以下の構成でMarkdownを出力してください。

```
# クリーンアーキテクチャ レビュー

## 問題点一覧

| 深刻度 | ファイル | 内容 |
|--------|----------|------|
| 高     | ...      | ...  |

## 問題の詳細

### [問題タイトル]
- **該当箇所**: `ファイルパス:行番号`
- **違反内容**: 何がどのルールに違反しているか
- **理由**: なぜ問題なのか

## 理想的な構成の提案

## 改善優先度
```
