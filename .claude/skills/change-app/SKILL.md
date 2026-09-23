---
name: change-app
description: 計画書に基づきアプリの変更を実施
disable-model-invocation: true
---

このプロジェクトに対して、指定された計画書または内容に基づき変更を実施してください。

## 入力

$ARGUMENTS

入力が計画書のファイルパスの場合はそのファイルを読み込み、テキストの場合はそのまま変更内容として扱うこと。

## 前提

アーキテクチャ、依存方向、`app.go` の二段階配線、ドメイン型・RPC・永続化の境界、設定データの復旧方針、コード生成、テスト構成は **CLAUDE.md を正とする**。ここには CLAUDE.md に無い手順と、取り違えやすい点だけを書く。

## 実装開始前の確認

- 計画書がある場合は、そのすべての節（「変更対象ファイル」「実装方針」「永続化への影響」「コード生成」「テスト方針」「副作用・注意事項」「Git運用」）に従う。計画書と実際のコードが食い違っていたら、実装前にユーザーに確認する。
- テキストで指示された場合は、関係するコードを読んで変更範囲を特定してから着手し、推測で埋めない。コード生成・テスト・永続化への影響は下記に沿って自分で判断する。
- 要件の解釈が複数通りある、変更範囲が不明確、既存アーキテクチャと矛盾する、前提情報が欠けている、といった点があれば、**実装前に 1 つの質問にまとめて聞くこと。**

## 実装時に取り違えやすい点

### 依存方向

- 出力ポートを実装するのは `infrastructure` だけで、`var _ domain.XxxRepository = (*XxxRepository)(nil)` で検査する。`application` は使う側で、実装はしない。
- adapters が domain の出力ポートを直接受け取ってよいのは、間にユースケースが無い場合だけ。業務ロジックが要るならユースケースを経由させる。
- localStorage 系のポートの実装は `infrastructure/storage/local-storage.ts` に置く。
- `application/` → `infrastructure/` の既存の例外は、そのための計画でない限り解消しない。

### RPC 型

- ドメイン型と同型の RPC 用 DTO を `internal/adapters/` に新設しない。例外は次の 2 つ。
  - ドメインに対応物の無いアダプタ固有の入力型（例: `log_handler.go` の `LogEntry`）
  - 名前付き文字列型の引数。Wails が型を生成しないため `string` で受けて内部で変換する（例: `UDPHandler.StartListen` の `encoding`）
- **`map[string][]Struct` を RPC に出す Go 型に使わない。** Wails 生成の `convertValues` が値を二重配列に壊し、型検査も CI も通る。キーごとのスライスのフィールドに分ける（例: `httpdomain.RequestBody` の `FormData` / `FormURLEncoded`）。値がプリミティブやそのスライスなら問題ない。
- **フロントエンドの domain 型は手書きで、再生成では変わらない。** Go の domain 型を変えたら `frontend/src/domain/<proto>/types.ts` と、`frontend/src/infrastructure/<proto>/client.ts`（OpenAPI は `file-io.ts`）の変換関数も確認する。フィールドを列挙する変換（例: `fromWailsKeyValuePair`）やユニオン型の型ガードは直す必要がある。

### RPC 引数を信頼しない

Wails の RPC は WebView 上の JS から誰でも呼べる（経緯は `docs/http-local-file-access-hardening.md`）。

- RPC で受け取ったパスを検証なしに読み書きしない。対象はユーザーがネイティブダイアログで選んだファイルだけとし、既存の方式に合わせる。
  - **token 方式**（HTTP）: 選択結果に backend が token を発行し、RPC では token で参照させる（例: `HTTPHandler.OpenFilePicker`、`httpdomain.FileReference.Token`）。
  - **許可リスト方式**（OpenAPI）: 選ばれたパスを許可リストに登録し、RPC で受け取ったパスを照合してから読み書きする（例: `openapiapp.FileService.checkGranted`）。
- backend が内部で作ったパスや一時ファイルはフロントエンドに返さず、ID で参照させる（例: `HTTPHandler.SaveResponseBody(executionID)`）。
- エラーメッセージやログにパスを出さない。

### 永続化

保存データ（設定ディレクトリの JSON と localStorage）はユーザーの手元に残るため、**旧形式を読めるように保つ**。

- 保存形式を変えるときは `testdata/*.golden.json` を更新し、旧形式のファイルを読めることをテストで確かめる。
- 新しい保存ファイルは、CLAUDE.md の「設定データの分類と復旧方針」で分類を決めて従う。
- localStorage（`frontend/src/infrastructure/storage/local-storage.ts`）のキーや値の形を変えるときは、旧形式を補って読む（例: `retain` の無い旧プリセットを補う `StoredPreset`）。

## 実装ルール

- RPC と内部 API の後方互換性は考慮しない（フロントエンドとバックエンドは同時に更新される）。保存データは上記「永続化」のとおり。
- 変更は最小単位で行う。過剰なリファクタリングはしないが、変更と同時にできる軽微な改善は許可。
- 命名は既存コードの規則に従う。不要な依存関係を追加しない。
- 既存の振る舞いが変わる変更はコミットメッセージで明示する。
- 振る舞いを足す・変える変更にはテストを追加・更新する。バグ修正では、修正前に落ちる再現テストを書く。系統は確かめたい範囲で選ぶ。
  - 1 つの層やパッケージで閉じる振る舞い: Go ユニット / フロント ユニット
  - 層をまたぐバックエンドの振る舞い（RPC から保存・通信まで）: Go 統合（`internal/integration/`）
  - 画面操作で見える振る舞い: UI e2e（`frontend/e2e/ui/`。fake-backend の追従も含む）
- コミットメッセージは `<type>(<scope>): <日本語の要約>` の形式にする。計画書にコミット分割方針があればそれに従う。
  - type は `feat` / `fix` / `refactor` / `perf` / `test` / `docs` / `chore` / `build` / `ci` など
  - scope はプロトコル（`http` / `mqtt` / `udp` / `openapi` など）か層・対象（`domain` / `frontend` / `e2e` / `integration` など）。絞れなければ省略してよい
  - 例: `fix(http): 予約済み root collection の削除とリネームを拒否する`

## コード生成の注意点

Go↔TS 境界を変更したら、コミット前に CLAUDE.md の生成コマンドを実行する（生成物は手で編集しない）。`task wails:generate` では次に注意する。

- Wails CLI のバージョンを go.mod と揃える（`wails version` で確かめる）。違うと生成結果がぶれて CI で落ちる。
- 新しい形の型を RPC に出したら、`frontend/wailsjs/go/models.ts` の生成結果を目で確かめる。
- ハンドラの型名の大文字・小文字だけを変えて再生成すると、Windows ではディスク上のファイル名・git の追跡名・import がずれることがある（`task frontend:tsc` が TS1261 で落ちる）。正は Go の型名で、`git mv -f <旧> <新>` で直し、`git ls-files frontend/wailsjs/go/adapters/` と実ファイル名の一致を確かめる。

バインド API の振る舞いを変えたら `frontend/e2e/fake-backend/` にも反映する。

## Git 運用ルール

1. **作業前にブランチを作る**: `git status` で状態を確かめ、main から `git switch -c <ブランチ名>` する。
   - 命名は `<type>/<変更内容の要約>`（例: `feat/add-user-validation`、`fix/mqtt-reconnect`）。計画書にブランチ名があればそれに従う
   - 未コミットの変更がある場合は、今回の変更に含めてよいかをユーザーに確認する。勝手に stash・破棄・コミットしない
   - ただし入力された計画書（`docs/<YYYY-MM-DD>-<要約>.md`）自体が未コミットなのは想定どおりで、確認は要らない。計画書に扱いが書かれていればそれに従い、無ければブランチの最初のコミット（`docs: <要約>の変更計画書を追加する`）にする

2. **論理単位でコミットを分割する**

3. **コミット前に `task format`、`task lint`、`task test` を実行する**。変更内容に応じて以下も実行する。
   - `task frontend:test:e2e`: `frontend/e2e/fake-backend/` を変更した、または UI の振る舞いを変えた場合
   - `task go:test:integration`: `internal/integration/` が検証する振る舞い（各プロトコルの RPC から保存・通信まで）に関わる場合。テストファイルを触っていなくても実行する
   - `task go:test:race`: 並行処理（goroutine・ロック・キャンセル）に触れた場合（CI は常に -race で実行する）
   - 任意（CI 対象外）: `task frontend:test:e2e:fullstack`（実バックエンドとの結合を確かめたい場合）

   エラーは原因を修正して再実行する。自力で解決できなければユーザーに報告して中断する。環境の都合で実行できなかったチェック（gcc が無く `go:test:race` を実行できない、など）はスキップしたことを報告する。

4. **完了したら main にマージしてブランチを削除する**: `git switch main && git merge <ブランチ名> && git branch -d <ブランチ名>`
