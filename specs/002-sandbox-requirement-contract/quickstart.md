# Quickstart: Sandbox Requirement Declaration Contract

## Purpose

Validate the shared consumer requirement declaration slice locally without touching
production state.

## Prerequisites

- Work from the repository root on branch `002-sandbox-requirement-contract`
- Use the default test environment (`DOPE_ENV=test`)
- Have Go available locally
- Keep live connectors and production provider state out of scope for verification

## Targeted Verification

Run the daemon packages most likely to change in this slice:

```bash
cd /Users/John/Code/dope-agent/daemon
go test ./internal/sandbox ./internal/managedproviders ./internal/api ./internal/store ./internal/runtime ./internal/skills ./internal/providers
```

During targeted verification, confirm that declaration evaluation, secret-scope resolution,
and durable provenance persistence stay within the existing managed-provider and high-risk
tool-call flows without adding more than `<=100 ms` daemon-side overhead to preflight
checks, excluding upstream provider latency and operator approval wait time. This slice
now has explicit timing coverage for managed-provider preflight in
`daemon/internal/managedproviders/managedproviders_test.go` and for high-risk local-tool
preflight in `daemon/internal/api/server_test.go`.

## Contract Verification

If API, schema, or event surfaces change, run the repository contract check:

```bash
cd /Users/John/Code/dope-agent
make daemon-contract-test
```

## Full Daemon Verification

Run the full Go test suite before claiming the slice is ready:

```bash
cd /Users/John/Code/dope-agent/daemon
go test ./...
```

## Optional Manual Validation

If you need to inspect the daemon behavior directly after implementation:

1. Start the test daemon:

```bash
cd /Users/John/Code/dope-agent
make daemon-run-test
```

2. Use existing authenticated routes to validate:

- `GET /v1/skills` and `GET /v1/skills/{skillId}` for skill declaration visibility
- `POST /v1/chat/query` for explicit skill-selection behavior
- provider auth routes for `claude_managed` and `codex_managed`
- runtime tool-call routes plus approval routes for `exec`, `shell`, or `browser`
- `POST /v1/sandboxes/explain` and sandbox execution inspection routes
- `GET /v1/config` and `GET /v1/events` for redaction and durable provenance review

3. Confirm operator-visible output shows:

- consumer kind and consumer instance
- declaration id or equivalent declaration summary
- durable provenance for launched, denied, unsupported, and preflight-only paths
- per-consumer-instance secret-scope resolution with redacted values
- truthful enforcement strength for the current subprocess backend

## Expected Scope Of Change

This quickstart assumes changes are limited to:

- shared sandbox declaration, secret-scope, and provenance handling
- current managed-provider integration surfaces
- current runtime high-risk tool-call and approval surfaces
- current skill registry and explicit skill-selection surfaces
- schema, event, persistence, and documentation updates needed to keep the slice explicit

## Verification Recording

Record final results in the implementation change set once the slice is executed:

- `make daemon-contract-test`
- targeted package verification command above
- `go test ./...`
- any measured overhead note for managed-provider or high-risk tool preflight paths

Recorded on `2026-04-19`:

- `cd /Users/John/Code/dope-agent/daemon && go test ./internal/sandbox ./internal/managedproviders ./internal/api ./internal/store ./internal/runtime ./internal/skills ./internal/providers` passed
- `cd /Users/John/Code/dope-agent && make daemon-contract-test` passed
- `cd /Users/John/Code/dope-agent/daemon && go test ./...` passed
- Dedicated timing coverage now verifies `<=100 ms` daemon-side preflight for managed-provider evaluation and the current high-risk local-tool approval gate

## Rollback And Readiness Notes

- Rollback path: revert the shared declaration, secret-scope, and provenance change set and
  return affected consumer surfaces to their current per-family behavior only if the slice
  causes a production regression.
- Operator-facing risk after rollout: the subprocess backend still provides policy-rich
  control-plane enforcement rather than stronger OS-level isolation; unsupported stronger
  guarantees must remain explicit denials.
- Remaining follow-on work stays outside this slice: MCP runtime lifecycle, generic
  executable-skill subprocess support, broader local-tool sandbox routing, and stronger
  backend implementation.
