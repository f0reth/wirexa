# Repository Guidelines

## Project Structure & Module Organization

Wirexa is a Wails desktop application. `main.go` and `app.go` assemble it and expose handlers to the UI. Backend code lives under `internal/`: `domain/` contains types and ports, `application/` contains use cases, `infrastructure/` implements persistence and network clients, and `adapters/` provides Wails-facing handlers. Integration tests are in `internal/integration/`.

The SolidJS/TypeScript frontend is under `frontend/src/`, with matching architecture layers. Component styles use colocated `*.module.css` files. Browser tests live in `frontend/e2e/ui/` (fake backend) and `frontend/e2e/integration/` (real backend). Treat `frontend/wailsjs/` as generated code.

## Build, Test, and Development Commands

- `wails dev`: run the desktop app with frontend hot reload.
- `wails build`: produce a release build in `build/bin/`.
- `go test ./...`: run Go unit tests.
- `go test -tags integration -v ./internal/integration/...`: run backend integration tests.
- `golangci-lint run`: apply the Go checks configured in `.golangci.yml`.
- `cd frontend && bun install`: install locked frontend dependencies.
- `cd frontend && bun run build`: type-check and build the UI.
- `cd frontend && bun run test`: run Vitest unit tests.
- `cd frontend && bun run ci`: run Biome formatting and lint checks.
- `cd frontend && bun run test:e2e`: run Playwright UI tests (after `bun run test:e2e:setup`).

After changing exported Go bindings, run `wails generate module` and commit the resulting `frontend/wailsjs/` updates.

## Coding Style & Naming Conventions

Format Go with `gofumpt` and `goimports`; follow standard Go naming (`MixedCaps`, short package names) and handle errors explicitly. TypeScript uses Biome: two-space indentation, double quotes, and organized imports. Use kebab-case filenames such as `request-service.ts`; name tests `*_test.go`, `*.test.ts`, or `*.spec.ts` according to their runner. Keep protocol-specific code in its HTTP, MQTT, UDP, or OpenAPI package.

## Testing Guidelines

Add focused tests beside changed code and regression tests for fixes. Prefer table-driven Go tests and deterministic Vitest tests. Use the fake-backend UI suite unless real backend behavior is the subject. No numeric coverage threshold is enforced; all CI checks must pass.

## Commit & Pull Request Guidelines

History follows Conventional Commit-style subjects: `feat(http): ...`, `refactor(udp): ...`, `test(e2e): ...`, or `chore: ...`. Keep subjects imperative and scoped where useful. Pull requests should explain behavior and architecture impacts, link relevant issues, list verification commands, and include screenshots or recordings for visible UI changes. Keep generated bindings and dependency lockfile changes in the same PR as their source change.
