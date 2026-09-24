# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Wirexa is a Wails v2 desktop app (Windows-focused) for network testing: HTTP requests/collections, MQTT client, UDP send/listen, and an OpenAPI viewer. Backend is Go; frontend is SolidJS + TypeScript built with Vite and managed with **bun** (not npm).

Code comments and commit messages are written in Japanese — follow that convention.

## Commands

このプロジェクトの開発環境立ち上げ・ビルド・整形・リンター・テストは **Taskfile.yml 経由の `task` コマンド**で実行する。一覧は `task --list` で確認できる。

### 全体

```
task dev     # 開発環境を起動する (wails dev)
task build   # 本番ビルドを行う (wails build)
task test    # Go とフロントエンドのユニットテストをまとめて実行
task lint    # Go とフロントエンドの Lint をまとめて実行
task format  # Go とフロントエンドのコードを整形する
```

### Go (repo root)

```
task go:test              # unit tests (go test ./...)
task go:test:race         # unit tests with -race (what CI runs; needs cgo/gcc on Windows)
task go:test:integration  # integration tests (build tag required)
task go:vet               # go vet ./...
task go:lint              # golangci-lint run
```

単体テストの絞り込みなど Taskfile がカバーしていない操作は、素の `go test ./internal/application/http/ -run TestName` を直接使ってよい。

Go tests and `go vet` require `frontend/dist/index.html` to exist (the `//go:embed` in main.go fails otherwise). CI stubs it with `mkdir -p frontend/dist && touch frontend/dist/index.html`.

Go lint is golangci-lint (v7 action in CI). Wails CLI version must match go.mod (`v2.16.0`).

### Frontend

```
task frontend:install            # bun install
task frontend:tsc                # typecheck (tsc -b)
task frontend:test                # vitest unit tests
task frontend:lint                # biome check
task frontend:format              # biome check --write
task frontend:ci                  # biome ci (what CI runs)
task frontend:test:e2e            # Playwright UI e2e (fake backend; no Go/wails needed)
task frontend:test:e2e:fullstack  # Playwright against real wails dev (Windows/local only)
```

単一テストファイルの実行など Taskfile がカバーしていない操作は `frontend/` 内で `bunx vitest run src/shared/array.test.ts` のように直接実行してよい。

### Code generation (two mechanisms — both checked/needed when crossing the Go↔TS boundary)

1. **Wails bindings** (`frontend/wailsjs/` — generated, do not edit): after changing any bound Go struct or handler method, run `task wails:generate` from the repo root. CI fails if bindings are stale (`git diff --exit-code frontend/wailsjs`).
2. **Event names** (`frontend/src/shared/wails-events.ts` — generated, do not edit): event name constants live only in `internal/domain/events.go`. Regenerate with `task go:generate:events`. Adding a new event also requires adding it to the list in `tools/gen-events/main.go`.

## Architecture

Both sides follow the same layered (ports & adapters) structure.

- Backend dependency direction (compile-time imports): adapters → domain ← application, infrastructure. Adapters never import application: each handler file defines the use-case input ports it calls (`HTTPRequestUseCase`, `MQTTProfileUseCase`, …), application services satisfy them structurally, and `app.go` checks that at compile time. Infrastructure implements the domain's output ports.
- Backend adapters may also take a domain output port directly through their `SetupXxxHandler` deps when no use case sits in between (`HTTPHandlerDeps.Responses` is a `ResponseBodyStore`, `SetupLogHandler` takes a `domain.Logger`). They still never import application.
- Frontend dependency direction: presentation → application → domain; infrastructure implements the ports.
  - `wailsjs/` を import してよいのは `infrastructure/` だけ。依存の注入は合成ルートの `presentation/providers/*.tsx` と `App.tsx` で行う。`presentation/components/` から `infrastructure/` を import しない（biome の `noRestrictedImports` がエラーにする）。
  - ポートの置き場所は用途で分かれる。localStorage などの保存は `domain/<proto>/ports.ts`（例: `PresetStorage`、`ConnectionPersistence`、`ThemeStorage`）、RPC は使う側の application ファイルが定義する `XxxApi` インターフェース（例: `CollectionsApi`、`MqttConnectionApi`、`UdpSendApi`）。infrastructure のモジュールは `XxxApi` を import せず構造的に満たし、Provider がそのまま注入する（例: `udp-provider.tsx` の `udpClient`）。
  - `application/` から `infrastructure/` と `wailsjs/` を import しない（biome の `noRestrictedImports` がエラーにする）。外部作用は上記の置き場所にポートを定義し、合成ルート（`presentation/providers/*.tsx`、`App.tsx`）から注入する。Wails や保存領域に依存せず Web 標準 API だけで完結するヘルパー（例: `generateId`）は `shared/` に置き、どの層からも import してよい。
  - `infrastructure/` から `application/` への import（`logger/client.ts` → `application/logger`、`openapi/parser.ts` → `application/openapi/ports`）も既存のものだけで、増やさない。

### Backend (`internal/`)

- `domain/` — types, output port interfaces (repositories, transports, emitter, …), event name constants, errors. No dependencies on other layers. Subpackages per protocol: `http/`, `mqtt/`, `udp/`, `openapi/`.
- `application/` — services implementing use cases (`http/`, `mqtt/`, `udp/`, `openapi/`, `store/`). Depend only on domain ports.
- `infrastructure/` — port implementations: `JSONStore[T]` (generic per-entity JSON file persistence with atomic writes) wrapped by repositories that own the persistence DTOs, paho MQTT client factory + embedded mochi broker, HTTP `NetClient`, UDP socket, `WailsEmitter` (domain events → Wails runtime events), file logger (lumberjack), window state manager.
- `adapters/` — Wails-bound handler structs (`MQTTHandler`, `HTTPHandler`, `UDPHandler`, `LogHandler`, `OpenAPIHandler`) and the use-case input port interfaces they depend on. These are the RPC surface exposed to the frontend.
- `integration/` — cross-layer tests behind the `integration` build tag.

Wiring lives in `app.go`: handlers are created **empty** in `NewApp()` (so Wails can bind them in `main.go`), then `initialize()` builds services and injects them via `adapters.SetupXxxHandler(...)` during startup. If you add a service, follow this two-phase pattern. All persistent state is JSON under `os.UserConfigDir()/Wirexa/`.

App shutdown is two-step: `beforeClose` emits `app:before-close` and blocks the close; the frontend decides (unsaved-work check) and calls the `ConfirmQuit` RPC to actually quit.

### ドメイン型・RPC・永続化の境界

- **ドメイン型は RPC とイベントの配線型を兼ねる。** adapters に RPC 用の DTO は作らず、ハンドラはドメイン型をそのまま受け渡す。json タグは配線形式だけを表す。ドメイン型に載せてよいのは RPC で受け渡す業務上の値だけで、パスや一時的なハンドルのような infrastructure 内部の状態は載せない。
- **永続化形式は infrastructure のリポジトリにある `storedXxx` DTO が持つ**（`http/collection_repository.go`、`http/sidebar_layout_repository.go`、`mqtt/profile_repository.go`、`udp/target_repository.go`、`openapi/recent_repository.go`）。ドメイン型の json タグを変えても保存形式は変わらず、保存形式を変えても RPC は変わらない。保存形式の基準は各パッケージの `testdata/*.golden.json`。
- **stored DTO はどの階層でも domain 型を埋め込まない**（値型の小さな struct でも埋め込むと、その部分の保存形式が domain の json タグで決まる）。例外は `udpdomain.PayloadEncoding` のような名前付きの基本型だけ。`testutil.AssertNoTypesFrom` で検査している。
- **domain 型にフィールドを足すとき**、RPC には `task wails:generate` で反映される。保存対象のフィールドなら stored DTO と変換関数（`toStoredXxx` / `fromStoredXxx` など）にも足す。足し忘れは、`testutil.Populate` で全フィールドを埋めた値の往復テスト（各リポジトリの `*_RoundTripKeepsEveryField`）が検出する（テストの書き換えは不要）。意図して保存しないフィールドは、往復テストの期待値を補正する関数（例: `persistedCollection`）に理由と一緒に書く。
- **入力ポート（ユースケースの interface）は使う側の adapters が定義する。** domain には出力ポートだけを置く。

### 設定データの分類と復旧方針

保存データが壊れていたり読めなかったりしたときの扱いは、次の表で決める。同じ表を `internal/application/store/recovery.go` の doc コメントにも載せている（内容を変えるときは両方を直す）。

| 分類 | 対象ファイル | 破損時 | 退避失敗時 | 破損以外の読み込み失敗 |
| --- | --- | --- | --- | --- |
| **必須**（ユーザーが作成し、再生成できない） | `collections/*.json`、`mqtt-profiles/*.json`、`udp-targets/*.json`（`JSONStore`、エンティティ単位のファイル） | そのファイルだけ退避してスキップし、残りで起動する | スキップし、元ファイルは残す。起動時に自動作成する `__root__` は、ファイルが存在する限り作り直さない | ファイル単位ならスキップ（`__root__` の扱いは退避失敗時と同じ）。ディレクトリ自体を読めなければ起動失敗 |
| **best effort**（再生成できないが、失っても作業は続けられる） | `openapi-recents.json` | 退避して空の状態から始める | 空で始め、このセッションでは保存しない | 空で始め、このセッションでは保存しない |
| **再生成可能**（他のデータや既定値から作り直せる） | `sidebar_layout.json`（← collections）、`window-state.json`（← 既定サイズ） | 退避して再生成する | 再生成した内容で上書きしてよい | 再生成した値で動作を続け、ファイルは退避しない |

- 破損（`domain.ErrCorruptData`、JSON として解釈できない）と読み込み失敗（I/O エラー）を区別し、退避するのは破損したファイルだけ。分類は `infrastructure.ReadJSONFile` の 1 か所で決める。
- 退避先は `infrastructure.QuarantineFile`（`<path>.corrupt`、既存なら `<path>.corrupt.<unixnano>`）に統一する。
- 「読み込めなかった」を「存在しない」と同一視しない。必須データを自動作成するときは、ファイルが本当に無いことを確かめてから書く（`CollectionRepository.Exists`）。
- 単一ファイル型の best effort / 再生成可能データは `store.LoadSingleFile` にポリシー（`PolicyBestEffort` / `PolicyRegenerable`）を渡して読む。`window-state.json` は application 層を経由しないので、同じ方針を `LoadWindowState` に直接実装している。

### Frontend (`frontend/src/`)

- `domain/` — TS types per protocol (mirror of Go domain types).
- `application/` — use-case logic, framework-free.
- `infrastructure/` — the only layer that imports `wailsjs/` bindings; wraps generated Go calls and converts generated models to domain types.
- `presentation/` — SolidJS components, providers, UI utils.
- `shared/` — cross-cutting helpers incl. generated `wails-events.ts`.

### Testing layout

- Go & TS unit tests are colocated with the code (`*_test.go`, `*.test.ts`). Vitest runs in `node` env by default; add `// @vitest-environment jsdom` at the top of files that need DOM.
- **UI e2e** (`e2e/ui`, `playwright.config.ts`): `vite.e2e.config.ts` injects `e2e/fake-backend/install.ts`, which replaces the Wails-injected `window.go`/`window.runtime` with an in-memory implementation (state in sessionStorage), mirroring Go service semantics (e.g. `__root__` reserved collection). Runs fully parallel, needs no Go. If you change a bound API's behavior, the fake backend likely needs the same change.
- **Fullstack e2e** (`e2e/integration`, `playwright.integration.config.ts`): drives the real app via `wails dev` proxied to a `vite preview` of the built bundle (a dev-server proxy exhausts ephemeral ports on Windows). Isolates app data by overriding `APPDATA` to a temp dir. Sequential only (single app instance); Windows/local only, not in CI.
