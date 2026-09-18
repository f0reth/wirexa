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
task format  # フロントエンドのコードを整形する
```

### Go (repo root)

```
task go:test              # unit tests (go test ./...)
task go:test:integration  # integration tests (build tag required)
task go:vet               # go vet ./...
task go:lint              # golangci-lint run
```

単体テストの絞り込みなど Taskfile がカバーしていない操作は、素の `go test ./internal/application/http/ -run TestName` を直接使ってよい。

Go tests and `go vet` require `frontend/dist/index.html` to exist (the `//go:embed` in main.go fails otherwise). CI stubs it with `mkdir -p frontend/dist && touch frontend/dist/index.html`.

Go lint is golangci-lint (v7 action in CI). Wails CLI version must match go.mod (`v2.12.0`).

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

Both sides follow the same layered (ports & adapters) structure. Dependency direction: adapters/presentation → application → domain; infrastructure implements domain ports.

### Backend (`internal/`)

- `domain/` — types, port interfaces, event name constants, errors. No dependencies on other layers. Subpackages per protocol: `http/`, `mqtt/`, `udp/`.
- `application/` — services implementing use cases (`http/`, `mqtt/`, `udp/`, `store/`). Depend only on domain ports.
- `infrastructure/` — port implementations: `JSONStore[T]` (generic per-entity JSON file persistence with atomic writes), paho MQTT client factory + embedded mochi broker, HTTP `NetClient`, UDP socket, `WailsEmitter` (domain events → Wails runtime events), file logger (lumberjack), window state manager.
- `adapters/` — Wails-bound handler structs (`MQTTHandler`, `HTTPHandler`, `UDPHandler`, `LogHandler`, `OpenAPIHandler`). These are the RPC surface exposed to the frontend.
- `integration/` — cross-layer tests behind the `integration` build tag.

Wiring lives in `app.go`: handlers are created **empty** in `NewApp()` (so Wails can bind them in `main.go`), then `initialize()` builds services and injects them via `adapters.SetupXxxHandler(...)` during startup. If you add a service, follow this two-phase pattern. All persistent state is JSON under `os.UserConfigDir()/Wirexa/`.

App shutdown is two-step: `beforeClose` emits `app:before-close` and blocks the close; the frontend decides (unsaved-work check) and calls the `ConfirmQuit` RPC to actually quit.

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
