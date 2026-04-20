# Quickstart: Sandbox Stronger Backends

## Purpose

Validate the implemented Roadmap 20 slice locally without touching production state.

## Prerequisites

- Work from the repository root on branch `005-sandbox-stronger-backends`
- Use the default test environment (`DOPE_ENV=test`)
- Have Go available locally
- Keep production state, live connectors, and live secrets out of scope for verification
- For positive-path stronger-backend verification, use a host where `docker` is installed,
  reachable, and explicitly allowed for local testing
- For negative-path verification, also validate behavior on a host or configuration where
  `docker` is absent or unavailable

## Targeted Verification

Run the daemon packages most likely to change in this slice:

```bash
cd /Users/John/Code/dope-agent/daemon
go test ./internal/sandbox ./internal/api ./internal/skills ./internal/runtime ./internal/store ./internal/app ./internal/contracts ./internal/mcp
```

During targeted verification, confirm that:

- sandbox profile and explain inspection distinguish `subprocess` from `docker`
- `docker`-required executable skills stay opt-in rather than becoming a new default
- `docker`-required requests fail as `unsupported` when host prerequisites are absent
- successful `docker` launches preserve the existing sandbox execution, runtime, and
  provenance linkage model
- stronger-backend restart, timeout, cancellation, and runtime failure paths remain
  explicitly classified

## Contract Verification

If API, schema, or event surfaces change, run the repository contract check:

```bash
cd /Users/John/Code/dope-agent
make daemon-contract-test
```

## Full Daemon Verification

Run the full Go test suite before claiming the roadmap is ready:

```bash
cd /Users/John/Code/dope-agent/daemon
go test ./...
```

## Optional Manual Validation

If you need to inspect daemon behavior directly after implementation:

1. Start the test daemon:

```bash
cd /Users/John/Code/dope-agent
make daemon-run-test
```

2. Use the existing operator-visible routes to validate:

- `GET /v1/sandboxes/profiles` and `GET /v1/sandboxes/profiles/{profileId}` for backend
  capability and host-prerequisite visibility
- `POST /v1/sandboxes/explain` for `docker` selection and `unsupported` mismatch truth
- `GET /v1/skills` and `GET /v1/skills/{skillId}` for explicit `docker` executable-skill
  declarations
- runtime tool-call routes for skill-backed execution provenance
- sandbox execution inspection and event history for backend identity and recovery state

3. Confirm operator-visible output shows:

- which backend is baseline and which is stronger
- whether `docker` is available on the current host
- which executable skills explicitly require `docker`
- explicit `unsupported` results when `docker` is required but unavailable
- truthful backend identity for launched stronger-backend executions
- no silent fallback from `docker` to `subprocess`

## Expected Scope Of Change

This quickstart assumes changes are limited to:

- backend-capability and host-prerequisite truth on the current sandbox profile model
- additive explain and execution semantics for `docker`
- executable-skill declaration updates needed to opt into `docker`
- additive runtime, schema, event, and contract changes needed to preserve one execution
  plane
- operator docs and migration inventory for future sandbox work

## Verification Recording

Recorded results for this implementation run (`2026-04-20`):

- Targeted daemon package verification: passed
  - `cd /Users/John/Code/dope-agent/daemon && go test ./internal/sandbox ./internal/api ./internal/skills ./internal/runtime ./internal/store ./internal/app ./internal/contracts ./internal/mcp`
- Contract verification: passed
  - `cd /Users/John/Code/dope-agent && make daemon-contract-test`
- Full daemon regression verification: passed
  - `cd /Users/John/Code/dope-agent/daemon && go test ./...`
- Positive-path stronger-backend coverage:
  - fake-`docker` sandbox execution and skill-backed runtime linkage passed in automated
    regression in `daemon/internal/sandbox/manager_test.go` and
    `daemon/internal/api/server_test.go`
  - real-`docker` executable-skill runtime linkage is covered by
    `TestSkillToolCallLaunchUsesRealDockerSandboxExecution`
  - local macOS development host still skips the real-`docker` test because `docker` is
    not installed there
  - real-`docker` verification passed on `zentalk-1` (`CentOS Stream 9`, `docker 29.4.0`,
    `go 1.24.0`) with:
    `cd /root/dope-agent/daemon && PATH=/usr/local/go/bin:$PATH go test ./internal/api -run TestSkillToolCallLaunchUsesRealDockerSandboxExecution -v`
- Negative-path `docker required -> unsupported` coverage: passed in automated regression
  - explain-time access mismatch, runtime unavailable, missing-image, MCP lifecycle
    blocking, and restart/runtime provenance paths are covered in
    `daemon/internal/sandbox/manager_test.go`, `daemon/internal/api/server_test.go`, and
    `daemon/internal/mcp/manager_test.go`
- stronger-backend package regression on the real-`docker` host: passed
  - `cd /root/dope-agent/daemon && PATH=/usr/local/go/bin:$PATH go test ./internal/sandbox ./internal/api ./internal/mcp`
- Manual daemon/operator inspection walkthrough: passed on a `docker`-unavailable test host
  - validated pairing bootstrap plus `GET /v1/config`, `GET /v1/sandboxes/profiles`, and
    `POST /v1/sandboxes/explain`
  - confirmed operator-visible backend matrix, host prerequisites, and
    `docker required -> unsupported` truth in under five minutes

## Rollback And Readiness Notes

- Rollback path: revert `docker` backend capability, stronger-backend selection, and
  executable-skill opt-in changes while preserving the existing subprocess-backed sandbox
  plane.
- Roadmap-readiness note: at least one real executable-skill consumer must use `docker`
  through the existing sandbox plane, and `docker` mismatch semantics must fail closed as
  `unsupported`.
- Operator-facing risk after rollout: `docker` introduces additional host dependency and
  lifecycle burden; docs and inspection must remain explicit about prerequisites and
  availability.
- Remaining follow-on work stays outside this slice: broader migration of high-risk local
  tools, MCP, and managed providers; SSH/remote backends; VM-grade isolation; and wider
  orchestration work.
