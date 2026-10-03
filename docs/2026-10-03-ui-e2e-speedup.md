# 変更計画書: UI e2e をビルド済みバンドルで実行して高速化する

## 概要

UI e2e（`task frontend:test:e2e`）の実行時間の大半は、テスト本体ではなくページの読み込みに使われている。テストの内容と検証する振る舞いは変えずに、配信方法を Vite の dev サーバーからビルド済みバンドル（`vite build` + `vite preview`）へ切り替えて短縮する。

### 計測結果（2026-10-03、ローカル Windows、24 論理コア、215 件）

| 配信方法 | ワーカー数 | 全体 | テスト 1 件の中央値 | 最短のテスト |
| --- | --- | --- | --- | --- |
| dev サーバー（現状） | 12（既定） | 112 秒 | 5.6 秒 | 3.9 秒 |
| ビルド済みバンドル | 12（既定） | 21 秒、25 秒（2 回） | 0.8 秒、1.0 秒 | 0.45 秒 |
| dev サーバー（現状） | 4 | 144 秒 | 2.3 秒 | 1.5 秒 |
| ビルド済みバンドル | 4 | 40 秒 | 0.56 秒 | 0.34 秒 |
| ビルド済みバンドル | 1 | 131 秒 | 0.49 秒 | 0.31 秒 |

- 現状は最短のテストでも 3.9 秒かかり、`page.reload()` を含むテストはそこから約 5 秒延びる。つまり 1 回のページ読み込みに 4〜5 秒かかっている。
- 原因は dev サーバーがモジュールを 1 ファイル 1 リクエストで返すこと。テストごとにブラウザコンテキストが新しくなるのでキャッシュも効かず、毎回すべてのモジュールを取り直す。フルスタック e2e が同じ理由でプレビューサーバーに切り替え済み（`playwright.integration.config.ts` のコメント）。
- バンドルのビルドは 2 秒で終わる（`tsc` を含まない `vite build` のみ）。
- バンドルに切り替えた状態で、既定のワーカー数では 215 件すべて通過した（2 回）。

### CI の現状（2026-10-01 の実行）

CI 全体 約 15 分半のうち `Frontend E2E (UI)` ジョブが 13 分。内訳は `bun run test:e2e` が 10 分 43 秒、`playwright install --with-deps chromium` が 2 分 5 秒。CI は `workers: 1` で実行している。

### 計測で見つかった既存の不具合（先に直す）

`e2e/ui/mqtt/profiles.spec.ts` の `failed mqtt connect shows an error toast` が、ワーカー数 1 と 4 のときに失敗する（dev サーバーでも、バンドルでも同じ）。2 回目の Connect のあと、偽バックエンドが `setTimeout` で接続を取り消すより先に `fake.snapshot()` を読んでいる競合で、マシンに余裕があるほど起きやすい。配信方式とは無関係だが、バンドル化で CI のテストが速くなると顕在化するので、同じ変更の中で先に直す。

### フルスタック e2e は対象外

`task frontend:test:e2e:fullstack` は 37 件で 61 秒（テスト 41 秒、起動 約 20 秒）。すでにプレビューサーバーを使っており、1 件あたり約 1.1 秒で、縮める余地は小さい。この計画では触らない。

## 変更対象ファイル

バインド API の振る舞いは変えないので `frontend/e2e/fake-backend/` は変更しない。

| ファイル | 層 | 変更種別 | 変更内容 |
| --- | --- | --- | --- |
| `frontend/e2e/ui/mqtt/profiles.spec.ts` | UI e2e | 変更 | 2 回目の Connect 後の `mqttConnections` の検証を `expect.poll` で待つ（競合の修正） |
| `frontend/vite.e2e.config.ts` | ビルド設定 | 変更 | `transformIndexHtml` を `order: "pre"` にする（ビルド時に `install.ts` をバンドルへ含めるため）。`build.outDir: "dist-e2e"`、`preview.port` / `strictPort` を追加。冒頭コメントの `main.tsx` は実在しないので `src/index.tsx` に直す |
| `frontend/package.json` | ビルド設定 | 変更 | `build:e2e`（`vite build --config vite.e2e.config.ts`）と `preview:e2e`（`vite preview --config vite.e2e.config.ts`）を追加。`preview:e2e` にも `--config` を付けないと `outDir` とポートの設定が読まれず、`dist` を 4173 番で配信してしまう。`dev:e2e` は残す |
| `frontend/playwright.config.ts` | UI e2e | 変更 | `webServer` をビルド + プレビューに変更、ポートと `baseURL` を変更、`reuseExistingServer: false`。`WIREXA_E2E_DEV=1` のときだけ従来の dev サーバーを使う。CI では `reporter` に `list` を足して flaky の件数をログに出す。CI のワーカー数を変更 |
| `.gitignore` | — | 変更 | `dist-e2e` を追加（既存の `dist` は別名のディレクトリに一致しない） |
| `frontend/biome.json` | — | 変更 | `files.includes` に `!!**/dist-e2e` を追加 |
| `.github/workflows/ci.yml` | CI | 変更 | Playwright のブラウザをキャッシュする（任意、後述） |
| `CLAUDE.md` | ドキュメント | 変更 | Testing layout の UI e2e の説明に、ビルド済みバンドルを配信することと `WIREXA_E2E_DEV` を追記 |
| `.claude/skills/e2e-test-review/SKILL.md` | ドキュメント | 変更 | `webServer.command` が `bun run dev:e2e` である前提の記述を直す |

アプリ本体（`frontend/src/`、`internal/`）は変更しない。

## 実装方針

### 1. 競合するテストの修正

`profiles.spec.ts` の 2 回目の Connect のあとを次の形にする。1 回目はトーストの表示を待ってから読んでいるので問題ない。

```ts
await expect
  .poll(async () => (await fake.snapshot()).mqttConnections)
  .toEqual([]);
```

### 2. ビルド済みバンドルの配信

- `vite.e2e.config.ts` の `fakeBackendPlugin` は、いまは `transformIndexHtml` を既定の順序で使っている。ビルドではこの順序だと Vite が HTML を処理したあとにタグが挿入され、`/e2e/fake-backend/install.ts` がバンドルされない。`order: "pre"` にすると `src/index.tsx`（`index.html` が読み込むエントリ）と同じエントリにまとめられる。モジュールスクリプトは文書順に import されるので、`install.ts` が `src/index.tsx` より先に実行される順序は保たれる（計測時にこの構成で 215 件が通ることを確認済み）。dev サーバーでも `order: "pre"` のまま動く。
- **出力先は `dist-e2e` とし、`dist` には絶対に出さない。** `dist` は `main.go` の `//go:embed` と `wails build` が使うので、偽バックエンド入りのバンドルが本番バイナリに混入する。
- ポートは 5173（`wails dev` / `bun run dev`）とも 5174（フルスタック e2e のプレビュー）とも別の 5175 にする。開発中の dev サーバーが動いていても衝突しない。
- `reuseExistingServer` は `false` にする。プレビューサーバーを使い回すとビルドが走らず、古いバンドルでテストが通ってしまう。ビルドは 2 秒なので毎回やり直す。ポートが埋まっていれば `strictPort` で起動に失敗させる。
- `bun run build` と違い `tsc -b` は挟まない。型検査は `task frontend:tsc`（CI では Frontend Unit Tests ジョブ）が受け持っている。
- 圧縮（minify）は既定のまま有効にする。フルスタック e2e と本番ビルドに合わせる。
- `WIREXA_E2E_DEV=1` のときは従来どおり `bun run dev:e2e`（ポート 5173、`reuseExistingServer: true`）を使う。`playwright test --ui` でアプリのソースを編集しながら調べるとき用。既定では使わない。

### 3. CI のワーカー数

`workers: process.env.CI ? 1 : undefined` を `process.env.CI ? 2 : undefined` にする。リポジトリは public で `ubuntu-latest` は 4 vCPU。このスイートは `fullyParallel: true` で、ローカルでは常に 12 並列で動いており、状態はページ内にしか無いので並列にして困る共有状態は無い。CI が不安定になった場合はこのコミットだけ戻せるよう、独立したコミットにする。

不安定かどうかは、ジョブの成否ではなく flaky の件数で判定する。CI は `retries: 2` なので、並列化で落ちるようになったテストも再試行で通ればジョブは成功になり、成否だけでは見逃す。いまの `reporter: "html"` は件数をログに出さないので、CI では `process.env.CI ? [["list"], ["html"]] : "html"` にして、末尾の集計（`N flaky`）をログで読めるようにする。この変更はバンドル配信のコミットに入れ、1 ワーカーの段階でも同じ基準で比べる。

### 4. Playwright ブラウザのキャッシュ（任意）

`actions/cache` で `~/.cache/ms-playwright` を `bun.lock` のハッシュをキーに保存し、ヒット時は `bunx playwright install-deps chromium` だけを実行する。2 分 5 秒のうちダウンロードと apt のどちらが長いかは未計測なので、効果は CI で確かめてから残すか決める。

### 依存方向

変更はテストとビルド設定だけで、バックエンド・フロントエンドどちらの層の import も変えない。

### GitHub Actions での動作

ローカルの Windows でしか確認していない。Actions（`ubuntu-latest`）で変わる点と見込みは次のとおり。

| 点 | 見込み | 根拠 |
| --- | --- | --- |
| Linux での `vite build` | 未確認 | いまの CI には Linux でフロントエンドをビルドするジョブが無い（ビルドは release の `windows-latest` だけ）。Vite は Linux 対応なので通る見込みだが、実績は無い |
| 1 ワーカーで落ちるテスト | 修正しないと落ちる | ローカルの 1 ワーカー実行で再現済み。CI は 1 ワーカーなので「1. 競合するテストの修正」は必須 |
| サーバーの起動待ち | 問題ない見込み | ビルドは 2 秒で、Playwright の既定の待ち時間は 60 秒 |
| サーバーの使い回し | 変化なし | CI ではいまも `reuseExistingServer` は無効 |
| 失敗時のレポートのアップロード | 変化なし | 出力先 `playwright-report/` は変えない |
| 2 ワーカー化 | 未確認 | ローカルでは 12 並列で通っているが、CI の 4 vCPU での安定性は実績が無い |
| ブラウザのキャッシュ | 未確認 | 新しく足す手順で、効果も動作も CI でしか確かめられない |

CI は `main` 向けのプルリクエストでも走るので、マージ前に実際の Actions で確かめる。原因を切り分けられるよう、変更は 3 段階に分けて push し、段階ごとに CI の結果を見てから次へ進む（手順は「Git運用」）。

## 永続化への影響

なし。

## コード生成

不要。

## テスト方針

- **Go ユニット / Go 統合 / フロント ユニット**: 変更なし。
- **UI e2e**: テストの追加・削除はしない。`profiles.spec.ts` の 1 件だけ待ち方を直す。確認として次を実行する。
  - `task frontend:test:e2e` を既定のワーカー数で 3 回。215 件すべて通ること、実行時間が 30 秒前後であること。
  - `bunx playwright test --workers=1` と `--workers=2`。競合の修正が効いていること（修正前はここで 1 件落ちる）。
  - `WIREXA_E2E_DEV=1` を設定して従来の dev サーバーでも通ること。`frontend/` で、PowerShell なら `$env:WIREXA_E2E_DEV = "1"; bunx playwright test; Remove-Item Env:WIREXA_E2E_DEV`（終わったら変数を消す。残すと以降の実行も dev サーバーになる）、bash なら `WIREXA_E2E_DEV=1 bunx playwright test`。
  - `frontend/dist` の中身が UI e2e の実行前後で変わらないこと。
- **フルスタック e2e**: 変更なし。

## 副作用・注意事項

- **テスト対象が開発ビルドから本番相当のビルドに変わる。** SolidJS は開発用ビルドでなく本番用ビルドになり、コードは圧縮される。アプリのコードに `import.meta.env` や `DEV` の分岐は無い（`frontend/src` を検索して確認済み）ので、アプリの振る舞いは同じ。実際に配布するものに近づく方向の変化である。
- 失敗時のスタックトレースは圧縮後のコード位置になる。読みにくければ `WIREXA_E2E_DEV=1` で再現するか、`vite.e2e.config.ts` で `build.minify: false` にする選択もある（今回は入れない）。
- テストの実行中にアプリのソースを編集しても反映されない（次の実行で再ビルドされる）。spec ファイルの編集はこれまでどおり反映される。
- dev サーバーが出していた `[vite] (client) [console.error] ...` のログ転送は無くなる。テストはこれに依存していない。
- CI の短縮幅は未計測。ローカルでは同じワーカー数で 3.6 倍（4 並列）〜 5 倍（12 並列）だったが、CI での実測は最初のプルリクエストで確かめる。
- `retries: 2`（CI）は変えない。

## Git運用

- **ブランチ名**: `perf/ui-e2e-bundle`
- **コミット分割方針**:
  1. `fix(e2e): MQTT の接続失敗テストで接続の取り消しを待つ`
  2. `perf(e2e): UI e2e をビルド済みバンドルで実行する`（`vite.e2e.config.ts`、`package.json`、`playwright.config.ts`、`.gitignore`、`biome.json`）
  3. `perf(e2e): CI の UI e2e を 2 ワーカーで実行する`
  4. `ci: Playwright のブラウザをキャッシュする`（任意。効果が無ければ入れない）
  5. `docs: UI e2e の配信方法の説明を更新する`（`CLAUDE.md`、`e2e-test-review/SKILL.md`）
- **push の順序（CI での段階確認）**: プルリクエストを 1 つ作り、次の段階ごとに push して `Frontend E2E (UI)` の結果と所要時間を記録してから次へ進む。
  1. コミット 1・2（競合の修正とバンドル配信、CI は 1 ワーカーのまま）。ここで落ちたら原因を直すまで先へ進まない。
  2. コミット 3（2 ワーカー化）。ログの集計で flaky が 1 件でも出たら不安定とみなし、このコミットを外して 1 ワーカーのままマージする（再試行で通ってジョブが成功していても同じ）。
  3. コミット 4（ブラウザのキャッシュ）。2 回実行してキャッシュのヒット時の時間を見る。短縮が無いか動かなければ外す。
  4. コミット 5（ドキュメント）。最終的に残した構成に合わせて書く。
- **CI の結果の確認方法**: `gh` コマンドを使う（ブラウザで Actions の画面を見に行かない）。
  - プルリクエストの作成: `gh pr create --base main`
  - 完了まで待つ: `gh pr checks --watch`（または `gh run watch <run-id>`）
  - 実行の一覧と ID: `gh run list --workflow ci.yml --branch perf/ui-e2e-bundle`
  - ジョブごとの結果と所要時間: `gh run view <run-id> --json jobs -q '.jobs[] | "\(.name)\t\(.conclusion)\t\(.startedAt)\t\(.completedAt)"'`
  - `Frontend E2E (UI)` のステップごとの所要時間（インストールとテスト実行の内訳）: `gh run view <run-id> --json jobs -q '.jobs[] | select(.name|test("E2E")) | .steps[] | "\(.name)\t\(.startedAt)\t\(.completedAt)"'`
  - flaky の件数（成功したジョブでも確認する）: `gh run view <run-id> --log | Select-String "flaky|passed|failed"`（bash なら `grep -E`）。`flaky` の行が無ければ 0 件
  - 失敗したときのログ: `gh run view <run-id> --log-failed`
  - 失敗時の Playwright レポート: `gh run download <run-id> -n playwright-report`
  - キャッシュのヒットを見るための再実行: `gh run rerun <run-id>`
- **完了条件**: 次がすべて通ってから main へマージする
  - `task format` → `task lint` → `task test`
  - `task frontend:test:e2e`（テスト方針に書いた回数とワーカー数で）
  - プルリクエストの CI で、最終的に残した構成の `Frontend E2E (UI)` が flaky 0 件で通ること。各段階の所要時間と flaky の件数をプルリクエストに書く
  - 任意: `task frontend:test:e2e:fullstack`（この変更では触らないので省いてよい）
