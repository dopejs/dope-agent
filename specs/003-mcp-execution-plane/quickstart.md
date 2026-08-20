# Quickstart: MCP Execution Plane

## Purpose

Validate the implemented MCP execution-plane roadmap locally without touching production state.

## Prerequisites

- Work from the repository root on branch `003-mcp-execution-plane`
- Use the default test environment (`KURA_ENV=test`)
- Have Go available locally
- Keep production state and live secrets out of scope for verification

## Targeted Verification

Run the daemon packages most likely to change in this slice:

```bash
cd /Users/John/Code/kura-agent/daemon
go test ./internal/mcp ./internal/sandbox ./internal/api ./internal/app ./internal/store ./internal/policy ./internal/contracts
```

During targeted verification, confirm that:

- MCP server registration and inspection remain restart-safe
- lifecycle starts, restarts, and cancellations execute through sandbox-backed paths
- blocked restarts and denied launches remain operator-visible even when no subprocess starts
- tool exposure remains deny-by-default until explicitly allowlisted
- tool-level approval does not block routine server lifecycle actions
- credential scope remains isolated per MCP server instance with redacted projections
- dedicated regression coverage records `<=100 ms` daemon-side preflight overhead for
  lifecycle start, restart, or cancellation evaluation

## Contract Verification

If API, schema, or event surfaces change, run the repository contract check:

```bash
cd /Users/John/Code/kura-agent
make daemon-contract-test
```

## Full Daemon Verification

Run the full Go test suite before claiming the roadmap is ready:

```bash
cd /Users/John/Code/kura-agent/daemon
go test ./...
```

## Verification Results

Recorded on `2026-04-19` from the repository workspace:

- `cd /Users/John/Code/kura-agent/daemon && go test ./internal/mcp ./internal/sandbox ./internal/api ./internal/app ./internal/store ./internal/policy ./internal/contracts`
  - result: passed
- `cd /Users/John/Code/kura-agent && make daemon-contract-test`
  - result: passed
- `cd /Users/John/Code/kura-agent/daemon && go test ./...`
  - result: passed

Timing note:

- lifecycle preflight timing coverage is enforced in `daemon/internal/mcp/manager_test.go`
  and `daemon/internal/api/mcp_server_test.go`
- current regression assertions require MCP lifecycle preflight to remain `<=100 ms`
  before subprocess startup and MCP readiness latency

## Optional Manual Validation

If you need to inspect daemon behavior directly after implementation:

1. Start the test daemon:

```bash
cd /Users/John/Code/kura-agent
make daemon-run-test
```

2. Use the MCP and existing operator-visible routes to validate:

- MCP server register, inspect, start, stop, restart, cancel, tool inspection, and tool
  exposure update routes
- MCP tool authorization route for approval-gated runtime-surface use
- `GET /v1/sandboxes/executions` and `POST /v1/sandboxes/explain`
- `GET /v1/policy/approvals` and approval resolution routes for approval-gated tools
- `GET /v1/config` and `GET /v1/events`

3. Confirm operator-visible output shows:

- MCP server identity, enabled state, lifecycle state, and last block or failure reason
- sandbox profile and declaration linkage for lifecycle attempts
- auto-restart behavior for previously enabled servers after daemon restart
- explicit cancellation classification for interrupted lifecycle attempts
- per-server-instance credential scope with redacted secret outcomes
- per-tool, per-runtime-surface exposure state with explicit allowlist and approval mode
- truthful enforcement strength for the current subprocess backend

## Expected Scope Of Change

This quickstart assumes changes are limited to:

- new daemon-managed MCP registry, lifecycle, and tool exposure packages
- sandbox declaration, secret-scope, and policy-record projection for `mcp_server`
- additive API, schema, and event surfaces for MCP resources and lifecycle
- persistence, app recovery, and documentation updates needed to keep MCP explicit

## Verification Recording

This roadmap now records:

- targeted package verification command and pass result
- `make daemon-contract-test` pass result
- `go test ./...` pass result
- recorded timing regression coverage for daemon-side lifecycle preflight overhead

## Rollback And Readiness Notes

- Rollback path: revert the MCP registry, lifecycle, exposure, and contract change set and
  return the daemon to its pre-MCP state while preserving the already-completed sandbox
  substrate and prerequisite declaration work.
- Roadmap-readiness note: the daemon now has a single operator-visible MCP control path
  covering registry, lifecycle, restore, tool discovery, exposure policy, and audit
  events; no unmanaged MCP launch path remains in scope for this slice.
- Operator-facing risk after rollout: the subprocess backend remains policy-rich rather
  than OS-hardened; stronger guarantees must continue to fail closed and be surfaced
  explicitly.
- Remaining follow-on work stays outside this slice: generic tool orchestration, generic
  executable-skill routing, and stronger sandbox backends.
