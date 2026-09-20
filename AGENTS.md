# Repository Guidelines

## Project Structure & Module Organization

Wirexa is a Wails desktop application. `main.go` and `app.go` assemble it and expose handlers to the UI. Backend code lives under `internal/`: `domain/` contains types and ports, `application/` contains use cases, `infrastructure/` implements persistence and network clients, and `adapters/` provides Wails-facing handlers. Integration tests are in `internal/integration/`.

The SolidJS/TypeScript frontend is under `frontend/src/`, with matching architecture layers. Component styles use colocated `*.module.css` files. Browser tests live in `frontend/e2e/ui/` (fake backend) and `frontend/e2e/integration/` (real backend). Treat `frontend/wailsjs/` as generated code.

## Build, Test, and Development Commands

Run development, build, generation, formatting, lint, and test operations through
Taskfile tasks. Narrower operations the Taskfile does not cover — such as running
a single test — may call the underlying tool directly, for example
`go test ./internal/application/http/ -run TestName` or
`bunx vitest run src/shared/array.test.ts` inside `frontend/`.

- `task dev`: run the desktop app with frontend hot reload (`wails dev`).
- `task build`: build the desktop app (`wails build`).
- `task test`: run Go and frontend unit tests.
- `task lint`: run `go vet`, `golangci-lint`, Biome, and the frontend type check.
- `task format`: format the frontend with Biome.
- `task go:test` / `task go:vet` / `task go:lint`: the Go halves of the above.
- `task go:test:integration`: run backend integration tests (`integration` build tag).
- `task frontend:install`: install frontend dependencies with bun.
- `task frontend:tsc`: type-check the frontend.
- `task frontend:ci`: run the Biome check that CI runs.
- `task frontend:test:e2e`: run Playwright UI tests with the fake backend.
- `task frontend:test:e2e:fullstack`: run Playwright tests against the real backend
  (Windows/local only; not part of CI). Install the browser first with
  `bun run test:e2e:setup` in `frontend/`.
- `task licenses`: regenerate `THIRD_PARTY_LICENSES.md`.
- `task --list`: show all available tasks.

`task go:test` and `task go:vet` require `frontend/dist/index.html` to exist,
because of the `//go:embed` in `main.go`.

After changing a bound Go struct or handler method, run `task wails:generate` and
commit the resulting `frontend/wailsjs/` updates; CI fails on stale bindings
(`git diff --exit-code frontend/wailsjs`). After changing an event name in
`internal/domain/events.go`, run `task go:generate:events` to regenerate
`frontend/src/shared/wails-events.ts`; a new event also has to be added to the
list in `tools/gen-events/main.go`.

## Coding Style & Naming Conventions

Run `task format` to format TypeScript with Biome. Go formatting is enforced by
golangci-lint, which has `gofumpt` and `goimports` enabled as formatters, so
`task go:lint` reports formatting problems as issues. Follow standard Go naming
(`MixedCaps`, short package names) and handle errors explicitly. TypeScript uses
two-space indentation, double quotes, and organized imports. Use kebab-case
filenames such as `request-service.ts`; name tests `*_test.go`, `*.test.ts`, or
`*.spec.ts` according to their runner. Keep protocol-specific code in its HTTP,
MQTT, UDP, or OpenAPI package. Write code comments and commit messages in
Japanese.

## Testing Guidelines

Add focused tests beside changed code and regression tests for fixes. Prefer table-driven Go tests and deterministic Vitest tests. Use the fake-backend UI suite unless real backend behavior is the subject. No numeric coverage threshold is enforced; all CI checks must pass.

## Commit & Pull Request Guidelines

History follows Conventional Commit-style subjects: `feat(http): ...`, `refactor(udp): ...`, `test(e2e): ...`, or `chore: ...`. Keep subjects imperative and scoped where useful. Pull requests should explain behavior and architecture impacts, link relevant issues, list verification commands, and include screenshots or recordings for visible UI changes. Keep generated bindings and dependency lockfile changes in the same PR as their source change.
