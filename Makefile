.PHONY: build-all build-agent build-cli build-tui build-gui run-agent run-tui run-gui install-gui dev-gui clean test test-go test-web test-coverage

ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BIN := $(ROOT)/bin

# Build everything (agent, TUI, and GUI)
build-all: build-agent build-tui build-gui

# Build the Go agent (HTTP API server)
build-agent:
	mkdir -p $(BIN)
	cd go-app && go build -o $(BIN)/docklite-agent ./cmd/docklite-agent

# Build the CLI tool (alias for TUI)
build-cli: build-tui

# Build the standalone TUI client
build-tui:
	mkdir -p $(BIN)
	cd cli-repo && go build -o $(BIN)/docklite-tui .

# Build the Next.js GUI for production
build-gui:
	cd webapp && bun install && bun run build

# Install GUI dependencies only (without building)
install-gui:
	cd webapp && bun install

# Run the agent in development mode
run-agent:
	cd go-app && DATABASE_PATH=../data/docklite.db LISTEN_ADDR=:9000 go run ./cmd/docklite-agent

# Run the TUI in development mode
run-tui:
	cd cli-repo && go run .

# Run the GUI in development mode (port 3000)
run-gui:
	cd webapp && bun run dev

# Run the GUI in production mode (port 3000)
dev-gui:
	cd webapp && bun run dev

# Start GUI in production mode (port 3000, or use PORT env var)
start-gui:
	cd webapp && bun run start

# Clean built binaries and Next.js build artifacts
clean:
	rm -rf $(BIN)
	rm -rf webapp/.next
	rm -rf webapp/node_modules

# Run all tests
test: test-go test-web

# Run Go tests
test-go:
	cd go-app && go test ./...

# Run webapp tests
test-web:
	cd webapp && bun run test

# Run tests with coverage
test-coverage:
	cd go-app && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
	cd webapp && bun run test --coverage

# Install test dependencies
install-test-deps:
	cd webapp && bun install
	@echo "Test dependencies installed"
