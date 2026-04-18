# AGENTS.md

This file gives repository-specific instructions for Codex and other agents working in `dope-agent`.

## Default Working Environment

- Default to the **test environment** for all local development, debugging, and manual validation.
- Test environment settings:
  - data dir: `~/.dope-test`
  - config file: `~/.dope-test/config.json`
  - bind addr: `127.0.0.1:19192`
- Production is opt-in only:
  - data dir: `~/.dope`
  - config file: `~/.dope/config.json`
  - bind addr: `127.0.0.1:19191`

Do not touch production config, tokens, or running processes unless the user explicitly asks for production validation.

## Required Workflow

- Before local daemon debugging, use the project skill at `.agents/skills/dope-test-env/SKILL.md`.
- Use the repository entrypoints instead of ad hoc shell commands:
  - `make daemon-run-test`
  - `make daemon-run-test-live`
  - `make daemon-run-prod`
  - `make daemon-test-status`
  - `make daemon-prod-status`
- If the expected daemon port is occupied:
  - automatically stop the previous Dope daemon for that same environment
  - do not automatically kill unrelated processes bound to the port
- When changing defaults, startup workflow, ports, or config loading, update:
  - `Makefile`
  - `scripts/`
  - `.agents/skills/dope-test-env/SKILL.md`
  - relevant docs in `docs/`

## Verification Expectations

- For daemon changes, run `go test ./...` in `daemon/`.
- For client-facing changes, also run `pnpm test:clients`.
- If startup workflow changes, verify at least:
  - test env boot path
  - health check against the expected port
  - config path and environment reported by `/v1/system/info` or `/v1/config`
  - the default test-start path does not require external connectors to succeed
