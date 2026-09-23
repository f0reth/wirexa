---
name: plan-app
description: アプリ変更の計画書を作成し、ユーザーの承認を得る
disable-model-invocation: true
---

このプロジェクトに対して、指定された変更の計画書を作成してください。

## 入力

$ARGUMENTS

## 実装開始前の確認

変更内容に曖昧な点・不明な点がある場合は、**計画書を作成する前に必ず質問すること。**
以下のような場合は質問が必要：

- 要件の解釈が複数通りある
- 変更範囲が不明確（どこまで変更すべきか判断できない）
- 既存のクリーンアーキテクチャ設計と矛盾する指示がある
- 前提となる情報が欠けている

複数の疑問点は1つの質問にまとめること。

## アーキテクチャの確認

計画書作成前に、変更が以下の層のどこに影響するかを特定すること：

### バックエンド（`internal/`）

| 層             | パス例                     | 役割                                                                                                    |
| -------------- | -------------------------- | ------------------------------------------------------------------------------------------------------- |
| Domain         | `internal/domain/`         | ビジネスルール・型定義・ポートインターフェース（外部依存なし）。プロトコル別に `http/`・`mqtt/`・`udp/` |
| Application    | `internal/application/`    | ユースケース・Service層（`*_service.go`）。domain のポートのみに依存                                    |
| Infrastructure | `internal/infrastructure/` | MQTT・HTTP・UDP・JSON ファイルストレージの実装（domain ポートの実装）                                   |
| Adapters       | `internal/adapters/`       | Wails RPC ハンドラ（`*_handler.go`）。**ドメイン型を直接送受信する薄いパススルー**                      |

**Adapters に重複 DTO 層を新設してはならない。**
`*_dto.go` はドメインに対応物の無いアダプタ固有の入力型のみ許可される（例: `log_handler.go` の `LogEntry`）。
例外として、名前付き文字列型を引数に直接使うと Wails が型を生成しないため、引数は `string` で受けて内部変換する。

### フロントエンド（`frontend/src/`）

| 層             | パス                           | 役割                                                                                        |
| -------------- | ------------------------------ | ------------------------------------------------------------------------------------------- |
| Domain         | `frontend/src/domain/`         | ユニオン型・型ガードで UI に意味付けする層（配線型の正は生成物）                            |
| Application    | `frontend/src/application/`    | ユースケース・状態管理（フレームワーク非依存）                                              |
| Infrastructure | `frontend/src/infrastructure/` | **`wailsjs/` を import してよい唯一の層。** 変換は素通し（`...wire`）＋ユニオン項目のみ検証 |
| Presentation   | `frontend/src/presentation/`   | SolidJS コンポーネント・プロバイダー                                                        |
| Shared         | `frontend/src/shared/`         | 横断ヘルパー（生成物 `wails-events.ts` を含む）                                             |

コンポーネントから `infrastructure/` を直接 import しない。通知などの外部作用はポート経由で注入する。

**依存の方向（コンパイル時の import 方向）を壊さないこと。**

```
adapters ──────┐
application ───┼──→ domain（インターフェースを定義）
infrastructure ┘
```

- `domain` は他のどのレイヤーにも依存しない。
- `application` / `infrastructure` は `domain` のインターフェースを実装する（依存性逆転）。
- `adapters` は `domain` のインターフェース経由でユースケースを呼び出し、`application` を直接 import しない。
- 具体型の組み立て・注入は合成ルート `app.go` でのみ行う。

### サービス追加時の二段階配線

`app.go` が合成ルート。ハンドラは `NewApp()` で**空のまま生成**し（Wails が `main.go` でバインドするため）、
`initialize()` でサービスを構築して `adapters.SetupXxxHandler(...)` により注入する。
サービスを追加する場合はこのパターンに従い、計画書の実装方針に配線手順を明記すること。
永続状態はすべて `os.UserConfigDir()/Wirexa/` 配下の JSON。

## コード生成の確認

Go↔TS の境界を跨ぐ変更では、以下の生成物の再生成が必要か判定し、必要なら計画書の手順に含めること。
どちらも**手で編集してはならない**生成物であり、CI が鮮度をチェックする。

| 生成物                                | 再生成コマンド            | 必要になる条件                                                                                                                             |
| ------------------------------------- | ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `frontend/wailsjs/`                   | `task wails:generate`     | バインド対象の Go 構造体・ハンドラメソッドを変更したとき（CI が `git diff --exit-code frontend/wailsjs` で落ちる）                         |
| `frontend/src/shared/wails-events.ts` | `task go:generate:events` | イベント名を追加・変更したとき。定数の正は `internal/domain/events.go`。**新規イベントは `tools/gen-events/main.go` の一覧への追記も必要** |

ドメイン型にフィールドを追加する場合の手順は「ドメイン 1 箇所 ＋ 生成 1 コマンド」で済む（adapters に重複 DTO を置かないため）。
フロントの変換は素通しのため `infrastructure/<proto>/client.ts` の編集は通常不要。ユニオン型の場合のみ型ガードを更新する。

## テスト方針の確認

このプロジェクトのテストは5系統ある。変更がどれに影響するかを特定し、計画書に明記すること。

| 系統              | 場所                        | 実行                                                                                            |
| ----------------- | --------------------------- | ----------------------------------------------------------------------------------------------- |
| Go ユニット       | コードと同居（`*_test.go`） | `task go:test`                                                                                  |
| Go 統合           | `internal/integration/`     | `task go:test:integration`（`integration` ビルドタグ）                                          |
| フロント ユニット | コードと同居（`*.test.ts`） | `task frontend:test`（既定は node 環境。DOM が要る場合は先頭に `// @vitest-environment jsdom`） |
| UI e2e            | `frontend/e2e/ui/`          | `task frontend:test:e2e`（Go 不要）                                                             |
| フルスタック e2e  | `frontend/e2e/integration/` | `task frontend:test:e2e:fullstack`（Windows/ローカルのみ、CI 非対象）                           |

**バインド API の振る舞いを変えた場合、`frontend/e2e/fake-backend/` に同じ変更が必要になる。**
UI e2e はこの偽バックエンドが Go サービスの意味論（例: `__root__` 予約コレクション）を模倣することで成立しているため、
計画書の「変更対象ファイル」表に偽バックエンド側の変更も含めること。

また `go test` / `go vet` は `main.go` の `//go:embed` のため `frontend/dist/index.html` の存在を要求する。

## 変更計画書の作成

**必ず変更計画書を `docs/<YYYY-MM-DD>-<変更内容の要約>.md` に出力すること。**

計画書には以下を含めること：

```markdown
# 変更計画書: <タイトル>

## 概要

変更の目的と背景を簡潔に説明する。

## 変更対象ファイル

変更・追加・削除するファイルの一覧と、各ファイルへの変更内容を記述する。

| ファイル        | 層     | 変更種別 | 変更内容 |
| --------------- | ------ | -------- | -------- |
| path/to/file.go | Domain | 変更     | ...      |

## 実装方針

アーキテクチャ上の判断・設計方針を記述する。
クリーンアーキテクチャの依存方向を維持する根拠も含めること。
サービスを追加する場合は `app.go` の二段階配線（`NewApp()` で空生成 → `initialize()` で `SetupXxxHandler` 注入）の手順も記述する。

## コード生成

再生成が必要な生成物と実行コマンドを記述する。不要な場合は「不要」と明記する。

- [ ] `task wails:generate`（バインド対象の Go 構造体・メソッドを変更した場合）
- [ ] `task go:generate:events`（イベント追加時。`tools/gen-events/main.go` への追記も必要）

## テスト方針

追加・変更するテストと、その系統を記述する。
バインド API の振る舞いを変える場合は `frontend/e2e/fake-backend/` の追従も記載する。

## 副作用・注意事項

既存の振る舞いが変わる箇所や、影響範囲を記述する。

## Git運用

- **ブランチ名**: `<type>/<変更内容の要約>` (例: `feat/mqtt-reconnect`, `fix/http-timeout`)
- **コミット分割方針**: 層ごと・機能ごとに分割する方針を記述する（コミットメッセージは日本語）
- **完了後**: `task format` → `task lint` → `task test` がすべて通ることを確認 → main へマージ
```

コード中のコメントとコミットメッセージは日本語で記述する。

計画書を作成した後、ユーザーに内容を確認し、**承認が得られるまで実装は行わないこと。**
