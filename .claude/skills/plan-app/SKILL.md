---
name: plan-app
description: アプリ変更の計画書を作成し、ユーザーの承認を得る
disable-model-invocation: true
---

このプロジェクトに対して、指定された変更の計画書を作成してください。

## 入力

$ARGUMENTS

## 前提

アーキテクチャ、依存方向、`app.go` の二段階配線、ドメイン型・RPC・永続化の境界、設定データの復旧方針、コード生成、テスト構成は **CLAUDE.md を正とする**。ここには計画書作成に固有の手順と確認観点だけを書く。

## 実装開始前の確認

要件の解釈が複数通りある、変更範囲が不明確、既存アーキテクチャと矛盾する指示がある、前提情報が欠けている、といった曖昧な点があれば、**計画書を作成する前に 1 つの質問にまとめて聞くこと。**

## 影響範囲の特定

計画書を書く前に関係するコードを実際に読み、変更が次のどこに及ぶかを特定すること。推測で埋めない。

- **対象プロトコル**: `http` / `mqtt` / `udp` / `openapi`、またはプロトコル横断（ログ、ウィンドウ状態、ライフサイクルなど）
- **バックエンドの層**: `domain` / `application` / `infrastructure` / `adapters` / `app.go`
- **フロントエンドの層**: `domain` / `application` / `infrastructure` / `presentation`（`components` / `providers` / `utils` / `constants`）/ `shared`、共通 UI 部品の `src/components/ui/`、設定値の `src/config/`

### 依存方向で取り違えやすい点

- 出力ポートを実装するのは `infrastructure` だけ（`var _ domain.XxxRepository = (*XxxRepository)(nil)` で検査）。`application` は使う側であり実装しない。
- 入力ポートの充足検証と具体型の組み立ては `app.go` だけで行う。
- adapters は `application` を import しない。間にユースケースが無い場合は、domain の出力ポートを `SetupXxxHandler` の引数で直接受け取る例もある（`HTTPHandlerDeps.Responses` の `ResponseBodyStore`、`SetupLogHandler` の `domain.Logger`）。業務ロジックが要るならユースケースを経由させる。
- adapters に許される独自の型は、ドメインに対応物の無いアダプタ固有の入力型だけ（例: `log_handler.go` の `LogEntry`）。名前付き文字列型の引数は Wails が型を生成しないため `string` で受けて内部で変換する（例: `UDPHandler.StartListen` の `encoding`）。
- フロントエンドの依存の注入は合成ルートの **`presentation/providers/*.tsx` と `App.tsx`** で行う。`presentation/components/` から `infrastructure/` を import しない（biome の `noRestrictedImports` がエラーにする）。`application/` から `infrastructure/` と `wailsjs/` も import しない（同じく biome がエラーにする）。`infrastructure/` から `application/` への import も既存の 2 か所（`logger/client.ts`、`openapi/parser.ts`）だけで、増やさない。
- 新しい外部作用のポートは用途で置き場所を分け、計画書にどちらかを書く。
  - localStorage などの保存: `domain/<proto>/ports.ts`（例: `PresetStorage`、`ThemeStorage`）。実装は `infrastructure/storage/local-storage.ts`。
  - RPC: 使う側の application ファイルが定義する `XxxApi` インターフェース（例: `CollectionsApi`、`UdpSendApi`）。infrastructure の `client.ts` は `XxxApi` を import せずに構造的に満たし、Provider がモジュールをそのまま注入する（例: `udp-provider.tsx` の `udpClient`）。

### RPC 型を設計するときの確認

- **`map[string][]Struct` を RPC に出す Go 型に使わない。** Wails 生成の `convertValues` が値を二重配列に壊し、型検査も CI も通るため実行時まで気づけない。キーごとに独立したスライスのフィールドに分ける（例: `httpdomain.RequestBody` の `FormData` / `FormURLEncoded`）。値がプリミティブやそのスライスなら問題ない（例: `HTTPResponse.Headers map[string][]string`）。
- **フロントエンドの domain 型は手書きで、再生成では変わらない。** Go の domain 型を変えるときは `frontend/src/domain/<proto>/types.ts` も変更対象に含め、`frontend/src/infrastructure/<proto>/client.ts`（OpenAPI は `file-io.ts`）の変換関数も確認する。スプレッドで素通しする型（例: `fromWailsRequestBody`）は通常そのままでよいが、フィールドを列挙して組み立てる型（例: `fromWailsKeyValuePair`、`fromWailsFileReference`）とユニオン型の型ガードは直す必要がある。

### RPC 引数を信頼しない

Wails の RPC は WebView 上の JS から誰でも呼べる。RPC 引数でローカルのファイルやリソースに触れる変更では、次を「実装方針」に書くこと（経緯は `docs/http-local-file-access-hardening.md`）。

- RPC で受け取ったパスを検証なしに読み書きしない。対象はユーザーがネイティブダイアログで選んだファイルだけとし、次のどちらの方式に合わせるかを書く。
  - **token 方式**（HTTP）: ダイアログの選択結果に backend が token を発行し、RPC では token で参照させる。フロントエンドは実パスを持たない（例: `HTTPHandler.OpenFilePicker` と `httpdomain.FileReference.Token`）。パス入力欄はダイアログの hint にとどめる。
  - **許可リスト方式**（OpenAPI）: 選ばれたパスを backend の許可リストに登録し、RPC で受け取ったパスは照合してから読み書きする（例: `OpenAPIHandler.ReadFile(path)` と `openapiapp.FileService.checkGranted`）。選んだパスはフロントエンドに返してよい。
- backend が内部で作ったパスや一時ファイルはフロントエンドに返さず、ID で参照させる（例: `HTTPHandler.SaveResponseBody(executionID)`）。
- 保持する件数・容量・有効期間の上限と、エラーメッセージやログにパスを出さないかを決める。

### 永続化に触れるときの確認

保存データの形式や保存ファイルに触れる変更では、次を「永続化への影響」に書くこと。

- 変更する stored DTO（`storedXxx`）と変換関数（`toStoredXxx` / `fromStoredXxx` など）
- `testdata/*.golden.json` を更新するか（旧形式のファイルを読めるかを含む）
- 保存しないフィールドを足すなら、往復テストの期待値補正関数（例: `persistedCollection`）に書く理由
- **新しい保存ファイルを足すなら、復旧方針の分類**（必須 / best effort / 再生成可能）と破損時・退避失敗時・読み込み失敗時の扱い。単一ファイル型なら `store.LoadSingleFile` のポリシーも決める。分類表を変えるなら CLAUDE.md と `internal/application/store/recovery.go` の両方を直す手順を含める
- **フロントエンドの localStorage も保存データである。** `frontend/src/infrastructure/storage/local-storage.ts` のキー（`mqtt:presets`、`app:theme` など）や値の形を変えるなら、旧形式を読めるか（例: `StoredPreset` で `retain` の無い旧プリセットを補う）と、古いキーの扱いを書く

## 変更計画書の作成

**変更計画書を `docs/<YYYY-MM-DD>-<要約>.md` に出力すること。**（要約は英小文字のケバブケース。例: `docs/2026-09-22-http-execution-id-separation.md`）

```markdown
# 変更計画書: <タイトル>

## 概要

変更の目的と背景。

## 変更対象ファイル

バインド API の振る舞いを変える場合は `frontend/e2e/fake-backend/` も含める。

| ファイル        | 層     | 変更種別 | 変更内容 |
| --------------- | ------ | -------- | -------- |
| path/to/file.go | Domain | 変更     | ...      |

## 実装方針

アーキテクチャ上の判断と、依存方向（バックエンド・フロントエンド両方）を維持できる根拠。
サービスを追加する場合は `app.go` の二段階配線の手順を、フロントエンドで依存を注入する場合はどの Provider（または `App.tsx`）で注入するかを書く。

## 永続化への影響

触れない場合は「なし」。触れる場合は「永続化に触れるときの確認」の各項目。

## コード生成

不要な場合は「不要」。

- [ ] `task wails:generate`（バインド対象の Go 構造体・メソッドを変更した場合。新しい形の型は `frontend/wailsjs/go/models.ts` を目視）
- [ ] `task go:generate:events`（イベント追加時。`tools/gen-events/main.go` への追記も必要）

## テスト方針

追加・変更するテストを系統ごと（Go ユニット / Go 統合 / フロント ユニット / UI e2e / フルスタック e2e）に書く。

## 副作用・注意事項

既存の振る舞いが変わる箇所と影響範囲。

## Git運用

- **ブランチ名**: `<type>/<要約>`（type は `feat` / `fix` / `refactor` / `perf` / `test` / `docs` / `chore` など。例: `feat/mqtt-reconnect`）
- **コミット分割方針**: 層ごと・機能ごとの分割方針。メッセージは `<type>(<scope>): <日本語の要約>`（scope はプロトコル名か `domain` / `adapters` / `frontend` / `e2e` / `integration` などの層・対象。例: `fix(http): 予約済み root collection の削除とリネームを拒否する`）
- **完了条件**: 実行するものを列挙し、すべて通ってから main へマージする
  - 常に: `task format` → `task lint` → `task test`
  - 変更内容に応じて（CI でも実行）: `task go:test:integration`（`internal/integration/` が検証する振る舞いに関わる場合）、`task frontend:test:e2e`（UI の振る舞いや fake-backend を変えた場合）、`task go:test:race`（並行処理に触れる場合。CI の Go ユニットテストは常に -race。cgo/gcc が必要で、実行できなければその旨を報告する）
  - 任意: `task frontend:test:e2e:fullstack`（CI 非対象。実バックエンドとの結合を確かめる場合）
```

計画書を作成したらユーザーに確認し、**承認が得られるまで実装は行わないこと。**
