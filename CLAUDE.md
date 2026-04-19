# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
<!-- SPECKIT END -->

## Project Overview

DopeAgent is a personal agent OS with a Go daemon backend, React web UI, Node.js TUI, and a shared TypeScript client SDK. The daemon is the system spine owning runtime state, provider dispatch, policy gates, and event fan-out. Clients are thin consumers.

## Build & Development Commands

### Daemon (Go 1.24, in `daemon/`)

```bash
make daemon-build              # Build the daemon binary
make daemon-run-test           # Start daemon in test env (~/.dope-test, :19192)
make daemon-run-test-live      # Test env with live connectors (Discord enabled)
make daemon-run-prod           # Start daemon in prod env (~/.dope, :19191)
make daemon-test-status        # Health check test daemon
make daemon-prod-status        # Health check prod daemon
make daemon-test               # Run all Go tests (cd daemon && go test ./...)
make daemon-contract-test      # Run contract/schema validation tests
```

Run a single Go test:
```bash
cd daemon && go test ./internal/<package>/... -run TestName
```

### Clients (TypeScript, pnpm)

```bash
pnpm build:sdk                 # Build @dope/client SDK
pnpm build:web                 # Build web UI
pnpm build:tui                 # Build TUI
pnpm build:clients             # Build all clients (sdk -> web -> tui)
pnpm test:clients              # Build + test all clients + smoke test
pnpm dev:web                   # Start web dev server (Vite)
pnpm test:sdk                  # Test SDK only
pnpm test:web                  # Test web only
pnpm typecheck:web             # TypeScript type check for web
```

## Architecture

### System Boundaries

- **`daemon/`** -- Go control plane. Entry point: `daemon/cmd/dope/main.go`, wired in `daemon/internal/app/app.go`. Key packages under `daemon/internal/`: `runtime` (run/step lifecycle), `llm` (provider abstraction), `providers`/`managedproviders` (provider registry), `api` (HTTP + WebSocket), `events` (event append + fan-out), `store` (SQLite persistence), `sandbox` (isolated execution), `policy` (permission gates), `connectors` (Discord etc.), `skills` (skill registry), `config`, `auth`, `router`.

- **`sdk/ts/`** -- TypeScript client SDK (`@dope/client`). Exports `DopeClient` with `queryChat()` and `streamChatQuery()`. Used by both web and TUI.

- **`web/`** -- React 19 + Vite web UI. Uses `@dope/client` SDK. Generated types from schemas live in `web/src/generated/`.

- **`tui/`** -- Node.js terminal client (`dope-chat` command). Uses `@dope/client` SDK.

- **`schemas/`** -- JSON Schema contracts: `api/` (82 files), `events/` (49 files), `config/`, `capability/`, `plugin/`. Source of truth for cross-language contracts. Generated client code derives from these.

- **`capabilities/`** -- Supervised child processes for isolated features (browser, connectors, media, memory).

### Contract Flow

Schemas define the API surface. When changing API shape, event payloads, or config: update `schemas/` first, regenerate client types, then run `make daemon-contract-test`.

### Environment Modes

| Mode | Data dir | Bind address | Start command |
|------|----------|-------------|---------------|
| test (default) | `~/.dope-test` | `127.0.0.1:19192` | `make daemon-run-test` |
| prod | `~/.dope` | `127.0.0.1:19191` | `make daemon-run-prod` |

Always use the test environment for development. Never touch prod config or live connectors without explicit intent.

## Roadmap-Driven Development

Execution follows numbered roadmaps in `docs/runtime/daemon-roadmaps.md`. A roadmap is the delivery unit containing multiple tasks. A round is complete only when the entire roadmap is closed. High-signal docs:

- Product scope: `docs/product/product-outline.md`
- Roadmaps & tasks: `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-tasks.md`
- Provider architecture: `docs/providers/provider-architecture.md`
- Sandbox design: `docs/harness/sandbox-execution-plane.md`
- Test workflow: `docs/dev/test-environment-workflow.md`
