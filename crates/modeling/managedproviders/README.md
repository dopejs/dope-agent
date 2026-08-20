# kura-managedproviders

Rust port of `daemon/internal/managedproviders` (Go): the managed provider
bridge layer for CLI-backed LLM providers (Claude Code, Codex), the bridge
registry, the sandbox-backed preflight evaluation machinery, and store-backed
state/check management.

## What is ported

- **`Bridge`** (sync trait, Go `Bridge` interface): `provider_id`,
  `display_name`, `family`, `auth_mode`, `detect`/`start`/`complete`/
  `refresh`/`revoke`, and `provider`.
- **`Runner` / `RunResult` / `RunError`** (Go `Runner` interface): the
  execution abstraction. `ExecRunner` is a faithful port of the Go
  `execRunner` (`CombinedOutput` + exit-code mapping) on `std::process`.
  `SandboxRunner` is the port of `sandboxRunner`, driving the sandbox
  manager and mapping execution statuses onto `RunResult`/`RunError`.
- **`SandboxManager`** (trait): the subset of the Go `sandbox.Manager` the
  runners and preflight evaluation use (`start_execution`, `wait_execution`,
  `finalize_execution`, `evaluate_access`, `get_profile`,
  `persist_consumer_view`).
- **`Registry`** (Go `Registry`): `new(cfg, sandboxes)`, `list`, `get`;
  insertion-ordered Claude then Codex. Also implements
  `kura_providers::ManagedRegistry` (through `ManagedBridgeAdapter`), so it
  plugs into `kura_providers::Manager` unchanged.
- **`ClaudeBridge` / `CodexBridge`** plus the `kura_llm::Provider` shims
  (`claudeCLIProvider` / `codexCLIProvider`), including the JWT claim
  decoding, auth-file parsing, model catalog reading (built-in Claude
  models; Codex `models_cache.json` + `config.toml`), and `classify_cli_error`.
- **Preflight evaluation** (`evaluate.rs`, the `bridges.go` machinery):
  operation plans, requirement declarations, consumer contract views with
  per-consumer secret scope outcomes, fails-closed declaration-scope checks,
  metadata construction, and execution finalization.
- **`Manager`** (new in this crate; the Go app layer does this wiring in
  `api/server.go` + `providers.Manager`): sync/action results persisted via
  the store provider CRUD (`upsert_provider_auth_state`,
  `replace_provider_models`, `upsert_provider_preference`,
  `upsert_provider_check`), restore helpers, default-model validation,
  check run/list/get, and the `kura-setupwizard` dependent-use gate.

## Porting notes / deferrals

- `context.Context` -> `kura_llm::CancelToken`; every bridge method is
  synchronous (the Go code is synchronous; nothing streams).
- The operation plan Go threads through `context.Context` is passed
  explicitly to `Runner::run`.
- `ExecRunner` honors cancellation before spawning and reports a cancelled
  token; killing an in-flight child process on cancellation is deferred (a
  sync port has no per-call worker to kill).
- **Live sandboxed CLI auth-bridge execution is deferred**: `kura-sandbox`
  currently ports types only (no `Manager`). The runner and preflight
  evaluation are ported against the `SandboxManager` trait; when the
  concrete sandbox manager lands it implements the trait and
  `Registry::new(cfg, Some(manager))` activates sandbox-backed execution.
  Until then, `Registry::new` with `None` builds exec-backed bridges, and
  all logic is exercised through stub runners/stub sandbox managers in
  `tests/managedproviders.rs`.
- Tenant-scoped persistence (Go `tenancy`) is not yet ported; the tenantless
  store write paths are used (the same paths the Go daemon uses when no
  tenant context is attached).

## Workspace wiring

The crate follows the workspace conventions (`edition.workspace = true`,
`version.workspace = true`, `rust-version.workspace = true`).
`rs/Cargo.toml` does not yet list `managedproviders` in `members`; add it
there (out of scope for this port) before building from the workspace root.
