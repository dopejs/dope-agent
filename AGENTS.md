# Repository Guidelines

> **MIGRATION COMPLETE (2026-08):** the Go `daemon/` control plane has been fully replaced by the Rust workspace (`rs/`) and deleted. The daemon binary is now `dope-cli` (`rs/cli`), wired by `dope-app` (`rs/app`), with the HTTP API in `dope-api`. `make daemon-build` / `daemon-test` / `daemon-contract-test` now map to `cargo` equivalents. Any remaining `daemon/`-related text below is historical.

## Project Structure & Module Organization

The Rust workspace `rs/` is the daemon control plane (runtime, providers, channels, store, API, and harness). `web/` and `tui/` are the client surfaces. `sdk/ts/` holds the TypeScript client SDK used by both. `schemas/` stores JSON schema contracts. `scripts/` contains local operator utilities. `docs/` is organized by module (`runtime/`, `providers/`, `channels/`, `harness/`, etc.) and should stay aligned with implementation changes.

## Build, Test, and Development Commands

- `make daemon-run-test`: start the daemon in the default test environment (`~/.dope-test`, `127.0.0.1:19192`).
- `make daemon-run-test-live`: start the test daemon with live connectors enabled.
- `make daemon-test-status`: check the local test daemon health.
- `go test ./...` (run in `daemon/`): execute all Go tests.
- `make daemon-contract-test`: validate schema and contract fixtures.
- `pnpm test:clients`: run SDK, web, and TUI client tests.
- `pnpm build`: build client packages.

**Disk hygiene (important):** the Rust `rs/target/` directory accumulates very large debug artifacts (~76 GB after a full build+test session) and can fill the disk. **After finishing any test/build session, run `cargo clean` from `rs/` (or `rm -rf rs/target`) to release that space.** Never leave a stale `rs/target/` behind across a long-running task.

## Coding Style & Naming Conventions

Use existing repository conventions before introducing new abstractions. Go code must be `gofmt`-clean and organized by package boundary, not by feature dumping. TypeScript code should follow the existing Vite/React structure and keep contracts explicit. Prefer clear, production-readable names such as `provider_manager.go`, `chat-service.ts`, and `discord-channel-loop.md`. Keep changes small and reversible.

## Testing Guidelines

Every production change should include targeted tests in the affected layer and preserve contract coverage where applicable. Use Go unit and integration tests under `daemon/internal/...` and client tests in package-local test files. When changing API shape, schema, or event payloads, run `make daemon-contract-test` and update `schemas/` plus fixtures together.
After completing each spec, run `go mod tidy` from `daemon/` before considering the work complete.

## Planning And Scope Discipline

Do not plan, specify, or implement demo-grade slices unless the user explicitly asks for a demo or prototype. Treat roadmap, spec, and architecture work as production planning by default.

When a capability spans both platform behavior and multiple domain implementations, split it into a parent planning document plus separate capability or domain specs rather than collapsing everything into one broad spec.

Do not count a happy-path integration or a narrow vertical proof as completion of a personal-agent capability. Completion requires durable operator-visible behavior, failure handling, and verification boundaries.

## Commit & Pull Request Guidelines

Follow the existing imperative commit style: `Complete roadmap 15 skill registry and prompt support`, `Add test environment workflow and repo skill`, `Improve openai-compatible base URL handling`. Keep commits scoped and descriptive. Pull requests should explain the operator impact, verification performed, rollback path, and any config or schema changes. Include screenshots only when UI behavior changes.

## Security & Configuration Tips

Default development work must use the test environment, not `~/.dope`. Never assume live connectors or managed providers are safe to touch; make the environment explicit. Treat secrets in config as operator-owned material and avoid logging or echoing them in tests, scripts, or API output.

## Feature Planning

Spec Kit tooling was removed. Planned feature work lives on as plain markdown under `specs/<NNN>-<name>/` (spec, plan, tasks, contracts) and `docs/specs/`; treat those documents as the authoritative scope for their features. New features are specified directly as markdown in `docs/specs/`.
