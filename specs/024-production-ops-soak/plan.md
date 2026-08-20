# Implementation Plan: Production Operations Soak

**Branch**: `024-production-ops-soak` | **Date**: 2026-04-29 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/024-production-ops-soak/spec.md`

## Summary

Close Roadmap 39 by adding the production operations baseline for a tenant-scoped
single-node daemon: install and upgrade runbooks, backup/restore workflows, migration
preflight/postflight verification, a reusable 24-hour test-environment soak harness,
external-service fault drills, opt-in real-account smoke evidence, resource-growth checks,
and a release-readiness gate that must be rerun after Roadmaps 40 and 41. The
implementation is additive and operationally focused: it strengthens docs, scripts,
fixtures, daemon test harnesses, report artifacts, and contract checks without creating a
new domain feature or expanding into multi-node managed hosting.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; Markdown operator docs; shell
scripts/Make targets for local operator workflows; JSON evidence fixtures or schemas only
where the implementation exposes machine-readable soak or backup/restore artifacts.  
**Primary Dependencies**: `daemon/internal/app`, `daemon/internal/store`,
`daemon/internal/store/migrationfixture`, `daemon/internal/store/tenancy`,
`daemon/internal/billing`, `daemon/internal/secrets`, `daemon/internal/integrations`,
`daemon/internal/calendar`, `daemon/internal/mail`, `daemon/internal/delivery`,
`daemon/internal/scheduler`, `daemon/internal/orchestration`,
`daemon/internal/evaluation`, `daemon/internal/audit`, `daemon/internal/events`,
`daemon/internal/contracts`, `schemas/`, `scripts/`, `Makefile`, `docs/runtime`,
`docs/harness`, `docs/providers`, and the upstream docs for Roadmaps 33, 35, 37, and 38.  
**Storage**: Existing SQLite daemon state remains the production data store. Backup and
restore validation must cover a representative multi-tenant data set with at least three
tenants and distinct credential, quota, and work states. No raw credential material may be
captured or restored; only secret metadata and references are recoverable.  
**Testing**: Targeted Go tests for backup/restore fixture creation, migration
verification, restore validation, credential redaction, billing recovery, integration
fault classification, scheduler/runtime/evaluation restart behavior, and contract
completeness; `go test ./...` in `daemon/`; `make daemon-contract-test`; `make
daemon-run-test` plus `make daemon-test-status`; the documented 24-hour
`KURA_ENV=test` soak; opt-in real-account smoke where safe credentials are available;
`pnpm test:clients` and `pnpm build` only if SDK/web/TUI-visible contracts change; `go
mod tidy` from `daemon/` after implementation.  
**Target Platform**: Tenant-scoped single-node production baseline, verified locally in
the default isolated test environment (`~/.kura-test`, `127.0.0.1:19192`) before any
explicit live smoke. Multi-node managed service rollout, clustering, and distributed
failover are out of scope.  
**Project Type**: Operational readiness and daemon harness change spanning Go daemon
fixtures/tests, operator docs, scripts/Make targets, contract fixtures, and release
checklists.  
**Performance Goals**: Clean install runbook completes in <=60 minutes; representative
upgrade verification completes in <=90 minutes; release-readiness evidence review
completes in <=30 minutes; soak report records restart recovery in <=5 minutes and queue
backlog recovery within <=30 minutes.  
**Constraints**: Default validation must not touch `~/.kura`, production user data, live
connectors, or raw credential material. The first readiness baseline is a 24-hour
`KURA_ENV=test` soak unless a temporary shorter threshold is explicitly documented with a
follow-up full-duration rerun. The soak hard-fails on any cross-tenant leakage,
unclassified failure, restart recovery over 5 minutes, retry exhaustion without
operator-action-needed state, queue backlog persisting over 30 minutes, or monotonic
resource growth over the full run. Missing safe real-account credentials do not block
readiness when fake-backend coverage passes and the skip is recorded.  
**Scale/Scope**: One single-node daemon hosting multiple tenants in representative test
state. Required fixture coverage is at least three tenants with distinct credential,
quota, and work states; soak workload covers runtime, scheduler, integrations, delivery,
approvals, quota enforcement, tenant switching, evaluation, restarts, and external-service
faults.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** — PASS. This plan closes Roadmap 39 end-to-end: install, upgrade,
  backup, restore, migration verification, rollback guidance, foundational 24-hour soak,
  fault drills, real-account smoke policy, resource-growth checks, diagnostics, and
  future release rerun gate. It does not split out a demo slice.
- **Production-grade, minimal, reversible change** — PASS. The change set is additive:
  runbooks, scripts, fixtures, harness/report artifacts, and tests. Production behavior is
  changed only where required to expose missing diagnostics or safe evidence; rollback is
  backup-restore or removal/disablement of the new harness and docs.
- **Contracts and auditability** — PASS. Planning contracts define backup/restore
  evidence, soak report thresholds, and release-readiness evidence. Any machine-readable
  report schema or API/event change discovered during implementation must update
  `schemas/`, fixtures, and contract tests together.
- **Verification and observability** — PASS. The plan requires automated backup/restore
  regression where practical, migration preflight/postflight checks, 24-hour soak evidence,
  fault classification, restart recovery timing, queue backlog/resource growth metrics,
  redaction checks, contract tests, and release gate review.
- **Environment and secrets** — PASS. Local work defaults to `~/.kura-test` and fake
  backends. Real-account smoke is opt-in, safe credentials are never logged, raw credential
  material is not backed up/restored, and missing safe credentials produce explicit skip
  evidence rather than blocking readiness when fake-backend coverage passes.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/024-production-ops-soak/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── backup-restore-evidence.md
│   ├── release-readiness-gate.md
│   └── soak-harness-report.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── app/                            # migration startup, restart, and app-level soak helpers
│   ├── store/                          # backup/restore fixtures and tenant data verification
│   │   ├── migrationfixture/           # representative three-tenant data set
│   │   └── tenancy/                    # tenant isolation assertions reused by restore checks
│   ├── integrations/                   # fake backend fault injection and real-account smoke hooks
│   ├── calendar/ and mail/             # integration smoke/fault coverage for supported domains
│   ├── delivery/                       # delivery outcome and retry/recovery workload
│   ├── scheduler/                      # long-running schedule and restart workload
│   ├── orchestration/ and runtime/      # in-flight work restart recovery
│   ├── evaluation/                     # replay/evaluation soak workload and evidence
│   ├── billing/                        # quota/retry/operator-action-needed checks
│   ├── secrets/                        # redaction and reconnect/revalidate checks
│   ├── audit/ and events/              # operator-visible diagnostics and evidence
│   └── contracts/                      # report/checklist/schema completeness tests
├── cmd/kura/
└── go.mod

scripts/
└── production/                         # install/upgrade/backup/restore/soak helpers if scripts are added

docs/
├── runtime/                            # install, upgrade, backup, restore, rollback, release readiness
├── harness/                            # soak harness, fault drills, report format
└── providers/                          # real-account smoke policy and supported-domain notes

schemas/
├── api/                                # update only if evidence/report surfaces become API resources
└── events/                             # update only if new operator-visible event payloads are added

sdk/ts/, web/, tui/                     # update only if new public/client-visible surfaces are introduced
```

**Structure Decision**: Keep Roadmap 39 centered on operational evidence rather than a new
runtime domain. Reuse existing daemon packages and fixtures for work generation,
credential redaction, billing recovery, evaluation replay, tenant migration, and
integration fake backends. Add a focused `scripts/production` or daemon test-harness
surface only when it reduces operator error or makes the 24-hour soak repeatable. Store
machine-readable evidence under this spec's contracts and test fixtures unless planning
during implementation proves a public API/schema is required.

## Roadmap 39 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/backup-restore-evidence.md`](./contracts/backup-restore-evidence.md) —
  backup contents, excluded credential material, restore validation, migration
  preflight/postflight evidence, and rollback decision points.
- [`contracts/soak-harness-report.md`](./contracts/soak-harness-report.md) — workload
  coverage, restarts, fault drills, resource observations, hard-fail thresholds, and
  report fields required for release evidence.
- [`contracts/release-readiness-gate.md`](./contracts/release-readiness-gate.md) —
  install, upgrade, backup, restore, rollback, real-account smoke skip policy, Roadmaps
  40/41 rerun gate, and ship/no-ship decision rules.

These artifacts are planning gates. Implementation is incomplete if operator readiness
can pass without recorded evidence for install, upgrade, backup, restore, migration
verification, soak duration, restarts, faults, resource growth, credential redaction,
tenant isolation, and explicit real-account smoke skip reasons.

## Post-Design Constitution Check

- **Roadmap closure** — PASS. `research.md`, `data-model.md`, `quickstart.md`, and the
  three contracts cover the full Roadmap 39 operational baseline and explicitly defer
  multi-node managed service rollout, payment launch, enterprise SSO, new integration
  domains, memory, and self-improvement.
- **Production-grade, minimal, reversible change** — PASS. The design adds repeatable
  operational workflows and evidence contracts around existing daemon behavior. Any
  storage/config changes discovered during implementation must retain backup-restore
  rollback and be documented before task execution.
- **Contracts and auditability** — PASS. Contracts define evidence shape and failure
  thresholds. If the implementation promotes report evidence into `schemas/` or API
  resources, schema fixtures and `daemon/internal/contracts` tests must change in the
  same task group.
- **Verification and observability** — PASS. Quickstart and contracts require targeted
  package tests, full daemon tests, contract tests, test daemon health, a recorded
  24-hour soak, resource-growth observations, restart/fault classifications, and
  operator-visible diagnostics for action-needed states.
- **Environment and secrets** — PASS. The design defaults to `~/.kura-test` and fake
  backends, records opt-in real-account smoke separately, treats missing safe credentials
  as explicit skip evidence, and prohibits raw credential material in backups, logs,
  reports, fixtures, events, and diagnostics.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |

## Implementation Completion Evidence

- Shared Roadmap 39 evidence validation was implemented in `daemon/internal/opsreadiness`.
- Representative three-tenant fixture coverage was implemented in
  `daemon/internal/store/migrationfixture/r39_production_ops.go`, including actual SQLite
  fixture creation, copy/restore, tenant state validation, and raw credential sentinel
  checks.
- Fake-backend fault drill controls were implemented in
  `daemon/internal/integrations/fake_backend.go`.
- Operator scripts and runbooks were added under `scripts/production`, `docs/runtime`,
  `docs/harness`, and `docs/providers`; the soak runner now writes structured report
  artifacts, enforces real elapsed time for `KURA_SOAK_DURATION=24h`, and restore
  validation checks Roadmap 39 fixture tables when present.
- Evidence fixtures were added under `specs/024-production-ops-soak/fixtures`.
- Verification passed for targeted Go tests, full daemon Go tests, `go mod tidy`,
  `make daemon-contract-test`, and test-daemon smoke.

Residual risk: the full 24-hour `KURA_ENV=test` soak was not run to completion in this
implementation session. A temporary shorter validation was recorded in `quickstart.md`;
the full-duration soak remains mandatory before final release readiness and before
Roadmaps 40/41 can ship.
