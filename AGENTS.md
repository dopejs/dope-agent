# Repository Guidelines

## Project Structure & Module Organization

`daemon/` contains the Go control plane, runtime, providers, channels, and harness code. `web/` and `tui/` are the client surfaces. `sdk/ts/` holds the TypeScript client SDK used by both. `schemas/` stores JSON schema contracts. `scripts/` contains local operator utilities. `docs/` is organized by module (`runtime/`, `providers/`, `channels/`, `harness/`, etc.) and should stay aligned with implementation changes.

## Build, Test, and Development Commands

- `make daemon-run-test`: start the daemon in the default test environment (`~/.dope-test`, `127.0.0.1:19192`).
- `make daemon-run-test-live`: start the test daemon with live connectors enabled.
- `make daemon-test-status`: check the local test daemon health.
- `go test ./...` (run in `daemon/`): execute all Go tests.
- `make daemon-contract-test`: validate schema and contract fixtures.
- `pnpm test:clients`: run SDK, web, and TUI client tests.
- `pnpm build`: build client packages.

## Coding Style & Naming Conventions

Use existing repository conventions before introducing new abstractions. Go code must be `gofmt`-clean and organized by package boundary, not by feature dumping. TypeScript code should follow the existing Vite/React structure and keep contracts explicit. Prefer clear, production-readable names such as `provider_manager.go`, `chat-service.ts`, and `discord-channel-loop.md`. Keep changes small and reversible.

## Testing Guidelines

Every production change should include targeted tests in the affected layer and preserve contract coverage where applicable. Use Go unit and integration tests under `daemon/internal/...` and client tests in package-local test files. When changing API shape, schema, or event payloads, run `make daemon-contract-test` and update `schemas/` plus fixtures together.

## Commit & Pull Request Guidelines

Follow the existing imperative commit style: `Complete roadmap 15 skill registry and prompt support`, `Add test environment workflow and repo skill`, `Improve openai-compatible base URL handling`. Keep commits scoped and descriptive. Pull requests should explain the operator impact, verification performed, rollback path, and any config or schema changes. Include screenshots only when UI behavior changes.

## Security & Configuration Tips

Default development work must use the test environment, not `~/.dope`. Never assume live connectors or managed providers are safe to touch; make the environment explicit. Treat secrets in config as operator-owned material and avoid logging or echoing them in tests, scripts, or API output.

<!-- SPECKIT START -->
Current active speckit plan: `specs/006-mcp-runtime-and-catalog/plan.md`
Use it for feature-specific scope, contracts, verification, and repo paths.
<!-- SPECKIT END -->
