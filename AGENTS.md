# AGENTS.md

This file provides guidance to coding agents working with code in this repository.

## Overview

Wirexa is a Wails v2 desktop app (Windows-focused) for network testing: HTTP requests/collections, MQTT client, UDP send/listen, and an OpenAPI viewer. Backend is Go; frontend is SolidJS + TypeScript built with Vite and managed with **bun** (not npm).

Code comments and commit messages are written in Japanese — follow that convention.

This is an internal tool that is still under development, so do not keep backward compatibility or migrate data. Always rewrite the code into its ideal, KISS form.

## Commands

Starting the dev environment, building, generating, formatting, linting and testing all go through **`task` commands defined in Taskfile.yml** (`task --list` shows them all).

```
# Whole project
task dev      # start the dev environment (wails dev)
task build    # production build (wails build)
task test     # run the Go and frontend unit tests together
task lint     # go:vet, go:lint, go:lint:integration, frontend:lint, frontend:tsc
task format   # format the Go and frontend code (go:fmt, frontend:format)
task licenses # regenerate THIRD_PARTY_LICENSES.md

# Go (repo root)
task go:test              # unit tests (go test ./...)
task go:test:race         # unit tests with -race (what CI runs; needs cgo/gcc on Windows)
task go:test:integration  # integration tests (build tag required)
task go:vet               # go vet ./...
task go:lint              # golangci-lint run
task go:lint:integration  # golangci-lint over internal/integration with the integration build tag
task go:fmt               # golangci-lint fmt (gofumpt, goimports)

# Frontend
task frontend:install            # bun install
task frontend:tsc                # typecheck (tsc -b)
task frontend:test               # vitest unit tests
task frontend:lint               # biome check
task frontend:format             # biome check --write
task frontend:ci                 # biome ci (what CI runs)
task frontend:test:e2e           # Playwright UI e2e (fake backend; no Go/wails needed)
task frontend:test:e2e:fullstack # Playwright against real wails dev (Windows/local only)
```

For anything the Taskfile does not cover, such as running a single test, call the tool directly: `go test ./internal/application/http/ -run TestName` from the repo root, or `bunx vitest run src/shared/array.test.ts` inside `frontend/`. Install the Playwright browser once with `bun run test:e2e:setup` inside `frontend/`.

Go tests, `go vet` and golangci-lint require `frontend/dist/index.html` to exist (the `//go:embed` in main.go fails otherwise). CI stubs it with `mkdir -p frontend/dist && touch frontend/dist/index.html`.

Go lint is golangci-lint (v7 action in CI). Wails CLI version must match go.mod (`v2.16.0`).

### Code generation (two mechanisms — both checked/needed when crossing the Go↔TS boundary)

1. **Wails bindings** (`frontend/wailsjs/` — generated, do not edit): after changing any bound Go struct or handler method, run `task wails:generate` from the repo root. CI fails if bindings are stale (`git diff --exit-code frontend/wailsjs`).
2. **Event names** (`frontend/src/shared/wails-events.ts` — generated, do not edit): event name constants live only in `internal/domain/events.go`. Regenerate with `task go:generate:events`. Adding a new event also requires adding it to the list in `tools/gen-events/main.go`.

## Coding style

- **Go**: formatting is gofumpt + goimports, applied by `task go:fmt`; `task go:lint` also reports unformatted code as issues. Linter set and settings are in `.golangci.yml`. File names are snake_case.
- **TypeScript**: Biome (`frontend/biome.json`) formats and lints — two-space indentation, double quotes, organized imports. `task frontend:format` applies it. Biome does not cover `frontend/e2e/` or `frontend/wailsjs/`; write e2e code in the same style by hand.
- **File names** in `frontend/` are kebab-case (`udp-provider.tsx`, `async-op.ts`); the only exception is `App.tsx` / `App.module.css`. Component styles are colocated `*.module.css` files.
- **Tests** are named by runner: `*_test.go` (Go), `*.test.ts` (Vitest), `*.spec.ts` (Playwright).

## Architecture

Both sides follow the same layered (ports & adapters) structure. `main.go` and `app.go` at the repo root assemble the app.

### Backend (`internal/`)

Dependency direction (compile-time imports): adapters → domain ← application, infrastructure.

- `domain/` — types, output port interfaces (repositories, transports, emitter, …), event name constants, errors. No dependencies on other layers. Holds output ports only. Subpackages per protocol: `http/`, `mqtt/`, `udp/`, `openapi/`.
- `application/` — services implementing use cases (`http/`, `mqtt/`, `udp/`, `openapi/`, `store/`). Depend only on domain ports.
- `infrastructure/` — implementations of the domain's output ports: `JSONStore[T]` (generic per-entity JSON file persistence with atomic writes) wrapped by repositories, paho MQTT client factory, HTTP `NetClient`, UDP socket, `WailsEmitter` (domain events → Wails runtime events), file logger (lumberjack), window state manager.
- `adapters/` — Wails-bound handler structs (`MQTTHandler`, `HTTPHandler`, `UDPHandler`, `LogHandler`, `OpenAPIHandler`), the RPC surface exposed to the frontend. Adapters never import application: each handler file defines the use-case input ports it calls (`HTTPRequestUseCase`, `MQTTProfileUseCase`, …), application services satisfy them structurally, and `app.go` checks that at compile time. A handler may also take a domain output port directly through its `SetupXxxHandler` deps when no use case sits in between (`HTTPHandlerDeps.Responses` is a `ResponseBodyStore`, `SetupLogHandler` takes a `domain.Logger`).
- `integration/` — cross-layer tests behind the `integration` build tag.
- `testutil/` — test helpers shared across packages (`Populate`, `AssertNoTypesFrom`, …).

Wiring lives in `app.go`: handlers are created **empty** in `NewApp()` (so Wails can bind them in `main.go`), then `initialize()` builds services and injects them via `adapters.SetupXxxHandler(...)` during startup. If you add a service, follow this two-phase pattern. All persistent state is JSON under `os.UserConfigDir()/Wirexa/`.

App shutdown is two-step: `beforeClose` emits `app:before-close` and blocks the close; the frontend decides (unsaved-work check) and calls the `ConfirmQuit` RPC to actually quit.

### Frontend (`frontend/src/`)

Dependency direction: presentation → application → domain; infrastructure implements the ports.

- `domain/` — TS types per protocol (mirror of Go domain types).
- `application/` — use-case logic, framework-free. Must not import `infrastructure/` or `wailsjs/` (biome's `noRestrictedImports` reports it as an error).
- `infrastructure/` — the only layer that may import `wailsjs/`; wraps generated Go calls and converts generated models to domain types. Imports from `infrastructure/` into `application/` are limited to the existing ones (`logger/client.ts` → `application/logger`, `openapi/parser.ts` → `application/openapi/ports`). Do not add more.
- `presentation/` — SolidJS components, providers, UI utils. Dependencies are injected at the composition roots, `presentation/providers/*.tsx` and `App.tsx`. `presentation/components/` must not import `infrastructure/` (also a `noRestrictedImports` error).
- `shared/` — cross-cutting helpers incl. generated `wails-events.ts`. Helpers that need only Web standard APIs and depend on neither Wails nor storage (e.g. `generateId`) go here and may be imported from any layer.
- `components/ui/` — protocol-agnostic UI primitives (button, input, dialog, tabs, toast, …) used by `presentation/` and `App.tsx`. Protocol-specific components go in `presentation/components/<proto>/`.
- `config/limits.ts` — size limits (max kept messages/topics) shared by application and providers.

Rules:

- Where a port lives depends on what it is for. Storage such as localStorage goes in `domain/<proto>/ports.ts` (e.g. `PresetStorage`, `ConnectionPersistence`, `ThemeStorage`). RPC goes in an `XxxApi` interface defined by the application file that uses it (e.g. `CollectionsApi`, `MqttConnectionApi`, `UdpSendApi`). Infrastructure modules satisfy `XxxApi` structurally without importing it, and the provider injects them as they are (e.g. `udpClient` in `udp-provider.tsx`). For any external effect, define a port this way and inject it from a composition root.
- Effects in `presentation/` must not write back to application state (calling a setter from a user-action event handler is fine). Derived state such as a filtered list, and the effects that follow it, belong in the application layer, and each piece of state is written by an effect in one place only (two effects that follow different sources keep overwriting each other and never settle).

### Boundaries between domain types, RPC and persistence

- **Domain types double as the wire types for RPC and events.** Adapters define no RPC DTOs; handlers pass domain types through as they are. The json tags describe the wire format only. A domain type carries only the business values exchanged over RPC, never infrastructure-internal state such as paths or temporary handles.
- **The persistence format is owned by the `storedXxx` DTOs in the infrastructure repositories** (`http/stored_collection.go`, `http/sidebar_layout_repository.go`, `mqtt/profile_repository.go`, `udp/target_repository.go`, `openapi/recent_repository.go`). Changing a domain type's json tags does not change the stored format, and changing the stored format does not change RPC. The reference for the stored format is each package's `testdata/*.golden.json`.
- **A stored DTO never embeds a domain type at any depth** (embedding even a small value struct makes the domain's json tags decide the stored format of that part). The only exception is named basic types such as `udpdomain.PayloadEncoding`. `testutil.AssertNoTypesFrom` checks this.
- **When you add a field to a domain type**, `task wails:generate` carries it to RPC. If the field is persisted, also add it to the stored DTO and the conversion functions (`toStoredXxx` / `fromStoredXxx` and the like). A forgotten field is caught by the round-trip tests that fill every field with `testutil.Populate` (`*_RoundTripKeepsEveryField` in each repository), with no change to the tests. A field that is deliberately not persisted is listed, with the reason, in the function that adjusts the round-trip test's expected value (e.g. `persistedCollection`).

### Classification of stored data and recovery policy

Stored data falls into three classes, which decide what happens when a file is corrupt or cannot be read. The full table (when corrupt / when quarantine fails / other read failure, per class) is defined only in the doc comment of `internal/application/store/recovery.go`. Read it before changing persistence or startup loading.

- **Required** (created by the user, cannot be regenerated): `collections/*.json`, `mqtt-profiles/*.json`, `udp-targets/*.json` (`JSONStore`, one file per entity). A bad file is skipped and the rest loads.
- **Best effort** (cannot be regenerated, but work can continue without it): `openapi-recents.json`. Starts empty.
- **Regenerable** (can be rebuilt from other data or defaults): `sidebar_layout.json` (← collections), `window-state.json` (← default size).

Rules:

- `__root__`, which is created automatically at startup, is not recreated as long as its file exists, whether quarantine failed or the file could not be read.
- Corruption (`domain.ErrCorruptData`, not parseable as JSON) and read failure (I/O error) are distinguished, and only corrupt files are quarantined. The classification is decided in one place, `infrastructure.ReadJSONFile`.
- Quarantine always goes through `infrastructure.QuarantineFile` (`<path>.corrupt`, or `<path>.corrupt.<unixnano>` if that already exists).
- "Could not be read" is never treated as "does not exist". Before automatically creating required data, confirm that the file is really absent (`CollectionRepository.Exists`).
- Single-file best effort / regenerable data is read by passing a policy (`PolicyBestEffort` / `PolicyRegenerable`) to `store.LoadSingleFile`. `window-state.json` does not go through the application layer, so `LoadWindowState` implements the same policy directly.

### Testing layout

- Go & TS unit tests are colocated with the code (`*_test.go`, `*.test.ts`). Vitest runs in `node` env by default; add `// @vitest-environment jsdom` at the top of files that need DOM. Add a regression test with every fix.
- **UI e2e** (`e2e/ui`, `playwright.config.ts`): `vite.e2e.config.ts` injects `e2e/fake-backend/install.ts`, which replaces the Wails-injected `window.go`/`window.runtime` with an in-memory implementation (state in sessionStorage), mirroring Go service semantics (e.g. `__root__` reserved collection). Runs fully parallel, needs no Go. Playwright serves a built bundle (`bun run build:e2e` → `dist-e2e`, then `vite preview` on port 5175), not the dev server: the dev server returns one module per request and made every page load take seconds. The bundle is rebuilt on every run and never goes to `dist` (which `//go:embed` and `wails build` use). Set `WIREXA_E2E_DEV=1` to use the dev server (`bun run dev:e2e`, port 5173) instead, e.g. when editing app source under `playwright test --ui`. If you change a bound API's behavior, the fake backend likely needs the same change. Use this suite unless real backend behavior is the subject.
- **Fullstack e2e** (`e2e/integration`, `playwright.integration.config.ts`): drives the real app via `wails dev` proxied to a `vite preview` of the built bundle (a dev-server proxy exhausts ephemeral ports on Windows). Isolates app data by overriding `APPDATA` to a temp dir. MQTT specs connect to a mochi-mqtt broker started from `tools/e2e-broker` (the same broker the Go integration tests in `internal/integration/` embed). Sequential only (single app instance); Windows/local only, not in CI.

## Commits

Subjects follow Conventional Commits with a Japanese description: `fix(mqtt): 未接続のときはトピック欄の Enter で購読しない`, `test(e2e): …`, `docs: …`. Types in use are `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `perf`; the scope is the protocol or area (`http`, `mqtt`, `udp`, `openapi`, `store`, `frontend`, `e2e`) and is optional. Commit regenerated bindings (`frontend/wailsjs/`, `wails-events.ts`) and `bun.lock` changes together with the source change that caused them.
