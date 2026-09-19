# Repository Guidelines

## Project Structure & Module Organization

Wirexa is a Wails desktop application. `main.go` and `app.go` assemble it and expose handlers to the UI. Backend code lives under `internal/`: `domain/` contains types and ports, `application/` contains use cases, `infrastructure/` implements persistence and network clients, and `adapters/` provides Wails-facing handlers. Integration tests are in `internal/integration/`.

The SolidJS/TypeScript frontend is under `frontend/src/`, with matching architecture layers. Component styles use colocated `*.module.css` files. Browser tests live in `frontend/e2e/ui/` (fake backend) and `frontend/e2e/integration/` (real backend). Treat `frontend/wailsjs/` as generated code.

## Build, Test, and Development Commands

Run development, build, generation, formatting, lint, and test operations through
Taskfile tasks. Do not invoke the underlying `go`, `bun`, `wails`, or
`golangci-lint` commands directly when an equivalent task exists.

- `task setup`: install Go and frontend dependencies plus Playwright Chromium.
- `task dev`: run the desktop app with frontend hot reload.
- `task dev:frontend`: run only the Vite development server.
- `task build`: build the desktop app in `build/bin/`.
- `task build:release`: build the optimized distributable desktop app.
- `task build:frontend`: type-check and build the UI.
- `task test`: run Go and frontend unit tests.
- `task test:go:integration`: run backend integration tests.
- `task test:e2e`: run Playwright UI tests with the fake backend.
- `task test:e2e:fullstack`: run Playwright tests against the real backend.
- `task test:all`: run every unit, integration, and E2E test suite.
- `task lint`: run Go and frontend lint checks.
- `task format`: format Go and frontend sources.
- `task typecheck`: type-check the frontend.
- `task check`: run unit tests, type-checking, and lint checks.
- `task ci`: run the local CI-equivalent checks except fullstack E2E.
- `task --list`: show all available tasks, including narrower component tasks.

Pass additional tool arguments after `--`, for example `task test:go -- -race`.
After changing exported Go bindings, run `task generate` and commit the resulting
`frontend/wailsjs/` updates. Use `task generate:check` to verify that committed
bindings are current.

## Coding Style & Naming Conventions

Run `task format` to format Go with `gofumpt` and `goimports` and TypeScript with
Biome. Follow standard Go naming (`MixedCaps`, short package names) and handle
errors explicitly. TypeScript uses two-space indentation, double quotes, and
organized imports. Use kebab-case filenames such as `request-service.ts`; name
tests `*_test.go`, `*.test.ts`, or `*.spec.ts` according to their runner. Keep
protocol-specific code in its HTTP, MQTT, UDP, or OpenAPI package.

## Testing Guidelines

Add focused tests beside changed code and regression tests for fixes. Prefer table-driven Go tests and deterministic Vitest tests. Use the fake-backend UI suite unless real backend behavior is the subject. No numeric coverage threshold is enforced; all CI checks must pass.

## Commit & Pull Request Guidelines

History follows Conventional Commit-style subjects: `feat(http): ...`, `refactor(udp): ...`, `test(e2e): ...`, or `chore: ...`. Keep subjects imperative and scoped where useful. Pull requests should explain behavior and architecture impacts, link relevant issues, list verification commands, and include screenshots or recordings for visible UI changes. Keep generated bindings and dependency lockfile changes in the same PR as their source change.
