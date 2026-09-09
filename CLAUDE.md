# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`AGENTS.md` holds the repository conventions (coding style, naming, testing, commit/PR rules). Read it too — this file covers commands and the architecture that spans multiple files.

## Commands

Everything goes through [Taskfile.yml](Taskfile.yml). Do not call `go`, `bun`, `wails`, or `golangci-lint` directly when a task exists — several tasks have prerequisites (see "Embedded frontend/dist" below). `task --list` shows the full set.

| Task | Purpose |
| --- | --- |
| `task setup` | Go + Bun deps and Playwright Chromium |
| `task dev` | Wails app with frontend hot reload |
| `task check` | unit tests + typecheck + lint (the usual pre-commit gate) |
| `task ci` | `check` plus Go integration tests, UI E2E, and binding freshness |
| `task test:go` / `task test:frontend` | Go and Vitest unit tests |
| `task test:go:integration` | Go integration tests (build tag `integration`) |
| `task test:e2e` / `task test:e2e:fullstack` | Playwright against the fake backend / the real Wails app |
| `task generate` | regenerate `frontend/wailsjs/` bindings; `task generate:check` verifies they're committed |
| `task format` / `task lint` | gofumpt+goimports / Biome |

Extra tool args go after `--`:

```
task test:go -- -race -run TestCollectionService ./internal/application/http/...
task test:frontend -- src/application/http/request.test.ts
task test:e2e -- e2e/ui/http/http.spec.ts --headed
task lint:go -- --fix
```

Go and frontend versions come from `go.mod` (Go 1.26) and `frontend/package.json` (Bun, TypeScript 6, Vite 8, Vitest 4). Wails and golangci-lint are pinned as `go run` module versions in the Taskfile, so no separate install is needed.

## Architecture

Wirexa is a Wails v2 desktop client for four protocols — MQTT, HTTP, UDP, and OpenAPI — with a SolidJS/TypeScript UI. Both halves use the same layering: `domain` (types + ports) ← `application` (use cases) ← `infrastructure` (network, disk, Wails runtime) and `adapters` (Go) / `presentation` (frontend) at the edge.

**Composition root.** `app.go` `initialize()` builds every repository, service, and handler and injects them; `main.go` only wires Wails options. Handlers are allocated empty in `NewApp()` *before* `wails.Run` because `Bind` needs the pointers, then filled by `adapters.Setup*Handler(...)` during `startup`. That is why handler fields are unexported and set via `Setup*` rather than a constructor. If startup fails, `a.ready` stays false and `shutdown` skips teardown.

**RPC surface.** Every exported method on the bound handlers becomes a frontend call. After changing one, run `task generate` and commit the `frontend/wailsjs/` diff — `task ci` fails otherwise. `frontend/wailsjs/` is generated; never hand-edit it.

**Events.** Backend→frontend event names live only in `internal/domain/events.go`. `go generate ./internal/domain` (via `tools/gen-events`) writes `frontend/src/shared/wails-events.ts`. Add an event by editing both `events.go` and the `events` slice in `tools/gen-events/main.go`, then regenerating — never by typing a string literal on either side.

**Persistence.** User data lives under `os.UserConfigDir()/Wirexa/`: per-entity JSON directories (`collections/`, `mqtt-profiles/`, `udp-targets/`), plus `sidebar_layout.json`, `window-state.json`, `openapi-recents.json`, and `logs/`. `infrastructure.JSONStore[T]` writes atomically and quarantines unparseable files to `*.corrupt` instead of failing startup; `application/store.CachedStore[T]` wraps any `Load/Save/Delete` repository with an in-memory map and ID assignment. Both are generic — protocol stores are instantiations, not new implementations.

**Security boundary.** Wails RPC is callable by any JS in the WebView, so handlers that touch the filesystem must not trust paths from the frontend. `OpenAPIHandler` keeps a `granted` allowlist seeded only from OS file dialogs and persisted recents. The HTTP side still passes raw paths (file bodies, form-data files, `SaveResponseBody` temp paths); `docs/http-local-file-access-hardening.md` specifies the opaque-handle design that replaces them, and `docs/backend-architecture-review.md` has the wider findings list. Treat those two docs as the current plan of record when touching file I/O.

**Large responses.** `httpinfra.NetClient` spills over-limit response bodies to temp files keyed by request ID; the path is handed to the frontend once via `ConsumeTempFilePath`. Leftovers are cleared by `Cleanup()` on shutdown and `SweepStaleTempFiles()` at startup (safe because the single-instance lock guarantees no peer process).

**Quit flow.** Only the frontend knows about unsaved work, so `beforeClose` emits `app:before-close` and vetoes the close; the UI calls `ConfirmQuit()` to let it through.

**Frontend state.** `presentation/providers/*-provider.tsx` are the DI seams: each builds application-layer stores (`createCollectionsState`, `createRequestState`, …) from an infrastructure client module and exposes them through a Solid context. Application stores take their API as a parameter, so unit tests inject fakes without touching Wails. All four protocol panels stay mounted after first visit (`App.tsx` keeps a `visited` set) — connections and buffers survive protocol switching.

## Gotchas

**Embedded `frontend/dist`.** `main.go` has `//go:embed all:frontend/dist`, but `dist` is gitignored, so any bare `go build`/`go test`/`golangci-lint` fails on a clean checkout. The `internal:frontend-dist` task creates a placeholder `index.html` first; this is the main reason to use tasks rather than raw commands (CI does the same `mkdir`/`touch` explicitly).

**Two E2E suites, two configs.** `playwright.config.ts` runs `e2e/ui/` against `e2e/fake-backend/install.ts`, which replaces `window.go`/`window.runtime` with an in-memory implementation mirroring the Go service semantics (`__root__` reserved collection, sidebar layout coupling) — fully parallel, no Go required. When you change backend behavior the fake backend must be updated to match. `playwright.integration.config.ts` runs `e2e/integration/` against a real `wails dev`, serialized, with `APPDATA` pointed at a temp dir for isolation, and serves a pre-built bundle through `vite preview` rather than the Vite dev server. Prefer the fake-backend suite unless real backend behavior is the subject.

**Comments and docs are in Japanese.** Code comments, `Taskfile.yml` descriptions, and `docs/` are Japanese; match that when editing those files. Identifiers, commit subjects, and `AGENTS.md` are English.

**Lint is strict.** `.golangci.yml` enables `gosec`, `errcheck` with `check-blank` and type assertions, `revive`'s `exported` rule (every exported symbol needs a doc comment), and `nolintlint` (every `//nolint` needs a reason). Wails DTOs passed by value trip `gocritic` `hugeParam` — the existing suppressions on RPC methods are deliberate; follow that pattern rather than switching to pointers.
