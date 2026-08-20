# crates/ — Kura Rust Workspace

The Rust rewrite of the Kura daemon control plane. This workspace fully
replaces the former Go `daemon/` (deleted); see `crates/MIGRATION.md` for the
migration record.

## Layout

- `foundation/` — config, contracts, errors, ids, telemetry (shared building blocks)
- `engine/` — llm (dispatcher/providers), runtime (run/step/tool-call ledger), events, checkpoints, model-provider
- `iam/` — identity, auth, tenancy
- `channels/` — connectors (discord/slack/telegram/matrix), IM message loop
- `modeling/` — providers, opsreadiness, and other modeling crates
- `domains/` — chat, sandbox, mcp, skills, scheduler, delivery, calendar, mail, reminders, workflows, evaluation, and the rest of the domain managers
- `persistence/` — SQLite store (`kura-store`) + DAOs
- `surface/` — HTTP API (`kura-api`), CLI package (`kura-cli`, binary `kura`), terminal package (`kura-tui`, binary `kura-tui`)

## Key binaries

| Cargo package | Path | Purpose |
|---------------|------|---------|
| `kura-cli` | `surface/cli` | daemon entry point; emits the user-facing `kura` binary |
| `kura-tui` | `surface/tui` | emits the user-facing `kura-tui` terminal client |

## Commands

```bash
cargo build --workspace     # or: make rs-build
cargo test --workspace      # or: make rs-test
cargo clippy --workspace --all-targets   # or: make rs-clippy
cargo fmt --all -- --check
```

Build the daemon binary:

```bash
cargo build --release -p kura-cli
```
