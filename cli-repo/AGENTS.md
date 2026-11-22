# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go TUI client for Docklite. Key paths:
- `main.go`: app entry point and startup wiring.
- `api/`: Docklite API client (`api/client.go`).
- `tui/`: Bubble Tea model/state and view rendering (`tui/model.go`, `tui/views.go`).
- `go.mod`, `go.sum`: module definition and dependencies.

## Build, Test, and Development Commands
- `go run .`: run the TUI locally.
- `go build -o docklite-tui`: build a local binary.
- `DOCKLITE_URL=http://localhost:9000 ./docklite-tui`: run a prebuilt binary against a specific API URL.
- `go mod download`: prefetch dependencies.

There is no dedicated test suite yet; use `go test ./...` after adding tests.

## Current Development Focus
- Recent work includes wiring container actions (start/stop/restart).
- When extending actions, keep `api/` calls, `tui/model.go` state updates, and `tui/views.go` status messaging in sync.

## Coding Style & Naming Conventions
- Use `gofmt` (tabs for indentation, canonical Go formatting).
- Follow Go naming: `CamelCase` for exported identifiers, `camelCase` for local names.
- Keep Bubble Tea update/view logic in `tui/` and API calls in `api/`.

## Testing Guidelines
- Preferred framework: Go’s standard `testing` package.
- Naming: `*_test.go`, with table-driven tests where helpful.
- Run tests with `go test ./...` and include any new coverage in PR notes.

## Commit & Pull Request Guidelines
- Commit history is minimal (single “Initial commit”), so use clear, imperative subjects like “Add container search” or “Refine summary view”.
- PRs should include: a short description, manual test notes, and a terminal screenshot or GIF for UI changes.
- Link related issues when applicable.

## Configuration Tips
- The TUI expects a running Docklite API; configure via `DOCKLITE_URL` (default `http://localhost:8080`).
- Keep new configuration options documented in `README.md` and `.env` examples if added later.
