# Implementation Plan: Public Release Soak And Launch Gate

**Branch**: `main` | **Spec**: [spec.md](./spec.md) | **Upstream**: [docs/specs/057-public-release-soak-and-launch-gate.md](../../docs/specs/057-public-release-soak-and-launch-gate.md)
**Phase / Roadmap**: Phase 72 — Roadmap 72

## Summary
Codify the public beta launch gate as a validator in `internal/opsreadiness`: RequiredLaunchWorkloads
+ LaunchGateEvidence + ValidateLaunchGate -> LaunchDecision (ship/no-ship + reasons + entry-gate
flag). Enforce >=3 channel entries, calendar+mail provider entries, all required workloads
(pass or skip-with-reason), soak/support-bundle/redaction evidence. Expose POST /v1/release/launch-gate.

## Constitution Check
- Roadmap closure: defensible hosted non-knowledge parity evidence; entry gate for memory work.
- Production-grade: missing required evidence is no-ship; skips need accepted reasons.
- Contracts first: launch-gate decision schema; contract test. RealAccountSmokeStatus gains
  camelCase JSON tags for the evidence contract.
- Verification: validator tests for ship + every no-ship rule.

## Project Structure
```
specs/057-public-release-soak-and-launch-gate/  spec.md plan.md tasks.md checklists/
daemon/internal/opsreadiness/launch_gate.go, launch_gate_test.go
daemon/internal/opsreadiness/types.go   # RealAccountSmokeStatus JSON tags
daemon/internal/api/release.go ; server.go route
schemas/api/launch-gate-decision.schema.json
daemon/internal/contracts/launch_gate_contracts_test.go
docs/runtime/release-readiness.md       # non-knowledge parity marker
```

## Complexity Tracking
No new feature domain; a validator + endpoint reusing existing smoke evidence tooling.
