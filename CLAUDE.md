# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Historical feature scope lives in plain markdown under `specs/<NNN>-<name>/` and `docs/specs/` (Spec Kit tooling was removed; specs 001-062 are frozen history). Since 2026-08-17 new features are planned directly in design docs (e.g. `docs/harness/plugin-architecture.md`) and implemented — no spec numbering.

## Project Overview

Kura is a personal agent OS with a Rust daemon backend, React web UI, Rust TUI (`dope-tui`), and a shared TypeScript client SDK. The daemon is the system spine owning runtime state, provider dispatch, policy gates, and event fan-out. Clients are thin consumers.

## Build & Development Commands

### Daemon (Rust, in `crates/`)

```bash
make daemon-build              # cargo build --release -p dope-cli
make daemon-run-test           # Start daemon in test env (~/.dope-test, :19192)
make daemon-run-test-live      # Test env with live connectors (Discord enabled)
make daemon-run-prod           # Start daemon in prod env (~/.dope, :19191)
make daemon-test-status        # Health check test daemon
make daemon-prod-status        # Health check prod daemon
make daemon-test               # cargo test --workspace
make daemon-contract-test      # cargo test -p dope-contracts
```

Run a single test:
```bash
cd crates && cargo test -p <crate> -- <filter>
```

### Clients (TypeScript, pnpm)

```bash
pnpm build:sdk                 # Build @dope/client SDK
pnpm build:web                 # Build web UI
pnpm build:clients             # Build all clients (sdk -> web)
pnpm test:clients              # Build + test all clients
pnpm dev:web                   # Start web dev server (Vite)
pnpm test:sdk                  # Test SDK only
pnpm test:web                  # Test web only
pnpm typecheck:web             # TypeScript type check for web
```

## Architecture

### System Boundaries

- **`crates/`** -- Rust workspace, the daemon control plane. Entry point: `dope-cli` (`crates/surface/cli`), wired by `dope-app` (`crates/surface/app`), HTTP API in `dope-api` (`crates/surface/api`). Key groups: `foundation/` (config, contracts, ids, telemetry), `engine/` (llm, runtime, events, checkpoints), `iam/` (identity, tenancy, secrets), `channels/` (connectors, im), `modeling/` (providers, adapters), `domains/` (chat, sandbox, mcp, skills, scheduler, delivery, calendar, mail, reminders, workflows, evaluation, policy, ...), `persistence/` (SQLite store), `surface/` (api, cli, tui).

- **`sdk/ts/`** -- TypeScript client SDK (`@dope/client`). Exports `DopeClient` with `queryChat()` and `streamChatQuery()`. Used by the web client.

- **`web/`** -- React 19 + Vite web UI. Uses `@dope/client` SDK. Generated types from schemas live in `web/src/generated/`.

- **`crates/surface/tui/`** -- Rust terminal client (`dope-tui` binary), the full-screen Claude-Code-style TUI.

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
