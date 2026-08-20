# Implementation Plan: Live Validation And Side-Effect Replay

**Branch**: `025-live-validation-replay` | **Date**: 2026-04-29 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/025-live-validation-replay/spec.md`

## Summary

Close Roadmap 40 by adding an explicit live-validation executor for replay candidates
that can safely cross the side-effect boundary. The implementation introduces a focused
`daemon/internal/livevalidation` package for replay support classification, permission
and quota gates, kill switches, approval requirements, side-effect ledgering,
abort/retry/ambiguous-commit behavior, reconciliation resolution, retention policy, and
original-versus-live comparison enrichment. It reuses the Roadmap 33 evaluation replay
surfaces, Roadmap 38 live-validation quota preflight gate, tenant identity permissions,
approval resources, fake integration backends, and existing daemon API/SDK/web
conventions rather than creating a parallel evaluation product.

Non-live replay remains the default. Live validation is additive, permission-gated,
quota-aware, approval-gated, disabled by kill switches when necessary, and auditable
through durable ledger and comparison records. Unsupported tool-call classes never run
silently; mixed candidates can proceed only when unsupported work is explicitly excluded
from scope.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK resources in `sdk/ts`; existing React web operator shell
surfaces where live-validation inspection or controls become user-visible.  
**Primary Dependencies**: `daemon/internal/evaluation`, `daemon/internal/billing`,
`daemon/internal/identity`, `daemon/internal/tenantctx`, `daemon/internal/store`,
`daemon/internal/store/tenancy`, `daemon/internal/api`, `daemon/internal/audit`,
`daemon/internal/events`, `daemon/internal/runtime`, `daemon/internal/orchestration`,
`daemon/internal/integrations`, `daemon/internal/calendar`, `daemon/internal/mail`,
`daemon/internal/delivery`, `daemon/internal/connectors`, `daemon/internal/mcp`,
`daemon/internal/contracts`, `schemas/api`, `schemas/events`, `sdk/ts`, `web`, and
operator docs under `docs/harness`, `docs/runtime`, and `docs/providers`. New live
validation semantics belong in `daemon/internal/livevalidation`; domain packages provide
tool-call evidence, execution hooks, and fake-backend outcomes.  
**Storage**: Existing SQLite daemon metadata store remains authoritative. Add durable
tenant-scoped records for live-validation attempts, support-matrix snapshots or
versioned declarations, side-effect ledger entries, kill-switch state, approval linkage,
ambiguous-commit reconciliation decisions, comparison links, and explicit retention
policy metadata. Ledger records must be durable before or atomically with external
mutation attempts where feasible.  
**Testing**: Targeted Go tests for livevalidation, evaluation, billing, identity,
store/tenancy, API, integration fake backends, calendar/mail/delivery connectors, audit,
events, and contracts; restart tests for after-submit and pending reconciliation states;
matrix completeness tests; fake-backend side-effect replay tests; `go test ./...` in
`daemon/`; `make daemon-contract-test`; `make daemon-run-test` plus manual live
validation smoke in `KURA_ENV=test`; `pnpm test:clients`; `pnpm build`; `go mod tidy`
from `daemon/` after implementation.  
**Target Platform**: Local-first daemon and hosted daemon behavior, verified in the
default isolated test environment (`~/.kura-test`, `127.0.0.1:19192`). Optional
real-account smoke uses explicit operator opt-in and live connector configuration; it is
not required for normal automated acceptance.  
**Project Type**: Multi-domain daemon platform change spanning evaluation replay,
side-effect execution boundaries, persistence, API contracts, tenant permissions, quota
gates, approval flows, SDK/web operator surfaces, fake-backend integration coverage, and
operator documentation.  
**Performance Goals**: Live validation start gating completes before external side
effects are attempted. Operator smoke verification can explain one successful side-effect
replay, one denied request, one unsupported tool class, and one ambiguous-commit
reconciliation path from structured evidence in under 20 minutes. Ledger and comparison
inspection must not require raw log reconstruction.  
**Constraints**: Live validation is never implicit and never the default replay mode.
`live_validation.execute` is required to start. Hosted quota enforcement fails closed
when quota state is unavailable. Scope-level approval may cover read-only and idempotent
classes; non-idempotent mutation replay requires per-action approval. Tenant/global kill
switches block new starts and abort pending/future side effects in running attempts.
Already-submitted side effects resolve to completed, failed, or operator-action-needed
evidence. Ambiguous commits stop automatic retry. Reconciliation resolution requires
tenant owner/admin authority or an explicit reconciliation permission. Live-validation
attempts, side-effect ledger entries, reconciliation decisions, and comparison evidence
retain indefinitely unless an explicit operator retention policy is later applied.
Unsupported classes never live-replay.  
**Scale/Scope**: One daemon may host multiple tenants and moderate volumes of replay
candidates, live-validation attempts, ledger entries, and reconciliation decisions.
Correctness, idempotency, auditability, restart safety, and bounded blast radius take
priority over high-throughput replay execution.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** — PASS. The plan closes Roadmap 40 end-to-end: explicit
  live-validation mode, permission and quota gates, kill switches, fresh approvals,
  support matrix completeness, side-effect ledgering, abort/retry semantics,
  ambiguous-commit handling, reconciliation resolution, comparison output, fake-backend
  validation, contracts, SDK/web visibility, and operator docs.
- **Production-grade, minimal, reversible change** — PASS. The change is additive and
  staged around a new `daemon/internal/livevalidation` boundary. Rollout can keep live
  side effects disabled until matrix, gates, approvals, ledger, fake backends, and
  contract tests pass. Rollback disables new live validation starts and preserves ledger
  evidence for audit.
- **Contracts and auditability** — PASS. API, event, schema, SDK, persistence, and docs
  changes are named below. The replay support matrix, live-validation surface contract,
  and side-effect ledger contract are required before task generation.
- **Verification and observability** — PASS. Required verification covers permission,
  quota, kill switch, approval granularity, unsupported classes, mixed candidates,
  completed/failed/skipped/denied/aborted ledger outcomes, timeout-after-submit,
  restart-after-submit, duplicate retry, ambiguous commit, reconciliation authority,
  retention, contract shapes, and operator-visible comparison evidence.
- **Environment and secrets** — PASS. Default verification uses `~/.kura-test` and fake
  backends. Real-account smoke is optional, explicitly opted in, and must record scope
  without logging secrets or raw credential material.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/025-live-validation-replay/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── live-validation-surfaces.md
│   ├── replay-support-matrix.md
│   └── side-effect-ledger.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── livevalidation/                 # NEW: executor, matrix, ledger,
│   │                                    # kill switches, reconciliation
│   ├── evaluation/                      # live mode handoff, attempt/comparison links
│   ├── billing/                         # live_validation_attempts quota reuse
│   ├── identity/                        # reconciliation permission and role tests
│   ├── store/                           # SQLite schema, persistence, retention fields
│   │   └── tenancy/                     # tenant isolation checks for live evidence
│   ├── api/                             # live validation, ledger, kill-switch,
│   │                                    # reconciliation, comparison routes
│   ├── runtime/                         # runtime local tool-call replay classification
│   ├── integrations/                    # fake backend side-effect attempts
│   ├── calendar/ and mail/              # domain mutation replay hooks
│   ├── delivery/ and connectors/        # dispatch/message-send classifications
│   ├── mcp/                             # MCP tool-call replay classification
│   ├── audit/ and events/               # live-validation audit/event evidence
│   └── contracts/                       # schema and matrix completeness tests
├── go.mod
└── go.sum

schemas/
├── api/                                 # live validation, ledger, matrix,
│                                        # kill-switch, reconciliation shapes
└── events/                              # live validation and side-effect events

sdk/ts/                                  # typed live validation resources/errors
web/                                     # operator shell controls and inspection
tui/                                     # update only if live validation is exposed there

docs/
├── harness/                             # live validation and fake-backend verification
├── runtime/                             # rollback, retention, kill-switch operations
└── providers/                           # opt-in real-account smoke guidance
```

**Structure Decision**: Centralize live-validation safety semantics in
`daemon/internal/livevalidation`. Evaluation remains the source of replay candidates and
attempt history; billing owns quota decisions; identity owns permission evaluation;
domain packages expose explicit replay support and fake/live execution adapters. This
keeps non-live replay backward compatible and prevents hidden per-domain side-effect
paths.

## Roadmap 40 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/replay-support-matrix.md`](./contracts/replay-support-matrix.md) — tool
  class safety classification, approval, idempotency, retry, ambiguous-commit,
  compensation, ledger, and test obligations.
- [`contracts/live-validation-surfaces.md`](./contracts/live-validation-surfaces.md) —
  API, SDK, event, permission, quota, kill-switch, approval, reconciliation, and
  comparison surfaces.
- [`contracts/side-effect-ledger.md`](./contracts/side-effect-ledger.md) — ledger record
  lifecycle, durability, idempotency/correlation, ambiguous-commit, abort, retry,
  retention, and reconciliation evidence rules.

These artifacts are planning gates. Implementation is incomplete if a live-validation
entry point, replay support class, side-effect outcome, or reconciliation path can exist
without a contract row and proving test.

## Post-Design Constitution Check

- **Roadmap closure** — PASS. `research.md`, `data-model.md`, `quickstart.md`, and the
  three contracts cover all Roadmap 40 gates, executor behavior, support matrix
  completeness, ledger states, abort/retry semantics, kill switches, approval granularity,
  reconciliation authority, retention, SDK/web contracts, and fake-backend validation.
- **Production-grade, minimal, reversible change** — PASS. Design is additive and
  rollback-safe: disable new live starts, preserve historical ledger/comparison evidence,
  and leave non-live replay unchanged. Domain work is explicit through the matrix rather
  than hidden in broad refactors.
- **Contracts and auditability** — PASS. Contracts define response/event shapes, stable
  denial states, matrix rows, ledger transitions, and reconciliation evidence. Schema,
  SDK, docs, and contract tests must change together when implementation changes those
  surfaces.
- **Verification and observability** — PASS. Quickstart names targeted package tests,
  full daemon tests, contract tests, client tests, test-daemon smoke, fake-backend
  side-effect paths, restart recovery, and operator-visible evidence checks.
- **Environment and secrets** — PASS. Design defaults to `~/.kura-test` and fake
  backends, treats real-account smoke as explicit opt-in, and requires redacted evidence
  with no secret logging.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
