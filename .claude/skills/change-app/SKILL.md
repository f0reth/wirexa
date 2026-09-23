---
name: change-app
description: 計画書に基づきアプリの変更を実施
disable-model-invocation: true
---

このプロジェクトに対して、指定された計画書または内容に基づき変更を実施してください。

## 入力

$ARGUMENTS

入力が計画書のファイルパスの場合はそのファイルを読み込み、テキストの場合はそのまま変更内容として扱うこと。

## 目的

- 指定された要件に従ってアプリを変更する
- 既存の設計思想（クリーンアーキテクチャ）を維持する
- 不要な複雑さを排除し、可読性と保守性を向上させる

## アーキテクチャ制約

- 既存のクリーンアーキテクチャを破壊しない
- レイヤーの依存方向（コンパイル時の import 方向）を守る

```
adapters ──────┐
application ───┼──→ domain（出力ポートを定義）
infrastructure ┘
```

- `domain` は他のどのレイヤーにも依存しない
- `application` / `infrastructure` は `domain` の出力ポートを実装する（依存性逆転）
- `adapters` は自分で定義したインターフェース（ユースケースの入力ポート）経由でユースケースを呼び出し、`application` を直接 import しない。`application` のサービスはこれを構造的に満たし、その検証は `app.go` で行う
- 具体型の組み立て・注入は合成ルート `app.go` でのみ行う
- 各レイヤーが持つ責務を逸脱する変更は禁止

## 実装ルール

- 後方互換性は考慮しない（内部コードのみ変更するため）
- 変更は最小単位で行う（過剰なリファクタリングは禁止）
- 変更と同時に改善できる軽微なリファクタリングは許可
- 命名は既存コードの規則に従い一貫性を維持する
- 不要な依存関係を追加しない
- 既存の振る舞いが変わる変更はコミットメッセージで明示する
- コード中のコメントとコミットメッセージは日本語で記述する
- ドメイン型と同型の RPC 用 DTO を `internal/adapters/` に新設しない（ハンドラはドメイン型の薄いパススルー）
  - 例外: ドメインに対応物の無いアダプタ固有の入力型は許可される（例: `log_handler.go` の `LogEntry`）
  - 例外: 名前付き文字列型を引数に直接使うと Wails が型を生成しないため、引数は `string` で受けて内部で変換する
  - この禁止は adapters の RPC DTO に限る。永続化形式は infrastructure のリポジトリにある `storedXxx` DTO が持ち、どの階層でも domain 型を埋め込まない
- 保存対象のフィールドを domain 型に足すときは、stored DTO と変換関数にも足す（足し忘れは全フィールドを埋めた往復テストで落ちる）

## コード生成（生成物は手で編集しない）

Go↔TS 境界を変更したら、コミット前に該当するコマンドを実行する。CI が生成物の鮮度をチェックする。

| 生成物 | コマンド | 必要になる条件 |
| --- | --- | --- |
| `frontend/wailsjs/` | `task wails:generate` | バインド対象の Go 構造体・ハンドラメソッドを変更したとき |
| `frontend/src/shared/wails-events.ts` | `task go:generate:events` | `internal/domain/events.go` のイベント名を変更したとき（新規追加は `tools/gen-events/main.go` への追記も必要） |

バインド API の振る舞いを変えた場合は、`frontend/e2e/ui/` が依存する `frontend/e2e/fake-backend/` にも同じ変更を反映する。

## Git 運用ルール

以下を必ず守ること：

1. **作業前にブランチを確認・作成する**

```bash
git status          # mainブランチにいることを確認
git switch main     # mainでなければ切り替える
git switch -c <ブランチ名>
```

- 命名規則: `feat/<変更内容の要約>` または `fix/<変更内容の要約>`
  - 例: `feat/add-user-validation`, `fix/mqtt-reconnect`

2. **論理単位でコミットを分割する**

3. **コミット前に必ず整形・Lint・テストを実行する**

```bash
task format   # フロントエンドの整形（biome）
task lint     # go vet + golangci-lint + biome + tsc
task test     # Go とフロントエンドのユニットテスト
```

変更内容に応じて以下も実行する：

```bash
task frontend:test:e2e    # frontend/e2e/fake-backend/ を変更した場合、または UI の振る舞いを変えた場合
task go:test:integration  # internal/integration/ に関わる変更をした場合
```

- エラーが出た場合は原因を修正して再実行する
- 自力で解決できないエラーはユーザーに報告し、作業を中断する
- `task go:test` / `task go:vet` は `frontend/dist/index.html` の存在を要求する（`main.go` の `//go:embed`）

4. **すべての変更が完了したら main にマージしてブランチを削除する**

```bash
git switch main
git merge <ブランチ名>
git branch -d <ブランチ名>
```
