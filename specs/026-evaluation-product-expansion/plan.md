# Implementation Plan: Evaluation Product Expansion

**Branch**: `026-evaluation-product-expansion` | **Date**: 2026-04-29 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/026-evaluation-product-expansion/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close Roadmap 41 by turning the existing evaluation and replay harness into a tenant-aware
operator product workflow. The implementation adds bounded historical discovery, product
managed fixtures, replay campaigns, dashboard projections, and tool-call replay
inspection while preserving Roadmap 33 non-live replay behavior and reusing Roadmap 40
live-validation ledger evidence.

The change is additive. Repo-managed fixtures remain immutable from product editing
paths, existing replay candidates and attempts keep their current routes, and new product
state is introduced through explicit tenant-scoped resources, audit events, permissions,
retention policies, and contract-backed SDK/web projections.

## Technical Context

**Language/Version**: Go 1.24 daemon/control-plane code; JSON Schema contracts under
`schemas/`; TypeScript SDK resources in `sdk/ts`; React web operator shell in `web`;
operator documentation under `docs/harness` and `docs/runtime`.  
**Primary Dependencies**: `daemon/internal/evaluation`, `daemon/internal/livevalidation`,
`daemon/internal/store`, `daemon/internal/store/tenancy`, `daemon/internal/api`,
`daemon/internal/audit`, `daemon/internal/events`, `daemon/internal/identity`,
`daemon/internal/tenantctx`, `daemon/internal/contracts`, `schemas/api`,
`schemas/events`, `sdk/ts`, `web`, Roadmap 33 replay harness, Roadmap 35 tenant-scoped
data foundations, Roadmap 36 tenant-aware shell/SDK, and Roadmap 40 live-validation
ledger.  
**Storage**: Existing SQLite daemon metadata store remains authoritative. Add durable
tenant-scoped product tables for discovery policies, discovery runs, discovered
candidates, redacted candidate evidence, suppression records, product-managed fixtures,
fixture revisions, campaigns, campaign items, campaign attempt summaries, dashboard
snapshots or projections, tool-call inspection records, and retention metadata. Use
document JSON for complete resources plus indexed columns for tenant, status, source,
updated time, and pagination.  
**Testing**: Targeted Go tests for evaluation discovery, product fixtures, campaigns,
dashboard projections, tool-call inspection, retention, redaction, permission denial,
tenant isolation, store migrations, query plans, audit/events, API routes, and contract
schemas; `go test ./...` in `daemon/`; `make daemon-contract-test`; `pnpm test:clients`;
`pnpm build`; `make daemon-run-test`; `make daemon-test-status`; `go mod tidy` from
`daemon/` after implementation.  
**Target Platform**: Local-first daemon and hosted daemon behavior, verified by default in
the isolated test environment (`~/.kura-test`, `127.0.0.1:19192`). Live connector evidence
is consumed only through Roadmap 40 live-validation records and does not require
production connector access for normal acceptance.  
**Project Type**: Multi-surface daemon product change spanning evaluation workflow logic,
background/incremental discovery, persistence, API contracts, tenant permissions, audit
events, SDK/web client surfaces, and operator documentation.  
**Performance Goals**: Discovery page loads and dashboard refreshes never trigger
unbounded historical scans. Discovery jobs honor configured per-tenant bounds and emit a
partial-result status when bounds are reached. Operator workflows can review top
candidates, create or edit one fixture, start a campaign, and inspect campaign evidence
from product surfaces without raw database or log reconstruction.  
**Constraints**: Discovery is tenant-scoped, bounded, explainable, privacy-aware, and
operator-controllable. Product fixture edits require explicit permissions and produce
immutable revision/audit evidence. Campaigns group replay attempts and live-validation
outcomes without replacing underlying runtime truth. Product editing never mutates
repo-managed fixtures. Retention and deletion requests must preserve immutable audit
history while removing or suppressing selectable product-side content.  
**Scale/Scope**: One daemon may host multiple tenants with moderate historical run,
workflow, tool-call, replay, live-validation, candidate, fixture, and campaign volumes.
Correct tenant isolation, bounded discovery cost, deterministic pagination, idempotent
background work, auditability, and rollback safety take priority over high-throughput
evaluation execution.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** - PASS. The plan closes Roadmap 41 end-to-end: automatic
  tenant-scoped candidate discovery, explanations, scan bounds, sensitive-data
  filtering, manual suppression, product fixture editing with provenance, replay
  campaigns, dashboards, tool-call inspection, permissions, retention, contracts,
  SDK/web projections, and final release-readiness workload coverage.
- **Production-grade, minimal, reversible change** - PASS. The change is additive around
  existing `evaluation`, `livevalidation`, `store`, `api`, SDK, and web boundaries.
  Rollback disables new discovery runs, fixture edits, campaign starts, and dashboard
  publication while keeping existing replay and live-validation evidence readable.
- **Contracts and auditability** - PASS. Required API, schema, event, SDK, persistence,
  retention, and audit changes are named below. Discovery and campaign/dashboard
  contracts are planning gates before implementation.
- **Verification and observability** - PASS. Required verification covers discovery
  bounds, redaction, retention, deletion/suppression, fixture provenance, campaign
  aggregation, live-validation linkage, pagination, query plans, cross-tenant leakage,
  SDK/web projections, contract schemas, and Roadmap 39 soak rerun with Roadmaps 40 and
  41 included.
- **Environment and secrets** - PASS. Default execution uses `~/.kura-test`; live
  connector evidence is referenced only from explicit Roadmap 40 records. Secrets,
  credentials, raw tokens, and configured sensitive fields must be redacted before
  discovery evidence, fixtures, dashboards, audits, or inspection views are persisted or
  displayed.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/026-evaluation-product-expansion/
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- contracts/
|   |-- candidate-discovery.md
|   |-- campaign-dashboard.md
|   |-- fixture-editing.md
|   `-- tool-call-inspection.md
|-- checklists/
|   `-- requirements.md
`-- tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
daemon/
|-- internal/
|   |-- evaluation/                      # discovery, product fixtures,
|   |                                    # campaigns, dashboards, inspection
|   |-- livevalidation/                  # Roadmap 40 ledger and comparison linkage
|   |-- store/                           # SQLite schema, migrations, persistence,
|   |                                    # retention/deletion/suppression queries
|   |   |-- migrationfixture/            # seeded migration coverage
|   |   `-- tenancy/                     # tenant-safe product evaluation accessors
|   |-- api/                             # evaluation product routes and handlers
|   |-- audit/ and events/               # tenant-scoped audit/event evidence
|   |-- identity/                        # evaluation permissions and denial tests
|   `-- contracts/                       # schema/fixture contract tests
|-- go.mod
`-- go.sum

schemas/
|-- api/                                 # discovery, suppression, fixture,
|                                        # campaign, dashboard, inspection resources
`-- events/                              # discovery, fixture, campaign, result events

sdk/ts/                                  # typed evaluation product resources/methods
web/                                     # operator shell product workflows
tui/                                     # update only if product evaluation is exposed there

docs/
|-- harness/                             # evaluation product workflow docs
`-- runtime/                             # retention, rollback, release readiness
```

**Structure Decision**: Extend the existing `daemon/internal/evaluation` boundary instead
of creating a parallel product package. Roadmap 33 replay remains the replay substrate,
Roadmap 40 `livevalidation` remains the source of live side-effect evidence, `store`
owns durable tenant-scoped records and migrations, and `api`/SDK/web expose product
workflow projections. This keeps legacy replay behavior compatible while adding explicit
product-managed state.

## Roadmap 41 Planning Contracts

The implementation plan MUST keep these artifacts complete before `/speckit.tasks`:

- [`contracts/candidate-discovery.md`](./contracts/candidate-discovery.md) - source
  tables and APIs, tenant context, permissions, scan bounds, incremental job behavior,
  scoring/explanations, redaction, retention, deletion, suppression, repo/product fixture
  behavior, and audit events.
- [`contracts/fixture-editing.md`](./contracts/fixture-editing.md) - product-managed
  fixture resources, revision model, review states, provenance, permission gates,
  repo-managed fixture immutability, SDK/web behavior, and audit events.
- [`contracts/campaign-dashboard.md`](./contracts/campaign-dashboard.md) - campaign
  identity, ownership, lifecycle, selected immutable sources, grouped replay attempts,
  comparison summaries, live-validation links, aggregate dashboard fields, pagination,
  retention, SDK, and web projections.
- [`contracts/tool-call-inspection.md`](./contracts/tool-call-inspection.md) - original,
  non-live replay, live-validation, unsupported, missing-evidence, redaction, and
  comparison evidence rules.

These artifacts are planning gates. Implementation is incomplete if a product workflow,
resource state, audit action, retention/deletion behavior, dashboard aggregate, SDK
method, or web projection can exist without a contract row and proving test.

## Migration And Rollback Plan

1. Add schema tables and indexes for product evaluation resources with no reads or writes
   enabled by default.
2. Add tenant-safe store accessors and backfill-compatible migration fixtures; existing
   replay and live-validation tables remain untouched except for additive references.
3. Add API/schema/SDK contracts behind explicit evaluation product routes while keeping
   existing Roadmap 33 and Roadmap 40 routes backward compatible.
4. Enable bounded discovery workers with per-tenant cursors, idempotent job keys,
   redaction-before-persist, and operator suppression checks.
5. Enable product fixture edits and campaign starts only after permissions, audit events,
   retention rules, and contract tests pass.
6. Enable dashboard and inspection projections after campaign and live-validation linkage
   is verified.

Rollback disables discovery scheduling, fixture edits, campaign starts, and dashboard
publication. Historical product records remain readable to authorized users for audit and
diagnosis. Existing repo-managed fixtures, replay attempts, comparisons, and
live-validation ledger records remain available.

## Post-Design Constitution Check

- **Roadmap closure** - PASS. `research.md`, `data-model.md`, `quickstart.md`, and the
  four contracts cover all Roadmap 41 gates, including discovery bounds, privacy,
  fixture provenance, campaigns, dashboards, tool-call inspection, retention, audit,
  SDK/web, and release-readiness verification.
- **Production-grade, minimal, reversible change** - PASS. Design is additive and staged:
  schema first, read-only contracts, bounded background discovery, permission-gated
  mutation, campaign execution, and dashboard publication. Rollback preserves evidence
  and disables new product actions without changing existing replay defaults.
- **Contracts and auditability** - PASS. Contracts define request/response shapes,
  pagination, idempotency keys, stable states, audit event names, retention behavior,
  suppression semantics, and compatibility rules. Schema, SDK, docs, and contract tests
  must change together.
- **Verification and observability** - PASS. Quickstart names targeted package tests,
  full daemon tests, schema contract tests, client tests, daemon smoke, cross-tenant
  leakage checks, query-plan checks, and final soak rerun evidence.
- **Environment and secrets** - PASS. Design defaults to `~/.kura-test`, treats live
  evidence as Roadmap 40 records, and requires redaction before evidence is displayed,
  persisted, audited, or exported.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                     |
