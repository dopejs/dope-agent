# rs/ — DopeAgent Rust Workspace

Rust rewrite of the DopeAgent runtime, layered after `openai/codex`'s
`codex-rs` architecture. The Go daemon under `daemon/` remains the control
plane of record; this workspace replaces it incrementally, starting from the
agent core.

## Crates

- `protocol` — pure wire/domain types (UUIDv7 IDs, `Op` submissions, `Event`
  stream, `ResponseItem` history). No I/O; everything cheap to serialize and
  safe to persist as runtime evidence.
- `model-provider` — `ModelProvider` trait plus concrete clients. Currently:
  `OpenAiCompatibleClient`, a streaming SSE client for `/chat/completions`
  endpoints, with tool-call fragment accumulation.
- `core` — agent session and turn loop: model stream → tool dispatch →
  history, emitting ordered protocol events. Tool failures are reported back
  to the model as output; rounds are capped to bound provider quota.
- `cli` — `dope-cli exec "<prompt>"` single-turn driver against any
  OpenAI-compatible endpoint (`--base-url`, `--model`, `--api-key` or
  `DOPE_*`/`OPENAI_API_KEY` env vars).

Not yet ported (deliberately): sandbox/exec, SQLite persistence, tenant
identity/permissions, event bus, channels/connectors. Those arrive with
their daemon modules, not ahead of them.

## Commands

- `cargo build --workspace` / `make rs-build`
- `cargo test --workspace` / `make rs-test`
- `cargo clippy --workspace --all-targets` / `make rs-clippy`
- `cargo fmt --all -- --check` before committing
