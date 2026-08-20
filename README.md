# Kura

A personal agent OS: a local daemon (control plane) plus thin clients — a React
web UI, a full-screen Rust TUI, and chat-channel connectors.

## Architecture

| Path | Role |
|------|------|
| `crates/` | Rust workspace — the daemon control plane (runtime, LLM providers, channels/connectors, store, events, HTTP API, harness). User binary: `kura` (Cargo package: `kura-cli`); HTTP API: `kura-api`. |
| `web/` | React 19 + Vite web UI. |
| `crates/surface/tui/` | Rust full-screen terminal client (`kura-tui`; Cargo package: `kura-tui`). |
| `sdk/ts/` | TypeScript client SDK (`@kura/client`). |
| `schemas/` | JSON Schema contracts (API, events, config) — source of truth for cross-language contracts. |
| `docs/` | Planning and design docs, organized by module. |

The Go `daemon/` control plane was fully replaced by the Rust workspace and
removed; see `crates/MIGRATION.md` for the migration record.

## Build & Test

Daemon (Rust, from `crates/`):

```bash
make daemon-build              # emits crates/target/release/kura
make daemon-test               # cargo test --workspace
make daemon-contract-test      # cargo test -p kura-contracts
```

TUI (Rust):

```bash
cd crates && cargo build -p kura-tui  # emits target/debug/kura-tui
```

Clients (TypeScript):

```bash
pnpm build:clients             # build SDK + web
pnpm test:clients              # build + test SDK + web
```

## Run

```bash
make daemon-run-test           # test env: ~/.kura-test, 127.0.0.1:19192
make daemon-run-test-live      # test env with Discord enabled
make daemon-run-prod           # prod env: ~/.kura, 127.0.0.1:19191
make daemon-test-status        # health check
```

For local debugging, use the project skill at `.agents/skills/kura-test-env/SKILL.md`.
`make daemon-run-test` is the safe default and keeps Discord disabled unless you
opt in with `make daemon-run-test-live` or `KURA_CONNECTORS_DISCORD_ENABLED=true`.

## Local Environment Modes

Development defaults to the **test** environment:

- `KURA_ENV=test`
- data dir: `~/.kura-test`
- config file: `~/.kura-test/config.json`
- bind addr: `127.0.0.1:19192`

Production is explicit:

- `KURA_ENV=prod`
- data dir: `~/.kura`
- config file: `~/.kura/config.json`
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

## License

Kura is licensed under the [Apache License 2.0](LICENSE).
