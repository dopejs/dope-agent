# Quickstart: Sandbox Managed Provider Convergence

## Purpose

Validate the managed-provider sandbox convergence slice locally without touching production
state.

## Prerequisites

- Work from the repository root on branch `001-sandbox-managed-providers`
- Use the default test environment (`KURA_ENV=test`)
- Have Go available locally
- In the test environment, managed-provider CLI state is isolated under `<KURA_DATA_DIR>/managed-provider-home` rather than the real user home

## Targeted Verification

Run the targeted daemon packages most likely to change in this slice:

```bash
cd /Users/John/Code/kura-agent/daemon
go test ./internal/managedproviders ./internal/sandbox ./internal/api ./internal/app ./internal/contracts
```

For the in-scope workflows, record whether sandbox requirement evaluation and local-state
preflight stay within a `<=100 ms` daemon-side overhead target per request relative to the
same test fixture path without the added convergence logic. Exclude upstream CLI runtime
and external provider latency from this check.

## Contract Verification

If API, schema, or event surfaces change, run the repository contract check:

```bash
cd /Users/John/Code/kura-agent
make daemon-contract-test
```

## Full Daemon Verification

Run the full Go test suite before claiming the slice is closed:

```bash
cd /Users/John/Code/kura-agent/daemon
go test ./...
```

## Verification Results

- `make daemon-contract-test`: passed
- `go test ./internal/managedproviders ./internal/sandbox ./internal/api ./internal/app ./internal/contracts`: passed
- `go test ./...`: passed
- daemon-side preflight evaluation for the in-scope Claude and Codex workflows is covered by regression tests with a `<=100 ms` guardrail

## Optional Manual Validation

If you need to inspect the daemon behavior directly after implementation:

1. Start the test daemon:

```bash
cd /Users/John/Code/kura-agent
make daemon-run-test
```

2. Use the existing authenticated provider routes and sandbox inspection routes to validate:

- managed-provider auth state for `claude_managed` and `codex_managed`
- logout behavior for both managed providers
- prompt-execution behavior through the existing managed-provider path
- sandbox execution inspection for subprocess-backed operations

3. Confirm operator-visible output shows:

- provider and action provenance
- fail-closed denial when access exceeds declarations
- redacted handling of credential-bearing local state
- truthful enforcement strength for the current subprocess backend

## Expected Scope Of Change

This quickstart assumes changes are limited to:

- daemon managed-provider bridge logic
- sandbox requirement and provenance handling
- provider auth state metadata or equivalent additive contract detail
- schema, event, and documentation updates needed to keep the slice explicit

## Rollback And Readiness Notes

- Rollback path: revert the managed-provider convergence change set and return the bridges to the prior unmanaged local-state behavior only if this slice causes a production regression.
- Operator-facing risk after rollout: subprocess network enforcement remains `declared_only`; this slice improves declaration, audit, and fail-closed behavior without claiming stronger OS-level isolation.
- Remaining follow-on work stays outside this slice: shared secret refs, remaining consumer convergence, and MCP lifecycle through sandbox.
