# Implementation Plan: Billing, Quotas, And Usage Accounting

**Branch**: `023-billing-quotas-usage` | **Date**: 2026-04-28 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/023-billing-quotas-usage/spec.md`

## Summary

Close Roadmap 38 by adding a tenant-scoped billing and quota control plane that can
project effective tenant entitlements, reserve usage before expensive or side-effecting
hosted work starts, commit/refund actual consumption, and expose operator-visible billing
and usage evidence. The implementation introduces a shared `daemon/internal/billing`
package for plans, quota definitions, period-aware usage accounting, idempotent
reservation lifecycle, denial decisions, audit records, and inspection projections.
Guarded runtime, workflow, tool-call, integration, artifact, replay, and evaluation entry
points consume that shared service rather than performing local quota checks. Roadmap 38
also defines the live-validation quota category and reusable preflight gate contract, but
does not implement the Roadmap 40 live-validation executor; any existing concrete
live-validation entry point discovered during implementation must use the gate before live
side effects.

This roadmap does not add external payment-provider checkout, invoices, taxes, revenue
recognition, provider-specific token billing, or cross-tenant pooled quota. Local-first
installations keep an explicit development or unlimited plan and do not fail closed unless
finite quotas are configured.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK resources must be updated because Roadmap 38 exposes tenant
plan, quota, usage, denial, and adjustment surfaces.  
**Primary Dependencies**: `daemon/internal/identity`, `daemon/internal/tenantctx`,
`daemon/internal/store`, `daemon/internal/store/tenancy`, `daemon/internal/audit`,
`daemon/internal/events`, `daemon/internal/runtime`, `daemon/internal/orchestration`,
`daemon/internal/integrations`, `daemon/internal/connectors`, `daemon/internal/mcp`,
`daemon/internal/sandbox`, `daemon/internal/artifacts`, `daemon/internal/evaluation`,
`daemon/internal/api`, `daemon/internal/contracts`, `schemas/api`, and `schemas/events`.
New billing and quota logic belongs in `daemon/internal/billing` rather than being
embedded in individual launch or domain handlers.  
**Storage**: SQLite remains the durable metadata store. Add tenant-owned plan, quota
definition, quota override, period, counter, reservation, usage event, denial, manual
adjustment, and audit-retention policy records. Counter and reservation changes must run
in one durable transaction per operation identity and quota period.  
**Testing**: `go test ./...` in `daemon/`, targeted package tests for billing/store/API/
runtime/orchestration/integrations/artifacts/evaluation/audit/contracts, restart recovery
tests for pending reservations, concurrent launch tests, `make daemon-contract-test`,
`make daemon-run-test` plus the manual quota smoke, `pnpm test:clients`, `pnpm build`,
and `go mod tidy` from `daemon/` after implementation.  
**Target Platform**: Local-first daemon and hosted daemon behavior using the default test
environment for local verification (`~/.dope-test`, `127.0.0.1:19192`).  
**Project Type**: Multi-domain daemon platform change: persistence migration, daemon API,
SDK/client contracts, runtime and workflow launch gating, Roadmap 38 live-validation gate
contract readiness, integration operation gating, storage/artifact accounting,
replay/evaluation attempt gating, audit/event contracts, and operator documentation.  
**Performance Goals**: Quota checks add one bounded tenant-scoped reservation transaction
before each guarded operation. Plan and usage inspection for a seeded tenant remains
completeable by a tenant owner in under 2 minutes, and the manual operator smoke explains
denial/refund/adjustment evidence in under 15 minutes.  
**Constraints**: Hosted quota enforcement fails closed when quota state is unavailable.
Quota periods reset on UTC boundaries. Lowered quotas take effect immediately without
rewriting existing usage. Storage/artifact bytes reserve an estimate before write and
reconcile actual bytes after write; if actual bytes exceed the estimate and place the
tenant over quota, actual usage remains committed with audit-visible over-limit evidence
and new quota-consuming work is denied until effective usage is within limit. Ambiguous
restart recovery marks reservations operator-action-needed and denies duplicate work until
resolved. Billing and usage audit records retain indefinitely unless an explicit operator
retention policy is applied. Billing administration uses the canonical `billing.manage`
permission.
Local-first development/unlimited plans remain explicit and non-denying by default.  
**Scale/Scope**: One daemon may host multiple tenants with finite plans and moderate
volumes of runs, workflows, tool calls, live validation attempts, integrations, artifacts,
and replay/evaluation attempts per tenant. The plan optimizes for correctness,
idempotency, auditability, and bounded transactional work rather than high-volume
payment-provider metering.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** — PASS. The plan closes Roadmap 38 end-to-end: tenant plans,
  quota definitions, effective quota projection, UTC periods, carryover, reservation,
  commit, refund, manual adjustment, stable denial, fail-closed hosted behavior,
  idempotency, concurrency safety, restart recovery, inspection APIs, audit events, quota
  catalog, enforcement matrix, and smoke/contract verification.
- **Production-grade, minimal, reversible change** — PASS. The change is additive and
  staged: introduce `daemon/internal/billing` and storage records first, expose
  inspection and admin surfaces, then wire guarded entry points through the shared service.
  Rollback preserves recorded usage/audit evidence and disables hosted enforcement by
  assigning tenants an explicit unlimited/development plan or restoring a pre-migration
  backup.
- **Contracts and auditability** — PASS. API, event, schema, persistence, SDK, and docs
  changes are named below. `contracts/quota-catalog.md`,
  `contracts/enforcement-matrix.md`, and `contracts/billing-usage-surfaces.md` are
  required before tasks so every quota category, guarded entry point, and response/event
  shape has an auditable contract.
- **Verification and observability** — PASS. Required tests include quota calculation,
  UTC period reset, carryover, reservation/commit/refund, idempotent retry/restart,
  concurrent launches, lowered-quota enforcement, storage estimate reconciliation,
  fail-closed hosted denial, unlimited local plan behavior, audit retention, contract
  shapes, and matrix completeness.
- **Environment and secrets** — PASS. Verification uses `~/.dope-test`, fake tenants, and
  fake integrations by default. No production tenant, live connector, payment-provider
  credential, invoice system, tax system, or revenue-recognition integration is required.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/023-billing-quotas-usage/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── billing-usage-surfaces.md
│   ├── enforcement-matrix.md
│   └── quota-catalog.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── billing/                         # NEW: plans, quotas, usage lifecycle,
│   │                                    # effective projection, denial decisions
│   ├── store/
│   │   ├── store.go                     # schema, migrations, billing tables,
│   │   │                                # transactional accounting helpers
│   │   ├── tenancy/                     # tenant-aware billing isolation tests
│   │   └── migrationfixture/            # hosted/default-plan migration fixtures
│   ├── api/
│   │   ├── server.go                    # billing routes and guarded entry wiring
│   │   ├── hosted_billing.go            # NEW: plan/usage/quota/admin handlers
│   │   ├── billing_enforcement.go       # NEW: route-level reservation helpers
│   │   ├── calendar.go / mail*.go       # integration operation quota hooks
│   │   └── evaluation*.go               # replay/evaluation attempt quota hooks
│   ├── runtime/                         # run, step, and tool-call operation keys
│   ├── orchestration/                   # workflow launch operation keys
│   ├── integrations/                    # integration operation quota categories
│   ├── artifacts/                       # storage/artifact byte estimate hooks
│   ├── evaluation/                      # replay/evaluation attempt hooks
│   ├── audit/ and events/               # billing audit and quota-denial events
│   └── contracts/                       # schema contract tests
├── go.mod
└── go.sum

schemas/
├── api/                                 # plan, quota, usage, denial, adjustment,
│                                        # reservation/commit/refund shapes
└── events/                              # billing audit and quota-denial events

docs/
├── runtime/                             # hosted quota behavior and rollback
├── harness/                             # live validation quota gate docs
└── providers/                           # integration operation quota notes

sdk/ts/                                  # plan/usage/quota resources and errors
web/ and tui/                            # update only if existing surfaces expose
                                         # plan/usage inspection during this roadmap
```

**Structure Decision**: Centralize quota semantics in `daemon/internal/billing`. Domain
packages supply tenant context, operation identity, category, and estimated amount; the
billing service owns effective quota projection, reservation, commit, refund, denial, and
audit behavior. This avoids per-domain quota drift and gives the enforcement matrix one
shared contract to verify.

## Roadmap 38 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/quota-catalog.md`](./contracts/quota-catalog.md) — first quota catalog with
  category, unit, period, carryover, reservation, commit, refund, operation identity,
  concurrency guard, denial shape, and tests.
- [`contracts/enforcement-matrix.md`](./contracts/enforcement-matrix.md) — guarded entry
  points and allowed/denied/retry/restart/concurrent-launch coverage.
- [`contracts/billing-usage-surfaces.md`](./contracts/billing-usage-surfaces.md) — API,
  SDK, event, audit, and error contracts for plan inspection, quota projection, usage
  lifecycle, denial, and manual adjustment.

These artifacts are planning gates. Any newly discovered expensive or side-effecting
hosted entry point must be added to the enforcement matrix or explicitly excluded before
implementation can be considered complete.

## Post-Design Constitution Check

- **Roadmap closure** — PASS. `research.md`, `data-model.md`, and the contracts cover all
  Roadmap 38 surfaces: tenant plan records, quota definitions, effective projection,
  UTC/carryover periods, usage counters, reservation lifecycle, denial behavior,
  manual adjustments, fail-closed hosted behavior, restart recovery, audit retention,
  quota catalog, enforcement matrix, and contract validation.
- **Production-grade, minimal, reversible change** — PASS. Design artifacts centralize
  accounting in one package, keep local/unlimited plan behavior explicit, make rollout
  plan-first and enforcement-gated, and preserve rollback through backup-restore or
  explicit unlimited/development plan assignment without deleting accounting evidence.
- **Contracts and auditability** — PASS. Contract artifacts define API shapes, stable
  denial errors, audit fields, usage lifecycle events, quota catalog completeness, and
  enforcement matrix completeness. Schema changes must update `schemas/api`,
  `schemas/events`, SDK resources, and contract tests together.
- **Verification and observability** — PASS. `quickstart.md` names targeted package tests,
  contract checks, full daemon tests, client tests, test daemon smoke, audit evidence
  checks, restart recovery checks, and concurrency checks.
- **Environment and secrets** — PASS. The design defaults to `~/.dope-test`, fake tenants,
  fake integration operations, and no payment-provider credentials or live connector
  access.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
