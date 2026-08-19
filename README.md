# Kura

A personal agent OS: a local daemon (control plane) plus thin clients — a React
web UI, a full-screen Rust TUI, and chat-channel connectors.

## Architecture

| Path | Role |
|------|------|
| `crates/` | Rust workspace — the daemon control plane (runtime, LLM providers, channels/connectors, store, events, HTTP API, harness). Daemon binary: `dope-cli`; HTTP API: `dope-api`. |
| `web/` | React 19 + Vite web UI. |
| `crates/surface/tui/` | Rust full-screen terminal client (`dope-tui`). |
| `sdk/ts/` | TypeScript client SDK (`@dope/client`). |
| `schemas/` | JSON Schema contracts (API, events, config) — source of truth for cross-language contracts. |
| `docs/` | Planning and design docs, organized by module. |

The Go `daemon/` control plane was fully replaced by the Rust workspace and
removed; see `crates/MIGRATION.md` for the migration record.

## Build & Test

Daemon (Rust, from `crates/`):

```bash
make daemon-build              # cargo build --release -p dope-cli
make daemon-test               # cargo test --workspace
make daemon-contract-test      # cargo test -p dope-contracts
```

TUI (Rust):

```bash
cd crates && cargo build -p dope-tui
```

Clients (TypeScript):

```bash
pnpm build:clients             # build SDK + web
pnpm test:clients              # build + test SDK + web
```

## Run

```bash
make daemon-run-test           # test env: ~/.dope-test, 127.0.0.1:19192
make daemon-run-test-live      # test env with Discord enabled
make daemon-run-prod           # prod env: ~/.dope, 127.0.0.1:19191
make daemon-test-status        # health check
```

For local debugging, use the project skill at `.agents/skills/dope-test-env/SKILL.md`.
`make daemon-run-test` is the safe default and keeps Discord disabled unless you
opt in with `make daemon-run-test-live` or `DOPE_CONNECTORS_DISCORD_ENABLED=true`.

## Local Environment Modes

Development defaults to the **test** environment:

- `DOPE_ENV=test`
- data dir: `~/.dope-test`
- config file: `~/.dope-test/config.json`
- bind addr: `127.0.0.1:19192`

Production is explicit:

- `DOPE_ENV=prod`
- data dir: `~/.dope`
- config file: `~/.dope/config.json`
- bind addr: `127.0.0.1:19191`

Never touch prod config or live connectors without explicit intent; live
connectors are disabled by default in the test environment.

## Planning Docs

High-signal entry points:

- Product scope: `docs/product/product-outline.md`
- Runtime roadmap & tasks: `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-tasks.md`
- Provider architecture: `docs/providers/provider-architecture.md`
- Channel behavior: `docs/channels/channel-reply-progression.md`
- Harness architecture: `docs/harness/harness-architecture.md`
- Sandbox design: `docs/harness/sandbox-execution-plane.md`
- Test environment workflow: `docs/dev/test-environment-workflow.md`
- Module map: `docs/architecture/module-map.md`

## Working Assumptions

- OpenClaw is treated as a feature and product benchmark, not a required runtime base.
- Context engineering, memory, planning, handoff, and policy will be redesigned rather than lightly patched.
- Long-lived agent state must be observable, replayable, and safe to evolve.
