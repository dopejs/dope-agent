# crates/ — DopeAgent Rust Workspace

The Rust rewrite of the DopeAgent daemon control plane. This workspace fully
replaces the former Go `daemon/` (deleted); see `crates/MIGRATION.md` for the
migration record.

## Layout

- `foundation/` — config, contracts, errors, ids, telemetry (shared building blocks)
- `engine/` — llm (dispatcher/providers), runtime (run/step/tool-call ledger), events, checkpoints, model-provider
- `iam/` — identity, auth, tenancy
- `channels/` — connectors (discord/slack/telegram/matrix), IM message loop
- `modeling/` — providers, opsreadiness, and other modeling crates
- `domains/` — chat, sandbox, mcp, skills, scheduler, delivery, calendar, mail, reminders, workflows, evaluation, and the rest of the domain managers
- `persistence/` — SQLite store (`dope-store`) + DAOs
- `surface/` — HTTP API (`dope-api`), daemon binary (`dope-cli`), terminal TUI (`dope-tui`)

## Key binaries

| Binary | Crate | Purpose |
|--------|-------|---------|
| `dope-cli` | `surface/cli` | daemon entry point (loads config, builds `dope-app`, serves the API) |
| `dope-tui` | `surface/tui` | full-screen terminal client |

## Commands

```bash
cargo build --workspace     # or: make rs-build
cargo test --workspace      # or: make rs-test
cargo clippy --workspace --all-targets   # or: make rs-clippy
cargo fmt --all -- --check
```

Build the daemon binary:

```bash
cargo build --release -p dope-cli
```
