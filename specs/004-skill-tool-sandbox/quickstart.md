# Quickstart: Skill And Local Tool Sandbox Execution

## Purpose

Validate the implemented Roadmap 19 slice locally without touching production state.

## Prerequisites

- Work from the repository root on branch `004-skill-tool-sandbox`
- Use the default test environment (`KURA_ENV=test`)
- Have Go available locally
- Keep production state, live connectors, and live secrets out of scope for verification

## Targeted Verification

Run the daemon packages most likely to change in this slice:

```bash
cd /Users/John/Code/kura-agent/daemon
go test ./internal/skills ./internal/api ./internal/runtime ./internal/sandbox ./internal/store ./internal/policy ./internal/app ./internal/contracts
```

During targeted verification, confirm that:

- executable-skill manifest parsing accepts valid manifests and leaves invalid ones visible
  as `unavailable`
- executable-skill launches and the current high-risk local tool path run through
  sandbox-backed execution instead of an unmanaged subprocess path
- approval, denial, timeout, cancellation, and stronger-than-backend failures remain
  explicitly classified
- runtime tool-call history links back to sandbox execution and consumer-policy truth
- daemon restart recovers interrupted in-flight executions as `cancelled`
- dedicated regression coverage keeps daemon-side manifest validation and execution
  preflight within `<=100 ms`, excluding subprocess runtime and approval wait time

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

## Optional Manual Validation

If you need to inspect daemon behavior directly after implementation:

1. Start the test daemon:

```bash
cd /Users/John/Code/kura-agent
make daemon-run-test
```

2. Use the existing operator-visible routes to validate:

- `GET /v1/skills` and `GET /v1/skills/{skillId}` for executable manifest and
  `unavailable` visibility
- runtime tool-call routes for executable-skill and high-risk local-tool execution
- approval list/get/resolve routes for `ask`-gated executions
- sandbox execution inspection and explain routes
- `GET /v1/events` and `GET /v1/config` for audit review and redaction checks

3. Confirm operator-visible output shows:

- skill or local-tool identity
- executable manifest or declaration summary
- approval outcome and policy-record linkage
- sandbox execution linkage for launched work
- explicit denial, unsupported, timeout, cancellation, and restart-recovery classification
- truthful enforcement strength for the current subprocess backend

## Expected Scope Of Change

This quickstart assumes changes are limited to:

- executable-skill manifest parsing and availability projection in the current skill registry
- runtime tool-call and approval surfaces for skill-targeted plus current high-risk local
  execution
- sandbox consumer declaration, execution, and provenance linkage for `skill` and
  `local_tool`
- additive schema, event, persistence, and documentation updates needed to keep the slice
  explicit

## Verification Recording

Record final results in the implementation change set once the slice is executed:

- `2026-04-19`: `cd /Users/John/Code/kura-agent/daemon && go test ./internal/api ./internal/sandbox ./internal/skills ./internal/runtime ./internal/store` ✅
- `2026-04-19`: `cd /Users/John/Code/kura-agent/daemon && go test ./internal/app ./internal/contracts` ✅
- `2026-04-19`: `cd /Users/John/Code/kura-agent && make daemon-contract-test` ✅
- `2026-04-19`: `cd /Users/John/Code/kura-agent/daemon && go test ./...` ✅
- timing note: targeted regression tests cover `<=100 ms` manifest validation and approval
  preflight overhead for the in-scope tool-call path

## Rollback And Readiness Notes

- Rollback path: revert the executable-skill manifest, runtime tool-call, sandbox linkage,
  and contract change set, restoring the pre-Roadmap-19 behavior while preserving the
  already-closed sandbox prerequisite and MCP slices.
- Roadmap-readiness note: supported executable skills and the current high-risk local tool
  path must have one operator-visible execution boundary with no documented unmanaged
  subprocess bypass remaining in scope.
- Operator-facing risk after rollout: the subprocess backend remains policy-rich rather
  than OS-hardened; unsupported stronger guarantees must continue to fail closed and be
  surfaced explicitly.
- Remaining follow-on work stays outside this slice: broader local capability migration,
  orchestration/graph planning, remote execution, and stronger sandbox backends.
